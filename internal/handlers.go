package internal

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
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

	h := ctx.Response().Header()
	h.Set("Cache-Control", "no-store")
	return ctx.JSON(http.StatusOK, r)
}

// healthHandler provides a simple health check endpoint.
// Returns HTTP 200 OK when the application is running.
func healthHandler(ctx echo.Context) error {
	return ctx.String(http.StatusOK, http.StatusText(http.StatusOK))
}

// redirectHandler redirects the root path to the message creation page,
// preserving the query string so links like /?lang=fr survive the hop.
func redirectHandler(ctx echo.Context) error {
	target := "/msg"
	if qs := ctx.Request().URL.RawQuery; qs != "" {
		target += "?" + qs
	}
	return ctx.Redirect(http.StatusPermanentRedirect, target)
}

// supportedLanguages holds the UI language codes. It is initialized at
// startup from the locales directory — the single source of truth — so
// adding a locale file (plus regenerating locales-manifest.json for the
// client) is all that is needed to support a new language.
var supportedLanguages []string

// InitSupportedLanguages derives the supported UI languages from the *.json
// files in dir. It must be called once at startup, before serving requests.
func InitSupportedLanguages(dir string) error {
	langs, err := loadSupportedLanguages(dir)
	if err != nil {
		return err
	}
	supportedLanguages = langs
	return nil
}

// loadSupportedLanguages lists the locale JSON files in dir, sorted.
func loadSupportedLanguages(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading locales directory %s: %w", dir, err)
	}
	var langs []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		langs = append(langs, strings.TrimSuffix(name, ".json"))
	}
	if len(langs) == 0 {
		return nil, fmt.Errorf("no locale files found in %s", dir)
	}
	sort.Strings(langs)
	return langs, nil
}

// isValidLanguage checks if the provided language code is supported.
func isValidLanguage(lang string) bool {
	return slices.Contains(supportedLanguages, lang)
}

func addToVaryHeader(h http.Header, value string) {
	existing := h.Get("Vary")
	if existing == "" {
		h.Set("Vary", value)
		return
	}

	for _, v := range strings.Split(existing, ",") {
		if strings.TrimSpace(v) == value {
			// Value already present, nothing to do.
			return
		}
	}

	h.Set("Vary", existing+", "+value)
}

// primaryLanguageTag normalizes a language tag to its lowercase primary
// subtag (e.g. "fr-CA" -> "fr"). Returns "" for empty, wildcard or otherwise
// regionless-unusable tags.
func primaryLanguageTag(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if i := strings.IndexAny(tag, "-_"); i != -1 {
		tag = tag[:i]
	}
	if tag == "*" {
		return ""
	}
	return tag
}

// parseAcceptLanguage extracts the supported language with the highest
// q-value from an Accept-Language header value (e.g. "fr;q=0.1,en;q=0.9").
// Tags without an explicit q default to 1.0; "*" entries are ignored.
// Returns "" when no supported language is listed.
func parseAcceptLanguage(header string) string {
	bestLang := ""
	bestQ := 0.0
	for _, entry := range strings.Split(header, ",") {
		parts := strings.Split(entry, ";")
		lang := primaryLanguageTag(parts[0])
		if !isValidLanguage(lang) {
			continue
		}
		q := 1.0
		for _, param := range parts[1:] {
			param = strings.TrimSpace(param)
			if strings.HasPrefix(param, "q=") {
				if parsed, err := strconv.ParseFloat(param[2:], 64); err == nil {
					q = parsed
				}
			}
		}
		if q > bestQ {
			bestLang = lang
			bestQ = q
		}
	}
	return bestLang
}

// resolveLanguage picks the response language following the same order as
// the client-side detectLanguage() in web/static/utils.js: explicit ?lang=
// param first (region subtags normalized away, e.g. fr-CA -> fr), then
// Accept-Language (q-value weighted), then English. An unsupported ?lang
// value falls through to the header instead of shadowing it.
func resolveLanguage(langParam, acceptLanguage string) string {
	if lang := primaryLanguageTag(langParam); isValidLanguage(lang) {
		return lang
	}
	if lang := parseAcceptLanguage(acceptLanguage); lang != "" {
		return lang
	}
	return "en"
}

// Cache-Control policies for the static cache tiers. shortCacheControl is
// also shared by the HTML pages, so a deploy's fresh HTML never pairs with
// stale assets or translations.
const (
	shortCacheControl = "public, max-age=300, must-revalidate"
	iconsCacheControl = "public, max-age=86400, must-revalidate"
	fontsCacheControl = "public, max-age=604800, must-revalidate"
)

// htmlHandler serves an HTML file with language preference handling.
// cacheControl picks the Cache-Control value for each request, since some
// pages need per-URL decisions (see getmsgHTMLCache).
func htmlHandler(ctx echo.Context, path string, cacheControl func(echo.Context) string) error {
	lang := resolveLanguage(ctx.QueryParam("lang"), ctx.Request().Header.Get("Accept-Language"))

	h := ctx.Response().Header()
	h.Set("Content-Language", lang)
	h.Set("Cache-Control", cacheControl(ctx))

	// Content-Language is derived from Accept-Language, so caches must vary
	// on it. Vary: Accept-Encoding is handled by the gzip middleware.
	addToVaryHeader(h, "Accept-Language")

	return ctx.File(path)
}

// publicHTMLCache caches HTML pages publicly for 5 minutes.
func publicHTMLCache(echo.Context) string {
	return shortCacheControl
}

// getmsgHTMLCache disables storage for token-bearing URLs: the query string
// carries a one-time secret token that must not land in shared caches.
func getmsgHTMLCache(ctx echo.Context) string {
	if ctx.QueryParam("token") != "" {
		return "no-store, private"
	}
	return shortCacheControl
}

// indexHandler serves the main message creation HTML page.
func indexHandler(ctx echo.Context) error {
	return htmlHandler(ctx, "static/index.html", publicHTMLCache)
}

// getmsgHandler serves the message retrieval HTML page.
func getmsgHandler(ctx echo.Context) error {
	return htmlHandler(ctx, "static/getmsg.html", getmsgHTMLCache)
}

// getCleanedPath sanitizes and validates the requested static file path.
func getCleanedPath(ctx echo.Context) (string, error) {
	// Get URL path (without query string)
	urlPath := ctx.Request().URL.Path

	// Remove leading slash and clean
	path := filepath.Clean(strings.TrimPrefix(urlPath, "/"))

	// Security: ensure path starts with "static/" after cleaning
	if !strings.HasPrefix(path, "static/") && path != "static" {
		return "", echo.NewHTTPError(http.StatusForbidden, "access denied")
	}

	return path, nil
}

// cacheHandler returns a static-file handler applying the given Cache-Control
// policy, so each route's cache tier is visible next to its registration in
// the route table.
func cacheHandler(cacheControl string) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		path, err := getCleanedPath(ctx)
		if err != nil {
			return err
		}

		// Check file existence before setting cache headers to avoid caching error responses
		if stat, err := os.Stat(path); err != nil || stat.IsDir() {
			return echo.NewHTTPError(http.StatusNotFound, "file not found")
		}

		h := ctx.Response().Header()

		if strings.HasSuffix(path, ".js") {
			h.Set("Content-Type", "application/javascript; charset=utf-8")
		} else if strings.HasSuffix(path, ".css") {
			h.Set("Content-Type", "text/css; charset=utf-8")
		} else if strings.HasSuffix(path, ".json") {
			h.Set("Content-Type", "application/json")
		}

		h.Set("Cache-Control", cacheControl)
		return ctx.File(path)
	}
}
