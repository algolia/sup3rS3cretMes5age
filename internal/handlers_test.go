package internal

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

type FakeSecretMsgStorer struct {
	msg           string
	token         string
	err           error
	lastUsedToken string
	lastMsg       string
}

func (f *FakeSecretMsgStorer) Get(token string) (msg string, err error) {
	f.lastUsedToken = token
	return f.msg, f.err
}

func (f *FakeSecretMsgStorer) Store(msg string, ttl string) (token string, err error) {
	f.lastMsg = msg
	return f.token, f.err
}

func TestGetMsgHandlerSuccess(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/?token=secrettoken", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	s := &FakeSecretMsgStorer{msg: "secret"}
	h := newSecretHandlers(s)
	err := h.GetMsgHandler(c)

	assert.NoError(t, err)
	assert.Equal(t, "secrettoken", s.lastUsedToken)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "{\"msg\":\"secret\"}\n", rec.Body.String())
}

func TestGetMsgHandlerError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/?token=secrettoken", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	s := &FakeSecretMsgStorer{msg: "secret", err: errors.New("expired")}
	h := newSecretHandlers(s)
	err := h.GetMsgHandler(c)

	assert.Error(t, err)
	if assert.IsType(t, &echo.HTTPError{}, err) {
		v, _ := err.(*echo.HTTPError)
		assert.Equal(t, http.StatusInternalServerError, v.Code)
	}
}

func TestHealthHandler(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := healthHandler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRedirectHandler(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := redirectHandler(c)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusPermanentRedirect, rec.Code)
	assert.Equal(t, "/msg", rec.Result().Header.Get("Location"))
}
