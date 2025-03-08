package auth

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/shivani-1505/zapier-clone/apps/gmail"
	"github.com/shivani-1505/zapier-clone/apps/slack"
	"github.com/shivani-1505/zapier-clone/internal/database"
)

// HandleSaveCredentials processes saving new credentials
func HandleSaveCredentials(w http.ResponseWriter, r *http.Request) {
	log.Printf("HandleSaveCredentials called from: %s", r.RemoteAddr)
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req SaveUserCredentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	log.Printf("Received credentials save request for user: %s, team: %s", req.UserID, req.SlackTeamID)
	// Validate Slack token if provided
	if req.SlackBotToken != "" {
		if err := slack.ValidateSlackToken(req.SlackBotToken); err != nil {
			log.Printf("⚠️ Provided Slack token appears invalid: %v", err)
			// Continue anyway - we'll save it, but log the warning
		}
	}
	// Save credentials in the user credentials store
	userCreds := UserCredentials(req)
	if err := CredManager.SaveCredentials(userCreds); err != nil {
		log.Printf("Error saving credentials: %v", err)
		http.Error(w, "Error saving credentials", http.StatusInternalServerError)
		return
	}
	log.Printf("Credentials saved successfully for user: %s", req.UserID)
	// Register the integration mapping in the database
	if req.SlackTeamID != "" && req.SlackBotToken != "" {
		log.Printf("Registering integration for team: %s with token", req.SlackTeamID)
		if err := database.RegisterIntegration(req.SlackTeamID, req.UserID, req.SlackBotToken); err != nil {
			log.Printf("Error registering integration in database: %v", err)
			http.Error(w, "Error registering integration", http.StatusInternalServerError)
			return
		}
		log.Printf("Integration registered successfully in database: %s -> %s", req.SlackTeamID, req.UserID)
	} else if req.SlackTeamID != "" {
		// If we have team ID but no token, still register the integration but without token
		log.Printf("Registering integration for team: %s without token", req.SlackTeamID)
		if err := database.RegisterIntegration(req.SlackTeamID, req.UserID, ""); err != nil {
			log.Printf("Error registering integration in database: %v", err)
			http.Error(w, "Error registering integration", http.StatusInternalServerError)
			return
		}
	}
	// Redirect to success page
	http.Redirect(w, r, "/success", http.StatusSeeOther) // 303 See Other redirects to the target using GET
}

// HandleTestSMTP tests the SMTP connection
func HandleTestSMTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		UserID string `json:"user_id"`
		Email  string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.UserID == "" {
		http.Error(w, "UserID is required", http.StatusBadRequest)
		return
	}
	if req.Email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}
	err := gmail.SendEmail(req.UserID, req.Email, "SMTP Test", "This is a test email to verify SMTP configuration.")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp := map[string]string{
		"status":  "success",
		"message": "Test email sent successfully",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleTestSlack tests the Slack API connection
func HandleTestSlack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		UserID    string `json:"user_id"`
		ChannelID string `json:"channel_id"`
		TeamID    string `json:"team_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.UserID == "" {
		http.Error(w, "UserID is required", http.StatusBadRequest)
		return
	}
	if req.ChannelID == "" {
		http.Error(w, "ChannelID is required", http.StatusBadRequest)
		return
	}
	if req.TeamID == "" {
		// Try to get TeamID from user credentials as a fallback
		userCreds, err := CredManager.GetCredentials(req.UserID)
		if err != nil || userCreds.SlackTeamID == "" {
			log.Printf("⚠️ No TeamID provided and couldn't find it in user credentials")
			http.Error(w, "TeamID is required", http.StatusBadRequest)
			return
		}
		req.TeamID = userCreds.SlackTeamID
		log.Printf("Using TeamID %s from user credentials", req.TeamID)
	}
	// Test the Slack integration by trying to get channel members
	users, err := slack.GetSlackUsers(req.UserID, req.ChannelID, req.TeamID)
	if err != nil {
		http.Error(w, "Slack API test failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	resp := map[string]interface{}{
		"status":        "success",
		"message":       "Slack API test successful",
		"channel_users": len(users),
		"team_id":       req.TeamID,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleUpdateSlackToken allows updating just the Slack token for a team
func HandleUpdateSlackToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		TeamID     string `json:"team_id"`
		SlackToken string `json:"slack_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.TeamID == "" {
		http.Error(w, "TeamID is required", http.StatusBadRequest)
		return
	}
	if req.SlackToken == "" {
		http.Error(w, "SlackToken is required", http.StatusBadRequest)
		return
	}
	// Validate the token
	if err := slack.ValidateSlackToken(req.SlackToken); err != nil {
		log.Printf("⚠️ Provided Slack token appears invalid: %v", err)
		http.Error(w, "Invalid Slack token: "+err.Error(), http.StatusBadRequest)
		return
	}
	// Update the token
	err := database.UpdateSlackToken(req.TeamID, req.SlackToken)
	if err != nil {
		http.Error(w, "Failed to update Slack token: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// Try to find the user associated with this team
	userID, found := database.GetUserIDForTeam(req.TeamID)
	if found {
		// Get existing user credentials
		userCreds, err := CredManager.GetCredentials(userID)
		if err == nil {
			// Update the Slack token
			userCreds.SlackBotToken = req.SlackToken
			// Save updated credentials
			if err := CredManager.SaveCredentials(userCreds); err != nil {
				log.Printf("Error updating Slack token in credentials manager: %v", err)
				// Continue anyway since we already updated the integration store
			} else {
				log.Printf("Updated Slack token in credentials manager for user %s", userID)
			}
		}
	}
	resp := map[string]string{
		"status":  "success",
		"message": "Slack token updated successfully",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleSetFallbackToken sets up the fallback Slack token
func HandleSetFallbackToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		SlackToken string `json:"slack_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.SlackToken == "" {
		http.Error(w, "SlackToken is required", http.StatusBadRequest)
		return
	}
	// Validate the token
	if err := slack.ValidateSlackToken(req.SlackToken); err != nil {
		log.Printf("⚠️ Provided fallback Slack token appears invalid: %v", err)
		http.Error(w, "Invalid Slack token: "+err.Error(), http.StatusBadRequest)
		return
	}
	// Set up fallback
	fallbackTeamID := "FALLBACK_TEAM"
	fallbackUserID := "fallback-system-user"
	// Update the token in the integration store
	err := database.RegisterIntegration(fallbackTeamID, fallbackUserID, req.SlackToken)
	if err != nil {
		http.Error(w, "Failed to register fallback integration: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// Also update the credentials manager
	fallbackCreds := UserCredentials{
		UserID:           fallbackUserID,
		SlackBotToken:    req.SlackToken,
		GmailAccount:     "connectify.workflow@gmail.com",
		GmailAppPassword: "dvhv tmod qdzu jyrj",
	}
	if err := CredManager.SaveCredentials(fallbackCreds); err != nil {
		log.Printf("Warning: Failed to save fallback credentials in cred manager: %v", err)
		// Continue anyway since we already updated the integration store
	}
	resp := map[string]string{
		"status":  "success",
		"message": "Fallback Slack token set successfully",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
