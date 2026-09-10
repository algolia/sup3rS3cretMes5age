package internal

import (
	"context"
	"errors"
	"net"
	"testing"

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
