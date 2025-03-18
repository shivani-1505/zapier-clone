// backend/internal/api/routes.go
package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/shivani-1505/zapier-clone/backend/internal/api/handlers"
	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/servicenow"
	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/slack"
)

// SetupRoutes configures all the API routes for the application
func SetupRoutes(r *mux.Router, serviceNowClient *servicenow.Client, slackClient *slack.Client) {
	// Create handlers
	serviceNowWebhookHandler := handlers.NewServiceNowWebhookHandler(serviceNowClient, slackClient)
	slackInteractionHandler := handlers.NewSlackInteractionHandler(serviceNowClient, slackClient)
	slackCommandHandler := handlers.NewSlackCommandHandler(serviceNowClient, slackClient)

	// ServiceNow webhook endpoints
	r.HandleFunc("/api/webhooks/servicenow", serviceNowWebhookHandler.HandleWebhook).Methods("POST")

	// Slack interaction endpoints
	r.HandleFunc("/api/slack/interactions", slackInteractionHandler.HandleInteraction).Methods("POST")

	// Slack command endpoints
	r.HandleFunc("/api/slack/commands", slackCommandHandler.HandleCommand).Methods("POST")

	// Health check endpoint
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")

	// API documentation endpoint
	r.HandleFunc("/api/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<!DOCTYPE html>
			<html>
			<head>
				<title>GRC Integration API Documentation</title>
				<style>
					body { font-family: Arial, sans-serif; margin: 40px; }
					h1 { color: #333; }
					h2 { color: #666; margin-top: 30px; }
					.endpoint { background: #f5f5f5; padding: 10px; margin: 10px 0; border-radius: 4px; }
					.method { font-weight: bold; color: #0066cc; }
				</style>
			</head>
			<body>
				<h1>GRC Integration API Documentation</h1>
				<p>This page documents the available API endpoints for the GRC Integration service.</p>
				
				<h2>ServiceNow Webhooks</h2>
				<div class="endpoint">
					<span class="method">POST</span> /api/webhooks/servicenow
					<p>Endpoint for receiving webhooks from ServiceNow GRC.</p>
				</div>
				
				<h2>Slack Interactions</h2>
				<div class="endpoint">
					<span class="method">POST</span> /api/slack/interactions
					<p>Endpoint for handling Slack interactive components.</p>
				</div>
				
				<h2>Slack Commands</h2>
				<div class="endpoint">
					<span class="method">POST</span> /api/slack/commands
					<p>Endpoint for handling Slack slash commands.</p>
				</div>
				
				<h2>Health Check</h2>
				<div class="endpoint">
					<span class="method">GET</span> /health
					<p>Endpoint for monitoring service health.</p>
				</div>
			</body>
			</html>
		`))
	}).Methods("GET")
}
