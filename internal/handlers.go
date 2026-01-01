package internal

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

// tokenRegex matches valid Vault token formats for hv.sb and legacy tokens.
var tokenRegex = regexp.MustCompile(`^hv[sb]\.(?:[A-Za-z0-9]{24}|[A-Za-z0-9_-]{91,})$`)

// TokenResponse represents the API response when creating a new secret message.
// It includes a token for retrieving the message, and optional file token and name
// if a file was uploaded alongside the message.
type TokenResponse struct {
	// Token is the unique identifier for retrieving the secret message.
	Token string `json:"token"`
	// FileToken is the unique identifier for retrieving an uploaded file (optional).
	FileToken string `json:"filetoken,omitempty"`
	// FileName is the original name of the uploaded file (optional).
	FileName string `json:"filename,omitempty"`
}

// MsgResponse represents the API response when retrieving a secret message.
type MsgResponse struct {
	// Msg is the secret message content retrieved from Vault.
	Msg string `json:"msg"`
}

// SecretHandlers provides HTTP handler methods for creating and retrieving secret messages.
type SecretHandlers struct {
	// store is the backend storage implementation (Vault) for secret messages.
	store SecretMsgStorer
}

// NewSecretHandlers creates a new SecretHandlers instance with the provided storage backend.
func NewSecretHandlers(s SecretMsgStorer) *SecretHandlers {
	return &SecretHandlers{s}
}

// validateMsg checks if the provided message is non-empty and within size limits.
func validateMsg(msg string) error {
	if msg == "" {
		return fmt.Errorf("message is required")
	}

	// 1MB limit for text
	if len(msg) > 1*1024*1024 {
		return fmt.Errorf("message too large")
	}

	return nil
}

// isValidTTL checks if the provided TTL string is a valid duration between 1 minute and 7 days.
func isValidTTL(ttl string) bool {
	// Verify duration
	d, err := time.ParseDuration(ttl)
	if err != nil {
		return false
	}

	// validate duration length (between 1 minute and 7 days)
	if d < 1*time.Minute || d > 168*time.Hour {
		return false
	}
	return true
}

// validateFileUpload checks the uploaded file for size and filename validity.
func validateFileUpload(file *multipart.FileHeader) error {
	// Parse Content-Disposition to extract filename
	mediatype, params, err := mime.ParseMediaType(file.Header.Get("Content-Disposition"))
	if mediatype != "form-data" || err != nil {
		return fmt.Errorf("invalid file upload")
	}

	// Check file size
	if file.Size > 50*1024*1024 {
		return fmt.Errorf("file too large")
	}

	// Check filename for path traversal
	if strings.Contains(params["filename"], "..") ||
		strings.Contains(params["filename"], "/") ||
		strings.Contains(params["filename"], "\\") ||
		strings.Contains(file.Filename, "..") ||
		strings.Contains(file.Filename, "/") ||
		strings.Contains(file.Filename, "\\") {
		return fmt.Errorf("invalid filename")
	}

	return nil
}

// validateVaultToken checks the format of Vault-generated tokens
func validateVaultToken(token string) error {
	// Check token format
	if !tokenRegex.MatchString(token) {
		return fmt.Errorf("invalid token format: %s", token)
	}
	return nil
}

// CreateMsgHandler handles POST requests to create a new self-destructing secret message.
// It accepts form data with 'msg' (required), 'ttl' (optional time-to-live), and 'file' (optional file upload).
// Files are base64 encoded before storage. Maximum file size is 50MB (enforced by middleware).
// Returns a JSON response with token(s) for retrieving the message and/or file.
func (s SecretHandlers) CreateMsgHandler(ctx echo.Context) error {

	msg := ctx.FormValue("msg")
	if err := validateMsg(msg); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// Get TTL (if any)
	ttl := ctx.FormValue("ttl")
	if ttl != "" && !isValidTTL(ttl) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid TTL format")
	}

	var tr TokenResponse
	// Upload file if any
	file, err := ctx.FormFile("file")
	if err == nil {
		if err := validateFileUpload(file); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		src, err := file.Open()
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err)
		}
		defer func() { _ = src.Close() }()

		b, err := io.ReadAll(src)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err)
		}

		if len(b) > 0 {
			tr.FileName = file.Filename
			encodedFile := base64.StdEncoding.EncodeToString(b)

			filetoken, err := s.store.Store(encodedFile, ttl)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, err)
			}
			tr.FileToken = filetoken
		}
	}

	// Handle the secret message
	tr.Token, err = s.store.Store(msg, ttl)
	if err != nil {
		ctx.Logger().Errorf("Failed to store secret: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to store secret")
	}

	return ctx.JSON(http.StatusOK, tr)
}

// GetMsgHandler handles GET requests to retrieve a self-destructing secret message.
// Accepts a 'token' query parameter. The message is deleted from Vault after retrieval,
// making it accessible only once. Returns a JSON response with the message content.
func (s SecretHandlers) GetMsgHandler(ctx echo.Context) error {
	token := ctx.QueryParam("token")
	if err := validateVaultToken(token); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	m, err := s.store.Get(token)
	if err != nil {
		ctx.Logger().Errorf("Failed to retrieve secret: %v", err)
		return echo.NewHTTPError(http.StatusNotFound, "secret not found or already consumed")
	}
	r := &MsgResponse{
		Msg: m,
	}
	return ctx.JSON(http.StatusOK, r)
}

// healthHandler provides a simple health check endpoint.
// Returns HTTP 200 OK when the application is running.
func healthHandler(ctx echo.Context) error {
	return ctx.String(http.StatusOK, http.StatusText(http.StatusOK))
}

// redirectHandler redirects the root path to the message creation page.
func redirectHandler(ctx echo.Context) error {
	return ctx.Redirect(http.StatusPermanentRedirect, "/msg")
}

// isValidLanguage checks if the provided language code is supported.
func isValidLanguage(lang string) bool {
	validLanguages := []string{"en", "fr", "es", "de", "it"}
	for _, valid := range validLanguages {
		if valid == lang {
			return true
		}
	}
	return false
}

// htmlHandler serves HTML files with language preference handling.
func htmlHandler(ctx echo.Context, path string) error {
	// Check for language preference in cookie or header
	lang := ctx.QueryParam("lang")
	if lang == "" {
		lang = ctx.Request().Header.Get("Accept-Language")
		if lang != "" {
			// Extract primary language (e.g., "en-US,en;q=0.9" -> "en")
			lang = strings.Split(lang, ",")[0]
			lang = strings.Split(lang, "-")[0]
		}
	}

	// Set default language if none found
	if lang == "" || !isValidLanguage(lang) {
		lang = "en"
	}

	// Pass language to template context
	ctx.Response().Header().Set("Content-Language", lang)
	return ctx.File(path)
}

// indexHandler serves the main message creation HTML page.
func indexHandler(ctx echo.Context) error {
	return htmlHandler(ctx, "static/index.html")
}

// getmsgHandler serves the message retrieval HTML page.
func getmsgHandler(ctx echo.Context) error {
	return htmlHandler(ctx, "static/getmsg.html")
}
