package main

import (
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"golang.org/x/crypto/acme/autocert"
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

	if conf.TLSAutoDomain != "" {
		e.Logger.Fatal(e.StartAutoTLS(conf.HttpsBindingAddress))
	} else if conf.TLSCertFilepath != "" {
		e.Logger.Fatal(e.StartTLS(conf.HttpsBindingAddress, conf.TLSCertFilepath, conf.TLSCertKeyFilepath))
	}
}
