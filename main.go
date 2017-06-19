package main

import (
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

func main() {
	ot := NewOTSecretService()

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.BodyLimit("50M"))

	e.GET("/", redirect)
	e.File("/robots.txt", "static/robots.txt")

	e.Any("/health", ot.HealthHandler)
	e.GET("/secret", ot.GetMsgHandler)
	e.POST("/secret", ot.CreateMsgHandler)
	e.File("/msg", "static/index.html")
	e.File("/getmsg", "static/getmsg.html")
	e.Static("/static", "static")

	e.Logger.Fatal(e.Start(":1234"))
}
