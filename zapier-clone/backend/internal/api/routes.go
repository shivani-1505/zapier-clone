// backend/internal/api/routes.go
package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/shivani-1505/zapier-clone/backend/internal/api/handlers"
	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/jira"
	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/servicenow"
	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/slack"
	"github.com/shivani-1505/zapier-clone/backend/internal/reporting"
)

// SetupRoutes configures all the API routes for the application
func SetupRoutes(r *mux.Router, serviceNowClient *servicenow.Client, slackClient *slack.Client, jiraClient *jira.Client, riskHandler *servicenow.RiskHandler, incidentHandler *servicenow.IncidentHandler) {
	// Create handlers
	serviceNowWebhookHandler := handlers.NewServiceNowWebhookHandler(
		serviceNowClient,
		slackClient,
		jiraClient,
	)
	slackInteractionHandler := handlers.NewSlackInteractionHandler(
		serviceNowClient,
		slackClient,
		jiraClient,
		incidentHandler,
	)
	slackCommandHandler := handlers.NewSlackCommandHandler(
		serviceNowClient,
		slackClient,
		jiraClient,
		riskHandler,
	)
	jiraWebhookHandler := handlers.NewJiraWebhookHandler(
		serviceNowClient,
		slackClient,
		jiraClient,
	)

	// ServiceNow webhook endpoints
	r.HandleFunc("/api/webhooks/servicenow", serviceNowWebhookHandler.HandleWebhook).Methods("POST")

	// Slack interaction endpoints
	r.HandleFunc("/api/slack/interactions", slackInteractionHandler.HandleInteraction).Methods("POST")

	// Slack command endpoints
	r.HandleFunc("/api/slack/commands", slackCommandHandler.HandleCommand).Methods("POST")
	r.HandleFunc("/api/slack/interaction", slackInteractionHandler.HandleInteraction).Methods("POST")

	// Jira webhook endpoints
	r.HandleFunc("/api/webhooks/jira", jiraWebhookHandler.HandleWebhook).Methods("POST")

	// Dashboard endpoint
	r.HandleFunc("/api/dashboard", func(w http.ResponseWriter, r *http.Request) {
		data, err := reporting.GetDashboardData(jiraClient)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to fetch dashboard data: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(data); err != nil {
			http.Error(w, fmt.Sprintf("Error encoding response: %v", err), http.StatusInternalServerError)
		}
	}).Methods("GET")

	// Health check endpoint
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")

	// Incident-related endpoints
	r.HandleFunc("/api/incidents/notify", func(w http.ResponseWriter, r *http.Request) {
		// Extract incident details from the request
		var incident servicenow.Incident
		if err := json.NewDecoder(r.Body).Decode(&incident); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Handle the new incident
		messageTS, err := incidentHandler.HandleNewIncident(incident)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error handling incident: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message_ts": messageTS})
	}).Methods("POST")

	r.HandleFunc("/api/incidents/{id}/update", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		incidentID := vars["id"]

		var update struct {
			ChannelID  string `json:"channel_id"`
			ThreadTS   string `json:"thread_ts"`
			UserID     string `json:"user_id"`
			UpdateText string `json:"update_text"`
		}

		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		err := incidentHandler.HandleIncidentUpdate(
			incidentID,
			update.ChannelID,
			update.ThreadTS,
			update.UserID,
			update.UpdateText,
		)

		if err != nil {
			http.Error(w, fmt.Sprintf("Error updating incident: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("POST")

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
                
                <h2>Jira Webhooks</h2>
                <div class="endpoint">
                    <span class="method">POST</span> /api/webhooks/jira
                    <p>Endpoint for receiving webhooks from Jira.</p>
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
