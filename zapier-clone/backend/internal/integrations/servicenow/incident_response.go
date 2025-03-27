// backend/internal/integrations/servicenow/incident_response.go
package servicenow

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/jira"
	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/slack"
)

// IncidentHandler handles security incident notifications and interactions
type IncidentHandler struct {
	ServiceNowClient    *Client
	SlackClient         *slack.Client
	JiraClient          *jira.Client
	IncidentJiraMapping *jira.IncidentJiraMapping
}

// NewIncidentHandler creates a new incident handler
func NewIncidentHandler(serviceNowClient *Client, slackClient *slack.Client, jiraClient *jira.Client) *IncidentHandler {
	incidentJiraMapping, err := jira.NewIncidentJiraMapping("./data")
	if err != nil {
		log.Printf("Warning: Failed to initialize incident-jira mapping: %v", err)
		// Create an empty mapping as fallback
		incidentJiraMapping = &jira.IncidentJiraMapping{
			IncidentIDToJiraKey: make(map[string]string),
			JiraKeyToIncidentID: make(map[string]string),
		}
	}

	return &IncidentHandler{
		ServiceNowClient:    serviceNowClient,
		SlackClient:         slackClient,
		JiraClient:          jiraClient,
		IncidentJiraMapping: incidentJiraMapping,
	}
}

// HandleNewIncident processes a new security incident and notifies Slack
func (h *IncidentHandler) HandleNewIncident(incident Incident) (string, error) {
	// Determine the emoji based on severity
	var severityEmoji string
	switch strings.ToLower(incident.Severity) {
	case "critical":
		severityEmoji = "🔴"
	case "high":
		severityEmoji = "🟠"
	case "medium":
		severityEmoji = "🟡"
	default:
		severityEmoji = "🟢"
	}

	// Create Jira epic for the incident
	epic, err := h.createJiraEpic(incident)
	if err != nil {
		log.Printf("Error creating Jira epic for incident %s: %v", incident.ID, err)
		// Continue execution - we'll just post to Slack without the Jira integration
	} else {
		// Save the mapping between ServiceNow incident and Jira epic
		err = h.IncidentJiraMapping.AddMapping(incident.ID, epic.Key)
		if err != nil {
			log.Printf("Error saving incident-jira mapping: %v", err)
		}

		// Create standard subtasks for incident response
		h.createIncidentSubtasks(incident, epic.Key)
	}

	// Create a Slack message for the incident
	message := slack.Message{
		Blocks: []slack.Block{
			{
				Type: "header",
				Text: slack.NewTextObject("plain_text", fmt.Sprintf("⚠️ Urgent: %s", incident.ShortDesc), true),
			},
			{
				Type: "section",
				Fields: []*slack.TextObject{
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Incident ID:*\n%s", incident.Number), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Category:*\n%s", incident.Category), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Severity:*\n%s %s", severityEmoji, incident.Severity), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Impact:*\n%s", incident.Impact), false),
				},
			},
			{
				Type: "section",
				Text: slack.NewTextObject("mrkdwn", fmt.Sprintf("*Description:*\n%s", incident.Description), false),
			},
			{
				Type: "actions",
				Elements: []interface{}{
					map[string]interface{}{
						"type": "button",
						"text": map[string]interface{}{
							"type":  "plain_text",
							"text":  "🚨 Acknowledge",
							"emoji": true,
						},
						"style":     "primary",
						"value":     fmt.Sprintf("ack_incident_%s", incident.ID),
						"action_id": "acknowledge_incident",
					},
					map[string]interface{}{
						"type": "button",
						"text": map[string]interface{}{
							"type":  "plain_text",
							"text":  "📝 Add Update",
							"emoji": true,
						},
						"value":     fmt.Sprintf("update_incident_%s", incident.ID),
						"action_id": "update_incident",
					},
					map[string]interface{}{
						"type": "button",
						"text": map[string]interface{}{
							"type":  "plain_text",
							"text":  "✅ Resolve",
							"emoji": true,
						},
						"style":     "danger",
						"value":     fmt.Sprintf("resolve_incident_%s", incident.ID),
						"action_id": "resolve_incident",
					},
					map[string]interface{}{
						"type": "button",
						"text": map[string]interface{}{
							"type":  "plain_text",
							"text":  "View in ServiceNow",
							"emoji": true,
						},
						"url":       fmt.Sprintf("%s/nav_to.do?uri=sn_si_incident.do?sys_id=%s", h.ServiceNowClient.BaseURL, incident.ID),
						"action_id": "view_incident",
					},
				},
			},
			{
				Type: "context",
				Elements: []interface{}{
					map[string]interface{}{
						"type": "mrkdwn",
						"text": "📍 Immediate Action Required! Security, IT, and Legal teams should coordinate response.",
					},
				},
			},
		},
	}

	// Post the message to the incident-response channel
	ts, err := h.SlackClient.PostMessage(slack.ChannelMapping["incident"], message)
	if err != nil {
		return "", fmt.Errorf("error posting incident message to Slack: %w", err)
	}

	// For critical incidents, also add a reaction to draw attention
	if strings.ToLower(incident.Severity) == "critical" {
		err = h.SlackClient.AddReaction(slack.ChannelMapping["incident"], ts, "rotating_light")
		if err != nil {
			// Non-fatal error, just log it
			fmt.Printf("Error adding reaction to incident message: %v\n", err)
		}
	}

	return ts, nil
}

// HandleIncidentAcknowledgment processes incident acknowledgment
func (h *IncidentHandler) HandleIncidentAcknowledgment(incidentID, channelID, threadTS, userID string) error {
	// Post the acknowledgment notification to the thread
	message := slack.Message{
		Text: fmt.Sprintf("<@%s> has acknowledged this incident and is investigating.", userID),
	}

	// Post the reply to the thread
	_, err := h.SlackClient.PostReply(channelID, threadTS, message)
	if err != nil {
		return fmt.Errorf("error posting incident acknowledgment to Slack thread: %w", err)
	}

	// Update ServiceNow with the assignment and state change
	body := map[string]string{
		"assigned_to": userID,
		"state":       "in_progress",
		"work_notes":  fmt.Sprintf("Incident acknowledged by %s via Slack integration", userID),
	}

	_, err = h.ServiceNowClient.makeRequest("PATCH", fmt.Sprintf("api/now/table/sn_si_incident/%s", incidentID), body)
	if err != nil {
		return fmt.Errorf("error updating incident acknowledgment in ServiceNow: %w", err)
	}

	return nil
}

// HandleIncidentUpdate processes incident status updates
func (h *IncidentHandler) HandleIncidentUpdate(incidentID, channelID, threadTS, userID, updateText string) error {
	// Post the update to the thread
	message := slack.Message{
		Text: fmt.Sprintf("<@%s> provided an update: %s", userID, updateText),
	}

	// Post the reply to the thread
	_, err := h.SlackClient.PostReply(channelID, threadTS, message)
	if err != nil {
		return fmt.Errorf("error posting incident update to Slack thread: %w", err)
	}

	// Update ServiceNow with the note
	body := map[string]string{
		"work_notes": fmt.Sprintf("Update from %s via Slack: %s", userID, updateText),
	}

	_, err = h.ServiceNowClient.makeRequest("PATCH", fmt.Sprintf("api/now/table/sn_si_incident/%s", incidentID), body)
	if err != nil {
		return fmt.Errorf("error updating incident notes in ServiceNow: %w", err)
	}

	// Add the update as a comment to the corresponding Jira epic if one exists
	jiraKey, exists := h.IncidentJiraMapping.GetJiraKeyFromIncidentID(incidentID)
	if exists {
		commentText := fmt.Sprintf("Update from %s:\n\n%s", userID, updateText)
		if err := h.JiraClient.AddComment(jiraKey, commentText); err != nil {
			log.Printf("Error adding update comment to Jira: %s\n", err)
		}
	}

	return nil
}

// HandleIncidentResolution processes incident resolution
func (h *IncidentHandler) HandleIncidentResolution(incidentID, channelID, threadTS, userID, resolutionNotes string) error {
	// Post the resolution to the thread
	message := slack.Message{
		Text: fmt.Sprintf("<@%s> has resolved this incident: %s", userID, resolutionNotes),
	}

	// Post the reply to the thread
	_, err := h.SlackClient.PostReply(channelID, threadTS, message)
	if err != nil {
		return fmt.Errorf("error posting incident resolution to Slack thread: %w", err)
	}

	// Update ServiceNow with the resolution
	body := map[string]string{
		"state":            "resolved",
		"resolution_notes": resolutionNotes,
		"resolved_by":      userID,
		"resolved_at":      time.Now().Format(time.RFC3339),
	}

	_, err = h.ServiceNowClient.makeRequest("PATCH", fmt.Sprintf("api/now/table/sn_si_incident/%s", incidentID), body)
	if err != nil {
		return fmt.Errorf("error updating incident resolution in ServiceNow: %w", err)
	}

	return nil
}

// ProcessIncidentCommand handles slash commands for incidents
func (h *IncidentHandler) ProcessIncidentCommand(command *slack.Command) (string, error) {
	// Handle different incident commands
	switch {
	case command.Command == "/incident-update":
		// Format: /incident-update INCIDENT_ID UPDATE_TEXT
		parts := strings.SplitN(command.Text, " ", 2)
		if len(parts) < 2 {
			return "Invalid command format. Usage: /incident-update INCIDENT_ID UPDATE_TEXT", nil
		}

		incidentID := parts[0]
		updateText := parts[1]

		err := h.HandleIncidentUpdate(incidentID, command.ChannelID, "", command.UserID, updateText)
		if err != nil {
			return fmt.Sprintf("Error updating incident: %s", err), nil
		}

		return "Incident update posted successfully!", nil

	case command.Command == "/resolve-incident":
		// Format: /resolve-incident INCIDENT_ID RESOLUTION_NOTES
		parts := strings.SplitN(command.Text, " ", 2)
		if len(parts) < 2 {
			return "Invalid command format. Usage: /resolve-incident INCIDENT_ID RESOLUTION_NOTES", nil
		}

		incidentID := parts[0]
		resolutionNotes := parts[1]

		err := h.HandleIncidentResolution(incidentID, command.ChannelID, "", command.UserID, resolutionNotes)
		if err != nil {
			return fmt.Sprintf("Error resolving incident: %s", err), nil
		}

		return "Incident resolved successfully!", nil

	default:
		return "Unknown command", nil
	}
}

// createJiraEpic creates a Jira epic for an incident
func (h *IncidentHandler) createJiraEpic(incident Incident) (*jira.Ticket, error) {
	// Map ServiceNow incident severity to Jira priority
	priority := "Medium" // Default priority
	switch strings.ToLower(incident.Severity) {
	case "critical":
		priority = "Highest"
	case "high":
		priority = "High"
	case "medium":
		priority = "Medium"
	case "low":
		priority = "Low"
	}

	// Create formatted description with details from ServiceNow
	description := fmt.Sprintf(`*Incident Details from ServiceNow*
    
*Incident Number:* %s
*Category:* %s
*Severity:* %s
*Impact:* %s
    
*Description:*
%s

----
This epic was automatically created from ServiceNow Incident %s.
Please update both systems when changes are made.`,
		incident.Number,
		incident.Category,
		incident.Severity,
		incident.Impact,
		incident.Description,
		incident.Number)

	// Create an Epic in the "Incident Response" project
	ticket := &jira.Ticket{
		Project:     h.JiraClient.ProjectKey, // Use "IR" or another project key for Incident Response
		IssueType:   "Epic",
		Summary:     fmt.Sprintf("[INCIDENT] %s", incident.ShortDesc),
		Description: description,
		Priority:    priority,
		Labels:      []string{"security-incident", "auto-created", strings.ToLower(incident.Category)},
		Epic: &jira.EpicDetails{
			Name:  fmt.Sprintf("Incident: %s", incident.ShortDesc),
			Color: "red", // Color for the epic
		},
	}

	// Create the Jira epic
	return h.JiraClient.CreateIssue(ticket)
}

// createIncidentSubtasks creates standard subtasks for incident response
func (h *IncidentHandler) createIncidentSubtasks(incident Incident, epicKey string) {
	// Define standard subtasks for incident response
	subtasks := []struct {
		title       string
		description string
	}{
		{
			title: "Investigation",
			description: `*Investigation Phase*
            
Investigate the incident and determine:
- Root cause
- Impact assessment
- Affected systems and data
- Timeline of events

Document all findings thoroughly.`,
		},
		{
			title: "Containment",
			description: `*Containment Phase*
            
Implement immediate actions to:
- Stop the incident from spreading
- Isolate affected systems
- Preserve evidence
- Mitigate immediate threats

Document all containment actions taken.`,
		},
		{
			title: "Remediation",
			description: `*Remediation Phase*
            
Develop and implement remediation plans:
- Remove root cause
- Restore systems securely
- Apply security patches or controls
- Verify remediation effectiveness

Document all remediation steps.`,
		},
		{
			title: "Lessons Learned",
			description: `*Lessons Learned Phase*
            
Conduct post-incident analysis:
- What worked well
- What could be improved
- Process/policy adjustments needed
- Security control recommendations

Update documentation and prepare final report.`,
		},
	}

	// Create each subtask
	for _, task := range subtasks {
		subtask := &jira.Ticket{
			Project:     h.JiraClient.ProjectKey,
			IssueType:   "Task",
			Summary:     fmt.Sprintf("%s - %s", task.title, incident.ShortDesc),
			Description: task.description,
			Parent:      epicKey,
			Priority:    "High",
			Labels:      []string{"security-incident", "auto-created"},
		}

		// Create the subtask
		result, err := h.JiraClient.CreateIssue(subtask)
		if err != nil {
			log.Printf("Error creating %s subtask for incident %s: %v", task.title, incident.ID, err)
		} else {
			log.Printf("Created %s subtask %s for incident %s", task.title, result.Key, incident.ID)
		}
	}
}
