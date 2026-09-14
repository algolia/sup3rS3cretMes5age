package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
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

// TestIsTerminalTokenError pins the classification used for LookupSelf
// revalidation and capability probes: auth rejections (403/404) mean the
// token is dead and must exit the process, while every other status —
// including a bare 400, which Vault answers for conditions unrelated to
// token death — must keep retrying. Renewal errors have their own classifier.
func TestIsTerminalTokenError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		terminal bool
	}{
		{"400 not terminal for lookups", &api.ResponseError{StatusCode: 400}, false},
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

// TestIsTerminalRenewalError pins the renewal-specific classification: 403
// and 404 are always terminal; a 400 is terminal only when Vault's response
// body says the lease can no longer be renewed (max TTL / non-renewable) —
// any other 400 body must be retried, not treated as a dead token.
func TestIsTerminalRenewalError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		terminal bool
	}{
		{"400 lease not renewable (max TTL)", &api.ResponseError{StatusCode: 400, Errors: []string{"lease is not renewable"}}, true},
		{"400 token not renewable", &api.ResponseError{StatusCode: 400, Errors: []string{"token is not renewable"}}, true},
		{"400 unrelated body", &api.ResponseError{StatusCode: 400, Errors: []string{"invalid request"}}, false},
		{"400 empty body", &api.ResponseError{StatusCode: 400}, false},
		{"403 auth rejection", &api.ResponseError{StatusCode: 403}, true},
		{"404 unknown token", &api.ResponseError{StatusCode: 404}, true},
		{"503 vault restarting", &api.ResponseError{StatusCode: 503}, false},
		{"transport error", errors.New("connection refused"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.terminal, isTerminalRenewalError(tt.err))
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
// request URL, whose path contains the one-time token — in raw and
// percent-encoded form. The wrapper must also preserve the error identity
// (errors.As reaches the underlying *api.ResponseError) so the status code
// keeps feeding the terminal-error classification.
func TestRedactTokenFromError(t *testing.T) {
	err := errors.New(`Get "http://vault:8200/v1/cubbyhole/hvs.SECRET123": dial tcp: connection refused`)

	redacted := redactTokenFromError(err, "hvs.SECRET123")

	assert.Error(t, redacted)
	assert.NotContains(t, redacted.Error(), "hvs.SECRET123")
	assert.Contains(t, redacted.Error(), "REDACTED")
	assert.Contains(t, redacted.Error(), "connection refused")

	// Percent-encoded variant (transport errors may quote the escaped URL).
	escaped := redactTokenFromError(
		errors.New(`Get "http://vault:8200/v1/cubbyhole/hvs.SECRET%2B123": dial tcp: connection refused`),
		"hvs.SECRET+123")
	assert.NotContains(t, escaped.Error(), "hvs.SECRET+123")
	assert.NotContains(t, escaped.Error(), "hvs.SECRET%2B123")

	// Error identity survives the wrapping.
	var respErr *api.ResponseError
	wrapped := redactTokenFromError(
		fmt.Errorf("store failed: %w", &api.ResponseError{StatusCode: 503, Errors: []string{"vault sealed"}}),
		"hvs.X")
	assert.ErrorAs(t, wrapped, &respErr)
	assert.Equal(t, 503, respErr.StatusCode)
}

// TestRevalidateToken pins the revalidation cycle the renewal loop relies
// on after a lease end or a malformed renewal confirmation (both paths now
// route through it instead of exiting on a malformed confirmation): against
// a live Vault it must produce a fresh lookup carrying a proven renewal TTL,
// and a cancelled context must stop it without exiting.
func TestRevalidateToken(t *testing.T) {
	ln, c := createTestVault(t)
	defer func() { _ = ln.Close() }()

	secret, err := c.Auth().Token().Create(&api.TokenCreateRequest{
		Renewable: boolPtr(true),
		TTL:       "1h",
	})
	if !assert.NoError(t, err) {
		return
	}
	v := vault{address: c.Address(), prefix: "secret/test/", token: secret.Auth.ClientToken}
	renewableClient := c
	renewableClient.SetToken(secret.Auth.ClientToken)

	fresh, ok := v.revalidateToken(t.Context(), renewableClient, 100*time.Millisecond)
	assert.True(t, ok, "revalidation against a live Vault must succeed")
	if assert.NotNil(t, fresh) {
		assert.Greater(t, leaseDuration(fresh), 0, "the fresh lookup must carry the proven renewal's TTL")
	}

	// A cancelled context stops the cycle gracefully (no Fatalf).
	stoppedCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, ok = v.revalidateToken(stoppedCtx, renewableClient, 100*time.Millisecond)
	assert.False(t, ok, "a cancelled context must stop revalidation without exiting")
}

// TestVaultHTTPTimeoutBoundsHangingRequests pins the client-level HTTP
// timeout: a Vault that accepts connections but never answers must not
// block the contextless Store call until the process exits — the request
// fails once the timeout elapses, so the boot self-test goroutine (and any
// request goroutine) terminates on its own.
func TestVaultHTTPTimeoutBoundsHangingRequests(t *testing.T) {
	block := make(chan struct{})
	// hanging.Close waits for outstanding handlers to return, so it must be
	// registered BEFORE close(block): defers run LIFO, the channel closes
	// first, then the server can shut down.
	hanging := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block // never answer until the test ends
	}))
	defer hanging.Close()
	defer close(block)

	old := vaultHTTPTimeout
	vaultHTTPTimeout = 200 * time.Millisecond
	defer func() { vaultHTTPTimeout = old }()

	v := vault{address: hanging.URL, prefix: "secret/test/", token: "hvs.ABCDEFGHIJKLMNOPQRSTUVWX"}
	start := time.Now()
	_, err := v.Store("msg", "")
	elapsed := time.Since(start)

	assert.Error(t, err, "a request to a hanging Vault must fail, not block forever")
	assert.Less(t, elapsed, 10*time.Second, "the client timeout must bound the request")
}

// TestTokenTTLSeconds pins the boot ttl validation: a wrong-typed, missing
// or negative ttl must be rejected, not silently turned into "no expiry" —
// leaseDuration alone would return zero for all of them.
func TestTokenTTLSeconds(t *testing.T) {
	tests := []struct {
		name    string
		data    map[string]any
		want    int
		wantErr bool
	}{
		{"float64 ttl", map[string]any{"ttl": float64(60)}, 60, false},
		{"json.Number ttl", map[string]any{"ttl": json.Number("60")}, 60, false},
		{"zero ttl is valid (no expiry)", map[string]any{"ttl": float64(0)}, 0, false},
		{"negative float64 ttl", map[string]any{"ttl": float64(-5)}, 0, true},
		{"negative json.Number ttl", map[string]any{"ttl": json.Number("-5")}, 0, true},
		{"string ttl", map[string]any{"ttl": "60s"}, 0, true},
		{"non-numeric json.Number ttl", map[string]any{"ttl": json.Number("abc")}, 0, true},
		{"missing ttl", map[string]any{}, 0, true},
		{"nil lookup", nil, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var lookup *api.Secret
			if tt.data != nil {
				lookup = &api.Secret{Data: tt.data}
			}
			got, err := tokenTTLSeconds(lookup)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestNewVaultRejectsFiniteUseToken pins the num_uses boot validation: a
// token with a finite use count is consumed by the boot probes themselves,
// then lets the server run until requests start failing — it must be
// rejected at boot instead. The service token must have unlimited uses.
func TestNewVaultRejectsFiniteUseToken(t *testing.T) {
	ln, c := createTestVault(t)
	defer func() { _ = ln.Close() }()

	uses := 5
	secret, err := c.Auth().Token().Create(&api.TokenCreateRequest{
		NumUses:   uses,
		Renewable: boolPtr(true),
	})
	if !assert.NoError(t, err) {
		return
	}

	_, err = NewVault(context.Background(), c.Address(), "secret/test/", secret.Auth.ClientToken)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "finite use count",
		"a finite-use token must be rejected at boot, before the probes consume its uses")
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
