package internal

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

// TestAccessLogRedactsTokens pins the access-log redaction: one-time Vault
// tokens must never reach the logs — a logged token is a second copy of the
// secret, readable before the first retrieval. Non-token query parameters
// must survive for debugging context.
func TestAccessLogRedactsTokens(t *testing.T) {
	var logBuf bytes.Buffer
	cnf := conf{
		HttpBindingAddress: ":8080",
		VaultPrefix:        "cubbyhole/",
		AllowedOrigins:     []string{"*"},
	}
	handlers := NewSecretHandlers(&FakeSecretMsgStorer{})
	server := NewServer(cnf, handlers)
	server.echo.Logger.SetOutput(&logBuf)

	token := "hvs.CABAAAAAAQAAAAAAAAAABBBBCCCCDDDDEEEE"
	req := httptest.NewRequest(http.MethodGet,
		"/secret?token="+token+"&lang=fr&filename=report.pdf&filetoken=hvs.SECOND", nil)
	rec := httptest.NewRecorder()
	server.handler().ServeHTTP(rec, req)

	logged := logBuf.String()
	assert.Contains(t, logged, "token=REDACTED", "token value must be redacted")
	assert.Contains(t, logged, "filetoken=REDACTED", "filetoken value must be redacted")
	assert.NotContains(t, logged, token, "the raw one-time token must never reach the logs")
	assert.NotContains(t, logged, "hvs.SECOND", "the raw file token must never reach the logs")
	assert.Contains(t, logged, "lang=fr", "non-token parameters must survive")
	assert.Contains(t, logged, "filename=report.pdf", "non-token parameters must survive")
}

func TestRedactTokens(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no query parameters stays untouched", "/msg", "/msg"},
		{"non-token parameters stay untouched", "/msg?lang=fr&ttl=48h", "/msg?lang=fr&ttl=48h"},
		{"token value redacted", "/secret?token=hvs.abc&lang=fr", "/secret?lang=fr&token=REDACTED"},
		{"filetoken value redacted", "/getmsg?token=hvs.a&filetoken=hvs.b&filename=f.pdf",
			"/getmsg?filename=f.pdf&filetoken=REDACTED&token=REDACTED"},
		{"param name matching is case-insensitive", "/secret?Token=hvs.abc", "/secret?Token=REDACTED"},
		{"unparseable query is returned unchanged", "/msg?%%zz", "/msg?%%zz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, redactTokens(tt.input))
		})
	}
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
	assert.Contains(t, rec.Body.String(), "secret message")
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
			rateLimitCount++
		}
	}

	// Should have some rate limited requests
	assert.Greater(t, rateLimitCount, 0, "Rate limiter should have triggered")
}
