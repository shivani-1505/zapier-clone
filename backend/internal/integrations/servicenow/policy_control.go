// backend/internal/integrations/servicenow/policy_control.go
package servicenow

import (
	"fmt"
	"strings"
	"time"

	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/slack"
)

// ControlTest represents a control test in ServiceNow GRC
type ControlTest struct {
	ID          string    `json:"sys_id"`
	Number      string    `json:"number"`
	ShortDesc   string    `json:"short_description"`
	Description string    `json:"description"`
	Control     string    `json:"control_name"`
	Framework   string    `json:"framework"`
	State       string    `json:"state"`
	AssignedTo  string    `json:"assigned_to"`
	CreatedOn   time.Time `json:"sys_created_on"`
	LastUpdated time.Time `json:"sys_updated_on"`
	DueDate     time.Time `json:"due_date"`
	Results     string    `json:"results"`
	Notes       string    `json:"notes"`
	Status      string    `json:"test_status"` // Pass, Fail, In Progress
}

// PolicyControlHandler handles control testing notifications and interactions
type PolicyControlHandler struct {
	ServiceNowClient *Client
	SlackClient      *slack.Client
}

// NewPolicyControlHandler creates a new policy and control handler
func NewPolicyControlHandler(serviceNowClient *Client, slackClient *slack.Client) *PolicyControlHandler {
	return &PolicyControlHandler{
		ServiceNowClient: serviceNowClient,
		SlackClient:      slackClient,
	}
}

// HandleNewControlTest processes a new control test and notifies Slack
func (h *PolicyControlHandler) HandleNewControlTest(test ControlTest) (string, error) {
	// Create a Slack message for the control test
	message := slack.Message{
		Blocks: []slack.Block{
			{
				Type: "header",
				Text: slack.NewTextObject("plain_text", fmt.Sprintf("✅ New Test Assigned: %s", test.Control), true),
			},
			{
				Type: "section",
				Fields: []*slack.TextObject{
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Test ID:*\n%s", test.Number), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Framework:*\n%s", test.Framework), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Due Date:*\n%s", test.DueDate.Format("Jan 2, 2006")), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Status:*\n%s", test.Status), false),
				},
			},
			{
				Type: "section",
				Text: slack.NewTextObject("mrkdwn", fmt.Sprintf("*Description:*\n%s", test.Description), false),
			},
			{
				Type: "actions",
				Elements: []interface{}{
					map[string]interface{}{
						"type": "button",
						"text": map[string]interface{}{
							"type":  "plain_text",
							"text":  "Submit Test Results",
							"emoji": true,
						},
						"value":     fmt.Sprintf("test_results_%s", test.ID),
						"action_id": "submit_test_results",
					},
					map[string]interface{}{
						"type": "button",
						"text": map[string]interface{}{
							"type":  "plain_text",
							"text":  "View in ServiceNow",
							"emoji": true,
						},
						"url":       fmt.Sprintf("%s/nav_to.do?uri=sn_policy_control_test.do?sys_id=%s", h.ServiceNowClient.BaseURL, test.ID),
						"action_id": "view_test",
					},
				},
			},
			{
				Type: "context",
				Elements: []interface{}{
					map[string]interface{}{
						"type": "mrkdwn",
						"text": fmt.Sprintf("📅 This control test is due by *%s*.", test.DueDate.Format("Jan 2, 2006")),
					},
				},
			},
		},
	}

	// Post the message to the control-testing channel
	ts, err := h.SlackClient.PostMessage(slack.ChannelMapping["control-testing"], message)
	if err != nil {
		return "", fmt.Errorf("error posting control test message to Slack: %w", err)
	}

	return ts, nil
}

// HandleTestResultSubmission processes test result submissions
func (h *PolicyControlHandler) HandleTestResultSubmission(testID, channelID, threadTS, userID, status, notes string) error {
	// Post the test result to the thread
	var emoji string
	switch strings.ToLower(status) {
	case "pass":
		emoji = "✅"
	case "fail":
		emoji = "❌"
	default:
		emoji = "⚠️"
	}

	message := slack.Message{
		Text: fmt.Sprintf("%s Test Result by <@%s>: *%s*\n%s", emoji, userID, status, notes),
	}

	// Post the reply to the thread
	_, err := h.SlackClient.PostReply(channelID, threadTS, message)
	if err != nil {
		return fmt.Errorf("error posting test result to Slack thread: %w", err)
	}

	// Update ServiceNow with the test result
	body := map[string]string{
		"test_status": status,
		"notes":       notes,
		"updated_by":  userID,
	}

	_, err = h.ServiceNowClient.makeRequest("PATCH", fmt.Sprintf("api/now/table/sn_policy_control_test/%s", testID), body)
	if err != nil {
		return fmt.Errorf("error updating test results in ServiceNow: %w", err)
	}

	return nil
}

// ProcessControlCommand handles slash commands for control testing
func (h *PolicyControlHandler) ProcessControlCommand(command *slack.Command) (string, error) {
	// Handle different control testing commands
	switch {
	case command.Command == "/submit-test":
		// Format: /submit-test TEST_ID PASS|FAIL Notes about the test
		parts := strings.SplitN(command.Text, " ", 3)
		if len(parts) < 3 {
			return "Invalid command format. Usage: /submit-test TEST_ID PASS|FAIL Notes about the test", nil
		}

		testID := parts[0]
		status := strings.ToUpper(parts[1])
		notes := parts[2]

		if status != "PASS" && status != "FAIL" {
			return "Status must be either PASS or FAIL", nil
		}

		err := h.HandleTestResultSubmission(testID, command.ChannelID, "", command.UserID, status, notes)
		if err != nil {
			return fmt.Sprintf("Error submitting test results: %s", err), nil
		}

		return "Test results submitted successfully!", nil

	default:
		return "Unknown command", nil
	}
}
