// backend/internal/integrations/servicenow/audit_management.go
package servicenow

import (
	"fmt"
	"strings"
	"time"

	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/jira"
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
	JiraClient       *jira.Client
}

// NewAuditHandler creates a new audit handler
func NewAuditHandler(serviceNowClient *Client, slackClient *slack.Client, jiraClient *jira.Client) *AuditHandler {
	return &AuditHandler{
		ServiceNowClient: serviceNowClient,
		SlackClient:      slackClient,
		JiraClient:       jiraClient,
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

	//-------------- JIRA FUNCTION CALLS ---------------------------

	// Create a Jira ticket for the audit finding
	jiraTicket, err := h.createJiraTicketForFinding(finding)
	if err != nil {
		// We don't want to fail the whole process if Jira creation fails
		// Just log the error and continue
		fmt.Printf("Error creating Jira ticket for finding %s: %s\n", finding.ID, err)
	} else {
		// Update the Slack message with the Jira ticket information
		err = h.updateSlackWithJiraInfo(slack.ChannelMapping["audit"], ts, jiraTicket)
		if err != nil {
			fmt.Printf("Error updating Slack message with Jira info: %s\n", err)
		}

		// Update ServiceNow with the Jira ticket ID
		err = h.updateServiceNowWithJiraInfo(finding.ID, jiraTicket.Key)
		if err != nil {
			fmt.Printf("Error updating ServiceNow with Jira info: %s\n", err)
		}
	}

	return ts, nil
}

// ---------------------- SLACK FUNCTIONS --------------------------------------------------------------------

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

		// Update the Jira ticket if available
		err = h.updateJiraFromSlackResolution(findingID, resolution)
		if err != nil {
			// Log error but don't fail the whole operation
			fmt.Printf("Error updating Jira from Slack resolution: %s\n", err)
		}

		return "Audit finding resolved successfully!", nil

	default:
		return "Unknown command", nil
	}
}

//--------------------------- JIRA FUNCTIONS -----------------------------------------------------------------

func (h *AuditHandler) createJiraTicketForFinding(finding AuditFinding) (*jira.Ticket, error) {
	// Create labels for better organization and filtering in Jira
	labels := []string{"audit-finding", strings.ToLower(finding.Severity)}
	if finding.Audit != "" {
		// Convert audit name to a valid label by removing spaces and special chars
		auditLabel := strings.ToLower(strings.ReplaceAll(finding.Audit, " ", "-"))
		labels = append(labels, auditLabel)
	}

	// Add more context to the description
	description := fmt.Sprintf(`*Audit Finding from ServiceNow*

*ID:* %s
*Audit:* %s
*Severity:* %s
*Due Date:* %s
*State:* %s 
*Created On:* %s

*Description:*
%s

_This ticket was automatically created from a ServiceNow audit finding. Updates made here will be synced back to ServiceNow._`,
		finding.Number,
		finding.Audit,
		finding.Severity,
		finding.DueDate.Format("Jan 2, 2006"),
		finding.State,
		finding.CreatedOn.Format("Jan 2, 2006"),
		finding.Description)

	// Create a new Jira ticket
	ticket := &jira.Ticket{
		Project:     "AUDIT", // Configure your Jira project key
		IssueType:   "Audit Finding",
		Summary:     fmt.Sprintf("[%s] %s", finding.Number, finding.ShortDesc),
		Description: description,
		Priority:    mapSeverityToPriority(finding.Severity),
		DueDate:     finding.DueDate,
		Labels:      labels,
		Fields: map[string]interface{}{
			"customfield_servicenow_id": finding.ID,    // Custom field to store ServiceNow ID
			"customfield_audit_name":    finding.Audit, // Additional custom field to make searching easier
		},
	}

	// Log the attempt to create a Jira ticket
	fmt.Printf("Creating Jira ticket for finding %s (%s)\n", finding.Number, finding.ShortDesc)

	// Create the ticket in Jira
	createdTicket, err := h.JiraClient.CreateIssue(ticket)
	if err != nil {
		return nil, fmt.Errorf("error creating Jira ticket: %w", err)
	}

	fmt.Printf("Successfully created Jira ticket %s for finding %s\n", createdTicket.Key, finding.Number)
	return createdTicket, nil
}

// updateJiraFromSlackResolution updates the Jira ticket when a finding is resolved from Slack
func (h *AuditHandler) updateJiraFromSlackResolution(findingID, resolution string) error {
	// First, get the Jira ticket ID from ServiceNow
	finding, err := h.ServiceNowClient.getFinding(findingID)
	if err != nil {
		return fmt.Errorf("error getting finding from ServiceNow: %w", err)
	}

	jiraKey, ok := finding["jira_ticket"].(string)
	if !ok || jiraKey == "" {
		return fmt.Errorf("no Jira ticket associated with this finding")
	}

	// Update the Jira ticket
	update := &jira.TicketUpdate{
		Status:     "Done",
		Resolution: "Fixed",
		Comment:    fmt.Sprintf("Resolution from ServiceNow: %s", resolution),
	}

	err = h.JiraClient.UpdateIssue(jiraKey, update)
	if err != nil {
		return fmt.Errorf("error updating Jira ticket: %w", err)
	}

	return nil
}

// mapSeverityToPriority maps ServiceNow severity to Jira priority
func mapSeverityToPriority(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return "Highest"
	case "high":
		return "High"
	case "medium":
		return "Medium"
	case "low":
		return "Low"
	default:
		return "Medium"
	}
}

// updateSlackWithJiraInfo updates the Slack message with Jira ticket information
func (h *AuditHandler) updateSlackWithJiraInfo(channel, threadTS string, jiraTicket *jira.Ticket) error {
	// Create a reply with Jira information
	message := slack.Message{
		Text: fmt.Sprintf("📎 Jira ticket created: <%s/browse/%s|%s>",
			h.JiraClient.BaseURL, jiraTicket.Key, jiraTicket.Key),
	}

	// Post the reply to the thread
	_, err := h.SlackClient.PostReply(channel, threadTS, message)
	if err != nil {
		return fmt.Errorf("error posting Jira information to Slack thread: %w", err)
	}

	return nil
}

// updateServiceNowWithJiraInfo updates the ServiceNow finding with Jira ticket ID
func (h *AuditHandler) updateServiceNowWithJiraInfo(findingID, jiraKey string) error {
	// Update ServiceNow with the Jira ticket info
	body := map[string]string{
		"jira_ticket": jiraKey, // Assuming there's a field for Jira ticket in ServiceNow
	}

	_, err := h.ServiceNowClient.makeRequest("PATCH", fmt.Sprintf("api/now/table/sn_audit_finding/%s", findingID), body)
	if err != nil {
		return fmt.Errorf("error updating audit finding with Jira info in ServiceNow: %w", err)
	}

	return nil
}

// HandleJiraUpdate processes updates from Jira and syncs them to ServiceNow
func (h *AuditHandler) HandleJiraUpdate(jiraEvent *jira.WebhookEvent) error {
	// Get the ServiceNow ID from the custom field
	servicenowID, ok := jiraEvent.Issue.Fields.CustomFields["customfield_servicenow_id"].(string)
	if !ok || servicenowID == "" {
		return fmt.Errorf("no ServiceNow ID found in Jira ticket")
	}

	// Get the current status and resolution from Jira
	status := jiraEvent.Issue.Fields.Status.Name
	resolution := ""
	if jiraEvent.Issue.Fields.Resolution != nil {
		resolution = jiraEvent.Issue.Fields.Resolution.Name
	}

	// Get the comment if this was a comment event
	comment := ""
	if jiraEvent.Comment != nil {
		comment = jiraEvent.Comment.Body
	}

	// Map Jira status to ServiceNow state
	var servicenowState string
	var servicenowResolution string

	switch status {
	case "To Do":
		servicenowState = "open"
	case "In Progress":
		servicenowState = "in_progress"
	case "Done":
		servicenowState = "resolved"
		// Use the Jira resolution or comment as ServiceNow resolution
		if resolution != "" {
			servicenowResolution = fmt.Sprintf("Resolved in Jira as: %s", resolution)
		} else if comment != "" {
			servicenowResolution = fmt.Sprintf("From Jira: %s", comment)
		} else {
			servicenowResolution = "Resolved in Jira"
		}
	}

	// Update ServiceNow with the new state and resolution if applicable
	body := map[string]string{
		"state": servicenowState,
	}

	if servicenowResolution != "" {
		body["resolution"] = servicenowResolution
	}

	// Also add any comments to ServiceNow audit log
	if comment != "" {
		body["work_notes"] = fmt.Sprintf("Update from Jira: %s", comment)
	}

	_, err := h.ServiceNowClient.makeRequest("PATCH", fmt.Sprintf("api/now/table/sn_audit_finding/%s", servicenowID), body)
	if err != nil {
		return fmt.Errorf("error updating ServiceNow from Jira update: %w", err)
	}

	// If the status changed, post an update to Slack as well
	if servicenowState != "" {
		// You would need to retrieve the original Slack thread details from a database
		// For this example, we'll assume you have a way to get this information
		channelID := slack.ChannelMapping["audit"] // This should be retrieved dynamically
		threadTS := ""                             // This should be retrieved from your database

		if threadTS != "" {
			message := slack.Message{
				Text: func() string {
					commentText := ""
					if comment != "" {
						commentText = fmt.Sprintf("\nComment: %s", comment)
					}
					return fmt.Sprintf("🔄 Jira ticket updated: Status changed to *%s*%s", status, commentText)
				}(),
			}

			_, err := h.SlackClient.PostReply(channelID, threadTS, message)
			if err != nil {
				fmt.Printf("Error posting Jira update to Slack: %s\n", err)
			}
		}
	}

	return nil
}
