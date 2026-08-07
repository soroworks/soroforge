package api

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
)

// minTokenLength is the shortest API token the server will accept.
//
// This endpoint can deploy contracts with a live signing key, so a token short
// enough to guess is not meaningfully different from no token at all. The limit
// is a floor, not advice: use a long random value.
const minTokenLength = 16

// BearerAuth rejects requests without a matching bearer token.
//
// The comparison is constant-time. A naive == leaks how much of the token
// matched through response timing, which is enough to recover a secret one byte
// at a time given enough requests.
func BearerAuth(token string) func(http.Handler) http.Handler {
	expected := []byte(token)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			presented, ok := bearerToken(r)
			if !ok {
				unauthorized(w, "missing bearer token")
				return
			}

			if subtle.ConstantTimeCompare([]byte(presented), expected) != 1 {
				unauthorized(w, "invalid bearer token")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// bearerToken extracts the token from an Authorization header.
func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", false
	}

	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}

	token := strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return "", false
	}
	return token, true
}

// validateToken checks a token before the server starts listening.
//
// The server refuses to boot on failure rather than starting in an open state.
// An HTTP endpoint that deploys contracts using a signing key should never be
// reachable without authentication, and "we'll put it behind a proxy" is a
// promise that outlives the person who made it.
func validateToken(token string) error {
	if token == "" {
		return fmt.Errorf(
			"no API token configured: set SOROFORGE_API_TOKEN to a long random value. " +
				"The HTTP API can deploy and upgrade contracts with the configured signing key, " +
				"so it will not start unauthenticated")
	}
	if len(token) < minTokenLength {
		return fmt.Errorf(
			"SOROFORGE_API_TOKEN is %d characters; use at least %d "+
				"(e.g. `openssl rand -hex 32`)", len(token), minTokenLength)
	}
	return nil
}

func unauthorized(w http.ResponseWriter, detail string) {
	// WWW-Authenticate tells a client how to authenticate, per RFC 7235.
	w.Header().Set("WWW-Authenticate", `Bearer realm="soroforge"`)
	writeError(w, http.StatusUnauthorized, detail)
}
