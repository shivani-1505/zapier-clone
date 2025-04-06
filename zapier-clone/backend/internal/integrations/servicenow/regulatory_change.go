// backend/internal/integrations/servicenow/regulatory_change.go
package servicenow

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/slack"
)

// RegulatoryChange represents a regulatory change in ServiceNow GRC
type RegulatoryChange struct {
	ID                 string    `json:"sys_id"`
	Number             string    `json:"number"`
	ShortDesc          string    `json:"short_description"`
	Description        string    `json:"description"`
	Regulation         string    `json:"regulation_name"`
	Jurisdiction       string    `json:"jurisdiction"`
	EffectiveDate      time.Time `json:"effective_date"`
	State              string    `json:"state"`
	AssignedTo         string    `json:"assigned_to"`
	CreatedOn          time.Time `json:"sys_created_on"`
	LastUpdated        time.Time `json:"sys_updated_on"`
	ImpactAssessment   string    `json:"impact_assessment"`
	ImplementationPlan string    `json:"implementation_plan"`
}

// RegulatoryChangeHandler handles regulatory change notifications and interactions
type RegulatoryChangeHandler struct {
	ServiceNowClient *Client
	SlackClient      *slack.Client
}

// NewRegulatoryChangeHandler creates a new regulatory change handler
func NewRegulatoryChangeHandler(serviceNowClient *Client, slackClient *slack.Client) *RegulatoryChangeHandler {
	return &RegulatoryChangeHandler{
		ServiceNowClient: serviceNowClient,
		SlackClient:      slackClient,
	}
}

// HandleNewRegulatoryChange processes a new regulatory change and notifies Slack
func (h *RegulatoryChangeHandler) HandleNewRegulatoryChange(change RegulatoryChange) (string, error) {
	// Create a Slack message for the regulatory change
	message := slack.Message{
		Blocks: []slack.Block{
			{
				Type: "header",
				Text: slack.NewTextObject("plain_text", fmt.Sprintf("📢 New %s - Effective %s", change.ShortDesc, change.EffectiveDate.Format("January 2006")), true),
			},
			{
				Type: "section",
				Fields: []*slack.TextObject{
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Change ID:*\n%s", change.Number), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Regulation:*\n%s", change.Regulation), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Jurisdiction:*\n%s", change.Jurisdiction), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Effective Date:*\n%s", change.EffectiveDate.Format("Jan 2, 2006")), false),
				},
			},
			{
				Type: "section",
				Text: slack.NewTextObject("mrkdwn", fmt.Sprintf("*Description:*\n%s", change.Description), false),
			},
			{
				Type: "actions",
				Elements: []interface{}{
					map[string]interface{}{
						"type": "button",
						"text": map[string]interface{}{
							"type":  "plain_text",
							"text":  "Add Impact Assessment",
							"emoji": true,
						},
						"value":     fmt.Sprintf("impact_%s", change.ID),
						"action_id": "add_impact_assessment",
					},
					map[string]interface{}{
						"type": "button",
						"text": map[string]interface{}{
							"type":  "plain_text",
							"text":  "Create Implementation Plan",
							"emoji": true,
						},
						"value":     fmt.Sprintf("implement_%s", change.ID),
						"action_id": "create_implementation_plan",
					},
					map[string]interface{}{
						"type": "button",
						"text": map[string]interface{}{
							"type":  "plain_text",
							"text":  "View in ServiceNow",
							"emoji": true,
						},
						"url":       fmt.Sprintf("%s/nav_to.do?uri=sn_regulatory_change.do?sys_id=%s", h.ServiceNowClient.BaseURL, change.ID),
						"action_id": "view_regulatory_change",
					},
				},
			},
			{
				Type: "context",
				Elements: []interface{}{
					map[string]interface{}{
						"type": "mrkdwn",
						"text": "🏛 Review Required by Legal and Compliance Teams.",
					},
				},
			},
		},
	}

	// Post the message to the regulatory-updates channel
	ts, err := h.SlackClient.PostMessage(slack.ChannelMapping["regulatory"], message)
	if err != nil {
		return "", fmt.Errorf("error posting regulatory change message to Slack: %w", err)
	}

	return ts, nil
}

// HandleRegulatoryChangeUpdate processes updates from Jira for regulatory changes
func (h *RegulatoryChangeHandler) HandleRegulatoryChangeUpdate(changeID, jiraKey, status, description string) error {
	// Map Jira status to ServiceNow state
	state := "in_progress"
	if status == "Done" {
		state = "implemented"
	}

	// Update the regulatory change in ServiceNow
	body := map[string]string{
		"state":                state,
		"implementation_notes": fmt.Sprintf("Update from Jira: %s", description),
	}

	_, err := h.ServiceNowClient.makeRequest("PATCH",
		fmt.Sprintf("api/now/table/sn_regulatory_change/%s", changeID), body)
	if err != nil {
		return fmt.Errorf("error updating regulatory change from Jira: %w", err)
	}

	log.Printf("Updated ServiceNow regulatory change %s from Jira %s (state: %s)",
		changeID, jiraKey, state)

	return nil
}

// HandleImpactAssessment processes an impact assessment for a regulatory change
func (h *RegulatoryChangeHandler) HandleImpactAssessment(changeID, channelID, threadTS, userID, assessment string) error {
	// Post the impact assessment to the thread
	message := slack.Message{
		Text: fmt.Sprintf("<@%s> has provided an impact assessment:\n```%s```", userID, assessment),
	}

	// Post the reply to the thread
	_, err := h.SlackClient.PostReply(channelID, threadTS, message)
	if err != nil {
		return fmt.Errorf("error posting impact assessment to Slack thread: %w", err)
	}

	// Update ServiceNow with the impact assessment
	body := map[string]string{
		"impact_assessment": assessment,
		"state":             "assessed",
	}

	_, err = h.ServiceNowClient.makeRequest("PATCH", fmt.Sprintf("api/now/table/sn_regulatory_change/%s", changeID), body)
	if err != nil {
		return fmt.Errorf("error updating impact assessment in ServiceNow: %w", err)
	}

	return nil
}

// HandleImplementationPlan processes an implementation plan for a regulatory change
func (h *RegulatoryChangeHandler) HandleImplementationPlan(changeID, channelID, threadTS, userID, plan string) error {
	// Post the implementation plan to the thread
	message := slack.Message{
		Text: fmt.Sprintf("<@%s> has created an implementation plan:\n```%s```", userID, plan),
	}

	// Post the reply to the thread
	_, err := h.SlackClient.PostReply(channelID, threadTS, message)
	if err != nil {
		return fmt.Errorf("error posting implementation plan to Slack thread: %w", err)
	}

	// Update ServiceNow with the implementation plan
	body := map[string]string{
		"implementation_plan": plan,
		"state":               "implementation_planned",
	}

	_, err = h.ServiceNowClient.makeRequest("PATCH", fmt.Sprintf("api/now/table/sn_regulatory_change/%s", changeID), body)
	if err != nil {
		return fmt.Errorf("error updating implementation plan in ServiceNow: %w", err)
	}

	return nil
}

// ProcessRegulatoryCommand handles slash commands for regulatory changes
func (h *RegulatoryChangeHandler) ProcessRegulatoryCommand(command *slack.Command) (string, error) {
	// Handle different regulatory change commands
	switch {
	case command.Command == "/assess-impact":
		// Format: /assess-impact CHANGE_ID Impact assessment details
		parts := strings.SplitN(command.Text, " ", 2)
		if len(parts) < 2 {
			return "Invalid command format. Usage: /assess-impact CHANGE_ID Impact assessment details", nil
		}

		changeID := parts[0]
		assessment := parts[1]

		err := h.HandleImpactAssessment(changeID, command.ChannelID, "", command.UserID, assessment)
		if err != nil {
			return fmt.Sprintf("Error adding impact assessment: %s", err), nil
		}

		return "Impact assessment added successfully!", nil

	case command.Command == "/plan-implementation":
		// Format: /plan-implementation CHANGE_ID Implementation plan details
		parts := strings.SplitN(command.Text, " ", 2)
		if len(parts) < 2 {
			return "Invalid command format. Usage: /plan-implementation CHANGE_ID Implementation plan details", nil
		}

		changeID := parts[0]
		plan := parts[1]

		err := h.HandleImplementationPlan(changeID, command.ChannelID, "", command.UserID, plan)
		if err != nil {
			return fmt.Sprintf("Error creating implementation plan: %s", err), nil
		}

		return "Implementation plan created successfully!", nil

	default:
		return "Unknown command", nil
	}
}
