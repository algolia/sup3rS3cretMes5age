package main

import (
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

func main() {
	handlers := NewSecretHandlers(NewVault("", ""))
	e := echo.New()
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

	e.Logger.Fatal(e.Start(":1234"))
}
