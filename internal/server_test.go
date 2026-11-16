package internal

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/acme/autocert"
)

func TestNewServer(t *testing.T) {
	cnf := conf{
		HttpBindingAddress: ":8080",
		VaultPrefix:        "cubbyhole/",
		AllowedOrigins:     []string{"*"},
	}
	handlers := NewSecretHandlers(&FakeSecretMsgStorer{})

	server := NewServer(cnf, handlers)

	assert.NotNil(t, server)
	assert.NotNil(t, server.echo)
	assert.NotNil(t, server.handlers)
	assert.Equal(t, cnf.HttpBindingAddress, server.config.HttpBindingAddress)
}

func TestServerHandler(t *testing.T) {
	cnf := conf{
		HttpBindingAddress: ":8080",
		VaultPrefix:        "cubbyhole/",
		AllowedOrigins:     []string{"*"},
	}
	handlers := NewSecretHandlers(&FakeSecretMsgStorer{})
	server := NewServer(cnf, handlers)

	// Test health endpoint
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	server.handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "OK", rec.Body.String())
}

func TestServerRoutesRegistered(t *testing.T) {
	cnf := conf{
		HttpBindingAddress: ":8080",
		VaultPrefix:        "cubbyhole/",
		AllowedOrigins:     []string{"*"},
	}
	handlers := NewSecretHandlers(&FakeSecretMsgStorer{})
	server := NewServer(cnf, handlers)

	routes := server.echo.Routes()
	assert.NotEmpty(t, routes)

	// Verify key routes exist
	routeMap := make(map[string]bool)
	for _, route := range routes {
		key := route.Method + " " + route.Path
		routeMap[key] = true
	}

	assert.True(t, routeMap["POST /secret"], "POST /secret should be registered")
	assert.True(t, routeMap["GET /secret"], "GET /secret should be registered")
	assert.True(t, routeMap["GET /health"] || routeMap["POST /health"], "/health should be registered")
	assert.True(t, routeMap["GET /"], "GET / should be registered")
}

func TestServerWithMiddlewares(t *testing.T) {
	cnf := conf{
		HttpBindingAddress: ":8080",
		VaultPrefix:        "cubbyhole/",
		AllowedOrigins:     []string{"http://localhost:3000"},
	}
	handlers := NewSecretHandlers(&FakeSecretMsgStorer{})
	server := NewServer(cnf, handlers)

	// Test CORS middleware
	req := httptest.NewRequest(http.MethodOptions, "/secret", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	server.handler().ServeHTTP(rec, req)

	assert.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestServerSecurityHeaders(t *testing.T) {
	cnf := conf{
		HttpBindingAddress: ":8080",
		VaultPrefix:        "cubbyhole/",
		AllowedOrigins:     []string{"*"},
	}
	handlers := NewSecretHandlers(&FakeSecretMsgStorer{})
	server := NewServer(cnf, handlers)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	server.handler().ServeHTTP(rec, req)

	// Verify security headers
	assert.Equal(t, "1; mode=block", rec.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	assert.Contains(t, rec.Header().Get("Content-Security-Policy"), "default-src 'self'")
}

func TestServerRedirect(t *testing.T) {
	cnf := conf{
		HttpBindingAddress: ":8080",
		VaultPrefix:        "cubbyhole/",
		AllowedOrigins:     []string{"*"},
	}
	handlers := NewSecretHandlers(&FakeSecretMsgStorer{})
	server := NewServer(cnf, handlers)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusPermanentRedirect, rec.Code)
	assert.Equal(t, "/msg", rec.Header().Get("Location"))
}

func TestServerWithTLSAutoDomain(t *testing.T) {
	cnf := conf{
		HttpBindingAddress: ":8080",
		TLSAutoDomain:      "example.com",
		VaultPrefix:        "cubbyhole/",
		AllowedOrigins:     []string{"*"},
	}
	handlers := NewSecretHandlers(&FakeSecretMsgStorer{})
	server := NewServer(cnf, handlers)

	assert.NotNil(t, server)
	// Verify TLS domain is configured (checking the pointer to avoid copylocks)
	assert.NotNil(t, server.echo)
	assert.Equal(t, "example.com", server.config.TLSAutoDomain)
	assert.Equal(t, autocert.DirCache("/var/www/.cache"), server.echo.AutoTLSManager.Cache)
}

func TestServerGracefulShutdown(t *testing.T) {
	cnf := conf{
		HttpBindingAddress: ":8080",
		VaultPrefix:        "cubbyhole/",
		AllowedOrigins:     []string{"*"},
	}
	handlers := NewSecretHandlers(&FakeSecretMsgStorer{})
	server := NewServer(cnf, handlers)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := server.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestServerHandlersIntegration(t *testing.T) {
	cnf := conf{
		HttpBindingAddress: ":8080",
		VaultPrefix:        "cubbyhole/",
		AllowedOrigins:     []string{"*"},
	}
	// Use valid Vault token format (hvs. prefix + 24 alphanumeric chars)
	validToken := "hvs.CABAAAAAAQAAAAAAAAAABBBB"
	storage := &FakeSecretMsgStorer{
		token: validToken,
		msg:   "secret message",
	}
	handlers := NewSecretHandlers(storage)
	server := NewServer(cnf, handlers)

	// Test GET /secret with valid token
	req := httptest.NewRequest(http.MethodGet, "/secret?token="+validToken, nil)
	rec := httptest.NewRecorder()
	server.handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "{\"msg\":\"secret message\"}\n", rec.Body.String())
	assert.Equal(t, "application/json", rec.Header().Get(echo.HeaderContentType))

	// Invalid token format is rejected with a JSON error body
	req = httptest.NewRequest(http.MethodGet, "/secret?token=bad", nil)
	rec = httptest.NewRecorder()
	server.handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var errBody map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errBody))
	assert.Len(t, errBody, 1)
	assert.NotEmpty(t, errBody["message"])
}

func TestServerRateLimiting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping rate limit test in short mode")
	}

	cnf := conf{
		HttpBindingAddress: ":8080",
		VaultPrefix:        "cubbyhole/",
		AllowedOrigins:     []string{"*"},
	}
	handlers := NewSecretHandlers(&FakeSecretMsgStorer{})
	server := NewServer(cnf, handlers)

	// Make rapid requests to trigger rate limit
	successCount := 0
	rateLimitCount := 0

	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.Header.Set("X-Real-IP", "192.168.1.1")
		rec := httptest.NewRecorder()
		server.handler().ServeHTTP(rec, req)

		switch rec.Code {
		case http.StatusOK:
			successCount++
		case http.StatusTooManyRequests:
			if rateLimitCount == 0 {
				assert.Equal(t, "{\"error\":\"rate limit exceeded\"}\n", rec.Body.String())
			}
			rateLimitCount++
		}
	}

	// Should have some rate limited requests
	assert.Greater(t, rateLimitCount, 0, "Rate limiter should have triggered")
}

// accessLogLine decodes the single access-log line in buf.
func accessLogLine(t *testing.T, buf *bytes.Buffer) (string, map[string]any) {
	t.Helper()
	line := buf.String()
	require.True(t, strings.HasSuffix(line, "\n"), "log line must end with a newline")
	require.Equal(t, 1, strings.Count(line, "\n"), "expected exactly one log line")
	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(line), &entry))
	return line, entry
}

func TestServerAccessLog(t *testing.T) {
	cnf := conf{
		HttpBindingAddress: ":8080",
		VaultPrefix:        "cubbyhole/",
		AllowedOrigins:     []string{"*"},
	}
	server := NewServer(cnf, NewSecretHandlers(&FakeSecretMsgStorer{msg: "msg"}))
	var buf bytes.Buffer
	server.echo.Logger.SetOutput(&buf)

	t.Run("request is logged as one JSON line", func(t *testing.T) {
		buf.Reset()
		req := httptest.NewRequest(http.MethodGet, "/secret?token=hvs.CABAAAAAAQAAAAAAAAAABBBB", nil)
		req.Header.Set("User-Agent", "e2e-agent")
		rec := httptest.NewRecorder()
		server.handler().ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		_, entry := accessLogLine(t, &buf)
		keys := make([]string, 0, len(entry))
		for k := range entry {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		want := []string{"time", "id", "remote_ip", "host", "method", "uri", "user_agent",
			"status", "error", "latency", "latency_human", "bytes_in", "bytes_out"}
		slices.Sort(want)
		assert.Equal(t, want, keys)
		assert.Equal(t, "GET", entry["method"])
		assert.Equal(t, float64(200), entry["status"])
		assert.Equal(t, "e2e-agent", entry["user_agent"])
		assert.Equal(t, "", entry["error"])
		assert.True(t, strings.HasPrefix(entry["uri"].(string), "/secret?"))
	})

	t.Run("health is not logged", func(t *testing.T) {
		buf.Reset()
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		server.handler().ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, 0, buf.Len())
	})

	t.Run("handler error is logged", func(t *testing.T) {
		buf.Reset()
		req := httptest.NewRequest(http.MethodGet, "/secret?token=bad", nil)
		rec := httptest.NewRecorder()
		server.handler().ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)

		_, entry := accessLogLine(t, &buf)
		assert.Equal(t, float64(400), entry["status"])
		assert.NotEmpty(t, entry["error"])
	})
}
