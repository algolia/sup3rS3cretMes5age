package internal

import (
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	for i := 0; i < 30; i++ {
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

func TestServerGzipCompression(t *testing.T) {
	validToken := "hvs.CABAAAAAAQAAAAAAAAAABBBB"

	tests := []struct {
		name           string
		path           string
		acceptEncoding string
		setupStorage   func() *FakeSecretMsgStorer
		expectedStatus int
		expectGzip     bool
		checkVary      bool
	}{
		{
			name:           "health endpoint with gzip support",
			path:           "/health",
			acceptEncoding: "gzip",
			setupStorage:   func() *FakeSecretMsgStorer { return &FakeSecretMsgStorer{} },
			expectedStatus: http.StatusOK,
			expectGzip:     true,
			checkVary:      true,
		},
		{
			name:           "health endpoint without gzip support",
			path:           "/health",
			acceptEncoding: "",
			setupStorage:   func() *FakeSecretMsgStorer { return &FakeSecretMsgStorer{} },
			expectedStatus: http.StatusOK,
			expectGzip:     false,
			checkVary:      false,
		},
		{
			name:           "API JSON response with gzip support",
			path:           "/secret?token=" + validToken,
			acceptEncoding: "gzip",
			setupStorage: func() *FakeSecretMsgStorer {
				return &FakeSecretMsgStorer{
					token: validToken,
					msg:   "This is a secret message that is long enough to benefit from gzip compression",
				}
			},
			expectedStatus: http.StatusOK,
			expectGzip:     true,
			checkVary:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cnf := conf{
				HttpBindingAddress: ":8080",
				VaultPrefix:        "cubbyhole/",
				AllowedOrigins:     []string{"*"},
			}
			storage := tt.setupStorage()
			handlers := NewSecretHandlers(storage)
			server := NewServer(cnf, handlers)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}
			rec := httptest.NewRecorder()
			server.handler().ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			if tt.expectGzip {
				assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"), "Response should be gzip compressed")
			} else {
				assert.Empty(t, rec.Header().Get("Content-Encoding"), "Response should not be compressed")
			}

			if tt.checkVary {
				varyHeader := rec.Header().Get("Vary")
				assert.NotEmpty(t, varyHeader, "Should have Vary header")
				// Vary header may contain "Origin" from CORS middleware, just verify it exists
				assert.Contains(t, varyHeader, "Accept-Encoding", "Vary header should be set by middleware")
			}
		})
	}
}

// TestGzipRangeRequests verifies that Range requests are not gzip-compressed:
// compressing a 206 Partial Content response would replace the body with gzip
// data while Content-Range/Content-Length still describe the identity
// representation, corrupting any range-capable client's download.
func TestGzipRangeRequests(t *testing.T) {
	// Static handlers resolve files relative to the working directory, so run
	// the test against a temporary "static/" tree.
	tmp := t.TempDir()
	t.Chdir(tmp)
	if err := os.MkdirAll(filepath.Join(tmp, "static", "locales"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte(strings.Repeat("0123456789abcdef", 16)) // 256 bytes, gzip-compressible
	if err := os.WriteFile(filepath.Join(tmp, "static", "locales", "test.json"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	cnf := conf{
		HttpBindingAddress: ":8080",
		VaultPrefix:        "cubbyhole/",
		AllowedOrigins:     []string{"*"},
	}
	server := NewServer(cnf, NewSecretHandlers(&FakeSecretMsgStorer{}))

	t.Run("range request is served uncompressed with correct Content-Range", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/static/locales/test.json", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		req.Header.Set("Range", "bytes=0-99")
		rec := httptest.NewRecorder()
		server.handler().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusPartialContent, rec.Code)
		assert.Empty(t, rec.Header().Get("Content-Encoding"), "Range responses must not be compressed")
		assert.Equal(t, "bytes 0-99/256", rec.Header().Get("Content-Range"))
		assert.Equal(t, content[0:100], rec.Body.Bytes(), "Body must be the exact identity byte range")
	})

	t.Run("full request is served gzip-compressed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/static/locales/test.json", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		server.handler().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"), "Response should be gzip compressed")

		zr, err := gzip.NewReader(rec.Body)
		require.NoError(t, err)
		decompressed, err := io.ReadAll(zr)
		require.NoError(t, err)
		assert.Equal(t, content, decompressed)
	})
}

// TestStaticCacheTiers pins the per-route Cache-Control policies. Locales
// must share the short tier with HTML pages so a deploy's fresh markup never
// pairs with stale locale JSON; fonts must not be immutable because their
// filenames are not content-hashed.
func TestStaticCacheTiers(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)
	for _, dir := range []string{"locales", "fonts", "icons"} {
		if err := os.MkdirAll(filepath.Join(tmp, "static", dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{
		"static/locales/en.json",
		"static/fonts/test.ttf",
		"static/icons/test.png",
	} {
		if err := os.WriteFile(filepath.Join(tmp, file), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cnf := conf{
		HttpBindingAddress: ":8080",
		VaultPrefix:        "cubbyhole/",
		AllowedOrigins:     []string{"*"},
	}
	server := NewServer(cnf, NewSecretHandlers(&FakeSecretMsgStorer{}))

	tests := []struct {
		name        string
		path        string
		cacheHeader string
	}{
		{"locale uses the short tier", "/static/locales/en.json", "public, max-age=300, must-revalidate"},
		{"font uses the week-long tier without immutable", "/static/fonts/test.ttf", "public, max-age=604800, must-revalidate"},
		{"icon uses the 24h tier", "/static/icons/test.png", "public, max-age=86400, must-revalidate"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			server.handler().ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tt.cacheHeader, rec.Header().Get("Cache-Control"))
			assert.NotContains(t, rec.Header().Get("Cache-Control"), "immutable")
		})
	}

	t.Run("missing file returns 404 without cache headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/static/locales/missing.json", nil)
		rec := httptest.NewRecorder()
		server.handler().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Empty(t, rec.Header().Get("Cache-Control"), "Error responses must not carry cache headers")
	})
}
