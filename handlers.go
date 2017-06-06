package main

import (
	"log"
	"net/http"

	"github.com/labstack/echo"
)

type TokenResponse struct {
	Token string `json:"token"`
}

type MsgReposne struct {
	Msg string `json:"msg"`
}

func (o *OTSecretSvc) CreateMsgHandler(ctx echo.Context) error {
	msg := ctx.FormValue("msg")
	log.Println("msg recieved ", msg)
	token, err := CreateSecretMsg(o, []byte(msg))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "cannot create msg "+err.Error())
	}
	tr := &TokenResponse{
		Token: string(token),
	}
	return ctx.JSON(http.StatusOK, tr)
}

func (o *OTSecretSvc) GetMsgHandler(ctx echo.Context) error {
	msg, err := GetSecretMsg([]byte(ctx.QueryParam("token")))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "cannot get msg "+err.Error())
	}
	m := &MsgReposne{
		Msg: string(msg),
	}
	return ctx.JSON(http.StatusOK, m)
}

func (o *OTSecretSvc) HealthHandler(ctx echo.Context) error {
	return ctx.String(http.StatusOK, "OK")
}
