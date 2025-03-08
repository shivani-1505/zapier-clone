package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/shivani-1505/zapier-clone/apps/gmail"
	"github.com/shivani-1505/zapier-clone/apps/slack"
	"github.com/shivani-1505/zapier-clone/internal/auth"
	"github.com/shivani-1505/zapier-clone/internal/database"
)

// SetupRoutes configures all routes for the application
func SetupRoutes(mux *http.ServeMux) {
	// Slack webhook endpoints
	mux.HandleFunc("/api/slack/events", slack.SlackMessageListener)
	mux.HandleFunc("/slack/events", slack.SlackMessageListener)

	// Auth endpoints
	mux.HandleFunc("/api/auth/credentials", auth.HandleSaveCredentials)

	// Email endpoints
	mux.HandleFunc("/api/email/test", auth.HandleTestSMTP)

	// Add a simple email sending endpoint
	mux.HandleFunc("/api/email/send", handleEmailSend)

	// Health check
	mux.HandleFunc("/health", handleHealthCheck)

	// Add debug endpoint to check stored integrations
	mux.HandleFunc("/api/debug/integrations", handleDebugIntegrations)

	// Add credentials check endpoint
	mux.HandleFunc("/api/debug/credentials", handleDebugCredentials)
}

// Handler functions

func handleEmailSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		UserID  string `json:"user_id"`
		To      string `json:"to"`
		Subject string `json:"subject"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if req.UserID == "" || req.To == "" {
		http.Error(w, "UserID and recipient (To) are required", http.StatusBadRequest)
		return
	}
	err := gmail.SendEmail(req.UserID, req.To, req.Subject, req.Message)
	if err != nil {
		log.Printf("Error sending email: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Email sent successfully",
	})
}

func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	// Check database connection
	if err := database.CheckDatabaseConnection(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(fmt.Sprintf("Database connection error: %v", err)))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK - Service is healthy"))
}

func handleDebugIntegrations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	allIntegrations := database.GetAllIntegrations()
	w.Write([]byte(fmt.Sprintf("Found %d integrations: %v",
		len(allIntegrations), allIntegrations)))
}

func handleDebugCredentials(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "user_id parameter is required", http.StatusBadRequest)
		return
	}
	creds, err := auth.CredManager.GetCredentials(userID)
	if err != nil {
		http.Error(w, fmt.Sprintf("No credentials found: %v", err), http.StatusNotFound)
		return
	}
	// Create a safe version of credentials that doesn't expose passwords
	// Convert boolean expressions to string values
	hasAppPassword := "false"
	if creds.GmailAppPassword != "" {
		hasAppPassword = "true"
	}
	hasSlackToken := "false"
	if creds.SlackBotToken != "" {
		hasSlackToken = "true"
	}
	safeCreds := map[string]string{
		"user_id":          creds.UserID,
		"gmail_account":    creds.GmailAccount,
		"has_app_password": hasAppPassword,
		"has_slack_token":  hasSlackToken,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(safeCreds)
}
