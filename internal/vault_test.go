package internal

import (
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
	defer ln.Close()

	v := newVault(c.Address(), "secret/test/", c.Token())
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
	defer ln.Close()

	v := newVault(c.Address(), "secret/test/", c.Token())
	secret := "my secret"
	token, err := v.Store(secret, "")
	if assert.NoError(t, err) {
		_, err = v.Get(token)
		assert.NoError(t, err)

		_, err = v.Get(token)
		assert.Error(t, err)
	}
}
