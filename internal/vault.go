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
	if err := v.verifyCapabilities(bootCtx, c, renewable); err != nil {
		return nil, err
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
func (v vault) verifyCapabilities(ctx context.Context, c *api.Client, renewable bool) error {
	caps, err := c.Sys().CapabilitiesSelfWithContext(ctx, "auth/token/create")
	if err != nil {
		return v.capabilitiesQueryError(err)
	}
	// Vault evaluates token creation as an update operation: a policy
	// granting only "create" reports "create" here yet still gets denied
	// on the actual create call.
	if !hasAnyCapability(caps, "root", "update") {
		return fmt.Errorf("vault token lacks the required capability on auth/token/create (need update; have %v)", caps)
	}

	if renewable {
		caps, err = c.Sys().CapabilitiesSelfWithContext(ctx, "auth/token/renew-self")
		if err != nil {
			return v.capabilitiesQueryError(err)
		}
		if !hasAnyCapability(caps, "root", "update") {
			return fmt.Errorf("renewable vault token lacks the required capability on auth/token/renew-self (need update; have %v)", caps)
		}
	}

	// Probe a concrete child path, not the bare prefix: a policy granting
	// only "cubbyhole/" (exact match, no glob) would pass a prefix probe
	// while every real read/write ("cubbyhole/<token>") is denied.
	childPath := v.prefix + "capabilitycheck"
	caps, err = c.Sys().CapabilitiesSelfWithContext(ctx, childPath)
	if err != nil {
		return v.capabilitiesQueryError(err)
	}
	// Logical().Write sends a PUT, which Vault authorizes with the update
	// capability alone: a create-only policy passes the check yet every
	// Store is denied.
	if !hasAnyCapability(caps, "root", "update") {
		return fmt.Errorf("vault token lacks the required write capability on %s (need update; have %v)", childPath, caps)
	}
	if !hasAnyCapability(caps, "root", "read") {
		return fmt.Errorf("vault token lacks the required read capability on %s (have %v)", childPath, caps)
	}
	return nil
}

// capabilitiesQueryError decides the outcome of a failed capabilities query:
// only an auth rejection is skippable (the token cannot self-inspect, but may
// still be fully functional); everything else means Vault itself is
// unreliable and the unverified-capability boot must fail.
func (v vault) capabilitiesQueryError(err error) error {
	if isTerminalTokenError(err) {
		log.Printf("warning: unable to verify Vault capabilities (sys/capabilities-self denied): %v", err)
		return nil
	}
	return fmt.Errorf("unable to verify Vault capabilities: %w", err)
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
				log.Printf("vault auth token lease ended (%v); revalidating in %s", err, retryDelay)
				select {
				case <-ctx.Done():
					return
				case <-time.After(retryDelay):
				}
				// Revalidate until we have fresh lease data: recreating the
				// watcher from the stale boot lookup could schedule its next
				// renewal past the token's actual expiry. An auth rejection
				// means the token can never renew again — exit loudly so the
				// supervisor restarts the process.
				for {
					fresh, lerr := c.Auth().Token().LookupSelfWithContext(ctx)
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
					select {
					case <-ctx.Done():
						return
					case <-time.After(retryDelay):
					}
				}
				if renewable, ok := lookup.Data["renewable"].(bool); !ok || !renewable {
					log.Println("vault token is no longer renewable; token renewal disabled")
					return
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
		case http.StatusForbidden, http.StatusNotFound:
			return true
		}
	}
	return false
}
