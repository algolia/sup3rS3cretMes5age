package main

import (
	"log"
	"os"
)

type conf struct {
	Domain string
	Local  bool
}

func getConfig() conf {
	var local bool
	domainName := os.Getenv("DOMAIN")
	if domainName == "" || domainName == "localhost" {
		domainName = "localhost"
		local = true
	}

	log.Println("[INFO] using domain:", domainName)

	return conf{
		Domain: domainName,
		Local:  local,
	}
}
