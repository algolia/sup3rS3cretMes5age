package main

import "fmt"
import vault "github.com/hashicorp/vault/api"
import "log"

func CreateSecretMsg(o *OTSecretSvc, msg []byte) (token []byte, err error) {
	token, err = createOneTimeToken(o)
	log.Println("writing msg ", string(msg), "with token", string(token))
	err = writeMsgToVault(token, msg)
	if err != nil {
		return nil, err
	}
	return token, nil
}

func createOneTimeToken(o *OTSecretSvc) ([]byte, error) {

	v, err := vault.NewClient(vault.DefaultConfig())
	vauth := v.Auth().Token()
	if err != nil {
		return nil, fmt.Errorf("could not create token service %s", err)
	}
	var notRenewable bool
	sec, err := vauth.Create(&vault.TokenCreateRequest{
		Metadata:       map[string]string{"name": "placeholder"},
		ExplicitMaxTTL: "24h",
		NumUses:        2,
		Renewable:      &notRenewable,
	})
	if err != nil {
		fmt.Println(err)
	}
	log.Println("got one time token : ", string([]byte(sec.Auth.ClientToken)))
	return []byte(sec.Auth.ClientToken), err
}

func writeMsgToVault(token, msg []byte) error {
	// We have to use a new client with the new one time token
	config := vault.DefaultConfig()
	otc, err := vault.NewClient(config)
	otc.SetToken(string(token))
	if err != nil {
		return err
	}

	raw := map[string]interface{}{"msg": string(msg)}

	log.Println("writting message to vault at /cubbyhole/", string(token))
	otc.Logical().Write("/cubbyhole/"+string(token), raw)

	return nil
}

func GetSecretMsg(token []byte) (msg []byte, err error) {
	msg, err = getMessageFromVault(token)
	if err != nil {
		return nil, fmt.Errorf("could not get secret message from vault %s", err)
	}
	return msg, nil
}

func getMessageFromVault(token []byte) (msg []byte, err error) {
	log.Println("getting msg with token :  ", token)
	otc, err := vault.NewClient(vault.DefaultConfig())
	otc.SetToken(string(token))
	if err != nil {
		return nil, fmt.Errorf("could not set token to get message from vault %s", err)
	}
	res, err := otc.Logical().Read("cubbyhole/" + string(token))
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return []byte(res.Data["msg"].(string)), nil
}
