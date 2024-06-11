package internal

import (
	"net"
	"testing"

	"github.com/hashicorp/vault/api"
	vaulthttp "github.com/hashicorp/vault/http"
	hashivault "github.com/hashicorp/vault/vault"
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
	if err != nil {
		t.Fatal(err)
	}
	c.SetToken(rootToken)

	_, err = c.Sys().Health()
	if err != nil {
		t.Fatal(err)
	}

	return ln, c
}

func TestStoreAndGet(t *testing.T) {
	ln, c := createTestVault(t)
	defer ln.Close()

	v := newVault(c.Address(), "secret/test/", c.Token())
	secret := "my secret"
	token, err := v.Store(secret, "")
	if err != nil {
		t.Fatalf("no error expected, got %v", err)
	}

	msg, err := v.Get(token)
	if err != nil {
		t.Fatalf("no error expected, got %v", err)
	}

	if msg != secret {
		t.Fatalf("expected message %s, got: %s", secret, msg)
	}
}

func TestMsgCanOnlyBeAccessedOnce(t *testing.T) {
	ln, c := createTestVault(t)
	defer ln.Close()

	v := newVault(c.Address(), "secret/test/", c.Token())
	secret := "my secret"
	token, err := v.Store(secret, "")
	if err != nil {
		t.Fatalf("no error expected, got %v", err)
	}

	_, err = v.Get(token)
	if err != nil {
		t.Fatalf("no error expected, got %v", err)
	}

	_, err = v.Get(token)
	if err == nil {
		t.Fatal("error expected, got nil")
	}
}
