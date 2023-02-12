package main

import (
	"net/http"
	"crypto/tls"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/crypto/acme"
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
