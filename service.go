package main

import (
	"fmt"

	vault "github.com/hashicorp/vault/api"
)

type OTSecretSvc struct {
	vc *vault.Client
}

func NewOTSecretService() *OTSecretSvc {
	o := &OTSecretSvc{}
	var err error
	//os.Setenv("VAULT_ADDR", "")
	//os.Setenv("VAULT_TOKEN", "")
	o.vc, err = vault.NewClient(vault.DefaultConfig())

	if err != nil {
		fmt.Println(err)
	}
	return o
}
