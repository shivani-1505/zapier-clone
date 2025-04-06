// backend/internal/api/handlers/jira_webhook.go
package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

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
	ServiceNowBaseURL  string
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
		ServiceNowBaseURL:  "http://localhost:3000",
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

// createRiskInServiceNow creates a risk record in ServiceNow from a Jira issue
func (h *JiraWebhookHandler) createRiskInServiceNow(event *jira.WebhookEvent) error {
	// Extract fields
	summary := event.Issue.Fields.Summary
	description := event.Issue.Fields.Description

	// Determine severity based on Jira priority
	severity := "Medium" // Default
	if event.Issue.Fields.Priority != nil {
		priorityName := event.Issue.Fields.Priority.Name
		switch priorityName {
		case "Highest", "High":
			severity = "High"
		case "Medium":
			severity = "Medium"
		case "Low", "Lowest":
			severity = "Low"
		}
	}

	// Create risk data
	riskData := map[string]interface{}{
		"title":       summary,
		"description": description,
		"severity":    severity,
		"status":      "New",
		"category":    "Operational",
		"jira_key":    event.Issue.Key,
	}

	// Send to ServiceNow
	if err := h.createOrUpdateServiceNowRecord("sn_risk_risk", riskData); err != nil {
		return fmt.Errorf("error creating risk: %v", err)
	}

	// Try to get the created record to extract its ID
	query := fmt.Sprintf("jira_key=%s", event.Issue.Key)
	resp, err := h.getServiceNowRecord("sn_risk_risk", query)
	if err != nil {
		log.Printf("Warning: Could not get ServiceNow ID for created risk: %v", err)
		return nil
	}

	// Extract the ServiceNow ID from the response
	sysID, ok := resp["sys_id"].(string)
	if !ok {
		return fmt.Errorf("invalid sys_id in response")
	}

	// Update the Jira issue with the ServiceNow ID
	ticketUpdate := &jira.TicketUpdate{
		Fields: map[string]interface{}{
			"customfield_servicenow_id": sysID,
		},
	}

	if err := h.JiraClient.UpdateIssue(event.Issue.Key, ticketUpdate); err != nil {
		log.Printf("Warning: Failed to update Jira issue with ServiceNow ID: %v", err)
	}

	log.Printf("Created risk in ServiceNow with ID %s for Jira issue %s", sysID, event.Issue.Key)
	return nil
}

// createOrUpdateServiceNowRecord creates or updates a record in ServiceNow
func (h *JiraWebhookHandler) createOrUpdateServiceNowRecord(table string, data map[string]interface{}) error {
	// Serialize the data
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshaling ServiceNow data: %v", err)
	}

	log.Printf("Sending request to ServiceNow: %s", string(jsonData))

	// Create the request URL
	url := fmt.Sprintf("%s/api/now/table/%s", h.ServiceNowBaseURL, table)

	// Create the HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request to ServiceNow: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %v", err)
	}
	log.Printf("Raw ServiceNow response (status %d): %s", resp.StatusCode, string(respBody))

	// ADD THIS: Reset the body for further processing
	resp.Body = io.NopCloser(bytes.NewBuffer(respBody))

	// Check response status
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ServiceNow API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Read and log the response
	var respData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return fmt.Errorf("error decoding ServiceNow response: %v", err)
	}

	log.Printf("ServiceNow record created/updated successfully: %v", respData)
	return nil
}

// getServiceNowRecord gets a record from ServiceNow by table and ID
func (h *JiraWebhookHandler) getServiceNowRecord(table, query string) (map[string]interface{}, error) {
	// Create the request URL
	url := fmt.Sprintf("%s/api/now/table/%s?sysparm_query=%s", h.ServiceNowBaseURL, table, query)

	// Create the HTTP request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/json")

	// Send the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request to ServiceNow: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ServiceNow API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Read and parse the response
	var respData struct {
		Result []map[string]interface{} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil, fmt.Errorf("error decoding ServiceNow response: %v", err)
	}

	// Check if we got any results
	if len(respData.Result) == 0 {
		return nil, fmt.Errorf("no records found in ServiceNow for query: %s", query)
	}

	return respData.Result[0], nil
}

func extractProjectKey(issueKey string) string {
	parts := strings.Split(issueKey, "-")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func (h *JiraWebhookHandler) handleIssueCreated(event *jira.WebhookEvent) {
	// Extract project key
	projectKey := extractProjectKey(event.Issue.Key)

	// Map to ServiceNow table
	serviceNowTable, _ := mapProjectToServiceNowTable(projectKey)

	// Prepare record data based on the table/project type
	var recordData map[string]interface{}

	switch serviceNowTable {
	case "sn_risk_risk":
		recordData = prepareRiskData(event.Issue)
	case "sn_compliance_task":
		recordData = prepareComplianceTaskData(event.Issue)
	case "sn_si_incident":
		recordData = prepareIncidentData(event.Issue)
	// Add cases for other types
	default:
		recordData = prepareGenericData(event.Issue)
	}

	// Create the record in ServiceNow
	if err := h.createOrUpdateServiceNowRecord(serviceNowTable, recordData); err != nil {
		log.Printf("Error creating ServiceNow record: %v", err)
	}
}

func prepareRiskData(issue *jira.WebhookIssue) map[string]interface{} {
	// Extract fields
	summary := issue.Fields.Summary
	description := issue.Fields.Description

	// Determine severity based on Jira priority
	severity := "Medium" // Default
	if issue.Fields.Priority != nil {
		priorityName := issue.Fields.Priority.Name
		switch priorityName {
		case "Highest", "High":
			severity = "High"
		case "Medium":
			severity = "Medium"
		case "Low", "Lowest":
			severity = "Low"
		}
	}

	// Create risk data
	return map[string]interface{}{
		"title":       summary,
		"description": description,
		"severity":    severity,
		"status":      "New",
		"category":    "Operational",
		"jira_key":    issue.Key,
	}
}

// prepareComplianceTaskData prepares data for a ServiceNow compliance task from a Jira issue
func prepareComplianceTaskData(issue *jira.WebhookIssue) map[string]interface{} {
	return map[string]interface{}{
		"name":        issue.Fields.Summary,
		"description": issue.Fields.Description,
		"status":      "Open",
		"jira_key":    issue.Key,
		"priority":    mapJiraPriorityToServiceNowPriority(issue.Fields.Priority),
		"due_date":    issue.Fields.DueDate,
	}
}

// prepareIncidentData prepares data for a ServiceNow incident from a Jira issue
func prepareIncidentData(issue *jira.WebhookIssue) map[string]interface{} {
	return map[string]interface{}{
		"short_description": issue.Fields.Summary,
		"description":       issue.Fields.Description,
		"state":             "New",
		"impact":            mapJiraPriorityToServiceNowImpact(issue.Fields.Priority),
		"urgency":           mapJiraPriorityToServiceNowUrgency(issue.Fields.Priority),
		"jira_key":          issue.Key,
	}
}

// prepareGenericData prepares generic data for any ServiceNow record from a Jira issue
func prepareGenericData(issue *jira.WebhookIssue) map[string]interface{} {
	return map[string]interface{}{
		"name":        issue.Fields.Summary,
		"description": issue.Fields.Description,
		"state":       "New",
		"jira_key":    issue.Key,
	}
}

// Helper functions for mapping Jira values to ServiceNow values

// mapJiraPriorityToServiceNowPriority maps Jira priority to ServiceNow priority
func mapJiraPriorityToServiceNowPriority(priority *jira.WebhookPriority) string {
	if priority == nil {
		return "3" // Default to Medium priority
	}

	switch priority.Name {
	case "Highest":
		return "1" // Critical
	case "High":
		return "2" // High
	case "Medium":
		return "3" // Moderate
	case "Low":
		return "4" // Low
	case "Lowest":
		return "5" // Planning
	default:
		return "3" // Default to Medium priority
	}
}

// mapJiraPriorityToServiceNowImpact maps Jira priority to ServiceNow impact
func mapJiraPriorityToServiceNowImpact(priority *jira.WebhookPriority) string {
	if priority == nil {
		return "3" // Default to Medium impact
	}

	switch priority.Name {
	case "Highest":
		return "1" // High
	case "High":
		return "2" // Medium
	case "Medium", "Low", "Lowest":
		return "3" // Low
	default:
		return "3" // Default to Low impact
	}
}

// mapJiraPriorityToServiceNowUrgency maps Jira priority to ServiceNow urgency
func mapJiraPriorityToServiceNowUrgency(priority *jira.WebhookPriority) string {
	if priority == nil {
		return "3" // Default to Medium urgency
	}

	switch priority.Name {
	case "Highest":
		return "1" // High
	case "High":
		return "2" // Medium
	case "Medium", "Low", "Lowest":
		return "3" // Low
	default:
		return "3" // Default to Low urgency
	}
}

// handleIssueUpdated handles Jira issue updated events
func (h *JiraWebhookHandler) handleIssueUpdated(event *jira.WebhookEvent) {
	// Check if issue is linked to ServiceNow via customfield
	if event.Issue == nil {
		log.Printf("Invalid issue structure in webhook event")
		return
	}

	// Check if CustomFields is initialized before accessing
	customFieldValue, ok := event.Issue.Fields.CustomFields["customfield_servicenow_id"]
	if !ok {
		log.Printf("No ServiceNow ID custom field found in issue %s", event.Issue.Key)
		return
	}

	// Check if the value can be converted to string
	snID, ok := customFieldValue.(string)
	if !ok || snID == "" {
		log.Printf("Invalid or empty ServiceNow ID found in Jira issue %s", event.Issue.Key)
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

func getProjectKey(issueKey string) string {
	parts := strings.Split(issueKey, "-")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func mapProjectToServiceNowTable(projectKey string) (string, string) {
	switch projectKey {
	case "RISK":
		return "sn_risk_risk", "risk"
	case "COMP":
		return "sn_compliance_task", "compliance_task"
	case "INC":
		return "sn_si_incident", "incident"
	case "AUDIT":
		return "sn_audit_finding", "audit_finding"
	case "TEST", "CTRL":
		return "sn_policy_control_test", "control_test"
	case "VEN":
		return "sn_vendor_risk", "vendor_risk"
	case "REG":
		return "sn_regulatory_change", "regulatory_change"
	default:
		log.Printf("Warning: Unknown project key %s, defaulting to risk table", projectKey)
		return "sn_risk_risk", "risk" // Default to risk table
	}
}
