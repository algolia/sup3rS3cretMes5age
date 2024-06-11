package internal

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/labstack/echo/v4"
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
	if err != nil {
		t.Fatalf("got error %v, none expected", err)
	}

	if s.lastUsedToken != "secrettoken" {
		t.Fatalf("Storer::Get was called with %s, expected %s", s.lastUsedToken, "secrettoken")
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("got statusCode %d, expected %d", rec.Code, http.StatusOK)
	}

	expected := "{\"msg\":\"secret\"}\n"
	actual := rec.Body.String()
	if expected != actual {
		t.Fatalf("got body %s, expected %s", expected, actual)
	}
}

func TestGetMsgHandlerError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/?token=secrettoken", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	s := &FakeSecretMsgStorer{msg: "secret", err: errors.New("expired")}
	h := newSecretHandlers(s)
	err := h.GetMsgHandler(c)
	if err == nil {
		t.Fatalf("got no error, expected one")
	}

	v, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected an HTTPError, got %s", reflect.TypeOf(v))
	}

	if v.Code != http.StatusInternalServerError {
		t.Fatalf("got statusCode %d, expected %d", v.Code, http.StatusInternalServerError)
	}
}

func TestHealthHandler(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := healthHandler(c)
	if err != nil {
		t.Fatalf("error returned %v, expected nil", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("got statusCode %d, expected %d", rec.Code, http.StatusOK)
	}
}

func TestRedirectHandler(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := redirectHandler(c)
	if err != nil {
		t.Fatalf("error returned %v, expected nil", err)
	}

	if rec.Code != http.StatusPermanentRedirect {
		t.Fatalf("got statusCode %d, expected %d", rec.Code, http.StatusOK)
	}

	l := rec.Result().Header.Get("Location")
	if l != "/msg" {
		t.Fatalf("redirect Location is %s, expected %s", l, "/msg")
	}
}
