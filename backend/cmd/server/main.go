package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/shivani-1505/zapier-clone/api"
	"github.com/shivani-1505/zapier-clone/apps/gmail"
	"github.com/shivani-1505/zapier-clone/apps/slack"
	"github.com/shivani-1505/zapier-clone/internal/auth"
	"github.com/shivani-1505/zapier-clone/internal/database"
)

// Add recovery middleware to prevent server crashes from panics
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("PANIC RECOVERED: %v\nStack trace: %s", err, debug.Stack())
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Add request logging middleware
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("→ %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
		log.Printf("← %s %s completed in %v", r.Method, r.URL.Path, time.Since(start))
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow requests from multiple origins for flexibility
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "3600") // Cache preflight for 1 hour
		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	log.Println("Starting AuditCue Integration Service...")

	// Initialize credential manager
	log.Println("Initializing credential manager...")
	if err := auth.InitCredentialsManager(); err != nil {
		log.Fatalf("Failed to initialize credentials manager: %v", err)
	}

	// Initialize PostgreSQL-based integration store
	log.Println("Initializing PostgreSQL database...")
	if err := database.InitIntegrationStore(); err != nil {
		log.Fatalf("Failed to initialize PostgreSQL: %v", err)
	}

	// Set up function callbacks to break circular dependencies
	gmail.GetCredentials = func(userID string) (gmail.Credentials, error) {
		authCreds, err := auth.CredManager.GetCredentials(userID)
		if err != nil {
			return gmail.Credentials{}, err
		}

		return gmail.Credentials{
			GmailAccount:     authCreds.GmailAccount,
			GmailAppPassword: authCreds.GmailAppPassword,
		}, nil
	}

	// Set up functions to break circular dependencies
	slack.GetSlackToken = auth.CredManager.GetSlackToken
	slack.SendEmailWithFallback = gmail.SendEmailWithFallback

	// Setup graceful shutdown to close database
	quit := setupGracefulShutdown()

	// Create router
	mux := http.NewServeMux()

	// Setup all routes
	api.SetupRoutes(mux)

	// Add static file server for frontend
	fs := http.FileServer(http.Dir("./frontend/dist"))
	mux.Handle("/assets/", fs)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Skip API paths
		if r.URL.Path == "/api/auth/credentials" ||
			r.URL.Path == "/api/slack/events" ||
			r.URL.Path == "/health" ||
			r.URL.Path == "/api/email/test" ||
			r.URL.Path == "/api/email/send" ||
			r.URL.Path == "/api/debug/integrations" ||
			r.URL.Path == "/api/debug/credentials" {
			return // Let the API routes handle these
		}

		// Check if the path exists as a static file
		path := filepath.Join("./frontend/dist", r.URL.Path)
		_, err := os.Stat(path)
		if err == nil {
			fs.ServeHTTP(w, r)
			return
		}

		// For client-side routing, serve index.html
		http.ServeFile(w, r, "./frontend/dist/index.html")
	})

	// Chain all middleware
	handler := recoveryMiddleware(loggingMiddleware(corsMiddleware(mux)))

	// Determine port
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine to allow for graceful shutdown
	go func() {
		log.Printf("Server ready! Listening on port %s...", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-quit

	// Create a deadline context for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Shutdown the server gracefully
	log.Println("Shutting down server...")
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server gracefully stopped")
}

// setupGracefulShutdown ensures database is properly closed on application shutdown
func setupGracefulShutdown() chan struct{} {
	quit := make(chan struct{})
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("Shutdown signal received...")
		database.CloseDB()
		close(quit)
	}()
	return quit
}
