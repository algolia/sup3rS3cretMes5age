package main

import (
	"github.com/labstack/echo"
)

func main() {
	ot := NewOTSecretService()

	e := echo.New()

	e.Any("/health", ot.HealthHandler)
	e.GET("/secret", ot.GetMsgHandler)
	e.POST("/secret", ot.CreateMsgHandler)
	e.File("/msg", "static/index.html")
	e.File("/getmsg", "static/getmsg.html")
	e.Static("/static", "static")

	e.Logger.Fatal(e.Start(":1234"))
}
