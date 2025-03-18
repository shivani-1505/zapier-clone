// backend/internal/integrations/servicenow/risk_management.go
package servicenow

import (
	"fmt"
	"strings"

	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/slack"
)

// RiskHandler handles risk notifications and interactions
type RiskHandler struct {
	ServiceNowClient *Client
	SlackClient      *slack.Client
}

// NewRiskHandler creates a new risk handler
func NewRiskHandler(serviceNowClient *Client, slackClient *slack.Client) *RiskHandler {
	return &RiskHandler{
		ServiceNowClient: serviceNowClient,
		SlackClient:      slackClient,
	}
}

// HandleNewRisk processes a new risk and notifies Slack
func (h *RiskHandler) HandleNewRisk(risk Risk) (string, error) {
	// Format risk severity for display
	severity := RiskSeverity(risk.RiskScore)

	// Create a Slack message for the risk
	message := slack.Message{
		Blocks: []slack.Block{
			{
				Type: "header",
				Text: slack.NewTextObject("plain_text", fmt.Sprintf("🚨 New %s-Severity Risk: %s", severity, risk.ShortDesc), true),
			},
			{
				Type: "section",
				Fields: []*slack.TextObject{
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Risk ID:*\n%s", risk.Number), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Category:*\n%s", risk.Category), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Severity:*\n%s", severity), false),
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
							"text":  "Discuss Mitigation",
							"emoji": true,
						},
						"value":     fmt.Sprintf("discuss_risk_%s", risk.ID),
						"action_id": "discuss_risk",
					},
					map[string]interface{}{
						"type": "button",
						"text": map[string]interface{}{
							"type":  "plain_text",
							"text":  "Assign Owner",
							"emoji": true,
						},
						"value":     fmt.Sprintf("assign_risk_%s", risk.ID),
						"action_id": "assign_risk",
					},
					map[string]interface{}{
						"type": "button",
						"text": map[string]interface{}{
							"type":  "plain_text",
							"text":  "View in ServiceNow",
							"emoji": true,
						},
						"url":       fmt.Sprintf("%s/nav_to.do?uri=sn_risk_risk.do?sys_id=%s", h.ServiceNowClient.BaseURL, risk.ID),
						"action_id": "view_risk",
					},
				},
			},
			{
				Type: "context",
				Elements: []interface{}{
					map[string]interface{}{
						"type": "mrkdwn",
						"text": fmt.Sprintf("👁️ This risk requires action by IT and Security teams due by *%s*.", risk.DueDate.Format("Jan 2, 2006")),
					},
				},
			},
		},
	}

	// Post the message to the risk-management channel
	ts, err := h.SlackClient.PostMessage(slack.ChannelMapping["risk-management"], message)
	if err != nil {
		return "", fmt.Errorf("error posting risk message to Slack: %w", err)
	}

	return ts, nil
}

// HandleRiskUpdate processes a risk update and updates the Slack message
func (h *RiskHandler) HandleRiskUpdate(risk Risk, channelID, threadTS string) error {
	// Format a message about the update
	updateText := fmt.Sprintf("Risk '%s' has been updated:\n• Status: %s\n", risk.ShortDesc, risk.State)

	if risk.MitigationPlan != "" {
		updateText += fmt.Sprintf("• Mitigation Plan: %s\n", risk.MitigationPlan)
	}

	if risk.AssignedTo != "" {
		updateText += fmt.Sprintf("• Assigned To: %s\n", risk.AssignedTo)
	}

	// Create a reply message
	message := slack.Message{
		Text: updateText,
	}

	// Post the reply to the thread
	_, err := h.SlackClient.PostReply(channelID, threadTS, message)
	if err != nil {
		return fmt.Errorf("error posting risk update to Slack thread: %w", err)
	}

	return nil
}

// HandleRiskDiscussion processes a discussion about a risk
func (h *RiskHandler) HandleRiskDiscussion(riskID, channelID, threadTS, userID, text string) error {
	// Post the discussion comment to the thread
	message := slack.Message{
		Text: fmt.Sprintf("<@%s> commented: %s", userID, text),
	}

	// Post the reply to the thread
	_, err := h.SlackClient.PostReply(channelID, threadTS, message)
	if err != nil {
		return fmt.Errorf("error posting risk discussion to Slack thread: %w", err)
	}

	// Update the mitigation plan in ServiceNow if the comment contains mitigation details
	if strings.Contains(strings.ToLower(text), "mitigation:") {
		// Extract the mitigation plan from the comment
		mitigationPlan := text[strings.Index(strings.ToLower(text), "mitigation:")+11:]

		// Update ServiceNow with the mitigation plan
		body := map[string]string{
			"mitigation_plan": mitigationPlan,
		}

		_, err := h.ServiceNowClient.makeRequest("PATCH", fmt.Sprintf("api/now/table/sn_risk_risk/%s", riskID), body)
		if err != nil {
			return fmt.Errorf("error updating mitigation plan in ServiceNow: %w", err)
		}
	}

	return nil
}

// HandleRiskAssignment processes a risk assignment
func (h *RiskHandler) HandleRiskAssignment(riskID, channelID, threadTS, assigneeID string) error {
	// Get the assignee's name from their Slack ID (in a real implementation, you'd look this up)
	assigneeName := fmt.Sprintf("<@%s>", assigneeID)

	// Post the assignment notification to the thread
	message := slack.Message{
		Text: fmt.Sprintf("Risk '%s' has been assigned to %s", riskID, assigneeName),
	}

	// Post the reply to the thread
	_, err := h.SlackClient.PostReply(channelID, threadTS, message)
	if err != nil {
		return fmt.Errorf("error posting risk assignment to Slack thread: %w", err)
	}

	// Update ServiceNow with the assignment
	body := map[string]string{
		"assigned_to": assigneeID,
	}

	_, err = h.ServiceNowClient.makeRequest("PATCH", fmt.Sprintf("api/now/table/sn_risk_risk/%s", riskID), body)
	if err != nil {
		return fmt.Errorf("error updating risk assignment in ServiceNow: %w", err)
	}

	return nil
}
