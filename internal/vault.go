package internal

import (
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/vault/api"
)

type SecretMsgStorer interface {
	Store(string, ttl string) (token string, err error)
	Get(token string) (msg string, err error)
}

type vault struct {
	address string
	prefix  string
	token   string
}

// NewVault creates a vault client to talk with underline vault server
func newVault(address string, prefix string, token string) vault {
	v := vault{address, prefix, token}

	go v.newVaultClientToRenewToken()
	return v
}

func (v vault) Store(msg string, ttl string) (token string, err error) {
	// Default TTL
	if ttl == "" {
		ttl = "48h"
	} else {
		// Verify duration
		d, err := time.ParseDuration(ttl)
		if err != nil {
			return "", fmt.Errorf("cannot parse duration %v", err)
		}

		// validate duration length
		if d > 168*time.Hour || d == 0*time.Hour {
			return "", fmt.Errorf("cannot set ttl to infinte or more than 7 days %v", err)
		}
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

func (v vault) writeMsgToVault(token, msg string) error {
	c, err := v.newVaultClientWithToken(token)
	if err != nil {
		return err
	}

	raw := map[string]interface{}{"msg": msg}

	_, err = c.Logical().Write("/"+v.prefix+token, raw)

	return err
}

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

func (v vault) newVaultClientWithToken(token string) (*api.Client, error) {
	c, err := v.newVaultClient()
	if err != nil {
		return nil, err
	}
	c.SetToken(token)
	return c, nil
}

func (v vault) newVaultClientToRenewToken() {
	c, err := v.newVaultClient()
	if err != nil {
		log.Println(err)
	}
	client_auth_token := &api.Secret{Auth: &api.SecretAuth{ClientToken: c.Token(), Renewable: true}}

	/* */
	log.Println("renew cycle: begin")
	defer log.Println("renew cycle: end")

	// auth token
	authTokenWatcher, err := c.NewLifetimeWatcher(&api.LifetimeWatcherInput{
		Secret: client_auth_token,
	})

	if err != nil {
		err := fmt.Errorf("unable to initialize auth token lifetime watcher: %w", err)
		fmt.Println(err.Error())
	}

	go authTokenWatcher.Start()
	defer authTokenWatcher.Stop()

	// monitor events from both watchers
	for {
		select {

		case err := <-authTokenWatcher.DoneCh():
			// Leases created by a token get revoked when the token is revoked.
			fmt.Println("Error is :", err)

		// RenewCh is a channel that receives a message when a successful
		// renewal takes place and includes metadata about the renewal.
		case info := <-authTokenWatcher.RenewCh():
			log.Printf("auth token: successfully renewed; remaining duration: %ds", info.Secret.Auth.LeaseDuration)
		}
	}
}
