// backend/internal/integrations/servicenow/compliance_task.go
package servicenow

import (
	"fmt"
	"strings"

	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/slack"
)

// ComplianceTaskHandler handles compliance task notifications and interactions
type ComplianceTaskHandler struct {
	ServiceNowClient *Client
	SlackClient      *slack.Client
}

// NewComplianceTaskHandler creates a new compliance task handler
func NewComplianceTaskHandler(serviceNowClient *Client, slackClient *slack.Client) *ComplianceTaskHandler {
	return &ComplianceTaskHandler{
		ServiceNowClient: serviceNowClient,
		SlackClient:      slackClient,
	}
}

// HandleNewComplianceTask processes a new compliance task and notifies Slack
func (h *ComplianceTaskHandler) HandleNewComplianceTask(task ComplianceTask) (string, error) {
	// Create a Slack message for the compliance task
	message := slack.Message{
		Blocks: []slack.Block{
			{
				Type: "header",
				Text: slack.NewTextObject("plain_text", fmt.Sprintf("📝 Compliance Task: %s", task.ShortDesc), true),
			},
			{
				Type: "section",
				Fields: []*slack.TextObject{
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Task ID:*\n%s", task.Number), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Framework:*\n%s", task.Framework), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Regulation:*\n%s", task.Regulation), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Due Date:*\n%s", task.DueDate.Format("Jan 2, 2006")), false),
				},
			},
			{
				Type: "section",
				Text: slack.NewTextObject("mrkdwn", fmt.Sprintf("*Description:*\n%s", task.Description), false),
			},
			{
				Type: "actions",
				Elements: []interface{}{
					map[string]interface{}{
						"type": "button",
						"text": map[string]interface{}{
							"type":  "plain_text",
							"text":  "Upload Evidence",
							"emoji": true,
						},
						"value":     fmt.Sprintf("upload_evidence_%s", task.ID),
						"action_id": "upload_evidence",
					},
					map[string]interface{}{
						"type": "button",
						"text": map[string]interface{}{
							"type":  "plain_text",
							"text":  "Assign Owner",
							"emoji": true,
						},
						"value":     fmt.Sprintf("assign_task_%s", task.ID),
						"action_id": "assign_task",
					},
					map[string]interface{}{
						"type": "button",
						"text": map[string]interface{}{
							"type":  "plain_text",
							"text":  "View in ServiceNow",
							"emoji": true,
						},
						"url":       fmt.Sprintf("%s/nav_to.do?uri=sn_compliance_task.do?sys_id=%s", h.ServiceNowClient.BaseURL, task.ID),
						"action_id": "view_task",
					},
				},
			},
			{
				Type: "context",
				Elements: []interface{}{
					map[string]interface{}{
						"type": "mrkdwn",
						"text": fmt.Sprintf("📅 This compliance task is due by *%s*.", task.DueDate.Format("Jan 2, 2006")),
					},
				},
			},
		},
	}

	// Post the message to the compliance-team channel
	ts, err := h.SlackClient.PostMessage(slack.ChannelMapping["compliance"], message)
	if err != nil {
		return "", fmt.Errorf("error posting compliance task message to Slack: %w", err)
	}

	return ts, nil
}

// HandleComplianceTaskUpdate processes a compliance task update and updates the Slack message
func (h *ComplianceTaskHandler) HandleComplianceTaskUpdate(task ComplianceTask, channelID, threadTS string) error {
	// Format a message about the update
	updateText := fmt.Sprintf("Compliance Task '%s' has been updated:\n• Status: %s\n", task.ShortDesc, task.State)

	if task.AssignedTo != "" {
		updateText += fmt.Sprintf("• Assigned To: %s\n", task.AssignedTo)
	}

	// Create a reply message
	message := slack.Message{
		Text: updateText,
	}

	// Post the reply to the thread
	_, err := h.SlackClient.PostReply(channelID, threadTS, message)
	if err != nil {
		return fmt.Errorf("error posting compliance task update to Slack thread: %w", err)
	}

	return nil
}

// HandleComplianceTaskAssignment processes a compliance task assignment
func (h *ComplianceTaskHandler) HandleComplianceTaskAssignment(taskID, channelID, threadTS, assigneeID string) error {
	// Get the assignee's name from their Slack ID (in a real implementation, you'd look this up)
	assigneeName := fmt.Sprintf("<@%s>", assigneeID)

	// Post the assignment notification to the thread
	message := slack.Message{
		Text: fmt.Sprintf("Compliance Task '%s' has been assigned to %s", taskID, assigneeName),
	}

	// Post the reply to the thread
	_, err := h.SlackClient.PostReply(channelID, threadTS, message)
	if err != nil {
		return fmt.Errorf("error posting compliance task assignment to Slack thread: %w", err)
	}

	// Update ServiceNow with the assignment
	body := map[string]string{
		"assigned_to": assigneeID,
	}

	_, err = h.ServiceNowClient.makeRequest("PATCH", fmt.Sprintf("api/now/table/sn_compliance_task/%s", taskID), body)
	if err != nil {
		return fmt.Errorf("error updating compliance task assignment in ServiceNow: %w", err)
	}

	return nil
}

// HandleEvidenceUpload processes evidence uploads for compliance tasks
func (h *ComplianceTaskHandler) HandleEvidenceUpload(taskID, channelID, threadTS, userID, fileName, fileContent string) error {
	// Upload the file to Slack
	_, err := h.SlackClient.UploadFile(channelID, fileName, fileContent)
	if err != nil {
		return fmt.Errorf("error uploading evidence to Slack: %w", err)
	}

	// Post a notification about the evidence upload
	message := slack.Message{
		Text: fmt.Sprintf("<@%s> uploaded evidence: `%s`", userID, fileName),
	}

	// Post the reply to the thread
	_, err = h.SlackClient.PostReply(channelID, threadTS, message)
	if err != nil {
		return fmt.Errorf("error posting evidence upload notification to Slack thread: %w", err)
	}

	// Attach the evidence to the compliance task in ServiceNow
	err = h.ServiceNowClient.AttachEvidenceToComplianceTask(taskID, fileName, fileContent)
	if err != nil {
		return fmt.Errorf("error attaching evidence to compliance task in ServiceNow: %w", err)
	}

	return nil
}

// ProcessComplianceTaskCommand handles slash commands for compliance tasks
func (h *ComplianceTaskHandler) ProcessComplianceTaskCommand(command *slack.Command) (string, error) {
	// Handle different compliance task commands
	switch {
	case command.Command == "/upload-evidence":
		// Format: /upload-evidence TASK_ID EVIDENCE_URL
		// This is a simplified version - in reality, you'd handle file uploads differently
		parts := splitCommand(command.Text)
		if len(parts) < 2 {
			return "Invalid command format. Usage: /upload-evidence TASK_ID EVIDENCE_URL", nil
		}

		taskID := parts[0]
		evidenceURL := parts[1]

		// Simulate evidence upload
		err := h.HandleEvidenceUpload(taskID, command.ChannelID, "", command.UserID, "evidence.pdf", evidenceURL)
		if err != nil {
			return fmt.Sprintf("Error uploading evidence: %s", err), nil
		}

		return "Evidence uploaded successfully!", nil

	default:
		return "Unknown command", nil
	}
}

// Helper function to split command text
func splitCommand(text string) []string {
	// Simple space-based splitting - in a real implementation, you might want to handle quoted strings, etc.
	return strings.Fields(text)
}
