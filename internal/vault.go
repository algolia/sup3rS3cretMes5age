package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/vault/api"
)

// SecretMsgStorer defines the interface for storing and retrieving self-destructing messages.
// Implementations must ensure messages are deleted after first retrieval (one-time access).
type SecretMsgStorer interface {
	// Store saves a message with the specified TTL and returns a unique retrieval token.
	Store(string, ttl string) (token string, err error)
	// Get retrieves a message by token and deletes it from storage (one-time read).
	Get(token string) (msg string, err error)
}

// vault implements SecretMsgStorer using HashiCorp Vault's cubbyhole backend.
// It manages one-time tokens and automatic token renewal for secure message storage.
type vault struct {
	// address is the Vault server URL (read from VAULT_ADDR if empty).
	address string
	// prefix is the Vault storage path prefix (e.g., "cubbyhole/").
	prefix string
	// token is the Vault authentication token (read from VAULT_TOKEN if empty).
	token string
}

// NewVault creates a new vault client, validates connectivity and the token
// with a LookupSelf call, and starts a background goroutine for token renewal.
// If address or token are empty, they will be read from VAULT_ADDR and VAULT_TOKEN
// environment variables respectively. The prefix determines the Vault storage path.
// A failed boot validation returns an error instead of a degraded store: the
// service cannot store or retrieve secrets without a working Vault connection,
// so failing loudly is preferable to serving 500s until the first request
// hits the broken client.
// bootValidationTimeout bounds the boot-time Vault requests (LookupSelf and
// the capability checks): they must never hang startup indefinitely.
const bootValidationTimeout = 30 * time.Second

func NewVault(ctx context.Context, address string, prefix string, token string) (*vault, error) {
	v := &vault{address: address, prefix: prefix, token: token}

	c, err := v.newVaultClient()
	if err != nil {
		return nil, fmt.Errorf("vault client initialization failed: %w", err)
	}

	// Boot validation (lookup + capability checks) must not hang forever on
	// a Vault that accepts connections but stops responding — a hung boot
	// never starts the HTTP server, so no restart policy can help. Bound
	// only the validation; the long-lived ctx keeps governing the renewal
	// goroutine.
	bootCtx, cancel := context.WithTimeout(ctx, bootValidationTimeout)
	defer cancel()

	lookup, err := c.Auth().Token().LookupSelfWithContext(bootCtx)
	if err != nil {
		return nil, fmt.Errorf("vault connection or token validation failed (check VAULT_ADDR and VAULT_TOKEN): %w", err)
	}
	// ParseSecret returns (nil, nil) for an empty response body; proceeding
	// would panic in the renewal goroutine on lookup.Data.
	if lookup == nil || lookup.Data == nil {
		return nil, fmt.Errorf("vault returned an empty token lookup response")
	}

	renewable, _ := lookup.Data["renewable"].(bool)
	if !renewable {
		// A finite non-renewable token would silently expire under the
		// running server (non-renewable does not mean non-expiring), with
		// no lease monitoring to catch it. Reject it at boot; the Vault dev
		// root token (non-renewable, no TTL) is unaffected.
		if ttl := leaseDuration(lookup); ttl > 0 {
			return nil, fmt.Errorf("vault token is not renewable and expires in %ds; use a renewable token or a non-expiring one", ttl)
		}
	}
	if err := v.verifyCapabilities(bootCtx, c); err != nil {
		return nil, err
	}
	if renewable {
		// Renewal is essential for a renewable token and the lifetime
		// watcher special-cases renew-self permission denials into a silent
		// non-renewable countdown, so prove it functionally with a real
		// renewal. Run it AFTER the self-test (which consumes part of the
		// lease) so the refreshed TTL below is measured close to the
		// watcher's start; this works even when the token cannot query its
		// own capabilities.
		renewed, rerr := c.Auth().Token().RenewSelfWithContext(bootCtx, 0)
		if rerr != nil {
			return nil, fmt.Errorf("renewable vault token cannot renew itself: %w", rerr)
		}
		// ParseSecret returns (nil, nil) for an empty body; accepting that
		// here would pass the renewal check without proving a valid lease.
		if renewed == nil || renewed.Auth == nil {
			return nil, fmt.Errorf("vault returned an empty token renewal response")
		}
		lookup.Data["ttl"] = float64(renewed.Auth.LeaseDuration)
	}

	go v.renewToken(ctx, c, lookup)
	return v, nil
}

// verifyCapabilities checks that the configured token's ACLs cover the
// operations the service performs: creating one-time tokens (auth/token/create),
// writing and reading the storage prefix, and — for a renewable token —
// renewing itself (the LifetimeWatcher special-cases renew-self permission
// denials into a silent non-renewable countdown, so the gap must be caught
// here). LookupSelf only proves authentication; a token missing these
// capabilities would pass boot and then fail on every secret operation, so
// the gap fails loudly here. An auth rejection on the capabilities query
// itself means the token cannot self-inspect — the check degrades to a
// warning rather than blocking a possibly-valid deployment — but any other
// error (transport, 5xx, …) fails boot: starting with unverified
// capabilities would recreate the degraded state this check exists to
// prevent.
func (v vault) verifyCapabilities(ctx context.Context, c *api.Client) error {
	caps, err := c.Sys().CapabilitiesSelfWithContext(ctx, "auth/token/create")
	if err != nil {
		// The probe itself failed: when it was merely denied (the token
		// cannot self-inspect), continue to the remaining checks instead of
		// treating this probe as passed.
		if qerr := v.capabilitiesQueryError(err); qerr != errSkippedCapabilityCheck {
			return qerr
		}
	} else if !hasAnyCapability(caps, "root", "update") {
		// Vault evaluates token creation as an update operation: a policy
		// granting only "create" reports "create" here yet still gets denied
		// on the actual create call.
		return fmt.Errorf("vault token lacks the required capability on auth/token/create (need update; have %v)", caps)
	}

	// A capability probe on a sentinel path cannot prove what the real
	// operations need: policies can grant the probe while denying the
	// actual token paths. Run one full store/retrieve cycle through the
	// real code path instead — it exercises token creation, the write and
	// the read exactly as requests will. The throwaway message is consumed
	// by the read, so the self-test leaves nothing behind.
	return v.boundedSelfTest(ctx)
}

// selfTest performs one full store/retrieve cycle with a throwaway message
// to prove the token can really do everything the service needs on the
// paths actually used by Store and Get.
func (v vault) selfTest() error {
	const probeMsg = "boot self-test message"
	probe, err := v.Store(probeMsg, "1m")
	if err != nil {
		return fmt.Errorf("vault boot self-test failed on store: %w", err)
	}
	msg, err := v.Get(probe)
	if err != nil {
		return fmt.Errorf("vault boot self-test failed on retrieve: %w", err)
	}
	if msg != probeMsg {
		return fmt.Errorf("vault boot self-test retrieved an unexpected message")
	}
	return nil
}

// capabilitiesQueryError decides the outcome of a failed capabilities query:
// only an auth rejection is skippable (the token cannot self-inspect, but may
// still be fully functional — the caller then continues to the remaining
// checks instead of trusting a nil result); everything else means Vault
// itself is unreliable and the unverified-capability boot must fail.
func (v vault) capabilitiesQueryError(err error) error {
	if isTerminalTokenError(err) {
		log.Printf("warning: unable to verify Vault capabilities (sys/capabilities-self denied): %v", err)
		return errSkippedCapabilityCheck
	}
	return fmt.Errorf("unable to verify Vault capabilities: %w", err)
}

// errSkippedCapabilityCheck signals that a capability probe was skipped
// (denied sys/capabilities-self) — non-fatal, but the caller must not treat
// the probe as passed.
var errSkippedCapabilityCheck = errors.New("capability check skipped")

// boundedSelfTest runs the boot self-test raced against the boot context:
// the underlying Vault calls in Store/Get are contextless, so without this
// race a Vault that accepts connections but hangs on requests could still
// block boot indefinitely. When ctx wins, NewVault fails boot and the
// process exits — so the losing goroutine cannot outlive it.
func (v vault) boundedSelfTest(ctx context.Context) error {
	done := make(chan error, 1)
	go func() { done <- v.selfTest() }()
	select {
	case err := <-done:
		if ctx.Err() != nil {
			// The boot bound expired; a self-test finishing late must not
			// let startup proceed past it.
			return fmt.Errorf("vault boot self-test timed out after %s", bootValidationTimeout)
		}
		return err
	case <-ctx.Done():
		return fmt.Errorf("vault boot self-test timed out after %s", bootValidationTimeout)
	}
}

// hasAnyCapability reports whether the capability list contains any of the
// given capabilities (Vault reports "root" for root tokens).
func hasAnyCapability(caps []string, wanted ...string) bool {
	for _, cap := range caps {
		for _, w := range wanted {
			if cap == w {
				return true
			}
		}
	}
	return false
}

// Store saves a message to Vault with the specified time-to-live (TTL).
// Default TTL is 48 hours if not specified. Maximum TTL is 168 hours (7 days).
// Returns a unique one-time token for retrieving the message.
// The token can be used exactly twice: once to store and once to retrieve.
func (v vault) Store(msg string, ttl string) (token string, err error) {
	// Default TTL
	if ttl == "" {
		ttl = "48h"
	}

	t, err := v.createOneTimeToken(ttl)
	if err != nil {
		return "", err
	}

	// The write failure must be returned, not the (nil) token-creation
	// error: swallowing it made Store report success with an empty token
	// while nothing was stored, and the client received a link that can
	// never be read.
	if werr := v.writeMsgToVault(t, msg); werr != nil {
		return "", redactTokenFromError(werr, t)
	}
	return t, nil
}

// redactTokenFromError removes a one-time Vault token from a non-nil error message.
// Vault transport errors embed the request URL, whose path contains the
// token, and handlers log store errors verbatim — without redaction the
// token would reach the logs, defeating the redaction applied to the
// access log.
func redactTokenFromError(err error, token string) error {
	return errors.New(strings.ReplaceAll(err.Error(), token, "REDACTED"))
}

// createOneTimeToken creates a non-renewable Vault token with exactly 2 uses.
// The token is used once to write the message and once to read it, ensuring
// one-time access. The token automatically expires after the specified TTL.
func (v vault) createOneTimeToken(ttl string) (string, error) {
	c, err := v.newVaultClient()
	if err != nil {
		return "", err
	}
	t := c.Auth().Token()

	var notRenewable bool
	s, err := t.Create(&api.TokenCreateRequest{
		Metadata:       map[string]string{"name": "placeholder"},
		ExplicitMaxTTL: ttl,
		NumUses:        2, //1 to create 2 to get
		Renewable:      &notRenewable,
	})
	if err != nil {
		return "", err
	}

	return s.Auth.ClientToken, nil
}

// newVaultClient creates a new Vault API client with the configured address and token.
// If the vault address is empty, it defaults to using the VAULT_ADDR environment variable.
// If the vault token is empty, it defaults to using the VAULT_TOKEN environment variable.
func (v vault) newVaultClient() (*api.Client, error) {
	c, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return nil, err
	}

	if v.token != "" {
		c.SetToken(v.token)
	}

	if v.address == "" {
		return c, nil
	}

	err = c.SetAddress(v.address)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// writeMsgToVault writes a message to Vault using the provided one-time token.
// The message is stored at the path: /<prefix>/<token>.
// This consumes the first use of the two-use token.
func (v vault) writeMsgToVault(token, msg string) error {
	c, err := v.newVaultClientWithToken(token)
	if err != nil {
		return err
	}

	raw := map[string]interface{}{"msg": msg}

	_, err = c.Logical().Write("/"+v.prefix+token, raw)

	return err
}

// Get retrieves and deletes a message from Vault using the provided token.
// This consumes the second (final) use of the two-use token, automatically
// deleting both the message and the token from Vault, ensuring one-time access.
func (v vault) Get(token string) (msg string, err error) {
	c, err := v.newVaultClientWithToken(token)
	if err != nil {
		return "", err
	}

	r, err := c.Logical().Read(v.prefix + token)
	if err != nil {
		return "", redactTokenFromError(err, token)
	}
	return r.Data["msg"].(string), nil
}

// newVaultClientWithToken creates a Vault client authenticated with a specific token.
// Used for one-time token operations when storing and retrieving messages.
func (v vault) newVaultClientWithToken(token string) (*api.Client, error) {
	c, err := v.newVaultClient()
	if err != nil {
		return nil, err
	}
	c.SetToken(token)
	return c, nil
}

// renewToken runs in a background goroutine to automatically renew the main
// Vault authentication token before it expires. This ensures continuous
// operation of the service without manual token refresh.
//
// The lookup result obtained during NewVault's boot validation is passed in:
// re-querying it here would open a window where a transient Vault/network
// hiccup permanently disables renewal for an otherwise renewable token. One
// lifetime watcher is monitored until its lease ends (DoneCh fires) — the
// watcher itself keeps renewing and reports each success on RenewCh, so the
// select must stay on the same watcher across renewals: recreating it after
// every RenewCh event would stack a new concurrent renewal loop on top of
// the still-running previous one. After DoneCh the token is re-validated: a
// transient failure just retries, but an auth rejection means the token can
// never renew again, so the process exits (fail-loud, matching NewVault) and
// the supervisor's restart policy brings it back with a fresh token. The
// goroutine exits cleanly on ctx cancellation (server shutdown) or when the
// token is not renewable (e.g. the Vault dev root token), which needs no
// renewal.
func (v vault) renewToken(ctx context.Context, c *api.Client, lookup *api.Secret) {
	if renewable, ok := lookup.Data["renewable"].(bool); !ok || !renewable {
		log.Println("vault token is not renewable; token renewal disabled")
		return
	}

	const retryDelay = 30 * time.Second
	for {
		// Seed the watcher with the token's current lease duration:
		// LifetimeWatcher schedules its first renewal from
		// SecretAuth.LeaseDuration, and leaving it zero would make the
		// watcher fall back to its own default timing instead of the
		// token's actual TTL. RenewBehaviorErrorOnErrors surfaces renewal
		// failures on DoneCh instead of silently converting them into a
		// non-renewable countdown.
		watcher, err := c.NewLifetimeWatcher(&api.LifetimeWatcherInput{
			Secret: &api.Secret{Auth: &api.SecretAuth{
				ClientToken:   c.Token(),
				Renewable:     true,
				LeaseDuration: leaseDuration(lookup),
			}},
			RenewBehavior: api.RenewBehaviorErrorOnErrors,
		})
		if err != nil {
			log.Printf("unable to initialize auth token lifetime watcher: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(retryDelay):
				continue
			}
		}

		// Start() blocks for the life of the watcher (its body is
		// `doneCh <- doRenew()`; it does not take a context), so it must
		// run in its own goroutine — calling it synchronously would prevent
		// the select below from ever observing DoneCh, RenewCh or ctx
		// cancellation. Shutdown relies on watcher.Stop(): Stop closes the
		// watcher's internal stopCh, the loop exits, and Start returns
		// (doneCh is buffered, so the final send cannot block).
		go watcher.Start()

		watcherDone := false
		for !watcherDone {
			select {
			case <-ctx.Done():
				watcher.Stop()
				return

			// The lease ended (expired, revoked, or renewal terminally
			// failed): stop the watcher, back off, then re-validate the
			// token before building a new one.
			case err := <-watcher.DoneCh():
				watcher.Stop()
				watcherDone = true
				// The watcher runs with RenewBehaviorErrorOnErrors, so a
				// failed renewal reaches DoneCh as an error instead of being
				// converted into a non-renewable countdown. A renew-self
				// rejection means this token can never renew again — exit
				// loudly so the supervisor restarts with a fresh token.
				if isTerminalTokenError(err) {
					log.Fatalf("vault auth token renewal terminally failed: %v; exiting so the supervisor can restart with a fresh token", err)
				}
				// Revalidate immediately, then back off only between failed
				// lookups: sleeping before the first attempt could let a
				// short-lived token expire during the wait, turning a
				// recoverable outage into a fatal auth rejection. An auth
				// rejection means the token can never renew again — exit
				// loudly so the supervisor restarts the process.
				log.Printf("vault auth token lease ended (%v); revalidating", err)
				for {
					// Bound each lookup attempt: an unresponsive Vault must
					// not block the renewal loop indefinitely.
					lookupCtx, cancelLookup := context.WithTimeout(ctx, bootValidationTimeout)
					fresh, lerr := c.Auth().Token().LookupSelfWithContext(lookupCtx)
					cancelLookup()
					if lerr == nil {
						if fresh == nil || fresh.Data == nil {
							// Malformed Vault response: same fail-loud
							// treatment as an empty boot lookup.
							log.Fatalf("vault returned an empty token lookup during renewal; exiting so the supervisor can restart")
						}
						// Seed the next watcher with the fresh state.
						lookup = fresh
						break
					}
					if isTerminalTokenError(lerr) {
						log.Fatalf("vault auth token is no longer valid: %v; exiting so the supervisor can restart with a fresh token", lerr)
					}
					// Transient (network, 5xx…): keep retrying the lookup
					// with backoff rather than seeding the next watcher from
					// stale lease data.
					log.Printf("vault token revalidation failed (%v); retrying in %s", lerr, retryDelay)
					select {
					case <-ctx.Done():
						return
					case <-time.After(retryDelay):
					}
				}
				if renewable, ok := lookup.Data["renewable"].(bool); !ok || !renewable {
					// A token that starts non-renewable needs no renewal
					// (handled at boot); one that stops being renewable
					// after a lease end is degrading — the HTTP server
					// would keep serving on a token that is about to die,
					// so exit fail-loud like the other terminal paths.
					log.Fatalf("vault auth token is no longer renewable; exiting so the supervisor can restart with a fresh token")
				}
				// Re-prove renew-self: the watcher special-cases renew-self
				// permission denials into a silent non-renewable countdown,
				// so a revocation mid-flight would otherwise cycle through
				// watchers without ever failing loudly. A successful renewal
				// also refreshes the lease, which seeds the next watcher.
				renewCtx, cancel := context.WithTimeout(ctx, retryDelay)
				renewed, rerr := c.Auth().Token().RenewSelfWithContext(renewCtx, 0)
				cancel()
				if rerr != nil {
					if isTerminalTokenError(rerr) {
						log.Fatalf("vault auth token can no longer renew itself: %v; exiting so the supervisor can restart with a fresh token", rerr)
					}
					// Non-terminal (network, 5xx…): the recreated watcher's
					// renewal loop retries with its own backoff.
					log.Printf("vault auth token renewal re-proof failed (%v); the recreated watcher will retry", rerr)
				} else if renewed == nil || renewed.Auth == nil {
					// Malformed success (empty body): same fail-loud
					// treatment as an empty lookup response.
					log.Fatalf("vault returned an empty token renewal response during renewal; exiting so the supervisor can restart")
				} else {
					lookup.Data["ttl"] = float64(renewed.Auth.LeaseDuration)
				}

			// RenewCh is a channel that receives a message when a successful
			// renewal takes place and includes metadata about the renewal.
			// Stay on the same watcher: it keeps running and renewing.
			case info := <-watcher.RenewCh():
				log.Printf("auth token: successfully renewed; remaining duration: %ds", info.Secret.Auth.LeaseDuration)
			}
		}
	}
}

// leaseDuration extracts the token's lease duration in seconds from a
// LookupSelf result, tolerating both float64 and json.Number representations.
func leaseDuration(lookup *api.Secret) int {
	if lookup == nil {
		return 0
	}
	switch ttl := lookup.Data["ttl"].(type) {
	case float64:
		return int(ttl)
	case json.Number:
		n, err := ttl.Int64()
		if err == nil {
			return int(n)
		}
	}
	return 0
}

// isTerminalTokenError reports whether a LookupSelf error means the token
// itself is dead (revoked/expired/unknown) rather than Vault being
// unreachable — only the former makes renewal retrying pointless.
func isTerminalTokenError(err error) bool {
	var respErr *api.ResponseError
	if errors.As(err, &respErr) {
		switch respErr.StatusCode {
		// 400 covers terminal renewal conditions — a token that reached its
		// max TTL or is no longer renewable answers renew-self with 400.
		case http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound:
			return true
		}
	}
	return false
}
