package internal

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				var he *echo.HTTPError
				require.ErrorAs(t, err, &he)
				assert.Equal(t, tt.expectedStatus, he.Code)
				if tt.expectedStatus == http.StatusBadRequest {
					// Token validation must run before any Vault lookup (path-injection guard).
					assert.Empty(t, s.lastUsedToken)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.token, s.lastUsedToken)
				assert.Equal(t, tt.expectedStatus, rec.Code)
				assert.Equal(t, tt.expectedMsg, rec.Body.String())
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
		{"168h", true},      // 7 days - maximum
		{"169h", false},     // exceeds maximum
		{"30s", false},      // below minimum
		{"0h", false},       // zero duration
		{"", false},         // empty
		{"invalid", false},  // invalid format
		{"1d", false},       // 'd' not supported by Go
		{"-1h", false},      // negative duration
		{"1m", true},        // minimum
		{"59s", false},      // just below minimum
		{"168h0m1s", false}, // just above maximum
	}

	for _, tt := range tests {
		t.Run(tt.ttl, func(t *testing.T) {
			assert.Equal(t, tt.valid, isValidTTL(tt.ttl))
		})
	}
}

func TestValidateVaultToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		valid bool
	}{
		{"hvs 24 chars", "hvs." + strings.Repeat("A", 24), true},
		{"hvb 24 chars", "hvb." + strings.Repeat("a", 24), true},
		{"legacy 93 chars", "hvs." + strings.Repeat("a_-", 31), true},
		{"empty", "", false},
		{"legacy s. prefix", "s." + strings.Repeat("A", 24), false},
		{"hvs 23 chars", "hvs." + strings.Repeat("A", 23), false},
		{"hvs 25 chars", "hvs." + strings.Repeat("A", 25), false},
		{"path traversal", "hvs." + strings.Repeat("A", 22) + "/..", false},
		{"legacy one short", "hvs." + strings.Repeat("a", 90), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVaultToken(tt.token)
			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
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
				var he *echo.HTTPError
				require.ErrorAs(t, err, &he)
				assert.Equal(t, http.StatusBadRequest, he.Code)
				assert.Equal(t, tt.errMessage, he.Message)
			} else {
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, rec.Code)
				assert.Equal(t, "{\"token\":\"testtoken\"}\n", rec.Body.String())
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
				var he *echo.HTTPError
				require.ErrorAs(t, handlerErr, &he)
				assert.Equal(t, tt.expectedCode, he.Code)
			} else {
				require.NoError(t, handlerErr)
				assert.Equal(t, tt.expectedCode, rec.Code)

				// Parse response
				var response TokenResponse
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)

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
