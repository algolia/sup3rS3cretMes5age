package main

import (
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"golang.org/x/crypto/acme/autocert"
)

func main() {

	conf := getConfig()

	handlers := NewSecretHandlers(newVault("", ""))
	e := echo.New()

	e.Pre(middleware.HTTPSRedirect())

	//AutoTLS
	e.AutoTLSManager.HostPolicy = autocert.HostWhitelist(conf.Domain)

	// Cache certificates
	e.AutoTLSManager.Cache = autocert.DirCache("/var/www/.cache")

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


	go func(c *echo.Echo){
		e.Logger.Fatal(e.Start(":80"))
    }(e)	
	if !conf.Local {
		e.Logger.Fatal(e.StartAutoTLS(":443"))
	} else {
		e.Logger.Fatal(e.StartTLS(":443", "cert.pem", "key.pem"))
	}
}
