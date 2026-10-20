package internal

import "strings"

// This file holds the shared redaction primitives. One-time Vault tokens
// must never reach the logs: a logged token is a second copy of the secret,
// readable by anyone with log access before the first retrieval consumes
// it. Both the access-log URI scrubbing (server.go) and the error-text
// scrubbing (vault.go) build on these helpers, and both must agree on the
// placeholder and on what counts as token-bearing.

// redactedPlaceholder replaces every redacted token occurrence, in logs and
// in error messages alike, so a reviewer can tell a redacted secret from an
// empty or missing value.
const redactedPlaceholder = "REDACTED"

// isTokenParamName reports whether a query-parameter name is token-bearing
// (token, filetoken, …) and its value must never reach the logs.
func isTokenParamName(name string) bool {
	return strings.Contains(strings.ToLower(name), "token")
}

// redactTokensFromText replaces every occurrence of the given secrets with
// redactedPlaceholder, leaving the surrounding text (URL, error message)
// readable for debugging.
func redactTokensFromText(text string, tokens []string) string {
	for _, token := range tokens {
		text = strings.ReplaceAll(text, token, redactedPlaceholder)
	}
	return text
}
