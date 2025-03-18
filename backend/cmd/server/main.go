// backend/cmd/server/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	routes "github.com/shivani-1505/zapier-clone/backend/internal/api" // Import with alias matching the package name
	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/servicenow"
	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/slack"
	"github.com/shivani-1505/zapier-clone/backend/internal/reporting"
)

func main() {
	// Initialize router
	r := mux.NewRouter()

	// Initialize clients
	serviceNowClient := servicenow.NewClient(
		getEnv("SERVICENOW_URL", "https://example.service-now.com"),
		getEnv("SERVICENOW_USERNAME", "admin"),
		getEnv("SERVICENOW_PASSWORD", "password"),
	)

	slackClient := slack.NewClient(
		getEnv("SLACK_API_TOKEN", "xoxb-your-token-here"),
	)

	// Setup API routes - use the package name you've set in routes.go
	routes.SetupRoutes(r, serviceNowClient, slackClient)

	// Initialize and start the report scheduler
	reportScheduler := reporting.NewReportScheduler(serviceNowClient, slackClient)
	reportScheduler.Start()
	defer reportScheduler.Stop()

	// Create server
	srv := &http.Server{
		Addr:         getEnv("SERVER_ADDR", ":8081"),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	// Gracefully shutdown
	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server gracefully stopped")
}

// Helper function to get environment variables with default fallback
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
