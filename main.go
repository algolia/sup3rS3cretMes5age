package main

import (
	"net/http"
	"crypto/tls"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/crypto/acme"
)

func main() {
	conf := loadConfig()

	handlers := NewSecretHandlers(newVault("", conf.VaultPrefix, "")) // Vault address and token are taken from VAULT_ADDR and VAULT_TOKEN environment variables
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

	e.GET("/", redirect)
	e.File("/robots.txt", "static/robots.txt")

	e.Any("/health", HealthHandler)
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
			GetCertificate: autoTLSManager.GetCertificate,
			NextProtos:     []string{acme.ALPNProto},
		},
		//ReadTimeout: 30 * time.Second, // use custom timeouts
	}
	if err := s.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
		e.Logger.Fatal(err)
	}}
