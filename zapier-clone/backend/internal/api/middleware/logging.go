// backend/internal/api/middleware/logging.go
package middleware

import (
	"log"
	"net/http"
	"time"
)

// LoggingMiddleware logs request information
type LoggingMiddleware struct{}

// NewLoggingMiddleware creates a new logging middleware
func NewLoggingMiddleware() *LoggingMiddleware {
	return &LoggingMiddleware{}
}

// Middleware logs information about the request
func (m *LoggingMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Log the request details
		log.Printf("Request started: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

		// Call the next handler
		next.ServeHTTP(w, r)

		// Log the request completion
		log.Printf("Request completed: %s %s in %v", r.Method, r.URL.Path, time.Since(start))
	})
}
