package main

import (
	"encoding/base64"
	"log"
	"net/http"

	"io/ioutil"

	"github.com/labstack/echo"
)

type TokenResponse struct {
	Token     string `json:"token"`
	FileToken string `json:"filetoken,omitempty"`
	FileName  string `json:"filename,omitempty"`
}

type MsgReposne struct {
	Msg string `json:"msg"`
}

func (o *OTSecretSvc) CreateMsgHandler(ctx echo.Context) error {
	var tr TokenResponse

	// Upload file if any
	file, err := ctx.FormFile("file")
	if err == nil {
		src, err := file.Open()
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "file fileopen failed"+err.Error())
		}
		defer src.Close()

		b, err := ioutil.ReadAll(src)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "file readall failed"+err.Error())
		}

		if len(b) > 0 {
			tr.FileName = file.Filename
			encodedFile := base64.StdEncoding.EncodeToString(b)
			filetoken, err := CreateSecretMsg(o, []byte(encodedFile))
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "cannot create msg "+err.Error())
			}
			tr.FileToken = string(filetoken)
		}
	}

	// Handle the secret message
	msg := ctx.FormValue("msg")
	log.Println("msg received", msg)
	token, err := CreateSecretMsg(o, []byte(msg))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "cannot create msg "+err.Error())
	}
	tr.Token = string(token)

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

func redirect(ctx echo.Context) error {
	return ctx.Redirect(http.StatusPermanentRedirect, "/msg")
}
