// Package internal contains the core business logic for the sup3rS3cretMes5age application,
// including configuration management, HTTP handlers, server setup, and Vault integration.
package internal

import (
	"log"
	"os"
	"strings"
)

// conf holds the application configuration settings loaded from environment variables.
// It includes HTTP/HTTPS binding addresses, TLS configuration, and Vault storage prefix.
type conf struct {
	// HttpBindingAddress is the HTTP server binding address (e.g., ":8080").
	HttpBindingAddress string
	// HttpsBindingAddress is the HTTPS server binding address (e.g., ":443").
	HttpsBindingAddress string
	// HttpsRedirectEnabled determines whether HTTP requests should redirect to HTTPS.
	HttpsRedirectEnabled bool
	// TLSAutoDomain is the domain for automatic Let's Encrypt TLS certificate generation.
	TLSAutoDomain string
	// TLSCertFilepath is the path to a manual TLS certificate file.
	TLSCertFilepath string
	// TLSCertKeyFilepath is the path to a manual TLS certificate key file.
	TLSCertKeyFilepath string
	// VaultPrefix is the Vault storage path prefix (defaults to "cubbyhole/").
	VaultPrefix string
}

// Environment variable names for application configuration.
const (
	// HttpBindingAddressVarenv is the environment variable for HTTP binding address.
	HttpBindingAddressVarenv = "SUPERSECRETMESSAGE_HTTP_BINDING_ADDRESS"
	// HttpsBindingAddressVarenv is the environment variable for HTTPS binding address.
	HttpsBindingAddressVarenv = "SUPERSECRETMESSAGE_HTTPS_BINDING_ADDRESS"
	// HttpsRedirectEnabledVarenv is the environment variable to enable HTTPS redirect.
	HttpsRedirectEnabledVarenv = "SUPERSECRETMESSAGE_HTTPS_REDIRECT_ENABLED"
	// TLSAutoDomainVarenv is the environment variable for automatic TLS domain.
	TLSAutoDomainVarenv = "SUPERSECRETMESSAGE_TLS_AUTO_DOMAIN"
	// TLSCertFilepathVarenv is the environment variable for manual TLS certificate path.
	TLSCertFilepathVarenv = "SUPERSECRETMESSAGE_TLS_CERT_FILEPATH"
	// TLSCertKeyFilepathVarenv is the environment variable for manual TLS key path.
	TLSCertKeyFilepathVarenv = "SUPERSECRETMESSAGE_TLS_CERT_KEY_FILEPATH"
	// VaultPrefixenv is the environment variable for Vault storage prefix.
	VaultPrefixenv = "SUPERSECRETMESSAGE_VAULT_PREFIX"
)

// LoadConfig loads and validates application configuration from environment variables.
// It validates TLS configuration mutual exclusivity, ensures required bindings are set,
// and sets default values where appropriate. Exits with fatal error on invalid configuration.
func LoadConfig() conf {
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
