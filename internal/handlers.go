package internal

import (
	"encoding/base64"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

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

// newSecretHandlers creates a new SecretHandlers instance with the provided storage backend.
func newSecretHandlers(s SecretMsgStorer) *SecretHandlers {
	return &SecretHandlers{s}
}

// CreateMsgHandler handles POST requests to create a new self-destructing secret message.
// It accepts form data with 'msg' (required), 'ttl' (optional time-to-live), and 'file' (optional file upload).
// Files are base64 encoded before storage. Maximum file size is 50MB (enforced by middleware).
// Returns a JSON response with token(s) for retrieving the message and/or file.
func (s SecretHandlers) CreateMsgHandler(ctx echo.Context) error {
	var tr TokenResponse

	// Get TTL (if any)
	ttl := ctx.FormValue("ttl")

	// Upload file if any
	file, err := ctx.FormFile("file")
	if err == nil {
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
	msg := ctx.FormValue("msg")
	tr.Token, err = s.store.Store(msg, ttl)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return ctx.JSON(http.StatusOK, tr)
}

// GetMsgHandler handles GET requests to retrieve a self-destructing secret message.
// Accepts a 'token' query parameter. The message is deleted from Vault after retrieval,
// making it accessible only once. Returns a JSON response with the message content.
func (s SecretHandlers) GetMsgHandler(ctx echo.Context) error {
	m, err := s.store.Get(ctx.QueryParam("token"))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
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
