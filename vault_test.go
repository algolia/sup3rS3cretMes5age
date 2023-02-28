package main

import (
	"log"
	"os"
	"testing"

	"github.com/hashicorp/vault/api"
	dockertest "github.com/ory/dockertest/v3"
)

var c *api.Client

func TestMain(m *testing.M) {
	// uses a sensible default on windows (tcp/http) and linux/osx (socket)
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	// pulls an image, creates a container based on it and runs it
	resource, err := pool.Run("vault", "latest", []string{"VAULT_ADDR", "VAULT_DEV_ROOT_TOKEN_ID=rootsecret"})
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
	}

	// exponential backoff-retry, because the application in the container might not be ready to accept connections yet
	if err := pool.Retry(func() error {
		var err error
		c, err = api.NewClient(api.DefaultConfig())
		if err != nil {
			return err
		}
		c.SetToken("rootsecret")
		err = c.SetAddress("http://127.0.0.1:" + resource.GetPort("8200/tcp"))
		if err != nil {
			return err
		}

		_, err = c.Sys().Health()
		return err
	}); err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	code := m.Run()

	// You can't defer this because os.Exit doesn't care for defer
	if err := pool.Purge(resource); err != nil {
		log.Fatalf("could not purge resource: %s", err)
	}

	os.Exit(code)
}

func TestStore(t *testing.T) {
	v := newVault(c.Address(), c.Token())

	var storeParams = []struct {
		secret string
		ttl    string
	}{
		// don't allow infinte ttl
		{"my secret", "0h"},
		// don't allow more than a week ttl
		{"my secret", "169h"},
	}

	for _, tt := range storeParams {
		_, err := v.Store(tt.secret, tt.ttl)

		if err == nil {
			t.Fatalf("expected error, got: nil")
		}
	}

}

func TestStoreAndGet(t *testing.T) {
	v := newVault(c.Address(), c.Token())
	secret := "my secret"
	token, err := v.Store(secret, "24h")
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
	v := newVault(c.Address(), c.Token())
	secret := "my secret"
	token, err := v.Store(secret, "24h")
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
