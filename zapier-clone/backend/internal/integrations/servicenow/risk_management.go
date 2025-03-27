// backend/internal/integrations/servicenow/risk_management.go
package servicenow

import (
	"fmt"
	"strings"

	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/jira"
	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/slack"
)

// RiskHandler handles risk notifications and interactions
type RiskHandler struct {
	ServiceNowClient *Client
	SlackClient      *slack.Client
	JiraClient       *jira.Client
	RiskJiraMapping  *jira.RiskJiraMapping
}

// NewRiskHandler creates a new risk handler
func NewRiskHandler(serviceNowClient *Client, slackClient *slack.Client, jiraClient *jira.Client, mapping *jira.RiskJiraMapping) *RiskHandler {
	return &RiskHandler{
		ServiceNowClient: serviceNowClient,
		SlackClient:      slackClient,
		JiraClient:       jiraClient,
		RiskJiraMapping:  mapping,
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

	// Create a Jira issue for the risk
	jiraIssue, err := h.createJiraIssue(risk, severity)
	if err != nil {
		// Log the error but continue - we don't want to fail the whole process if just Jira fails
		// In a real implementation, you might want more sophisticated error handling/retries
		fmt.Printf("Error creating Jira issue: %s\n", err)
	} else {
		// Store the mapping between ServiceNow risk and Jira issue
		if err := h.RiskJiraMapping.AddMapping(risk.ID, jiraIssue.Key); err != nil {
			fmt.Printf("Error storing risk-jira mapping: %s\n", err)
		}

		// Add a comment to the Slack thread about the Jira issue
		jiraMessage := slack.Message{
			Text: fmt.Sprintf("📋 This risk has been synced with Jira as issue *<%s/browse/%s|%s>*",
				h.JiraClient.BaseURL, jiraIssue.Key, jiraIssue.Key),
		}

		_, err := h.SlackClient.PostReply(slack.ChannelMapping["risk-management"], ts, jiraMessage)
		if err != nil {
			fmt.Printf("Error posting Jira link to Slack: %s\n", err)
		}
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

	// Update the corresponding Jira issue if one exists
	jiraKey, exists := h.RiskJiraMapping.GetJiraKeyFromRiskID(risk.ID)
	if exists {
		// Map the fields to update in Jira
		fields := map[string]interface{}{}

		// Update status based on ServiceNow state
		switch risk.State {
		case "Draft":
			fields["status"] = map[string]string{"name": "To Do"}
		case "In Progress":
			fields["status"] = map[string]string{"name": "In Progress"}
		case "Completed":
			fields["status"] = map[string]string{"name": "Done"}
			// Add more state mappings as needed
		}

		// Update mitigation plan as a field or comment
		if risk.MitigationPlan != "" {
			// Add as comment instead of field update
			commentText := fmt.Sprintf("Mitigation Plan updated in ServiceNow:\n%s", risk.MitigationPlan)
			if err := h.JiraClient.AddComment(jiraKey, commentText); err != nil {
				fmt.Printf("Error adding mitigation plan comment to Jira: %s\n", err)
			}
		}

		// Only update Jira if we have fields to update
		if len(fields) > 0 {
			ticketUpdate := &jira.TicketUpdate{
				Fields: fields,
			}

			if err := h.JiraClient.UpdateIssue(jiraKey, ticketUpdate); err != nil {
				fmt.Printf("Error updating Jira issue: %s\n", err)
			}
		}
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

	// Update the corresponding Jira issue with the comment if one exists
	jiraKey, exists := h.RiskJiraMapping.GetJiraKeyFromRiskID(riskID)
	if exists {
		commentText := fmt.Sprintf("Comment from Slack by %s:\n%s", userID, text)
		if err := h.JiraClient.AddComment(jiraKey, commentText); err != nil {
			fmt.Printf("Error adding comment to Jira: %s\n", err)
		}
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

// Add this method to your RiskHandler implementation:

// createJiraIssue creates a Jira issue for a ServiceNow risk
func (h *RiskHandler) createJiraIssue(risk Risk, severity string) (*jira.Ticket, error) {
	// Map ServiceNow risk severity to Jira priority
	priority := "Medium" // Default priority
	switch severity {
	case "Critical":
		priority = "Highest"
	case "High":
		priority = "High"
	case "Medium":
		priority = "Medium"
	case "Low":
		priority = "Low"
	}

	// Create formatted description with details from ServiceNow
	description := fmt.Sprintf(`*Risk Details from ServiceNow*
    
*Risk Number:* %s
*Category:* %s
*Severity:* %s
*Risk Score:* %.1f
*Due Date:* %s
    
*Description:*
%s

*Possible Impact:*
%s

----
This issue was automatically created from ServiceNow Risk %s.
Please update both systems when changes are made.`,
		risk.Number,
		risk.Category,
		severity,
		risk.RiskScore,
		risk.DueDate.Format("May 2, 2025"),
		risk.Description,
		risk.Impact,
		risk.Number)

	// Create a Jira ticket struct
	ticket := &jira.Ticket{
		Summary:     fmt.Sprintf("[%s] %s", risk.Number, risk.ShortDesc),
		Description: description,
		IssueType:   "Risk",
		Priority:    priority,
		DueDate:     risk.DueDate,
	}

	// Create the Jira issue
	issue, err := h.JiraClient.CreateIssue(ticket)
	if err != nil {
		return nil, fmt.Errorf("error creating Jira issue: %w", err)
	}

	return issue, nil
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

	// Update the corresponding Jira issue if one exists
	jiraKey, exists := h.RiskJiraMapping.GetJiraKeyFromRiskID(riskID)
	if exists {
		// Create fields for Jira update - we're just updating the assignee here
		fields := map[string]interface{}{
			"assignee": map[string]string{
				"name": assigneeID, // In a real implementation, you'd map this to a Jira username
			},
		}

		// Only update Jira if we have fields to update
		if len(fields) > 0 {
			ticketUpdate := &jira.TicketUpdate{
				Fields: fields,
			}

			if err := h.JiraClient.UpdateIssue(jiraKey, ticketUpdate); err != nil {
				fmt.Printf("Error updating Jira issue: %s\n", err)
			}
		}

		// Add a comment about the assignment
		commentText := fmt.Sprintf("Risk assigned to %s", assigneeName)
		if err := h.JiraClient.AddComment(jiraKey, commentText); err != nil {
			fmt.Printf("Error adding assignment comment to Jira: %s\n", err)
		}
	}

	return nil
}
