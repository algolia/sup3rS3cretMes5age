package main

import (
	vault "github.com/hashicorp/vault/api"
)

func newVaultClient() (*vault.Client, error) {
	return vault.NewClient(vault.DefaultConfig())

}

func newVaultClientWithToken(token string) (*vault.Client, error) {
	c, err := newVaultClient()
	if err != nil {
		return nil, err
	}
	c.SetToken(token)
	return c, nil
}

func CreateSecretMsg(msg string) (token string, err error) {
	t, err := createOneTimeToken()
	if err != nil {
		return "", err
	}

	if writeMsgToVault(t, msg) != nil {
		return "", err
	}
	return t, nil
}

func createOneTimeToken() (string, error) {
	c, err := newVaultClient()
	if err != nil {
		return "", err
	}
	t := c.Auth().Token()

	var notRenewable bool
	s, err := t.Create(&vault.TokenCreateRequest{
		Metadata:       map[string]string{"name": "placeholder"},
		ExplicitMaxTTL: "24h",
		NumUses:        2,
		Renewable:      &notRenewable,
	})
	if err != nil {
		return "", err
	}

	return s.Auth.ClientToken, nil
}

func writeMsgToVault(token, msg string) error {
	c, err := newVaultClientWithToken(token)
	if err != nil {
		return err
	}

	raw := map[string]interface{}{"msg": msg}

	_, err = c.Logical().Write("/cubbyhole/"+token, raw)

	return err
}

func GetSecretMsg(token string) (msg string, err error) {
	c, err := newVaultClientWithToken(token)
	if err != nil {
		return "", err
	}

	r, err := c.Logical().Read("cubbyhole/" + token)
	if err != nil {
		return "", err
	}
	return r.Data["msg"].(string), nil
}
