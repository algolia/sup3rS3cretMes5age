package internal

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
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
		// A control character makes url.Parse fail outright; the query must
		// be dropped rather than logged unredacted (it carries a token here).
		{"unparseable URI drops the query, path kept", "/msg?token=hvs.AAA\x00", "/msg"},
		// u.Query() silently discards a pair whose value has an invalid %
		// escape; the raw query must still not reach the log with a token.
		{"malformed query value drops the query", "/msg?token=hvs.secret%ZZ", "/msg"},
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

func TestParseTrustedProxies(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected int
	}{
		{"empty", "", 0},
		{"whitespace only", "   ", 0},
		{"single bare IPv4", "10.0.0.1", 1},
		{"single CIDR", "10.0.0.0/8", 1},
		{"bare IPv6", "fd00::1", 1},
		{"multiple entries", "10.0.0.1, 192.168.0.0/16, fd00::/8", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			networks := parseTrustedProxies(tt.raw)
			assert.Len(t, networks, tt.expected)
		})
	}
}

func TestTrustedClientIP(t *testing.T) {
	proxy := parseTrustedProxies("10.0.0.0/8")
	proxyV6 := parseTrustedProxies("fd00::1")

	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		trusted    []*net.IPNet
		expected   string
		expectErr  bool
	}{
		{
			name:       "no trusted proxies: header ignored, connection peer used",
			remoteAddr: "203.0.113.7:1234",
			xff:        "1.2.3.4",
			trusted:    nil,
			expected:   "203.0.113.7",
		},
		{
			name:       "untrusted peer: spoofed header ignored",
			remoteAddr: "203.0.113.7:1234",
			xff:        "1.2.3.4, 5.6.7.8",
			trusted:    proxy,
			expected:   "203.0.113.7",
		},
		{
			name:       "trusted proxy: client taken from X-Forwarded-For",
			remoteAddr: "10.0.0.1:1234",
			xff:        "203.0.113.7",
			trusted:    proxy,
			expected:   "203.0.113.7",
		},
		{
			name:       "trusted proxy: right-to-left walk skips trusted hops",
			remoteAddr: "10.0.0.1:1234",
			xff:        "203.0.113.7, 10.0.0.9",
			trusted:    proxy,
			expected:   "203.0.113.7",
		},
		{
			name:       "trusted proxy, no header: falls back to peer",
			remoteAddr: "10.0.0.1:1234",
			xff:        "",
			trusted:    proxy,
			expected:   "10.0.0.1",
		},
		{
			name:       "trusted proxy, malformed header: falls back to peer",
			remoteAddr: "10.0.0.1:1234",
			xff:        "not-an-ip",
			trusted:    proxy,
			expected:   "10.0.0.1",
		},
		{
			name:       "all entries trusted: connection peer used, not an attacker-chosen entry",
			remoteAddr: "10.0.0.1:1234",
			xff:        "10.0.0.2, 10.0.0.3",
			trusted:    proxy,
			expected:   "10.0.0.1",
		},
		{
			name:       "malformed entry after trusted hop: connection peer used, not the parsed trusted hop",
			remoteAddr: "10.0.0.1:1234",
			xff:        "garbage, 10.0.0.2",
			trusted:    proxy,
			expected:   "10.0.0.1",
		},
		{
			name:       "IPv6 trusted proxy",
			remoteAddr: "[fd00::1]:1234",
			xff:        "2001:db8::1",
			trusted:    proxyV6,
			expected:   "2001:db8::1",
		},
		{
			name:       "unparseable remote address fails closed",
			remoteAddr: "garbage",
			xff:        "1.2.3.4",
			trusted:    proxy,
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := trustedClientIP(tt.remoteAddr, tt.xff, tt.trusted)
			if tt.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestRateLimitSpoofedHeadersShareOneBucket pins finding #5 end to end
// through the middleware stack: with no trusted proxy configured, rotating
// X-Forwarded-For from one connection must NOT earn a fresh bucket per
// request — the shared bucket must exhaust and start answering 429. This
// drives the middleware's actual IdentifierExtractor: it would fail if
// setupMiddlewares ever reverted to ctx.RealIP().
func TestRateLimitSpoofedHeadersShareOneBucket(t *testing.T) {
	cnf := conf{
		HttpBindingAddress: ":8080",
		VaultPrefix:        "cubbyhole/",
	}
	e := echo.New()
	setupMiddlewares(e, cnf)
	e.GET("/probe", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	// Rate 5/s, burst 10: 15 rapid requests from one RemoteAddr with a
	// different spoofed header each must hit the single shared bucket.
	saw429 := false
	for i := 0; i < 15; i++ {
		req := httptest.NewRequest(http.MethodGet, "/probe", nil)
		req.RemoteAddr = "203.0.113.7:55555"
		req.Header.Set(echo.HeaderXForwardedFor, fmt.Sprintf("1.2.3.%d", i))
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			saw429 = true
			break
		}
		assert.Equal(t, http.StatusOK, rec.Code, "unexpected status on request %d", i+1)
	}

	assert.True(t, saw429,
		"15 requests from one RemoteAddr with rotating X-Forwarded-For must exhaust one shared bucket (no 429 seen: each spoof got a fresh bucket)")
}

// TestRateLimitExtractorErrorFailsClosedWith429 pins the extractor-error
// path: Echo routes IdentifierExtractor errors to RateLimiterConfig's
// ErrorHandler (not DenyHandler), whose default would answer 403 with the
// raw error. An unusable client identifier must fail closed with the same
// constant 429 response as an exhausted bucket.
func TestRateLimitExtractorErrorFailsClosedWith429(t *testing.T) {
	cnf := conf{
		HttpBindingAddress: ":8080",
		VaultPrefix:        "cubbyhole/",
	}
	e := echo.New()
	setupMiddlewares(e, cnf)
	e.GET("/probe", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.RemoteAddr = "not-an-address"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}
