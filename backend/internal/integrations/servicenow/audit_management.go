// backend/internal/integrations/servicenow/audit_management.go
package servicenow

import (
	"fmt"
	"strings"
	"time"

	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/slack"
)

// AuditFinding represents an audit finding in ServiceNow GRC
type AuditFinding struct {
	ID          string    `json:"sys_id"`
	Number      string    `json:"number"`
	ShortDesc   string    `json:"short_description"`
	Description string    `json:"description"`
	Audit       string    `json:"audit_name"`
	Severity    string    `json:"severity"`
	State       string    `json:"state"`
	AssignedTo  string    `json:"assigned_to"`
	CreatedOn   time.Time `json:"sys_created_on"`
	LastUpdated time.Time `json:"sys_updated_on"`
	DueDate     time.Time `json:"due_date"`
	Resolution  string    `json:"resolution"`
}

// AuditHandler handles audit findings notifications and interactions
type AuditHandler struct {
	ServiceNowClient *Client
	SlackClient      *slack.Client
}

// NewAuditHandler creates a new audit handler
func NewAuditHandler(serviceNowClient *Client, slackClient *slack.Client) *AuditHandler {
	return &AuditHandler{
		ServiceNowClient: serviceNowClient,
		SlackClient:      slackClient,
	}
}

// HandleNewAuditFinding processes a new audit finding and notifies Slack
func (h *AuditHandler) HandleNewAuditFinding(finding AuditFinding) (string, error) {
	// Create a Slack message for the audit finding
	message := slack.Message{
		Blocks: []slack.Block{
			{
				Type: "header",
				Text: slack.NewTextObject("plain_text", fmt.Sprintf("📌 New Finding: %s", finding.ShortDesc), true),
			},
			{
				Type: "section",
				Fields: []*slack.TextObject{
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Finding ID:*\n%s", finding.Number), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Audit:*\n%s", finding.Audit), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Severity:*\n%s", finding.Severity), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Due Date:*\n%s", finding.DueDate.Format("Jan 2, 2006")), false),
				},
			},
			{
				Type: "section",
				Text: slack.NewTextObject("mrkdwn", fmt.Sprintf("*Description:*\n%s", finding.Description), false),
			},
			{
				Type: "actions",
				Elements: []interface{}{
					map[string]interface{}{
						"type": "button",
						"text": map[string]interface{}{
							"type":  "plain_text",
							"text":  "Assign Owner",
							"emoji": true,
						},
						"value":     fmt.Sprintf("assign_finding_%s", finding.ID),
						"action_id": "assign_finding",
					},
					map[string]interface{}{
						"type": "button",
						"text": map[string]interface{}{
							"type":  "plain_text",
							"text":  "Resolve Finding",
							"emoji": true,
						},
						"value":     fmt.Sprintf("resolve_finding_%s", finding.ID),
						"action_id": "resolve_finding",
					},
					map[string]interface{}{
						"type": "button",
						"text": map[string]interface{}{
							"type":  "plain_text",
							"text":  "View in ServiceNow",
							"emoji": true,
						},
						"url":       fmt.Sprintf("%s/nav_to.do?uri=sn_audit_finding.do?sys_id=%s", h.ServiceNowClient.BaseURL, finding.ID),
						"action_id": "view_finding",
					},
				},
			},
			{
				Type: "context",
				Elements: []interface{}{
					map[string]interface{}{
						"type": "mrkdwn",
						"text": "🔎 Assign an owner for resolution.",
					},
				},
			},
		},
	}

	// Post the message to the audit-team channel
	ts, err := h.SlackClient.PostMessage(slack.ChannelMapping["audit"], message)
	if err != nil {
		return "", fmt.Errorf("error posting audit finding message to Slack: %w", err)
	}

	return ts, nil
}

// HandleAuditFindingAssignment processes an audit finding assignment
func (h *AuditHandler) HandleAuditFindingAssignment(findingID, channelID, threadTS, assigneeID string) error {
	// Get the assignee's name from their Slack ID (in a real implementation, you'd look this up)
	assigneeName := fmt.Sprintf("<@%s>", assigneeID)

	// Post the assignment notification to the thread
	message := slack.Message{
		Text: fmt.Sprintf("Audit Finding '%s' has been assigned to %s", findingID, assigneeName),
	}

	// Post the reply to the thread
	_, err := h.SlackClient.PostReply(channelID, threadTS, message)
	if err != nil {
		return fmt.Errorf("error posting audit finding assignment to Slack thread: %w", err)
	}

	// Update ServiceNow with the assignment
	body := map[string]string{
		"assigned_to": assigneeID,
		"state":       "assigned",
	}

	_, err = h.ServiceNowClient.makeRequest("PATCH", fmt.Sprintf("api/now/table/sn_audit_finding/%s", findingID), body)
	if err != nil {
		return fmt.Errorf("error updating audit finding assignment in ServiceNow: %w", err)
	}

	return nil
}

// HandleAuditFindingResolution processes an audit finding resolution
func (h *AuditHandler) HandleAuditFindingResolution(findingID, channelID, threadTS, userID, resolution string) error {
	// Post the resolution to the thread
	message := slack.Message{
		Text: fmt.Sprintf("<@%s> has resolved this finding: %s", userID, resolution),
	}

	// Post the reply to the thread
	_, err := h.SlackClient.PostReply(channelID, threadTS, message)
	if err != nil {
		return fmt.Errorf("error posting audit finding resolution to Slack thread: %w", err)
	}

	// Update ServiceNow with the resolution
	body := map[string]string{
		"state":      "resolved",
		"resolution": resolution,
	}

	_, err = h.ServiceNowClient.makeRequest("PATCH", fmt.Sprintf("api/now/table/sn_audit_finding/%s", findingID), body)
	if err != nil {
		return fmt.Errorf("error updating audit finding resolution in ServiceNow: %w", err)
	}

	return nil
}

// ProcessAuditCommand handles slash commands for audit findings
func (h *AuditHandler) ProcessAuditCommand(command *slack.Command) (string, error) {
	// Handle different audit commands
	switch {
	case command.Command == "/resolve-finding":
		// Format: /resolve-finding FINDING_ID Resolution notes
		parts := strings.SplitN(command.Text, " ", 2)
		if len(parts) < 2 {
			return "Invalid command format. Usage: /resolve-finding FINDING_ID Resolution notes", nil
		}

		findingID := parts[0]
		resolution := parts[1]

		err := h.HandleAuditFindingResolution(findingID, command.ChannelID, "", command.UserID, resolution)
		if err != nil {
			return fmt.Sprintf("Error resolving audit finding: %s", err), nil
		}

		return "Audit finding resolved successfully!", nil

	default:
		return "Unknown command", nil
	}
}
