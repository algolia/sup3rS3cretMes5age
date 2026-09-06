package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain points the supported-language list at the real locales directory
// so tests run against the same source of truth as production startup.
func TestMain(m *testing.M) {
	langs, err := loadSupportedLanguages("../web/static/locales")
	if err != nil {
		fmt.Fprintf(os.Stderr, "test setup: %v\n", err)
		os.Exit(1)
	}
	supportedLanguages = langs
	os.Exit(m.Run())
}

// TestLocalesManifestMatchesDirectory fails when a locale file is added or
// removed without regenerating web/static/locales-manifest.json, which the
// client uses as its language list.
func TestLocalesManifestMatchesDirectory(t *testing.T) {
	langs, err := loadSupportedLanguages("../web/static/locales")
	require.NoError(t, err)

	data, err := os.ReadFile("../web/static/locales-manifest.json")
	require.NoError(t, err)

	var manifest struct {
		Languages []string `json:"languages"`
	}
	require.NoError(t, json.Unmarshal(data, &manifest))
	assert.Equal(t, langs, manifest.Languages)
}

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

func TestPrimaryLanguageTag(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"fr", "fr"},
		{"fr-CA", "fr"},
		{"fr_CA", "fr"},
		{"DE", "de"},
		{"  it  ", "it"},
		{"*", ""},
		{"", ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, primaryLanguageTag(tt.input), "primaryLanguageTag(%q)", tt.input)
	}
}

func TestParseAcceptLanguage(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{"simple preference", "de", "de"},
		{"q-values respected", "de;q=0.9,fr;q=0.8", "de"},
		{"higher q wins regardless of order", "fr;q=0.1,en;q=0.9", "en"},
		{"bare tag outranks weighted tag", "fr;q=0.5,en", "en"},
		{"region subtag stripped", "fr-CA,de", "fr"},
		{"case insensitive", "FR,DE", "fr"},
		{"unsupported languages ignored", "pt-BR,ja;q=0.9,es;q=0.5", "es"},
		{"wildcard entries skipped", "*,es;q=0.5", "es"},
		{"unsupported only yields empty", "pt-BR,ja", ""},
		{"empty header yields empty", "", ""},
		{"malformed q ignored, default 1.0 used", "de;q=abc,fr;q=0.2", "de"},
		{"uppercase Q parsed (param names case-insensitive)", "en;Q=0,fr;q=0.5", "fr"},
		{"out-of-range q ignored, default 1.0 used", "de;q=5,fr;q=0.5", "de"},
		{"negative q ignored, default 1.0 used", "de;q=-1,fr;q=0.5", "de"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, parseAcceptLanguage(tt.header))
		})
	}
}

func TestResolveLanguage(t *testing.T) {
	tests := []struct {
		name      string
		langParam string
		header    string
		expected  string
	}{
		{"lang param wins", "de", "fr", "de"},
		{"region param normalized", "fr-CA", "de", "fr"},
		{"invalid param falls through to header", "pt", "fr", "fr"},
		{"no param uses header", "", "fr-CA", "fr"},
		{"q-values parsed from header", "", "de;q=0.9,fr;q=0.8", "de"},
		{"wildcard falls to default", "", "*", "en"},
		{"nothing set defaults to en", "", "", "en"},
		{"uppercase param lowercased", "FR", "", "fr"},
		{"uppercase header lowercased", "", "DE", "de"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, resolveLanguage(tt.langParam, tt.header))
		})
	}
}

func TestInitSupportedLanguagesRequiresEnglishFallback(t *testing.T) {
	tmp := t.TempDir()
	// Non-English locales alone are not enough: 'en' is the hardcoded
	// fallback in resolveLanguage and the client, so startup must refuse.
	if err := os.WriteFile(filepath.Join(tmp, "fr.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := InitSupportedLanguages(tmp)
	assert.ErrorContains(t, err, "en.json")

	if err := os.WriteFile(filepath.Join(tmp, "en.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	assert.NoError(t, InitSupportedLanguages(tmp))
}

func TestHTMLCachePolicies(t *testing.T) {
	t.Run("publicHTMLCache uses the shared short tier", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/msg", nil)
		assert.Equal(t, shortCacheControl, publicHTMLCache(e.NewContext(req, nil)))
	})

	t.Run("getmsgHTMLCache disables storage for token-bearing URLs", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/getmsg?token=hvs.CABAAAAAAQAAAAAAAAAABBBB", nil)
		assert.Equal(t, "no-store, private", getmsgHTMLCache(e.NewContext(req, nil)))
	})

	t.Run("getmsgHTMLCache uses the shared short tier without token", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/getmsg", nil)
		assert.Equal(t, shortCacheControl, getmsgHTMLCache(e.NewContext(req, nil)))
	})
}

func TestGetMsgHandler(t *testing.T) {
	tests := []struct {
		name           string
		token          string
		storedMsg      string
		storeErr       error
		expectedStatus int
		expectedMsg    string
		expectError    bool
	}{
		{
			name:           "successful message retrieval",
			token:          "hvs.CABAAAAAAQAAAAAAAAAABBBB",
			storedMsg:      "secret",
			storeErr:       nil,
			expectedStatus: http.StatusOK,
			expectedMsg:    "{\"msg\":\"secret\"}\n",
			expectError:    false,
		},
		{
			name:           "message retrieval with error",
			token:          "hvs.CABAAAAAAQAAAAAAAAAABBBB",
			storedMsg:      "secret",
			storeErr:       errors.New("expired"),
			expectedStatus: http.StatusNotFound,
			expectedMsg:    "",
			expectError:    true,
		},
		{
			name:           "invalid token format",
			token:          "invalid-token-123",
			storedMsg:      "",
			storeErr:       nil,
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/?token="+tt.token, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			s := &FakeSecretMsgStorer{msg: tt.storedMsg, err: tt.storeErr}
			h := NewSecretHandlers(s)
			err := h.GetMsgHandler(c)

			if tt.expectError {
				assert.Error(t, err)
				if assert.IsType(t, &echo.HTTPError{}, err) {
					v, _ := err.(*echo.HTTPError)
					assert.Equal(t, tt.expectedStatus, v.Code)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.token, s.lastUsedToken)
				assert.Equal(t, tt.expectedStatus, rec.Code)
				assert.Equal(t, tt.expectedMsg, rec.Body.String())
				// One-time secret responses must never be cacheable: a
				// cached copy would let a second reader get the message.
				assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
			}
		})
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
	tests := []struct {
		name             string
		url              string
		expectedLocation string
	}{
		{"root without query", "/", "/msg"},
		{"query string preserved", "/?lang=fr", "/msg?lang=fr"},
		{"multiple params preserved", "/?lang=fr&x=1", "/msg?lang=fr&x=1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			err := redirectHandler(c)
			assert.NoError(t, err)

			assert.Equal(t, http.StatusPermanentRedirect, rec.Code)
			assert.Equal(t, tt.expectedLocation, rec.Result().Header.Get("Location"))
		})
	}
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
			h := NewSecretHandlers(s)
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

func TestCreateMsgHandlerWithFile(t *testing.T) {
	tests := []struct {
		name         string
		msg          string
		ttl          string
		fileName     string
		fileContent  []byte
		expectError  bool
		expectedCode int
		checkToken   bool
		checkFile    bool
	}{
		{
			name:         "valid message with file",
			msg:          "secret message",
			ttl:          "1h",
			fileName:     "test.txt",
			fileContent:  []byte("file content"),
			expectError:  false,
			expectedCode: http.StatusOK,
			checkToken:   true,
			checkFile:    true,
		},
		{
			name:         "valid message with file, no TTL",
			msg:          "secret message",
			ttl:          "",
			fileName:     "document.pdf",
			fileContent:  []byte("PDF content here"),
			expectError:  false,
			expectedCode: http.StatusOK,
			checkToken:   true,
			checkFile:    true,
		},
		{
			name:         "empty file should not create file token",
			msg:          "secret message",
			ttl:          "1h",
			fileName:     "empty.txt",
			fileContent:  []byte{},
			expectError:  false,
			expectedCode: http.StatusOK,
			checkToken:   true,
			checkFile:    false,
		},
		{
			name:         "file with path traversal",
			msg:          "secret message",
			ttl:          "1h",
			fileName:     "../etc/passwd",
			fileContent:  []byte("malicious"),
			expectError:  true,
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "file with slash in name",
			msg:          "secret message",
			ttl:          "1h",
			fileName:     "path/to/file.txt",
			fileContent:  []byte("content"),
			expectError:  true,
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "file too big",
			msg:          "secret message",
			ttl:          "1h",
			fileName:     "bigfile.txt",
			fileContent:  make([]byte, 50*1024*1024+1), // 50MB + 1 byte
			expectError:  true,
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create multipart form
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			// Add message field
			err := writer.WriteField("msg", tt.msg)
			assert.NoError(t, err)

			// Add TTL field if provided
			if tt.ttl != "" {
				err = writer.WriteField("ttl", tt.ttl)
				assert.NoError(t, err)
			}

			// Add file field
			part, err := writer.CreateFormFile("file", tt.fileName)
			assert.NoError(t, err)
			_, err = part.Write(tt.fileContent)
			assert.NoError(t, err)

			err = writer.Close()
			assert.NoError(t, err)

			// Create request
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/secret", body)
			req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// Create fake store that returns tokens
			s := &FakeSecretMsgStorer{token: "msg-token-123"}
			h := NewSecretHandlers(s)

			// Execute handler
			handlerErr := h.CreateMsgHandler(c)

			if tt.expectError {
				assert.Error(t, handlerErr)
				if httpErr, ok := handlerErr.(*echo.HTTPError); ok {
					assert.Equal(t, tt.expectedCode, httpErr.Code)
				}
			} else {
				assert.NoError(t, handlerErr)
				assert.Equal(t, tt.expectedCode, rec.Code)

				// Parse response
				var response TokenResponse
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				assert.NoError(t, err)

				if tt.checkToken {
					assert.Equal(t, "msg-token-123", response.Token)
					assert.Equal(t, tt.msg, s.lastMsg)
				}

				if tt.checkFile {
					assert.NotEmpty(t, response.FileToken)
					assert.Equal(t, tt.fileName, response.FileName)
				} else {
					assert.Empty(t, response.FileToken)
					assert.Empty(t, response.FileName)
				}
			}
		})
	}
}
