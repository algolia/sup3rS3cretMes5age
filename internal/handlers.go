package internal

import (
	"encoding/base64"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

type TokenResponse struct {
	Token     string `json:"token"`
	FileToken string `json:"filetoken,omitempty"`
	FileName  string `json:"filename,omitempty"`
}

type MsgResponse struct {
	Msg string `json:"msg"`
}

type SecretHandlers struct {
	store SecretMsgStorer
}

func newSecretHandlers(s SecretMsgStorer) *SecretHandlers {
	return &SecretHandlers{s}
}

func (s SecretHandlers) CreateMsgHandler(ctx echo.Context) error {
	var tr TokenResponse

	//Get TTL (if any)
	ttl := ctx.FormValue("ttl")

	// Upload file if any
	file, err := ctx.FormFile("file")
	if err == nil {
		src, err := file.Open()
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err)
		}
		defer src.Close()

		b, err := io.ReadAll(src)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err)
		}

		if len(b) > 0 {
			tr.FileName = file.Filename
			encodedFile := base64.StdEncoding.EncodeToString(b)

			filetoken, err := s.store.Store(encodedFile, ttl)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, err)
			}
			tr.FileToken = filetoken
		}
	}

	// Handle the secret message
	msg := ctx.FormValue("msg")
	tr.Token, err = s.store.Store(msg, ttl)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return ctx.JSON(http.StatusOK, tr)
}

func (s SecretHandlers) GetMsgHandler(ctx echo.Context) error {
	m, err := s.store.Get(ctx.QueryParam("token"))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}
	r := &MsgResponse{
		Msg: m,
	}
	return ctx.JSON(http.StatusOK, r)
}

func healthHandler(ctx echo.Context) error {
	return ctx.String(http.StatusOK, http.StatusText(http.StatusOK))
}

func redirectHandler(ctx echo.Context) error {
	return ctx.Redirect(http.StatusPermanentRedirect, "/msg")
}
