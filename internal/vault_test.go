package internal

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/vault/api"
	vaulthttp "github.com/hashicorp/vault/http"
	hashivault "github.com/hashicorp/vault/vault"
	"github.com/stretchr/testify/assert"
)

func createTestVault(t *testing.T) (net.Listener, *api.Client) {
	t.Helper()

	// Create an in-memory, unsealed core (the "backend", if you will).
	core, _, rootToken := hashivault.TestCoreUnsealed(t)

	// Start an HTTP server for the core.
	ln, addr := vaulthttp.TestServer(t, core)

	// Create a client that talks to the server, initially authenticating with
	// the root token.
	conf := api.DefaultConfig()
	conf.Address = addr

	c, err := api.NewClient(conf)

	if assert.NoError(t, err) {
		c.SetToken(rootToken)
		_, err = c.Sys().Health()
		assert.NoError(t, err)
	}

	return ln, c
}

func TestStoreAndGet(t *testing.T) {
	ln, c := createTestVault(t)
	defer func() { _ = ln.Close() }()

	v, err := NewVault(context.Background(), c.Address(), "secret/test/", c.Token())
	assert.NoError(t, err)
	secret := "my secret"
	token, err := v.Store(secret, "")
	if assert.NoError(t, err) {
		msg, err := v.Get(token)
		assert.NoError(t, err)
		assert.Equal(t, secret, msg)
	}
}

func TestMsgCanOnlyBeAccessedOnce(t *testing.T) {
	ln, c := createTestVault(t)
	defer func() { _ = ln.Close() }()

	v, err := NewVault(context.Background(), c.Address(), "secret/test/", c.Token())
	assert.NoError(t, err)
	secret := "my secret"
	token, err := v.Store(secret, "")
	if assert.NoError(t, err) {
		_, err = v.Get(token)
		assert.NoError(t, err)

		_, err = v.Get(token)
		assert.Error(t, err)
	}
}

// TestNewVaultFailsFastOnUnreachableVault pins the fail-loud boot validation:
// an unreachable Vault or an invalid token must surface as an error from
// NewVault, not as a degraded store that 500s on every later request.
func TestNewVaultFailsFastOnUnreachableVault(t *testing.T) {
	_, err := NewVault(context.Background(), "http://invalid:9999", "secret/", "fake-token")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault connection or token validation failed")
}

// TestStoreWithInvalidAddress exercises the Store error path on a vault
// constructed directly (bypassing NewVault's boot validation).
func TestStoreWithInvalidAddress(t *testing.T) {
	v := vault{address: "http://invalid:9999", prefix: "secret/", token: "fake-token"}

	_, err := v.Store("msg", "1h")

	assert.Error(t, err)
}

// TestIsTerminalTokenError pins the classification used after a lifetime
// watcher's lease ends: auth rejections (403/404) mean the token can never
// renew again and must exit the process, while transport-level errors must
// keep the renewal loop retrying.
func TestIsTerminalTokenError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		terminal bool
	}{
		{"400 terminal renewal condition (max TTL)", &api.ResponseError{StatusCode: 400}, true},
		{"403 auth rejection", &api.ResponseError{StatusCode: 403}, true},
		{"404 unknown token", &api.ResponseError{StatusCode: 404}, true},
		{"503 vault restarting", &api.ResponseError{StatusCode: 503}, false},
		{"500 internal", &api.ResponseError{StatusCode: 500}, false},
		{"transport error", errors.New("connection refused"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.terminal, isTerminalTokenError(tt.err))
		})
	}
}

// TestStoreReturnsWriteError pins that a failed write to Vault surfaces as
// an error from Store: the previous implementation returned the (nil)
// token-creation error on the write-failure branch, so the handler reported
// success with an empty token while nothing was stored.
func TestStoreReturnsWriteError(t *testing.T) {
	ln, c := createTestVault(t)
	defer func() { _ = ln.Close() }()

	// A policy that can create tokens but has no access to the storage
	// prefix: the one-time token created under it inherits these policies
	// and cannot write the message, so the write fails.
	policy := `path "auth/token/create" { capabilities = ["update"] }`
	assert.NoError(t, c.Sys().PutPolicy("creator", policy))
	secret, err := c.Auth().Token().Create(&api.TokenCreateRequest{
		Policies: []string{"creator"},
	})
	if !assert.NoError(t, err) {
		return
	}

	v := vault{address: c.Address(), prefix: "secret/test/", token: secret.Auth.ClientToken}

	token, err := v.Store("my secret", "")

	assert.Error(t, err, "a swallowed write error would report success with an empty token")
	assert.Empty(t, token)
}

// TestRedactTokenFromError pins the error sanitization applied before store errors
// reach the handlers (which log them): Vault transport errors embed the
// request URL, whose path contains the one-time token.
func TestRedactTokenFromError(t *testing.T) {
	err := errors.New(`Get "http://vault:8200/v1/cubbyhole/hvs.SECRET123": dial tcp: connection refused`)

	redacted := redactTokenFromError(err, "hvs.SECRET123")

	assert.Error(t, redacted)
	assert.NotContains(t, redacted.Error(), "hvs.SECRET123")
	assert.Contains(t, redacted.Error(), "REDACTED")
	assert.Contains(t, redacted.Error(), "connection refused")
}

// TestNewVaultFailsWhenCapabilitiesMissing pins the boot capability check:
// LookupSelf only proves authentication; a token that can create one-time
// tokens but cannot write the storage prefix must fail at boot (fail-loud)
// instead of starting a service that 500s on every secret operation.
func TestNewVaultFailsWhenCapabilitiesMissing(t *testing.T) {
	ln, c := createTestVault(t)
	defer func() { _ = ln.Close() }()

	policy := `path "auth/token/create" { capabilities = ["update"] }`
	assert.NoError(t, c.Sys().PutPolicy("creator", policy))
	// Renewable: a finite non-renewable token is rejected earlier at boot
	// (no lease monitoring possible), which would mask the self-test gap.
	secret, err := c.Auth().Token().Create(&api.TokenCreateRequest{
		Policies:  []string{"creator"},
		Renewable: boolPtr(true),
	})
	if !assert.NoError(t, err) {
		return
	}

	_, err = NewVault(context.Background(), c.Address(), "secret/test/", secret.Auth.ClientToken)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "boot self-test failed")
}

// TestNewVaultAcceptsTokenWithSufficientCapabilities is the positive case:
// a token whose ACLs cover token creation and the storage prefix boots.
func TestNewVaultAcceptsTokenWithSufficientCapabilities(t *testing.T) {
	ln, c := createTestVault(t)
	defer func() { _ = ln.Close() }()

	policy := `
path "auth/token/create" { capabilities = ["update"] }
path "secret/test/*" { capabilities = ["create", "read", "update"] }`
	assert.NoError(t, c.Sys().PutPolicy("worker", policy))
	secret, err := c.Auth().Token().Create(&api.TokenCreateRequest{
		Policies:  []string{"worker"},
		Renewable: boolPtr(true),
	})
	if !assert.NoError(t, err) {
		return
	}

	// t.Context() is cancelled when the test finishes: this is the only
	// test whose NewVault succeeds, so it starts the renewal goroutine —
	// without a cancellable context that goroutine would keep retrying
	// against the closed Vault for the rest of the test binary's lifetime.
	v, err := NewVault(t.Context(), c.Address(), "secret/test/", secret.Auth.ClientToken)
	assert.NoError(t, err)

	token, err := v.Store("round trip", "")
	if assert.NoError(t, err) {
		msg, err := v.Get(token)
		if assert.NoError(t, err) {
			assert.Equal(t, "round trip", msg)
		}
	}
}

// TestRenewTokenStopsOnContextCancel pins the shutdown path of the renewal
// lifecycle: with a renewable token whose watcher is running, cancelling the
// context must stop the watcher and make renewToken return (no deadlock, no
// leaked watcher). A regression to the old defects — synchronously blocking
// Start() or a dead-watcher select — would hang here until the test timeout.
func TestRenewTokenStopsOnContextCancel(t *testing.T) {
	ln, c := createTestVault(t)
	defer func() { _ = ln.Close() }()

	// Renewable token with a short lease so the watcher is live and
	// scheduling renewals while we wait for cancellation.
	secret, err := c.Auth().Token().Create(&api.TokenCreateRequest{
		Lease:     "60s",
		Renewable: boolPtr(true),
		Period:    "60s",
	})
	if !assert.NoError(t, err) {
		return
	}

	renewableClient, err := c.Clone()
	if !assert.NoError(t, err) {
		return
	}
	renewableClient.SetToken(secret.Auth.ClientToken)
	lookup, err := renewableClient.Auth().Token().LookupSelfWithContext(context.Background())
	if !assert.NoError(t, err) {
		return
	}
	if assert.True(t, lookup.Data["renewable"].(bool)) {
		assert.NotZero(t, leaseDuration(lookup), "LookupSelf must report a TTL to seed the watcher")
	}

	v := vault{address: c.Address(), prefix: "secret/test/", token: secret.Auth.ClientToken}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		v.renewToken(ctx, renewableClient, lookup)
		close(done)
	}()

	select {
	case <-done:
		// renewToken returned promptly on cancellation.
	case <-time.After(5 * time.Second):
		assert.Fail(t, "renewToken did not return on context cancellation")
	}
}

func boolPtr(b bool) *bool { return &b }

// TestNewVaultFailsWhenRenewSelfDenied pins the functional renewal check:
// a renewable token that cannot renew itself (renew-self denied, e.g. no
// default policy) must fail at boot — the lifetime watcher would otherwise
// silently convert the denial into a non-renewable countdown and the
// service would die at lease end instead of failing loudly.
func TestNewVaultFailsWhenRenewSelfDenied(t *testing.T) {
	ln, c := createTestVault(t)
	defer func() { _ = ln.Close() }()

	policy := `
path "auth/token/lookup-self" { capabilities = ["read"] }
path "auth/token/create" { capabilities = ["update"] }
path "secret/test/*" { capabilities = ["create", "read", "update"] }`
	assert.NoError(t, c.Sys().PutPolicy("worker-norenew", policy))
	secret, err := c.Auth().Token().Create(&api.TokenCreateRequest{
		Policies:        []string{"worker-norenew"},
		TTL:             "60s",
		Renewable:       boolPtr(true),
		NoDefaultPolicy: true,
	})
	if !assert.NoError(t, err) {
		return
	}

	_, err = NewVault(context.Background(), c.Address(), "secret/test/", secret.Auth.ClientToken)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot renew itself")
}

// TestNewVaultRejectsFiniteNonRenewableToken pins the boot rejection of a
// finite non-renewable token: it would silently expire under the running
// server with no lease monitoring to catch it.
func TestNewVaultRejectsFiniteNonRenewableToken(t *testing.T) {
	ln, c := createTestVault(t)
	defer func() { _ = ln.Close() }()

	secret, err := c.Auth().Token().Create(&api.TokenCreateRequest{
		TTL:             "60s",
		Lease:           "60s",
		Renewable:       boolPtr(false),
		NoDefaultPolicy: true,
	})
	if !assert.NoError(t, err) {
		return
	}

	_, err = NewVault(context.Background(), c.Address(), "secret/test/", secret.Auth.ClientToken)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not renewable and expires")
}

// TestTokenFromCreateResponse pins the malformed-response guard on the
// token-create path: ParseSecret can return (nil, nil) for an empty body,
// and dereferencing s.Auth.ClientToken would panic instead of returning the
// intended boot-validation error.
func TestTokenFromCreateResponse(t *testing.T) {
	tests := []struct {
		name    string
		secret  *api.Secret
		wantTok string
		wantErr bool
	}{
		{"nil response", nil, "", true},
		{"empty auth", &api.Secret{}, "", true},
		{"empty token value", &api.Secret{Auth: &api.SecretAuth{}}, "", true},
		{"valid", &api.Secret{Auth: &api.SecretAuth{ClientToken: "hvs.abc"}}, "hvs.abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok, err := tokenFromCreateResponse(tt.secret)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, tok)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantTok, tok)
		})
	}
}
