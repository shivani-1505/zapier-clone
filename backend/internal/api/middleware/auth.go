// backend/internal/api/middleware/auth.go
package middleware

import (
	"net/http"
	"strings"
)

// AuthMiddleware handles API authentication
type AuthMiddleware struct{}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware() *AuthMiddleware {
	return &AuthMiddleware{}
}

// Middleware validates API authentication
func (m *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// For webhook endpoints, skip authentication
		if strings.HasPrefix(r.URL.Path, "/api/webhooks/") {
			next.ServeHTTP(w, r)
			return
		}

		// For Slack endpoints, use token validation (simplified for this example)
		if strings.HasPrefix(r.URL.Path, "/api/slack/") {
			next.ServeHTTP(w, r)
			return
		}

		// Check for authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Simple token validation - in a real app, you'd have a more robust system
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
			return
		}

		// Extract the token
		token := strings.TrimPrefix(authHeader, "Bearer ")

		// Validate the token (this is a simplified example)
		if token != "valid-api-key-would-go-here" {
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
			return
		}

		// If authentication passed, call the next handler
		next.ServeHTTP(w, r)
	})
}
