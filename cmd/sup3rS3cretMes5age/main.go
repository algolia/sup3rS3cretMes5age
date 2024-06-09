package main

import (
	"crypto/tls"
	"net/http"

	"github.com/algolia/sup3rS3cretMes5age/internal"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

func main() {
	conf := internal.LoadConfig()

	// Vault address and token are taken from VAULT_ADDR and VAULT_TOKEN environment variables
	handlers := internal.NewSecretHandlers(internal.NewVault("", conf.VaultPrefix, ""))
	e := echo.New()

	if conf.HttpsRedirectEnabled {
		e.Pre(middleware.HTTPSRedirect())
	}

	if conf.TLSAutoDomain != "" {
		e.AutoTLSManager.HostPolicy = autocert.HostWhitelist(conf.TLSAutoDomain)
		e.AutoTLSManager.Cache = autocert.DirCache("/var/www/.cache")
	}

	e.Use(middleware.Logger())
	e.Use(middleware.BodyLimit("50M"))
	e.Use(middleware.Secure())

	e.GET("/", internal.RedirectHandler)
	e.File("/robots.txt", "static/robots.txt")

	e.Any("/health", internal.HealthHandler)
	e.GET("/secret", handlers.GetMsgHandler)
	e.POST("/secret", handlers.CreateMsgHandler)
	e.File("/msg", "static/index.html")
	e.File("/getmsg", "static/getmsg.html")
	e.Static("/static", "static")

	if conf.HttpBindingAddress != "" {
		if conf.HttpsBindingAddress != "" {
			go func(c *echo.Echo) {
				e.Logger.Fatal(e.Start(conf.HttpBindingAddress))
			}(e)
		} else {
			e.Logger.Fatal(e.Start(conf.HttpBindingAddress))
		}
	}

	autoTLSManager := autocert.Manager{
		Prompt: autocert.AcceptTOS,
		// Cache certificates to avoid issues with rate limits (https://letsencrypt.org/docs/rate-limits)
		Cache: autocert.DirCache("/var/www/.cache"),
		//HostPolicy: autocert.HostWhitelist("<DOMAIN>"),
	}
	s := http.Server{
		Addr:    ":443",
		Handler: e, // set Echo as handler
		TLSConfig: &tls.Config{
			//Certificates: nil, // <-- s.ListenAndServeTLS will populate this field
			GetCertificate:   autoTLSManager.GetCertificate,
			NextProtos:       []string{acme.ALPNProto},
			MinVersion:       tls.VersionTLS12,
			CurvePreferences: []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.X25519, tls.CurveP256},
			CipherSuites: []uint16{
				// TLS 1.2 safe cipher suites
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				// TLS 1.3 cipher suites
				tls.TLS_AES_128_GCM_SHA256,
				tls.TLS_AES_256_GCM_SHA384,
				tls.TLS_CHACHA20_POLY1305_SHA256,
			},
			PreferServerCipherSuites: true,
		},
		//ReadTimeout: 30 * time.Second, // use custom timeouts
	}
	if err := s.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
		e.Logger.Fatal(err)
	}
}
