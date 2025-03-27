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
	routes "github.com/shivani-1505/zapier-clone/backend/internal/api"
	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/jira"
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

	jiraClient := jira.NewClient(
		getEnv("JIRA_URL", "https://your-domain.atlassian.net"),
		getEnv("JIRA_EMAIL", "your-email@example.com"),
		getEnv("JIRA_API_TOKEN", "your-api-token"),
		getEnv("JIRA_PROJECT_KEY", "AUDIT"),
	)

	// Initialize risk-jira mapping
	riskJiraMapping, err := jira.NewRiskJiraMapping("./data")
	if err != nil {
		log.Printf("Warning: Failed to initialize risk-jira mapping: %v", err)
		// Create an empty mapping as fallback
		riskJiraMapping = &jira.RiskJiraMapping{
			RiskIDToJiraKey: make(map[string]string),
			JiraKeyToRiskID: make(map[string]string),
		}
	}

	// Create the risk handler with all dependencies
	riskHandler := servicenow.NewRiskHandler(
		serviceNowClient,
		slackClient,
		jiraClient,
		riskJiraMapping,
	)

	// Create the incident handler with dependencies
	incidentHandler := servicenow.NewIncidentHandler(
		serviceNowClient,
		slackClient,
		jiraClient,
	)

	// Setup API routes - use the package name you've set in routes.go
	routes.SetupRoutes(r, serviceNowClient, slackClient, jiraClient, riskHandler, incidentHandler)

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
