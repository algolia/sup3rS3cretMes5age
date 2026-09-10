package internal

import (
	"context"
	"fmt"
	"log"
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
func NewVault(ctx context.Context, address string, prefix string, token string) (*vault, error) {
	v := &vault{address: address, prefix: prefix, token: token}

	c, err := v.newVaultClient()
	if err != nil {
		return nil, fmt.Errorf("vault client initialization failed: %w", err)
	}

	if _, err := c.Auth().Token().LookupSelfWithContext(ctx); err != nil {
		return nil, fmt.Errorf("vault connection or token validation failed (check VAULT_ADDR and VAULT_TOKEN): %w", err)
	}

	go v.renewToken(ctx, c)
	return v, nil
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

	if v.writeMsgToVault(t, msg) != nil {
		return "", err
	}
	return t, nil
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
		return "", err
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
// The lifetime watcher is recreated whenever the current lease ends (DoneCh
// fires) — the previous implementation never exited its monitoring loop, so
// after the first lease end it spun on a closed channel and never renewed
// again. A retry backoff keeps the loop from spinning hot when Vault is
// unreachable or the token cannot be renewed. The goroutine exits when ctx is
// cancelled (server shutdown) or the token is not renewable (e.g. the Vault
// dev root token), which needs no renewal.
func (v vault) renewToken(ctx context.Context, c *api.Client) {
	lookup, err := c.Auth().Token().LookupSelfWithContext(ctx)
	if err != nil {
		// NewVault already confirmed connectivity at boot; reaching here
		// means Vault became unreachable. Renewal cannot proceed.
		log.Printf("vault token renewal disabled: token lookup failed: %v", err)
		return
	}
	if renewable, ok := lookup.Data["renewable"].(bool); !ok || !renewable {
		log.Println("vault token is not renewable; token renewal disabled")
		return
	}

	const retryDelay = 30 * time.Second
	for {
		watcher, err := c.NewLifetimeWatcher(&api.LifetimeWatcherInput{
			Secret: &api.Secret{Auth: &api.SecretAuth{ClientToken: c.Token(), Renewable: true}},
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

		watcher.Start()

		select {
		case <-ctx.Done():
			watcher.Stop()
			return

		// The lease ended (expired or revoked): recreate the watcher after a
		// backoff — Vault may be restarting, or the token may have been
		// re-issued out of band.
		case err := <-watcher.DoneCh():
			watcher.Stop()
			log.Printf("vault auth token lease ended (%v); retrying renewal in %s", err, retryDelay)
			select {
			case <-ctx.Done():
				return
			case <-time.After(retryDelay):
			}

		// RenewCh is a channel that receives a message when a successful
		// renewal takes place and includes metadata about the renewal.
		case info := <-watcher.RenewCh():
			log.Printf("auth token: successfully renewed; remaining duration: %ds", info.Secret.Auth.LeaseDuration)
		}
	}
}
