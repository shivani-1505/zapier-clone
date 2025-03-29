// backend/internal/api/handlers/jira_webhook.go
package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/jira"
	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/servicenow"
	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/slack"
)

// JiraWebhookHandler handles incoming webhooks from Jira
type JiraWebhookHandler struct {
	ServiceNowClient   *servicenow.Client
	SlackClient        *slack.Client
	JiraClient         *jira.Client
	AuditHandler       *servicenow.AuditHandler
	VendorRiskHandler  *servicenow.VendorRiskHandler
	ControlTestHandler *servicenow.PolicyControlHandler
	RegulatoryHandler  *servicenow.RegulatoryChangeHandler
}

// NewJiraWebhookHandler creates a new Jira webhook handler
func NewJiraWebhookHandler(
	serviceNowClient *servicenow.Client,
	slackClient *slack.Client,
	jiraClient *jira.Client,
) *JiraWebhookHandler {
	vendorRiskHandler := servicenow.NewVendorRiskHandler(serviceNowClient, slackClient)
	vendorRiskHandler.SetJiraClient(jiraClient) // Set Jira client for vendor risk handler

	controlTestHandler := servicenow.NewPolicyControlHandler(serviceNowClient, slackClient)

	return &JiraWebhookHandler{
		ServiceNowClient:   serviceNowClient,
		SlackClient:        slackClient,
		JiraClient:         jiraClient,
		AuditHandler:       servicenow.NewAuditHandler(serviceNowClient, slackClient, jiraClient),
		VendorRiskHandler:  vendorRiskHandler,
		ControlTestHandler: controlTestHandler,
		RegulatoryHandler:  servicenow.NewRegulatoryChangeHandler(serviceNowClient, slackClient),
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
		h.handleIssueUpdated(event)
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

// handleIssueUpdated handles Jira issue updated events
// handleIssueUpdated handles Jira issue updated events
func (h *JiraWebhookHandler) handleIssueUpdated(event *jira.WebhookEvent) {
	// Check if issue is linked to ServiceNow via customfield
	if event.Issue == nil {
		log.Printf("Invalid issue structure in webhook event")
		return
	}

	// Get the ServiceNow ID from custom field
	snID, ok := event.Issue.Fields.CustomFields["customfield_servicenow_id"].(string)
	if !ok || snID == "" {
		log.Printf("No ServiceNow ID found in Jira issue %s", event.Issue.Key)
		return
	}

	// Determine which ServiceNow table this issue belongs to
	table := determineServiceNowTable(snID)
	if table == "" {
		// Determine table by issue type or summary
		if strings.Contains(event.Issue.Fields.Summary, "Vendor Risk") {
			table = "sn_vendor_risk"
		} else if strings.Contains(event.Issue.Fields.Summary, "Test Control") {
			table = "sn_policy_control_test"
		} else if strings.Contains(event.Issue.Fields.Summary, "Regulatory Change") ||
			event.Issue.Fields.IssueType.Name == "Epic" {
			table = "sn_regulatory_change"
		} else if strings.Contains(event.Issue.Fields.Summary, "Audit Finding") {
			table = "sn_audit_finding"
		} else {
			log.Printf("Could not determine ServiceNow table for %s (ID: %s)",
				event.Issue.Key, snID)
			return
		}
	}

	// Get the current issue status
	status := event.Issue.Fields.Status.Name

	// Get issue description and possible comment
	description := event.Issue.Fields.Description
	comment := ""
	if event.Comment != nil {
		comment = event.Comment.Body
	}

	log.Printf("Processing Jira update for %s/%s (status=%s)", table, snID, status)

	// Process based on the table
	switch table {
	case "sn_vendor_risk":
		// Handle vendor risk updates
		vendorComment := description
		if comment != "" {
			vendorComment = comment
		}
		if err := h.VendorRiskHandler.SyncFromJira(snID, event.Issue.Key, status, vendorComment); err != nil {
			log.Printf("Error syncing vendor risk from Jira: %v", err)
		}

	case "sn_policy_control_test":
		// Get the control test from ServiceNow
		controlTest, err := h.ControlTestHandler.GetControlTest(snID)
		if err != nil {
			log.Printf("Error getting control test %s: %v", snID, err)
			return
		}

		// Update the control test with Jira data
		testStatus := mapJiraStatusToControlTestStatus(status)
		if testStatus != "" {
			controlTest.Status = testStatus
		}

		if description != "" {
			controlTest.Results = fmt.Sprintf("From Jira: %s", description)
		}

		if comment != "" {
			controlTest.Notes = fmt.Sprintf("Comment from Jira: %s", comment)
		}

		// Sync the updated control test
		if err := h.ControlTestHandler.HandlePolicyControlSync(*controlTest, event.Issue.Key, "", ""); err != nil {
			log.Printf("Error syncing control test to ServiceNow: %v", err)
		}

	case "sn_regulatory_change":
		// Handle regulatory change updates
		if err := h.RegulatoryHandler.HandleRegulatoryChangeUpdate(snID, event.Issue.Key, status, description); err != nil {
			log.Printf("Error processing regulatory change update: %v", err)
		}

	case "sn_audit_finding":
		// Handle audit finding updates
		if err := h.AuditHandler.HandleJiraUpdate(event); err != nil {
			log.Printf("Error processing Jira issue update: %v", err)
		}
	}
}

// determineServiceNowTable determines which ServiceNow table the ID belongs to
func determineServiceNowTable(snID string) string {
	// You would implement logic to determine the table from the ID format
	// This could be based on prefix, ID format, checking with ServiceNow API, etc.

	if strings.HasPrefix(snID, "RISK") {
		return "sn_vendor_risk"
	} else if strings.HasPrefix(snID, "CTRL") {
		return "sn_policy_control_test"
	} else if strings.HasPrefix(snID, "REG") {
		return "sn_regulatory_change"
	} else if strings.HasPrefix(snID, "AUDIT") {
		return "sn_audit_finding"
	}

	return ""
}

// mapJiraStatusToControlTestStatus maps Jira status to ServiceNow control test status
func mapJiraStatusToControlTestStatus(jiraStatus string) string {
	switch strings.ToLower(jiraStatus) {
	case "done":
		return "Completed"
	case "in progress":
		return "In Progress"
	case "to do":
		return "Open"
	default:
		return ""
	}
}
