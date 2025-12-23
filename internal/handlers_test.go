package internal

import (
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
		assert.Equal(t, http.StatusNotFound, v.Code)
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

func TestIsValidTTL(t *testing.T) {
	tests := []struct {
		ttl   string
		valid bool
	}{
		{"1h", true},
		{"30m", true},
		{"2h30m", true},
		{"48h", true},
		{"168h", true},     // 7 days - maximum
		{"169h", false},    // exceeds maximum
		{"30s", false},     // below minimum
		{"0h", false},      // zero duration
		{"", false},        // empty
		{"invalid", false}, // invalid format
		{"1d", false},      // 'd' not supported by Go
		{"-1h", false},     // negative duration
	}

	for _, tt := range tests {
		result := isValidTTL(tt.ttl)
		assert.Equal(t, result, tt.valid)
	}
}
func TestValidateMsg(t *testing.T) {
	tests := []struct {
		name    string
		msg     string
		wantErr bool
	}{
		{"valid message", "test secret", false},
		{"empty message", "", true},
		{"message too large", strings.Repeat("a", 1024*1024+1), true},
		{"message at limit", strings.Repeat("a", 1024*1024), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMsg(tt.msg)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateFileUpload(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		size     int64
		wantErr  bool
	}{
		{"valid file", "test.pdf", 1000, false},
		{"file too large", "large.pdf", 51 * 1024 * 1024, true},
		{"path traversal dots", "../etc/passwd", 100, true},
		{"path traversal slash", "path/to/file", 100, true},
		{"at size limit", "limit.pdf", 50 * 1024 * 1024, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := &multipart.FileHeader{
				Filename: tt.filename,
				Size:     tt.size,
			}
			err := validateFileUpload(file)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateMsgHandler(t *testing.T) {
	tests := []struct {
		name       string
		msg        string
		ttl        string
		errMessage string
	}{
		{"valid message and ttl", "hello world", "1h", ""},
		{"valid message, no ttl", "hello world", "", ""},
		{"empty message", "", "1h", "message is required"},
		{"message too large", strings.Repeat("a", 1024*1024+1), "1h", "message too large"},
		{"invalid ttl", "hello world", "30s", "invalid TTL format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			form := make(url.Values)
			form.Set("msg", tt.msg)
			form.Set("ttl", tt.ttl)

			req := httptest.NewRequest(http.MethodPost, "/secret", strings.NewReader(form.Encode()))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			s := &FakeSecretMsgStorer{token: "testtoken"}
			h := newSecretHandlers(s)
			err := h.CreateMsgHandler(c)

			if tt.errMessage != "" {
				assert.Error(t, err)
				if httpErr, ok := err.(*echo.HTTPError); ok {
					assert.Equal(t, http.StatusBadRequest, httpErr.Code)
					assert.Equal(t, tt.errMessage, httpErr.Message)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, http.StatusOK, rec.Code)
				assert.Equal(t, tt.msg, s.lastMsg)
			}
		})
	}
}
