// Package internal provides HTTP server setup and request handlers for the sup3rS3cretMes5age application.
// It includes server lifecycle management with graceful shutdown, middleware configuration,
// route setup, and integration with HashiCorp Vault for secure message storage.
package internal

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

// Server encapsulates the HTTP/HTTPS server configuration and lifecycle management.
// It provides testable server initialization and graceful shutdown capabilities.
type Server struct {
	echo        *echo.Echo
	config      conf
	handlers    *SecretHandlers
	httpServer  *http.Server
	httpsServer *http.Server
}

// NewServer creates a new Server instance with the provided configuration and handlers.
// It configures Echo with all middleware and routes but does not start the server.
// This allows the server to be tested without binding to network ports.
func NewServer(cnf conf, handlers *SecretHandlers) *Server {
	e := echo.New()
	e.HideBanner = true

	// Configure Auto TLS if enabled
	if cnf.TLSAutoDomain != "" {
		e.AutoTLSManager.HostPolicy = autocert.HostWhitelist(cnf.TLSAutoDomain)
		e.AutoTLSManager.Cache = autocert.DirCache("/var/www/.cache")
	}

	s := &Server{
		echo:     e,
		config:   cnf,
		handlers: handlers,
	}

	setupMiddlewares(e, cnf)
	setupRoutes(e, handlers)

	return s
}

// Start begins listening for HTTP and/or HTTPS requests based on configuration.
// It supports three modes:
// 1. HTTP only (when only HttpBindingAddress is set)
// 2. HTTPS only with Auto TLS or Manual TLS
// 3. Both HTTP and HTTPS (HTTP typically for redirect)
//
// The function blocks until the server is shut down via context cancellation
// or encounters a fatal error.
func (s *Server) Start(ctx context.Context) error {
	// Channel to collect errors from goroutines
	errChan := make(chan error, 2)

	// Start HTTP server if configured
	if s.config.HttpBindingAddress != "" {
		if s.config.HttpsBindingAddress != "" {
			// Both HTTP and HTTPS - run HTTP in goroutine
			go func() {
				if err := s.startHTTP(); err != nil && err != http.ErrServerClosed {
					errChan <- err
				}
			}()
		} else {
			// HTTP only
			go func() {
				if err := s.startHTTP(); err != nil && err != http.ErrServerClosed {
					errChan <- err
				}
			}()
		}
	}

	// Start HTTPS server if TLS is configured
	if s.config.HttpsBindingAddress != "" || s.config.TLSAutoDomain != "" || s.config.TLSCertFilepath != "" {
		go func() {
			if err := s.startHTTPS(); err != nil && err != http.ErrServerClosed {
				errChan <- err
			}
		}()
	}

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		return s.Shutdown(context.Background())
	case err := <-errChan:
		return err
	}
}

// startHTTP starts the HTTP server on the configured binding address.
func (s *Server) startHTTP() error {
	s.httpServer = &http.Server{
		Addr:           s.config.HttpBindingAddress,
		Handler:        s.echo,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	s.echo.Logger.Infof("Starting HTTP server on %s", s.config.HttpBindingAddress)
	return s.httpServer.ListenAndServe()
}

// startHTTPS starts the HTTPS server with TLS configuration.
// Supports both automatic TLS (Let's Encrypt) and manual certificate configuration.
func (s *Server) startHTTPS() error {
	autoTLSManager := autocert.Manager{
		Prompt: autocert.AcceptTOS,
		Cache:  autocert.DirCache("/var/www/.cache"),
	}

	// Use HTTPS binding address if set, otherwise default to :443
	addr := s.config.HttpsBindingAddress
	if addr == "" {
		addr = ":443"
	}

	s.httpsServer = &http.Server{
		Addr:           addr,
		Handler:        s.echo,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
		TLSConfig: &tls.Config{
			GetCertificate:   autoTLSManager.GetCertificate,
			NextProtos:       []string{acme.ALPNProto},
			MinVersion:       tls.VersionTLS12,
			CurvePreferences: []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.X25519, tls.CurveP256},
			CipherSuites: []uint16{
				// TLS 1.2 safe cipher suites
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				// TLS 1.3 cipher suites
				tls.TLS_AES_128_GCM_SHA256,
				tls.TLS_AES_256_GCM_SHA384,
				tls.TLS_CHACHA20_POLY1305_SHA256,
			},
		},
	}

	s.echo.Logger.Infof("Starting HTTPS server on %s", addr)

	// Start with manual certificates if provided, otherwise use auto TLS
	if s.config.TLSCertFilepath != "" && s.config.TLSCertKeyFilepath != "" {
		return s.httpsServer.ListenAndServeTLS(s.config.TLSCertFilepath, s.config.TLSCertKeyFilepath)
	}

	return s.httpsServer.ListenAndServeTLS("", "")
}

// Shutdown gracefully shuts down the server without interrupting active connections.
// It stops accepting new requests and waits for existing requests to complete
// within the provided context timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	s.echo.Logger.Info("Shutting down server...")

	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			s.echo.Logger.Errorf("HTTP server shutdown error: %v", err)
		}
	}

	if s.httpsServer != nil {
		if err := s.httpsServer.Shutdown(ctx); err != nil {
			s.echo.Logger.Errorf("HTTPS server shutdown error: %v", err)
		}
	}

	return s.echo.Shutdown(ctx)
}

// handler returns the underlying http.Handler for testing purposes.
// This allows tests to use httptest.ResponseRecorder without starting a real server.
func (s *Server) handler() http.Handler {
	return s.echo
}

// redactTokens strips the values of token-bearing query parameters from a
// request URI before it reaches the access logs. One-time Vault tokens must
// never be logged: a token in the access log is a second copy of the secret,
// readable by anyone with log access before the first retrieval consumes it.
// Parameter names are preserved (token, filetoken, lang, filename, ttl, …)
// so debugging keeps its context; only the values are masked.
func redactTokens(rawURI string) string {
	// dropQuery removes everything from '?' onward: the fallback when the
	// query cannot be parsed reliably enough to redact it.
	dropQuery := func() string {
		if idx := strings.Index(rawURI, "?"); idx >= 0 {
			return rawURI[:idx]
		}
		return rawURI
	}

	u, err := url.Parse(rawURI)
	if err != nil {
		// Unparseable URI: drop the query entirely rather than risk
		// logging a token we failed to redact.
		return dropQuery()
	}
	// u.Query() would silently discard malformed pairs (e.g. a token value
	// containing an invalid % escape), leaving such a token unredacted;
	// parse the raw query explicitly and treat a failure like an
	// unparseable URI.
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return dropQuery()
	}
	changed := false
	for name := range q {
		if strings.Contains(strings.ToLower(name), "token") {
			q.Set(name, "REDACTED")
			changed = true
		}
	}
	if !changed {
		return rawURI
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// trustedClientIP identifies the client to rate-limit on, without trusting
// X-Forwarded-For unless it is safe to do so.
//
// Echo's ctx.RealIP() honors X-Forwarded-For unconditionally, so behind no
// proxy (or an untrusted one) an attacker can send a fresh header on every
// request and get a fresh rate-limit bucket, defeating the per-IP limit.
// Instead:
//   - no trusted proxies configured → always the connection peer;
//   - peer is NOT a trusted proxy → the connection peer (headers ignored —
//     only a trusted intermediary can speak for the client);
//   - peer IS a trusted proxy → walk X-Forwarded-For right to left, skipping
//     trusted hops, and use the first untrusted address as the client
//     (the standard interpretation, robust to proxies that append). If a
//     malformed entry is hit, or every entry claims to be a trusted proxy,
//     the chain cannot be vouched for and the connection peer is used —
//     never an address the request itself selected.
//
// An unparseable RemoteAddr fails closed (error → 429) rather than opening
// an unauthenticated bucket.
func trustedClientIP(remoteAddr string, forwardedFor string, trusted []*net.IPNet) (string, error) {
	peerHost, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// RemoteAddr without a port (rare in tests): use it as-is.
		peerHost = remoteAddr
	}
	peer := net.ParseIP(peerHost)
	if peer == nil {
		return "", fmt.Errorf("unable to determine client address from %q", remoteAddr)
	}

	if len(trusted) == 0 || !containsIP(trusted, peer) {
		return peer.String(), nil
	}

	// Walk X-Forwarded-For right to left: entries on the right were added by
	// the closest proxies and are the only ones a trusted proxy chain vouches for.
	parts := strings.Split(forwardedFor, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		candidate := net.ParseIP(strings.TrimSpace(parts[i]))
		if candidate == nil {
			// Malformed entry: the chain is not trustworthy past this point,
			// and returning any already-seen entry would let an attacker
			// behind the proxy rotate buckets with crafted garbage. Fall
			// back to the connection peer.
			return peer.String(), nil
		}
		if !containsIP(trusted, candidate) {
			return candidate.String(), nil
		}
	}
	// Every entry claims to be a trusted proxy: the real client sits to the
	// left of anything we can vouch for, so the leftmost entry is
	// attacker-chosen too. Fall back to the connection peer.
	return peer.String(), nil
}

// containsIP reports whether ip falls within any of the networks.
func containsIP(networks []*net.IPNet, ip net.IP) bool {
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// setupMiddlewares configures Echo's middleware stack with security, rate limiting, and logging.
// It applies HTTPS redirect (if enabled), CORS policy, rate limiting (5 RPS), request logging,
// security headers (CSP, XSS protection, HSTS), body size limits (50MB), and panic recovery.
// Middleware is applied in order: pre-routing (HTTPS redirect), then request-level middleware.
func setupMiddlewares(e *echo.Echo, cnf conf) {
	if cnf.HttpsRedirectEnabled {
		e.Pre(middleware.HTTPSRedirect())
	}

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: cnf.AllowedOrigins,
		AllowMethods: []string{http.MethodGet, http.MethodPost},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType},
		MaxAge:       86400,
	}))

	// Limit to 5 RPS (burst 10) (only human should use this service)
	e.Use(middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{
				Rate:      5,
				Burst:     10,
				ExpiresIn: 1 * time.Minute,
			},
		),
		IdentifierExtractor: func(ctx echo.Context) (string, error) {
			// Header.Get returns only the first field; a trusted proxy that
			// APPENDS its entry as a second X-Forwarded-For field would be
			// invisible to the walk, leaving the attacker-controlled first
			// field in charge. Combine every field into one chain.
			return trustedClientIP(ctx.Request().RemoteAddr,
				strings.Join(ctx.Request().Header.Values(echo.HeaderXForwardedFor), ","), cnf.TrustedProxies)
		},
		DenyHandler: func(ctx echo.Context, identifier string, err error) error {
			return ctx.JSON(http.StatusTooManyRequests, map[string]string{
				"error": "rate limit exceeded",
			})
		},
		// Echo routes IdentifierExtractor errors here, not to DenyHandler;
		// the default ErrorHandler would answer 403 with the raw error.
		// An unusable client identifier must fail closed with the same
		// constant 429 response as an exhausted bucket.
		ErrorHandler: func(ctx echo.Context, err error) error {
			return ctx.JSON(http.StatusTooManyRequests, map[string]string{
				"error": "rate limit exceeded",
			})
		},
	}))

	// Keep the previous JSON access log format while moving off deprecated Logger middleware.
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		Skipper: func(c echo.Context) bool {
			return c.Path() == "/health"
		},
		LogRemoteIP:      true,
		LogHost:          true,
		LogMethod:        true,
		LogURI:           true,
		LogUserAgent:     true,
		LogStatus:        true,
		LogError:         true,
		LogLatency:       true,
		LogContentLength: true,
		LogResponseSize:  true,
		LogRequestID:     true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			logEntry := map[string]any{
				"time":          v.StartTime.UTC().Format(time.RFC3339Nano),
				"id":            v.RequestID,
				"remote_ip":     v.RemoteIP,
				"host":          v.Host,
				"method":        v.Method,
				"uri":           redactTokens(v.URI),
				"user_agent":    v.UserAgent,
				"status":        v.Status,
				"error":         "",
				"latency":       v.Latency.Nanoseconds(),
				"latency_human": v.Latency.String(),
				"bytes_in":      v.ContentLength,
				"bytes_out":     v.ResponseSize,
			}

			if v.Error != nil {
				logEntry["error"] = v.Error.Error()
			}

			payload, err := json.Marshal(logEntry)
			if err != nil {
				return err
			}

			payload = append(payload, '\n')
			_, err = e.Logger.Output().Write(payload)
			return err
		},
	}))

	e.Use(middleware.SecureWithConfig(middleware.SecureConfig{
		XSSProtection:         "1; mode=block",
		ContentTypeNosniff:    "nosniff",
		XFrameOptions:         "DENY",
		HSTSMaxAge:            31536000,
		HSTSPreloadEnabled:    true,
		ContentSecurityPolicy: "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; frame-ancestors 'none'",
	}))

	e.Use(middleware.BodyLimit("50M"))

	e.Use(middleware.Recover())
}

// setupRoutes registers all HTTP endpoints and static file routes.
// API endpoints: GET/POST /secret (secret management), ANY /health (health check), GET / (redirect).
// Static routes: /msg and /getmsg (HTML pages), /static (assets), /robots.txt (SEO).
func setupRoutes(e *echo.Echo, handlers *SecretHandlers) {
	e.GET("/", redirectHandler)

	e.File("/robots.txt", "static/robots.txt")

	e.Any("/health", healthHandler)

	e.GET("/secret", handlers.GetMsgHandler)
	e.POST("/secret", handlers.CreateMsgHandler)

	e.File("/msg", "static/index.html")

	e.File("/getmsg", "static/getmsg.html")

	e.Static("/static", "static")
}
