// Package internal provides HTTP server setup and request handlers for the sup3rS3cretMes5age application.
// It includes server lifecycle management with graceful shutdown, middleware configuration,
// route setup, and integration with HashiCorp Vault for secure message storage.
package internal

import (
	"context"
	"crypto/tls"
	"net/http"
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
			GetCertificate:           autoTLSManager.GetCertificate,
			NextProtos:               []string{acme.ALPNProto},
			MinVersion:               tls.VersionTLS12,
			CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.X25519, tls.CurveP256},
			PreferServerCipherSuites: true,
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
			return ctx.RealIP(), nil
		},
		DenyHandler: func(ctx echo.Context, identifier string, err error) error {
			return ctx.JSON(http.StatusTooManyRequests, map[string]string{
				"error": "rate limit exceeded",
			})
		},
	}))

	// do not log the /health endpoint
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Skipper: func(c echo.Context) bool {
			return c.Path() == "/health"
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

	// API secret endpoints
	e.GET("/secret", handlers.GetMsgHandler)
	e.POST("/secret", handlers.CreateMsgHandler)

	// HTML page handlers
	e.GET("/msg", indexHandler)
	e.GET("/getmsg", getmsgHandler)

	// Static assets with tiered caching
	static := e.Group("/static")
	staticMethods := []string{"GET", "HEAD"}
	static.Match(staticMethods, "/fonts/*", fontCacheHandler)
	static.Match(staticMethods, "/icons/*", longCacheHandler)
	static.Match(staticMethods, "/locales/*", mediumCacheHandler)
	static.Match(staticMethods, "/*", shortCacheHandler)
}
