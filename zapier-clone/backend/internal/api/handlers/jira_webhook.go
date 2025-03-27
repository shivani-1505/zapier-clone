package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/jira"
	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/servicenow"
	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/slack"
)

// JiraWebhookHandler handles incoming webhooks from Jira
type JiraWebhookHandler struct {
	ServiceNowClient *servicenow.Client
	SlackClient      *slack.Client
	JiraClient       *jira.Client
	AuditHandler     *servicenow.AuditHandler
}

// NewJiraWebhookHandler creates a new Jira webhook handler
func NewJiraWebhookHandler(
	serviceNowClient *servicenow.Client,
	slackClient *slack.Client,
	jiraClient *jira.Client,
) *JiraWebhookHandler {
	return &JiraWebhookHandler{
		ServiceNowClient: serviceNowClient,
		SlackClient:      slackClient,
		JiraClient:       jiraClient,
		AuditHandler:     servicenow.NewAuditHandler(serviceNowClient, slackClient, jiraClient),
	}
}

// HandleWebhook processes incoming webhooks from Jira
func (h *JiraWebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	// Parse the incoming webhook payload
	var event jira.WebhookEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// Log the received webhook
	log.Printf("Received Jira webhook: %s", event.WebhookEvent)

	// Process the webhook asynchronously
	go h.processWebhook(&event)

	// Respond immediately to Jira
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"received"}`))
}

// processWebhook processes the webhook payload asynchronously
func (h *JiraWebhookHandler) processWebhook(event *jira.WebhookEvent) {
	// Handle different types of events
	switch event.WebhookEvent {
	case "jira:issue_updated":
		if err := h.AuditHandler.HandleJiraUpdate(event); err != nil {
			log.Printf("Error processing Jira issue update: %v", err)
		}
	case "jira:issue_created":
		log.Printf("Issue created: %s", event.Issue.Key)
	case "jira:issue_deleted":
		log.Printf("Issue deleted: %s", event.Issue.Key)
	case "comment_created", "comment_updated", "comment_deleted":
		if err := h.AuditHandler.HandleJiraUpdate(event); err != nil {
			log.Printf("Error processing Jira comment event: %v", err)
		}
	default:
		log.Printf("Unhandled Jira event type: %s", event.WebhookEvent)
	}
}
