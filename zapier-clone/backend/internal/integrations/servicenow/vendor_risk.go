// backend/internal/integrations/servicenow/vendor_risk.go
package servicenow

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/jira"
	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/slack"
)

// VendorRisk represents a vendor risk in ServiceNow GRC
type VendorRisk struct {
	ID               string    `json:"sys_id"`
	Number           string    `json:"number"`
	ShortDesc        string    `json:"short_description"`
	Description      string    `json:"description"`
	VendorName       string    `json:"vendor_name"`
	Category         string    `json:"category"`
	Severity         string    `json:"severity"`
	State            string    `json:"state"`
	AssignedTo       string    `json:"assigned_to"`
	CreatedOn        time.Time `json:"sys_created_on"`
	LastUpdated      time.Time `json:"sys_updated_on"`
	DueDate          time.Time `json:"due_date"`
	ComplianceStatus string    `json:"compliance_status"`
	MitigationPlan   string    `json:"mitigation_plan"`
}

// VendorRiskHandler handles vendor risk notifications and interactions
type VendorRiskHandler struct {
	ServiceNowClient *Client
	SlackClient      *slack.Client
	JiraClient       *jira.Client // Add JiraClient for direct Jira interaction
}

// NewVendorRiskHandler creates a new vendor risk handler
func NewVendorRiskHandler(serviceNowClient *Client, slackClient *slack.Client) *VendorRiskHandler {
	return &VendorRiskHandler{
		ServiceNowClient: serviceNowClient,
		SlackClient:      slackClient,
	}
}

// HandleNewVendorRisk processes a new vendor risk and notifies Slack
func (h *VendorRiskHandler) HandleNewVendorRisk(risk VendorRisk) (string, error) {
	// Create a Slack message for the vendor risk
	message := slack.Message{
		Blocks: []slack.Block{
			{
				Type: "header",
				Text: slack.NewTextObject("plain_text", fmt.Sprintf("🚨 Vendor Issue: %s", risk.ShortDesc), true),
			},
			{
				Type: "section",
				Fields: []*slack.TextObject{
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Risk ID:*\n%s", risk.Number), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Vendor:*\n%s", risk.VendorName), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Severity:*\n%s", risk.Severity), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Due Date:*\n%s", risk.DueDate.Format("Jan 2, 2006")), false),
				},
			},
			{
				Type: "section",
				Text: slack.NewTextObject("mrkdwn", fmt.Sprintf("*Description:*\n%s", risk.Description), false),
			},
			{
				Type: "actions",
				Elements: []interface{}{
					map[string]interface{}{
						"type": "button",
						"text": map[string]interface{}{
							"type":  "plain_text",
							"text":  "Request Compliance Report",
							"emoji": true,
						},
						"value":     fmt.Sprintf("request_report_%s", risk.ID),
						"action_id": "request_compliance_report",
					},
					map[string]interface{}{
						"type": "button",
						"text": map[string]interface{}{
							"type":  "plain_text",
							"text":  "Update Status",
							"emoji": true,
						},
						"value":     fmt.Sprintf("update_vendor_%s", risk.ID),
						"action_id": "update_vendor_status",
					},
					map[string]interface{}{
						"type": "button",
						"text": map[string]interface{}{
							"type":  "plain_text",
							"text":  "View in ServiceNow",
							"emoji": true,
						},
						"url":       fmt.Sprintf("%s/nav_to.do?uri=sn_vendor_risk.do?sys_id=%s", h.ServiceNowClient.BaseURL, risk.ID),
						"action_id": "view_vendor_risk",
					},
				},
			},
			{
				Type: "context",
				Elements: []interface{}{
					map[string]interface{}{
						"type": "mrkdwn",
						"text": "📍 Action Needed: Request updated compliance report.",
					},
				},
			},
		},
	}

	// Post the message to the vendor-risk channel
	ts, err := h.SlackClient.PostMessage(slack.ChannelMapping["vendor-risk"], message)
	if err != nil {
		return "", fmt.Errorf("error posting vendor risk message to Slack: %w", err)
	}

	return ts, nil
}

// HandleComplianceReportRequest processes a compliance report request
// HandleComplianceReportRequest processes a compliance report request
func (h *VendorRiskHandler) HandleComplianceReportRequest(riskID, channelID, threadTS, userID string) error {
	// Get the vendor risk info to get the vendor name
	resp, err := h.ServiceNowClient.makeRequest("GET", fmt.Sprintf("api/now/table/sn_vendor_risk/%s", riskID), nil)
	if err != nil {
		return fmt.Errorf("error getting vendor risk details: %w", err)
	}
	defer resp.Body.Close()

	var response struct {
		Result VendorRisk `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("error decoding vendor risk response: %w", err)
	}

	// Post the request notification to the thread
	message := slack.Message{
		Text: fmt.Sprintf("<@%s> has requested a compliance report from %s", userID, response.Result.VendorName),
	}

	// Post the reply to the thread
	_, err = h.SlackClient.PostReply(channelID, threadTS, message)
	if err != nil {
		return fmt.Errorf("error posting compliance report request to Slack thread: %w", err)
	}

	// Update ServiceNow with the request
	body := map[string]string{
		"state": "report_requested",
		"notes": fmt.Sprintf("Compliance report requested by %s on %s", userID, time.Now().Format(time.RFC3339)),
	}

	_, err = h.ServiceNowClient.makeRequest("PATCH", fmt.Sprintf("api/now/table/sn_vendor_risk/%s", riskID), body)
	if err != nil {
		return fmt.Errorf("error updating vendor risk in ServiceNow: %w", err)
	}

	// If we have a Jira client, update the corresponding Jira ticket
	if h.JiraClient != nil {
		// Find the Jira ticket associated with this vendor risk
		jql := fmt.Sprintf("project=%s AND customfield_servicenow_id=\"%s\"", h.JiraClient.ProjectKey, riskID)
		searchResult, err := h.JiraClient.SearchIssues(jql)
		if err != nil {
			log.Printf("Error searching for Jira ticket for vendor risk %s: %v", riskID, err)
			return nil // Continue without Jira update
		}

		if len(searchResult.Issues) > 0 {
			issueKey := searchResult.Issues[0].Key
			commentText := fmt.Sprintf("Compliance report requested by %s on %s", userID, time.Now().Format(time.RFC3339))

			// Add a comment to the Jira ticket
			if err := h.JiraClient.AddComment(issueKey, commentText); err != nil {
				log.Printf("Error adding comment to Jira ticket %s: %v", issueKey, err)
			} else {
				log.Printf("Added comment about compliance report request to Jira ticket %s", issueKey)
			}
		}
	}

	return nil
}

// HandleVendorStatusUpdate processes a vendor status update
func (h *VendorRiskHandler) HandleVendorStatusUpdate(riskID, channelID, threadTS, userID, status, notes string) error {
	// Post the status update to the thread
	message := slack.Message{
		Text: fmt.Sprintf("<@%s> has updated the vendor status to *%s*: %s", userID, status, notes),
	}

	// Post the reply to the thread
	_, err := h.SlackClient.PostReply(channelID, threadTS, message)
	if err != nil {
		return fmt.Errorf("error posting vendor status update to Slack thread: %w", err)
	}

	// Update ServiceNow with the status update
	body := map[string]string{
		"compliance_status": status,
		"notes":             notes,
	}

	_, err = h.ServiceNowClient.makeRequest("PATCH", fmt.Sprintf("api/now/table/sn_vendor_risk/%s", riskID), body)
	if err != nil {
		return fmt.Errorf("error updating vendor status in ServiceNow: %w", err)
	}

	// If we have a Jira client, update the corresponding Jira ticket
	if h.JiraClient != nil {
		// Find the Jira ticket associated with this vendor risk
		jql := fmt.Sprintf("project=%s AND customfield_servicenow_id=\"%s\"", h.JiraClient.ProjectKey, riskID)
		searchResult, err := h.JiraClient.SearchIssues(jql)
		if err != nil {
			log.Printf("Error searching for Jira ticket for vendor risk %s: %v", riskID, err)
			return nil // Continue without Jira update
		}

		if len(searchResult.Issues) > 0 {
			issueKey := searchResult.Issues[0].Key

			// Determine if we should mark the issue as Done
			jiraStatus := "In Progress"
			if strings.ToLower(status) == "received" || strings.ToLower(status) == "resolved" {
				jiraStatus = "Done"
			}

			// Create update object
			update := &jira.TicketUpdate{
				Status:  jiraStatus,
				Comment: fmt.Sprintf("Vendor status updated to %s: %s", status, notes),
			}

			// Update the Jira ticket
			if err := h.JiraClient.UpdateIssue(issueKey, update); err != nil {
				log.Printf("Error updating Jira ticket %s: %v", issueKey, err)
			} else {
				log.Printf("Updated Jira ticket %s with vendor status %s", issueKey, status)
			}
		}
	}

	return nil
}

// ProcessVendorCommand handles slash commands for vendor risk management
func (h *VendorRiskHandler) ProcessVendorCommand(command *slack.Command) (string, error) {
	// Handle different vendor risk commands
	switch {
	case command.Command == "/update-vendor":
		// Format: /update-vendor RISK_ID STATUS Notes about the update
		parts := strings.SplitN(command.Text, " ", 3)
		if len(parts) < 3 {
			return "Invalid command format. Usage: /update-vendor RISK_ID STATUS Notes about the update", nil
		}

		riskID := parts[0]
		status := parts[1]
		notes := parts[2]

		err := h.HandleVendorStatusUpdate(riskID, command.ChannelID, "", command.UserID, status, notes)
		if err != nil {
			return fmt.Sprintf("Error updating vendor status: %s", err), nil
		}

		return "Vendor status updated successfully!", nil

	default:
		return "Unknown command", nil
	}
}

// SetJiraClient sets the Jira client for this handler
// This allows for adding the client after initialization
func (h *VendorRiskHandler) SetJiraClient(jiraClient *jira.Client) {
	h.JiraClient = jiraClient
}

// SyncFromJira synchronizes updates from Jira to ServiceNow
func (h *VendorRiskHandler) SyncFromJira(riskID, jiraKey, jiraStatus, jiraComment string) error {
	// Map Jira status to ServiceNow vendor risk state
	vendorState := "In Progress"
	if jiraStatus == "Done" {
		vendorState = "Resolved"
	}

	// Update ServiceNow with data from Jira
	body := map[string]string{
		"state": vendorState,
		"notes": fmt.Sprintf("Update from Jira: %s", jiraComment),
	}

	_, err := h.ServiceNowClient.makeRequest("PATCH",
		fmt.Sprintf("api/now/table/sn_vendor_risk/%s", riskID), body)
	if err != nil {
		return fmt.Errorf("error updating vendor risk from Jira in ServiceNow: %w", err)
	}

	log.Printf("Updated ServiceNow vendor risk %s from Jira %s (state: %s)",
		riskID, jiraKey, vendorState)

	return nil
}
