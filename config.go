package main

import (
	"log"
	"os"
	"strings"
)

type conf struct {
	HttpBindingAddress string
	HttpsBindingAddress string
	HttpsRedirectEnabled bool
	TLSAutoDomain string
	TLSCertFilepath string
	TLSCertKeyFilepath string
	VaultPrefix string
}

const HttpBindingAddressVarenv = "SUPERSECRETMESSAGE_HTTP_BINDING_ADDRESS"
const HttpsBindingAddressVarenv = "SUPERSECRETMESSAGE_HTTPS_BINDING_ADDRESS"
const HttpsRedirectEnabledVarenv = "SUPERSECRETMESSAGE_HTTPS_REDIRECT_ENABLED"
const TLSAutoDomainVarenv = "SUPERSECRETMESSAGE_TLS_AUTO_DOMAIN"
const TLSCertFilepathVarenv = "SUPERSECRETMESSAGE_TLS_CERT_FILEPATH"
const TLSCertKeyFilepathVarenv = "SUPERSECRETMESSAGE_TLS_CERT_KEY_FILEPATH"
const VaultPrefixenv = "SUPERSECRETMESSAGE_VAULT_PREFIX"

func loadConfig() conf {
	var cnf conf

	cnf.HttpBindingAddress = os.Getenv(HttpBindingAddressVarenv)
	cnf.HttpsBindingAddress = os.Getenv(HttpsBindingAddressVarenv)
	cnf.HttpsRedirectEnabled = strings.ToLower(os.Getenv(HttpsRedirectEnabledVarenv)) == "true"
	cnf.TLSAutoDomain = os.Getenv(TLSAutoDomainVarenv)
	cnf.TLSCertFilepath = os.Getenv(TLSCertFilepathVarenv)
	cnf.TLSCertKeyFilepath = os.Getenv(TLSCertKeyFilepathVarenv)
	cnf.VaultPrefix = os.Getenv(VaultPrefixenv)

	if cnf.TLSAutoDomain != "" && (cnf.TLSCertFilepath != "" || cnf.TLSCertKeyFilepath != "") {
		log.Fatalf("Auto TLS (%s) is mutually exclusive with manual TLS (%s and %s)", TLSAutoDomainVarenv,
			TLSCertFilepathVarenv, TLSCertKeyFilepathVarenv)
	}

	if (cnf.TLSCertFilepath != "" && cnf.TLSCertKeyFilepath == "") ||
		(cnf.TLSCertFilepath == "" && cnf.TLSCertKeyFilepath != "") {
		log.Fatalf("Both certificate filepath (%s) and certificate key filepath (%s) must be set when using manual TLS",
			TLSCertFilepathVarenv, TLSCertKeyFilepathVarenv)
	}

	if cnf.HttpsBindingAddress == "" && (cnf.TLSAutoDomain != "" || cnf.TLSCertFilepath != "") {
		log.Fatalf("HTTPS binding address (%s) must be set when using either auto TLS (%s) or manual TLS (%s and %s)",
			HttpsBindingAddressVarenv, TLSAutoDomainVarenv, TLSCertFilepathVarenv, TLSCertKeyFilepathVarenv)
	}

	if cnf.HttpBindingAddress == "" && cnf.TLSAutoDomain == "" && cnf.TLSCertFilepath == "" {
		log.Fatalf("HTTP binding address (%s) must be set if auto TLS (%s) and manual TLS (%s and %s) are both disabled",
			HttpBindingAddressVarenv, TLSAutoDomainVarenv, TLSCertFilepathVarenv, TLSCertKeyFilepathVarenv)
	}

	if cnf.HttpsBindingAddress != "" && cnf.TLSAutoDomain == "" && cnf.TLSCertFilepath == "" {
		log.Fatalf("HTTPS binding address (%s) is set but neither auto TLS (%s) nor manual TLS (%s and %s) are enabled",
			HttpsBindingAddressVarenv, TLSAutoDomainVarenv, TLSCertFilepathVarenv, TLSCertKeyFilepathVarenv)
	}

	if cnf.VaultPrefix == "" {
		cnf.VaultPrefix = "cubbyhole/"
	}

	log.Println("[INFO] HTTP Binding Address:", cnf.HttpBindingAddress)
	log.Println("[INFO] HTTPS Binding Address:", cnf.HttpsBindingAddress)
	log.Println("[INFO] HTTPS Redirect enabled:", cnf.HttpsRedirectEnabled)
	log.Println("[INFO] TLS Auto Domain:", cnf.TLSAutoDomain)
	log.Println("[INFO] TLS Cert Filepath:", cnf.TLSCertFilepath)
	log.Println("[INFO] TLS Cert Key Filepath:", cnf.TLSCertKeyFilepath)
	log.Println("[INFO] Vault prefix:", cnf.VaultPrefix)

	return cnf
}
