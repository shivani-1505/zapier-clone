package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/shivani-1505/zapier-clone/apps/gmail"
	"github.com/shivani-1505/zapier-clone/internal/database"
)

var SendEmail func(userID, to, subject, messageText string) error

// Function variable for sending emails
var SendEmailWithFallback func(to, subject, messageText string) error

// SlackMessageListener processes Slack messages and verifies challenge requests
func SlackMessageListener(w http.ResponseWriter, r *http.Request) {
	requestID := fmt.Sprintf("req-%d", time.Now().UnixNano())
	log.Printf("[%s] Received Slack webhook request", requestID)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[%s] ❌ Error reading request body: %v", requestID, err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	// Verify URL challenge first
	var challenge SlackChallenge
	if err := json.Unmarshal(body, &challenge); err == nil && challenge.Type == "url_verification" {
		log.Printf("[%s] Processing URL verification challenge", requestID)
		response := map[string]string{"challenge": challenge.Challenge}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}
	var event SlackEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("[%s] ❌ JSON parsing error: %v", requestID, err)
		http.Error(w, "Invalid event JSON", http.StatusBadRequest)
		return
	}
	// Log only essential information
	log.Printf("[%s] Event type: %s, Team ID: %s", requestID, event.Type, event.TeamID)
	if event.Type == "event_callback" {
		if event.TeamID == "" {
			log.Printf("[%s] ❌ No team ID found in event", requestID)
			http.Error(w, "No team ID in event", http.StatusBadRequest)
			return
		}
		// Find the associated userID for this team
		userID, exists := database.GetUserIDForTeam(event.TeamID)
		if !exists {
			log.Printf("[%s] ❌ No integration found for team ID: %s", requestID, event.TeamID)
			http.Error(w, "Integration not found", http.StatusNotFound)
			return
		}
		log.Printf("[%s] Found integration for TeamID=%s -> UserID=%s", requestID, event.TeamID, userID)
		if event.Event.Type == "message" {
			if event.Event.BotID != "" ||
				event.Event.Subtype == "message_changed" ||
				event.Event.Subtype == "bot_message" {
				log.Printf("[%s] Skipping bot message or edit", requestID)
				w.WriteHeader(http.StatusOK)
				return
			}
			log.Printf("[%s] Processing message from user %s in channel %s",
				requestID, event.Event.User, event.Event.Channel)
			// Get user details for notification
			senderDetails := "unknown user"
			if email, err := GetUserEmail(context.Background(), userID, event.Event.User, event.TeamID); err == nil && email != "" {
				senderDetails = fmt.Sprintf("%s (%s)", event.Event.User, email)
				log.Printf("[%s] Sender identified as: %s", requestID, senderDetails)
			} else if err != nil {
				log.Printf("[%s] Error getting sender email: %v", requestID, err)
			}
			// Get all channel members
			log.Printf("[%s] 🔍 Attempting to retrieve member emails for channel %s...", requestID, event.Event.Channel)
			channelMembers, err := GetSlackUsers(userID, event.Event.Channel, event.TeamID)
			if err != nil {
				log.Printf("[%s] ❌ Error getting channel members: %v", requestID, err)
				// Fall back to the default recipient if we can't get channel members
				channelMembers = []string{"bshivani1505@gmail.com"}
				log.Printf("[%s] Using fallback recipient: %s", requestID, channelMembers[0])
			} else {
				// Log each retrieved email (up to a reasonable limit)
				log.Printf("[%s] ✅ Successfully retrieved %d channel member emails:", requestID, len(channelMembers))
				maxLogEmails := 10 // Limit number of emails to log
				if len(channelMembers) == 0 {
					log.Printf("[%s] ⚠️ WARNING: No channel member emails found", requestID)
				} else {
					for i, email := range channelMembers {
						if i < maxLogEmails {
							log.Printf("[%s]   - Recipient %d: %s", requestID, i+1, email)
						} else if i == maxLogEmails {
							log.Printf("[%s]   - ... and %d more (truncated from logs)", requestID, len(channelMembers)-maxLogEmails)
							break
						}
					}
				}
			}
			// If no channel members, use default recipient
			if len(channelMembers) == 0 {
				log.Printf("[%s] No channel members found, using default recipient", requestID)
				channelMembers = []string{"bshivani1505@gmail.com"}
			}
			// Create recipient list as comma-separated
			recipients := strings.Join(channelMembers, ",")
			log.Printf("[%s] 📧 Final recipient list contains %d emails", requestID, len(channelMembers))
			// Prepare email notification content
			subject := fmt.Sprintf("New Slack message in channel %s", event.Event.Channel)
			// Create a more detailed message body
			messageBody := fmt.Sprintf(
				"User %s sent a message in channel %s: \n\n%s\n\n"+
					"This notification was sent to %d channel members.",
				senderDetails,
				event.Event.Channel,
				event.Event.Text,
				len(channelMembers),
			)
			// Send email to all channel members
			err = gmail.SendEmailWithFallback(recipients, subject, messageBody)
			if err != nil {
				log.Printf("[%s] ❌ Error sending notification email: %v", requestID, err)
			} else {
				log.Printf("[%s] ✅ Notification email sent successfully to %d recipients",
					requestID, len(channelMembers))
			}
		}
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Event processed"))
	log.Printf("[%s] Request processing completed", requestID)
}
