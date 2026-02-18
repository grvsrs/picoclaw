// API authentication middleware — static bearer token.
//
// When gateway.api_key is non-empty in config, all API requests MUST carry:
//
//	Authorization: Bearer <api_key>
//
// or:
//
//	X-API-Key: <api_key>
//
// Exempt routes (no token required):
//   - GET /api/health
//   - GET /   (dashboard static files)
//
// WebSocket upgrade requests check the token in the query param as fallback:
//   wss://host/api/ws?token=<api_key>
//
// When api_key is empty (development mode), all requests are allowed through
// and a warning is logged once at startup.
package api

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/sipeed/picoclaw/pkg/logger"
)

// authMiddleware wraps a handler with bearer token checking.
// If apiKey is empty, the middleware is a pass-through (dev mode only —
// NewServer auto-generates a key so this branch should not be reached
// under normal operation).
func authMiddleware(apiKey string, next http.Handler) http.Handler {
	if apiKey == "" {
		logger.WarnC("auth", "API auth DISABLED — this should not happen; auto-keygen failed")
		return next
	}

	logger.InfoC("auth", "API bearer token auth ENABLED")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always allow health check and static dashboard (no token needed)
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// OPTIONS preflight — let CORS middleware handle it
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Extract token from request
		token := extractToken(r)

		if !tokenValid(token, apiKey) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="picoclaw"`)
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "unauthorized — bearer token required",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// extractToken pulls the bearer token from Authorization header,
// X-API-Key header, or ?token= query param (for WebSocket upgrades).
func extractToken(r *http.Request) string {
	// Authorization: Bearer <token>
	if auth := r.Header.Get("Authorization"); auth != "" {
		if after, ok := strings.CutPrefix(auth, "Bearer "); ok {
			return strings.TrimSpace(after)
		}
	}

	// X-API-Key: <token>
	if key := r.Header.Get("X-API-Key"); key != "" {
		return strings.TrimSpace(key)
	}

	// ?token=<token> — WebSocket safe
	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}

	return ""
}

// tokenValid does a constant-time comparison to prevent timing attacks.
func tokenValid(provided, expected string) bool {
	if provided == "" || expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

// isPublicPath returns true for paths that never require authentication.
func isPublicPath(path string) bool {
	switch {
	case path == "/api/health":
		return true
	case path == "/" || strings.HasPrefix(path, "/assets/") || strings.HasSuffix(path, ".js") ||
		strings.HasSuffix(path, ".css") || strings.HasSuffix(path, ".ico") ||
		strings.HasSuffix(path, ".png") || strings.HasSuffix(path, ".svg"):
		return true
	default:
		return false
	}
}
