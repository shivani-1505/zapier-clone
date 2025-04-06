package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// JiraTicket represents a Jira issue in our mock system
type JiraTicket struct {
	ID          string                 `json:"id"`
	Key         string                 `json:"key"`
	Self        string                 `json:"self"`
	Type        string                 `json:"issuetype"` // Added field for ticket type (Epic, Task, Subtask)
	Summary     string                 `json:"summary"`
	Description string                 `json:"description"`
	Status      string                 `json:"status"`
	Resolution  string                 `json:"resolution,omitempty"`
	Priority    string                 `json:"priority,omitempty"`
	Assignee    string                 `json:"assignee,omitempty"`
	Created     string                 `json:"created"`
	Updated     string                 `json:"updated"`
	DueDate     string                 `json:"dueDate,omitempty"`
	Labels      []string               `json:"labels,omitempty"`
	Components  []string               `json:"components,omitempty"`
	Fields      map[string]interface{} `json:"fields,omitempty"`
	Comments    []JiraComment          `json:"comments,omitempty"`
}

// JiraComment represents a comment on a Jira issue
type JiraComment struct {
	ID      string `json:"id"`
	Body    string `json:"body"`
	Author  string `json:"author"`
	Created string `json:"created"`
}

// JiraTransition represents a status transition in Jira
type JiraTransition struct {
	ID   string            `json:"id"`
	Name string            `json:"name"`
	To   map[string]string `json:"to"`
}

// JiraWebhookPayload represents a webhook payload sent by Jira
type JiraWebhookPayload struct {
	WebhookEvent string                 `json:"webhookEvent"`
	Issue        map[string]interface{} `json:"issue,omitempty"`
	Comment      map[string]interface{} `json:"comment,omitempty"`
	Changelog    map[string]interface{} `json:"changelog,omitempty"`
	User         map[string]interface{} `json:"user,omitempty"`
}

// ServiceNowWebhookPayload represents a webhook payload sent by ServiceNow
type ServiceNowWebhookPayload struct {
	SysID      string                 `json:"sys_id"`
	TableName  string                 `json:"table_name"`
	ActionType string                 `json:"action_type"`
	Data       map[string]interface{} `json:"data"`
}

// MockDatabase holds our mock Jira data
var MockDatabase = struct {
	Tickets            map[string]JiraTicket
	Projects           map[string]interface{}
	ServiceNowJiraMap  map[string]string
	WebhookLog         []map[string]interface{}
	TicketCounters     map[string]int
	AutoCreateMappings map[string]bool
}{
	Tickets:            make(map[string]JiraTicket),
	Projects:           make(map[string]interface{}),
	ServiceNowJiraMap:  make(map[string]string),
	WebhookLog:         make([]map[string]interface{}, 0),
	TicketCounters:     make(map[string]int),
	AutoCreateMappings: make(map[string]bool),
}

// Initialize projects and mappings
func init() {
	MockDatabase.Projects = map[string]interface{}{
		"AUDIT": map[string]interface{}{
			"key":  "AUDIT",
			"name": "Audit Management",
		},
		"RISK": map[string]interface{}{
			"key":  "RISK",
			"name": "Risk Management",
		},
		"INC": map[string]interface{}{
			"key":  "INC",
			"name": "Incident Management",
		},
		"COMP": map[string]interface{}{
			"key":  "COMP",
			"name": "Compliance Tasks",
		},
		"VEN": map[string]interface{}{
			"key":  "VEN",
			"name": "Vendor Risk",
		},
		"REG": map[string]interface{}{
			"key":  "REG",
			"name": "Regulatory Changes",
		},
	}

	// Initialize counters for each project
	for key := range MockDatabase.Projects {
		MockDatabase.TicketCounters[key] = 0
	}

	// Set default auto-create mappings
	MockDatabase.AutoCreateMappings = map[string]bool{
		"sn_risk_risk":           true,
		"sn_compliance_task":     true,
		"sn_si_incident":         true,
		"sn_policy_control_test": true,
		"sn_audit_finding":       true,
		"sn_vendor_risk":         true,
		"sn_regulatory_change":   true,
	}
}

func main() {
	r := mux.NewRouter()

	// Jira REST API endpoints
	r.HandleFunc("/rest/api/2/issue", handleIssues).Methods("GET", "POST")
	r.HandleFunc("/rest/api/2/issue/{key}", handleIssueByKey).Methods("GET", "PUT", "DELETE")
	r.HandleFunc("/rest/api/2/issue/{key}/comment", handleComments).Methods("GET", "POST")
	r.HandleFunc("/rest/api/2/issue/{key}/transitions", handleTransitions).Methods("GET", "POST")
	r.HandleFunc("/rest/api/2/project", handleProjects).Methods("GET")
	r.HandleFunc("/rest/api/2/search", handleSearchIssues).Methods("GET")

	// Webhook receiver for ServiceNow
	r.HandleFunc("/api/webhooks/servicenow", handleServiceNowWebhook).Methods("POST")

	r.HandleFunc("/test_servicenow_connectivity", testServiceNowConnectivity).Methods("GET")

	// Webhook receiver for Jira
	r.HandleFunc("/api/webhooks/jira", handleReceiveWebhook).Methods("POST")

	// Webhook trigger endpoints
	r.HandleFunc("/trigger_webhook/{event_type}", triggerWebhook).Methods("POST")

	// Integration configuration
	r.HandleFunc("/api/config/auto_create", handleAutoCreateConfig).Methods("GET", "POST")

	// Webhook log access
	r.HandleFunc("/api/webhook_logs", getWebhookLogs).Methods("GET")

	// Reset endpoint
	r.HandleFunc("/reset", handleReset).Methods("POST")

	// Health check and UI
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "ok",
			"version":  "1.2.0",
			"database": fmt.Sprintf("%d tickets", len(MockDatabase.Tickets)),
		})
	}).Methods("GET")

	r.HandleFunc("/test_risk_creation", func(w http.ResponseWriter, r *http.Request) {
		// Create a test ServiceNow webhook payload for a risk
		payload := ServiceNowWebhookPayload{
			SysID:      fmt.Sprintf("RISK%d", time.Now().Unix()),
			TableName:  "sn_risk_risk",
			ActionType: "insert",
			Data: map[string]interface{}{
				"title":       fmt.Sprintf("Test Risk %s", time.Now().Format("15:04:05")),
				"description": "This is a test risk creation from the mock endpoint",
				"severity":    "High",
			},
		}

		// Process directly (as if received as webhook)
		createJiraTicketFromServiceNow(payload)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "success",
			"message": "Test risk creation processed",
			"details": fmt.Sprintf("Created risk %s", payload.SysID),
		})
	}).Methods("GET")

	r.HandleFunc("/", handleUI).Methods("GET")

	// Start server
	port := "4000"
	log.Printf("[JIRA MOCK] Starting mock Jira server on port %s...\n", port)

	log.Fatal(http.ListenAndServe(":"+port, r))
}

// JIRA API HANDLERS
func handleIssues(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		var results []JiraTicket
		for _, ticket := range MockDatabase.Tickets {
			results = append(results, ticket)
		}
		json.NewEncoder(w).Encode(results)

	case "POST":
		var requestData map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		fields, ok := requestData["fields"].(map[string]interface{})
		if !ok {
			http.Error(w, "Invalid fields format", http.StatusBadRequest)
			return
		}

		// Determine project key from the project field or default to AUDIT
		projectKey := "AUDIT"
		if project, ok := fields["project"].(map[string]interface{}); ok {
			if key, ok := project["key"].(string); ok && key != "" {
				projectKey = key
			}
		}

		// Generate ID and issue key
		id := fmt.Sprintf("10%d", len(MockDatabase.Tickets)+1)

		// Get the counter for this project and increment it
		counter := MockDatabase.TicketCounters[projectKey]
		counter++
		MockDatabase.TicketCounters[projectKey] = counter

		key := fmt.Sprintf("%s-%d", projectKey, counter)

		summary := ""
		if sum, ok := fields["summary"].(string); ok {
			summary = sum
		}
		description := ""
		if desc, ok := fields["description"].(string); ok {
			description = desc
		}
		priority := "Medium"
		if p, ok := fields["priority"].(map[string]interface{}); ok {
			if name, ok := p["name"].(string); ok {
				priority = name
			}
		}
		dueDate := ""
		if dd, ok := fields["duedate"].(string); ok {
			dueDate = dd
		}
		assignee := ""
		if a, ok := fields["assignee"].(map[string]interface{}); ok {
			if name, ok := a["name"].(string); ok {
				assignee = name
			}
		}

		// Initialize Fields map if nil
		if fields == nil {
			fields = make(map[string]interface{})
		}

		// Extract customfield_servicenow_id explicitly
		var serviceNowID string
		if snID, ok := fields["customfield_servicenow_id"].(string); ok && snID != "" {
			serviceNowID = snID
		} else if snID, ok := requestData["customfield_servicenow_id"].(string); ok && snID != "" {
			// Fallback to root level if not in fields
			serviceNowID = snID
			fields["customfield_servicenow_id"] = snID
		}

		ticketType := r.FormValue("ticketType")
		if ticketType == "" {
			ticketType = "Task" // Default to Task if not specified
		}

		ticket := JiraTicket{
			ID:          id,
			Key:         key,
			Self:        fmt.Sprintf("http://localhost:4000/rest/api/2/issue/%s", key),
			Type:        ticketType, // Add the ticket type here
			Summary:     summary,
			Description: description,
			Status:      "To Do",
			Priority:    priority,
			Created:     time.Now().Format(time.RFC3339),
			Updated:     time.Now().Format(time.RFC3339),
			DueDate:     dueDate,
			Assignee:    assignee,
			Fields:      fields,
			Comments:    []JiraComment{},
		}

		if serviceNowID != "" {
			MockDatabase.ServiceNowJiraMap[serviceNowID] = key
		} else {
			// Generate a ServiceNow ID for Jira-initiated tickets
			// This ensures new tickets created in Jira get a ServiceNow record
			serviceNowID = fmt.Sprintf("JIRA-%s-%d", projectKey, time.Now().Unix())
			fields["customfield_servicenow_id"] = serviceNowID
			ticket.Fields["customfield_servicenow_id"] = serviceNowID
			MockDatabase.ServiceNowJiraMap[serviceNowID] = key
		}

		MockDatabase.Tickets[key] = ticket

		// Log the creation
		log.Printf("[JIRA MOCK] Created issue: %s - %s with ServiceNow ID: %s",
			key, summary, serviceNowID)

		// IMPORTANT: Create corresponding item in ServiceNow for Jira-initiated tickets
		go createServiceNowItemFromJira(ticket)

		json.NewEncoder(w).Encode(map[string]string{
			"id":   id,
			"key":  key,
			"self": ticket.Self,
		})
		log.Printf("[JIRA MOCK] Created issue: %s - %s with ServiceNow ID: %s",
			key, summary, serviceNowID)

		// Add this line to send Slack notification
		go notifySlack(ticket, "created")
	}
}

// Add this new function to create ServiceNow items from Jira tickets
func createServiceNowItemFromJira(ticket JiraTicket) {
	// Determine if this ticket has a ServiceNow ID from fields
	serviceNowID := ""
	if ticket.Fields != nil {
		if id, ok := ticket.Fields["customfield_servicenow_id"].(string); ok {
			serviceNowID = id
		}
	}

	if serviceNowID == "" {
		log.Printf("[JIRA MOCK] No ServiceNow ID found for ticket %s, cannot sync", ticket.Key)
		return
	}

	/// MODIFIED LOGIC: If ServiceNow ID is user-provided (not auto-generated), we need to update ServiceNow
	isJiraOriginatedTicket := true // Default to true - assume created in Jira

	// If the ID looks like a real ServiceNow ID pattern, assume it came from ServiceNow
	if strings.HasPrefix(serviceNowID, "INC") ||
		strings.HasPrefix(serviceNowID, "RISK") ||
		strings.HasPrefix(serviceNowID, "AUDIT") ||
		strings.HasPrefix(serviceNowID, "TASK") {
		// This appears to be a genuine ServiceNow ID
		isJiraOriginatedTicket = false
	}

	// Skip ServiceNow update if this ticket originated from ServiceNow
	if !isJiraOriginatedTicket {
		log.Printf("[JIRA MOCK] Skipping ServiceNow update for ticket %s with ID %s - originated from ServiceNow",
			ticket.Key, serviceNowID)
		return
	}

	log.Printf("[JIRA MOCK] Processing Jira-originated ticket %s for ServiceNow update", ticket.Key)

	// Determine the ServiceNow table based on the project key
	var tableName string
	switch {
	case strings.HasPrefix(ticket.Key, "RISK"):
		tableName = "sn_risk_risk"
	case strings.HasPrefix(ticket.Key, "INC"):
		tableName = "sn_si_incident"
	case strings.HasPrefix(ticket.Key, "COMP"):
		tableName = "sn_compliance_task"
	case strings.HasPrefix(ticket.Key, "AUDIT"):
		tableName = "sn_audit_finding"
	case strings.HasPrefix(ticket.Key, "VEN"):
		tableName = "sn_vendor_risk"
	case strings.HasPrefix(ticket.Key, "REG"):
		tableName = "sn_regulatory_change"
	default:
		tableName = "sn_risk_risk" // Default to risk if no specific mapping
	}

	// Create the base payload with common fields
	payload := map[string]interface{}{
		"title":             ticket.Summary,
		"short_description": ticket.Summary, // Add this field for ServiceNow compatibility
		"name":              ticket.Summary, // Add this field for audit records
		"description":       ticket.Description,
		"details":           ticket.Description, // Alternative field for description
		"status":            mapJiraStatusToServiceNow(ticket.Status),
		"state":             mapJiraStatusToServiceNow(ticket.Status),
		"jira_key":          ticket.Key,
		"priority":          mapJiraPriorityToServiceNow(ticket.Priority),
		"assigned_to":       ticket.Assignee,
		"owner":             ticket.Assignee, // Add owner field
		"due_date":          ticket.DueDate,
		"source":            "jira",
		"update_time":       time.Now().Format(time.RFC3339),
		"sync_source":       "jira_initiated",
		"category":          mapRiskCategory(ticket.Fields),
		"severity":          mapJiraPriorityToRiskSeverity(ticket.Priority),
		"likelihood":        "Medium",
		"impact":            "Medium",
		"type":              "Operational",
		"risk_name":         ticket.Summary,
		"audit_name":        ticket.Summary, // For audit tables
		"incident_name":     ticket.Summary, // For incident tables
		"task_name":         ticket.Summary, // For task tables
	}

	// Add project-specific fields based on the ticket key prefix
	switch {
	case strings.HasPrefix(ticket.Key, "AUDIT"):
		// Audit-specific fields
		payload["name"] = ticket.Summary // This appears to be the key field for display
		payload["audit_name"] = ticket.Summary
		payload["audit_summary"] = ticket.Summary
		payload["finding_name"] = ticket.Summary
		payload["finding_title"] = ticket.Summary
		payload["finding_short_desc"] = ticket.Summary // Add this field
		payload["audit_details"] = ticket.Description
		payload["audit_status"] = mapJiraStatusToServiceNow(ticket.Status)
		payload["finding_status"] = mapJiraStatusToServiceNow(ticket.Status)
		payload["audit_owner"] = ticket.Assignee
		payload["audit_type"] = "Standard"
		payload["audit_classification"] = "Internal" // Add classification
		payload["finding_classification"] = "Risk"   // Add classification
		payload["finding_severity"] = mapJiraPriorityToRiskSeverity(ticket.Priority)
		payload["finding_category"] = mapRiskCategory(ticket.Fields)

		// Log specific fields being sent
		log.Printf("[JIRA MOCK] AUDIT fields: name=%s, audit_name=%s, finding_name=%s",
			payload["name"], payload["audit_name"], payload["finding_name"])

	case strings.HasPrefix(ticket.Key, "RISK"):
		// Risk-specific fields
		payload["risk_name"] = ticket.Summary
		payload["risk_title"] = ticket.Summary
		payload["risk_description"] = ticket.Description
		payload["risk_category"] = mapRiskCategory(ticket.Fields)
		payload["risk_severity"] = mapJiraPriorityToRiskSeverity(ticket.Priority)
		payload["risk_likelihood"] = "Medium"
		payload["risk_impact"] = "Medium"
		payload["risk_type"] = "Operational"
		payload["risk_owner"] = ticket.Assignee

	case strings.HasPrefix(ticket.Key, "INC"):
		// Incident-specific fields
		payload["incident_name"] = ticket.Summary
		payload["incident_title"] = ticket.Summary
		payload["incident_description"] = ticket.Description
		payload["incident_status"] = mapJiraStatusToServiceNow(ticket.Status)
		payload["incident_priority"] = mapJiraPriorityToServiceNow(ticket.Priority)
		payload["incident_owner"] = ticket.Assignee
		payload["incident_type"] = "General"

	case strings.HasPrefix(ticket.Key, "COMP"):
		// Compliance-specific fields
		payload["task_name"] = ticket.Summary
		payload["task_description"] = ticket.Description
		payload["task_state"] = mapJiraStatusToServiceNow(ticket.Status)
		payload["task_priority"] = mapJiraPriorityToServiceNow(ticket.Priority)
		payload["task_assigned_to"] = ticket.Assignee
		payload["task_owner"] = ticket.Assignee
		payload["compliance_standard"] = "Default"

	case strings.HasPrefix(ticket.Key, "VEN"):
		// Vendor-specific fields
		payload["vendor_name"] = ticket.Summary
		payload["vendor_description"] = ticket.Description
		payload["vendor_status"] = mapJiraStatusToServiceNow(ticket.Status)
		payload["vendor_priority"] = mapJiraPriorityToServiceNow(ticket.Priority)
		payload["contact_person"] = ticket.Assignee
		payload["vendor_type"] = "Standard"

	case strings.HasPrefix(ticket.Key, "REG"):
		// Regulatory-specific fields
		payload["reg_name"] = ticket.Summary
		payload["reg_title"] = ticket.Summary
		payload["reg_description"] = ticket.Description
		payload["reg_status"] = mapJiraStatusToServiceNow(ticket.Status)
		payload["reg_priority"] = mapJiraPriorityToServiceNow(ticket.Priority)
		payload["reg_owner"] = ticket.Assignee
		payload["regulatory_authority"] = "Default"
	}

	// Add detailed logging to help troubleshoot field mapping
	log.Printf("[JIRA MOCK] Sending ServiceNow payload for %s with title=%s, name=%s, category=%s",
		ticket.Key, payload["title"], payload["name"], payload["category"])

	if strings.HasPrefix(ticket.Key, "AUDIT") {
		log.Printf("[JIRA MOCK] Audit fields: audit_name=%s, finding_name=%s",
			payload["audit_name"], payload["finding_name"])
	}

	if ticket.DueDate != "" {
		payload["due_date"] = ticket.DueDate
	}

	// Convert to JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[JIRA MOCK] Error marshalling ServiceNow payload: %v", err)
		return
	}

	log.Printf("[JIRA MOCK] Sending update to ServiceNow for item %s with data: %s",
		serviceNowID, string(jsonPayload))

	// Send to ServiceNow
	serviceNowURL := os.Getenv("GRC_URL")
	if serviceNowURL == "" {
		serviceNowURL = "http://localhost:3000"
	}

	// Check if this ID already exists in ServiceNow (to determine POST vs PATCH)
	endpointURL := fmt.Sprintf("%s/api/now/table/%s", serviceNowURL, tableName)
	method := "POST"

	// If this seems like an update to existing record, use PATCH instead
	if strings.HasPrefix(serviceNowID, "RISK") || strings.HasPrefix(serviceNowID, "INC") ||
		strings.HasPrefix(serviceNowID, "TASK") || strings.HasPrefix(serviceNowID, "TEST") ||
		strings.HasPrefix(serviceNowID, "AUDIT") || strings.HasPrefix(serviceNowID, "VR") ||
		strings.HasPrefix(serviceNowID, "REG") {
		endpointURL = fmt.Sprintf("%s/api/now/table/%s/%s", serviceNowURL, tableName, serviceNowID)
		method = "PATCH"
	}

	req, err := http.NewRequest(method, endpointURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Printf("[JIRA MOCK] Error creating ServiceNow request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[JIRA MOCK] Error sending update to ServiceNow: %v", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("[JIRA MOCK] Successfully updated ServiceNow item %s. Response: %s",
			serviceNowID, string(respBody))

		// Add to webhook log
		logEntry := map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"type":      "outgoing",
			"target":    "servicenow",
			"action":    "create_or_update",
			"item_id":   serviceNowID,
			"source":    "jira",
			"jira_key":  ticket.Key,
		}
		MockDatabase.WebhookLog = append(MockDatabase.WebhookLog, logEntry)
	} else {
		log.Printf("[JIRA MOCK] Failed to update ServiceNow item %s. Status: %d, Response: %s",
			serviceNowID, resp.StatusCode, string(respBody))
	}
}

func sendWebhookWithRetry(url string, payload []byte) {
	maxAttempts := 3

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Post(url, "application/json", bytes.NewBuffer(payload))

		if err != nil {
			log.Printf("[JIRA MOCK] Webhook attempt %d failed: %v", attempt, err)
			if attempt < maxAttempts {
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			log.Printf("[JIRA MOCK] All webhook attempts failed, giving up: %v", err)
			return
		}

		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("[JIRA MOCK] Webhook sent successfully to %s", url)
			return
		}

		log.Printf("[JIRA MOCK] Webhook failed with status %d: %s", resp.StatusCode, string(body))
		if attempt < maxAttempts {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}
}

func mapJiraPriorityToRiskSeverity(jiraPriority string) string {
	switch jiraPriority {
	case "Highest":
		return "Critical"
	case "High":
		return "High"
	case "Medium":
		return "Medium"
	case "Low":
		return "Low"
	case "Lowest":
		return "Very Low"
	default:
		return "Medium"
	}
}

// Helper function to determine risk category from ticket fields
func mapRiskCategory(fields map[string]interface{}) string {
	// First check if there's an explicit risk category field
	if fields != nil {
		if category, ok := fields["risk_category"].(string); ok && category != "" {
			return category
		}

		// Try to infer from project key
		if key, ok := fields["key"].(string); ok {
			if strings.HasPrefix(key, "SEC") {
				return "Security"
			} else if strings.HasPrefix(key, "COMP") {
				return "Compliance"
			} else if strings.HasPrefix(key, "OPS") {
				return "Operational"
			} else if strings.HasPrefix(key, "FIN") {
				return "Financial"
			} else if strings.HasPrefix(key, "RISK") {
				return "Strategic"
			}
		}
	}

	// Default value
	return "Operational"
}

// Helper function to map Jira status to ServiceNow status
func mapJiraStatusToServiceNow(jiraStatus string) string {
	switch jiraStatus {
	case "To Do":
		return "Open"
	case "In Progress":
		return "In Progress"
	case "Done", "Closed":
		return "Closed"
	case "Resolved":
		return "Resolved"
	default:
		return jiraStatus
	}
}

// Helper function to map Jira priority to ServiceNow priority
func mapJiraPriorityToServiceNow(jiraPriority string) string {
	switch jiraPriority {
	case "Highest":
		return "Critical"
	case "High":
		return "High"
	case "Medium":
		return "Medium"
	case "Low":
		return "Low"
	case "Lowest":
		return "Planning"
	default:
		return "Medium"
	}
}

// Add this function at the end of your file
func notifySlack(ticket JiraTicket, action string) {
	slackWebhookURL := os.Getenv("SLACK_WEBHOOK_URL")
	if slackWebhookURL == "" {
		slackWebhookURL = "https://hooks.slack.com/services/YOUR_DEFAULT_WEBHOOK_PATH"
	}

	// If URL doesn't look like a valid Slack webhook, log and return
	if !strings.Contains(slackWebhookURL, "hooks.slack.com") {
		// Try for local development - default to localhost:3000
		slackWebhookURL = os.Getenv("SLACK_WEBHOOK_URL")
		if slackWebhookURL == "" {
			slackWebhookURL = "http://localhost:3000/api/slack/webhook"
		}
		log.Printf("[JIRA MOCK] Using development Slack webhook URL: %s", slackWebhookURL)
	}

	// Format message color based on ticket priority
	color := "#36a64f" // Default green
	if strings.ToLower(ticket.Priority) == "high" || strings.ToLower(ticket.Priority) == "highest" {
		color = "#d00000" // Red for high priority
	} else if strings.ToLower(ticket.Priority) == "medium" {
		color = "#ffaa00" // Orange for medium priority
	}

	// Create message payload
	payload := map[string]interface{}{
		"attachments": []map[string]interface{}{
			{
				"fallback":   fmt.Sprintf("[%s] %s %s", ticket.Key, action, ticket.Summary),
				"color":      color,
				"pretext":    fmt.Sprintf("Jira ticket %s:", action),
				"title":      fmt.Sprintf("[%s] %s", ticket.Key, ticket.Summary),
				"title_link": fmt.Sprintf("http://localhost:4000/browse/%s", ticket.Key),
				"text":       ticket.Description,
				"fields": []map[string]interface{}{
					{
						"title": "Status",
						"value": ticket.Status,
						"short": true,
					},
					{
						"title": "Priority",
						"value": ticket.Priority,
						"short": true,
					},
				},
				"footer": "Jira Notification",
				"ts":     time.Now().Unix(),
			},
		},
	}

	// Convert to JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[JIRA MOCK] Error marshalling Slack payload: %v", err)
		return
	}

	// Send to Slack
	req, err := http.NewRequest("POST", slackWebhookURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Printf("[JIRA MOCK] Error creating Slack request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[JIRA MOCK] Error sending notification to Slack: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("[JIRA MOCK] Successfully sent Slack notification for %s", ticket.Key)
	} else {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[JIRA MOCK] Failed to send Slack notification. Status: %d, Response: %s",
			resp.StatusCode, string(respBody))
	}
}

func handleIssueByKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	key := vars["key"]

	ticket, exists := MockDatabase.Tickets[key]
	if !exists {
		http.Error(w, "Issue not found", http.StatusNotFound)
		return
	}

	switch r.Method {
	case "GET":
		json.NewEncoder(w).Encode(ticket)

	case "PUT":
		var updateData map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		fields, ok := updateData["fields"].(map[string]interface{})
		if ok {
			if summary, ok := fields["summary"].(string); ok {
				ticket.Summary = summary
			}
			if desc, ok := fields["description"].(string); ok {
				ticket.Description = desc
			}
			if priority, ok := fields["priority"].(map[string]interface{}); ok {
				if name, ok := priority["name"].(string); ok {
					ticket.Priority = name
				}
			}
			if assignee, ok := fields["assignee"].(map[string]interface{}); ok {
				if name, ok := assignee["name"].(string); ok {
					ticket.Assignee = name
				}
			}
			if dueDate, ok := fields["duedate"].(string); ok {
				ticket.DueDate = dueDate
			}
			if status, ok := fields["status"].(map[string]interface{}); ok {
				if name, ok := status["name"].(string); ok {
					ticket.Status = name
				}
			}
			for k, v := range fields {
				if ticket.Fields == nil {
					ticket.Fields = make(map[string]interface{})
				}
				ticket.Fields[k] = v
			}
		}

		previousStatus := ticket.Status // Store previous status for changelog
		ticket.Updated = time.Now().Format(time.RFC3339)
		MockDatabase.Tickets[key] = ticket

		log.Printf("[JIRA MOCK] Updated issue: %s to status: %s", key, ticket.Status)

		go notifySlack(ticket, "updated")
		w.WriteHeader(http.StatusNoContent)

		// Send update back to ServiceNow if this has a ServiceNow ID
		if ticket.Fields != nil {
			if serviceNowID, ok := ticket.Fields["customfield_servicenow_id"].(string); ok && serviceNowID != "" {
				go notifyServiceNowOfStatusChange(serviceNowID, ticket.Status, previousStatus)
			}
		}

		go triggerStatusChangeWebhook(key, ticket.Status, previousStatus)

	case "DELETE":
		delete(MockDatabase.Tickets, key)
		w.WriteHeader(http.StatusNoContent)

		log.Printf("[JIRA MOCK] Deleted issue: %s", key)
	}
}

func handleComments(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	key := vars["key"]

	ticket, exists := MockDatabase.Tickets[key]
	if !exists {
		http.Error(w, "Issue not found", http.StatusNotFound)
		return
	}

	switch r.Method {
	case "GET":
		response := map[string]interface{}{
			"comments": ticket.Comments,
			"total":    len(ticket.Comments),
		}
		json.NewEncoder(w).Encode(response)

	case "POST":
		var commentData map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&commentData); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		body, ok := commentData["body"].(string)
		if !ok {
			http.Error(w, "Missing comment body", http.StatusBadRequest)
			return
		}

		// Determine author
		author := "mock-user"
		if userData, ok := commentData["author"].(map[string]interface{}); ok {
			if name, ok := userData["name"].(string); ok {
				author = name
			}
		}

		comment := JiraComment{
			ID:      fmt.Sprintf("comment-%d", len(ticket.Comments)+1),
			Body:    body,
			Author:  author,
			Created: time.Now().Format(time.RFC3339),
		}

		ticket.Comments = append(ticket.Comments, comment)
		MockDatabase.Tickets[key] = ticket

		log.Printf("[JIRA MOCK] Added comment to issue %s: %s", key, body)

		// Forward comment to ServiceNow if applicable
		if ticket.Fields != nil {
			if serviceNowID, ok := ticket.Fields["customfield_servicenow_id"].(string); ok && serviceNowID != "" {
				go addCommentToServiceNow(serviceNowID, body, author)
			}
		}

		json.NewEncoder(w).Encode(comment)
	}
}

func handleTransitions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	key := vars["key"]

	ticket, exists := MockDatabase.Tickets[key]
	if !exists {
		http.Error(w, "Issue not found", http.StatusNotFound)
		return
	}

	switch r.Method {
	case "GET":
		transitions := []JiraTransition{}
		switch ticket.Status {
		case "To Do":
			transitions = append(transitions, JiraTransition{
				ID:   "21",
				Name: "Start Progress",
				To:   map[string]string{"id": "3", "name": "In Progress"},
			})
			// Add Resolve Issue transition for To Do state
			transitions = append(transitions, JiraTransition{
				ID:   "51",
				Name: "Resolve Issue",
				To:   map[string]string{"id": "6", "name": "Resolved"},
			})
		case "In Progress":
			transitions = append(transitions, JiraTransition{
				ID:   "31",
				Name: "Done",
				To:   map[string]string{"id": "5", "name": "Done"},
			})
			transitions = append(transitions, JiraTransition{
				ID:   "11",
				Name: "Stop Progress",
				To:   map[string]string{"id": "1", "name": "To Do"},
			})
			// Add Resolve Issue transition for In Progress state
			transitions = append(transitions, JiraTransition{
				ID:   "51",
				Name: "Resolve Issue",
				To:   map[string]string{"id": "6", "name": "Resolved"},
			})
		case "Done":
			transitions = append(transitions, JiraTransition{
				ID:   "41",
				Name: "Reopen",
				To:   map[string]string{"id": "1", "name": "To Do"},
			})
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"transitions": transitions,
		})

	case "POST":
		var transitionRequest map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&transitionRequest); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		transitionData, ok := transitionRequest["transition"].(map[string]interface{})
		if !ok {
			http.Error(w, "Invalid transition format", http.StatusBadRequest)
			return
		}

		transitionID, ok := transitionData["id"].(string)
		if !ok {
			http.Error(w, "Missing transition ID", http.StatusBadRequest)
			return
		}

		previousStatus := ticket.Status // Store previous status for changelog
		switch transitionID {
		case "21": // To Do -> In Progress
			ticket.Status = "In Progress"
		case "31": // In Progress -> Done
			ticket.Status = "Done"
			fieldsData, ok := transitionRequest["fields"].(map[string]interface{})
			if ok {
				resolutionData, ok := fieldsData["resolution"].(map[string]interface{})
				if ok {
					if name, ok := resolutionData["name"].(string); ok {
						ticket.Resolution = name
					}
				}
			}
		case "11": // In Progress -> To Do
			ticket.Status = "To Do"
		case "41": // Done -> To Do
			ticket.Status = "To Do"
			ticket.Resolution = ""
		case "51": // Any -> Resolved (and remove from ServiceNow)
			ticket.Status = "Resolved"
			ticket.Resolution = "Issue Resolved"
			// Update ServiceNow status to Resolved
			if ticket.Fields != nil {
				if snID, ok := ticket.Fields["customfield_servicenow_id"]; ok {
					if serviceNowID, ok := snID.(string); ok && serviceNowID != "" {
						log.Printf("[JIRA MOCK] Updating ServiceNow ticket %s to Resolved", serviceNowID)

						// Notify ServiceNow of the status change to Resolved
						go notifyServiceNowOfStatusChange(serviceNowID, "Resolved", previousStatus)

						// Optional: If still required, remove from ServiceNow after update
						done := make(chan bool)
						go func() {
							removeFromServiceNow(serviceNowID)
							done <- true
						}()
						select {
						case <-done:
							log.Printf("[JIRA MOCK] Successfully removed from ServiceNow after resolving %s", key)
						case <-time.After(15 * time.Second):
							log.Printf("[JIRA MOCK] Timeout waiting for ServiceNow removal for %s", key)
						}
					} else {
						log.Printf("[JIRA MOCK] ServiceNow ID for issue %s is not a valid string or is empty", key)
					}
				} else {
					log.Printf("[JIRA MOCK] No ServiceNow ID found for issue %s", key)
				}
			}
		default:
			http.Error(w, "Invalid transition ID", http.StatusBadRequest)
			return
		}

		ticket.Updated = time.Now().Format(time.RFC3339)
		MockDatabase.Tickets[key] = ticket

		log.Printf("[JIRA MOCK] Transitioned issue %s from %s to %s",
			key, previousStatus, ticket.Status)

		w.WriteHeader(http.StatusNoContent)

		// Handle ServiceNow integration based on the new status
		if ticket.Fields != nil {
			var serviceNowID string
			if snID, ok := ticket.Fields["customfield_servicenow_id"]; ok {
				if strID, ok := snID.(string); ok && strID != "" {
					serviceNowID = strID

					// If status is Resolved, ensure ServiceNow is updated too
					if ticket.Status == "Resolved" {
						log.Printf("[JIRA MOCK] Updating ServiceNow for issue %s to Resolved", key)
						go notifyServiceNowOfStatusChange(serviceNowID, "Resolved", previousStatus)
					} else if transitionID != "51" { // For other transitions, just notify status change
						go notifyServiceNowOfStatusChange(serviceNowID, ticket.Status, previousStatus)
					}
				} else {
					log.Printf("[JIRA MOCK] ServiceNow ID for issue %s is not a valid string or is empty", key)
				}
			} else {
				log.Printf("[JIRA MOCK] No ServiceNow ID found for issue %s", key)
			}
		}

		go triggerStatusChangeWebhook(key, ticket.Status, previousStatus)
	}
}

func removeFromServiceNow(serviceNowID string) {
	if serviceNowID == "" {
		log.Printf("[ERROR] Cannot remove from ServiceNow: Empty ServiceNow ID")
		return
	}

	// Log the action
	log.Printf("[INFO] Attempting to completely remove record with ServiceNow ID %s from ServiceNow database", serviceNowID)

	// Use the GRC_URL environment variable or default to localhost
	serviceNowURL := os.Getenv("GRC_URL")
	if serviceNowURL == "" {
		serviceNowURL = "http://localhost:3000"
	}

	// Try multiple possible table names and approaches
	tables := []string{"risk_task", "rm_risk", "sn_risk_task", "risk_assessment", "task"}

	// Try deletion with different table names and with/without prefix
	success := false

	// First attempt - try with the exact ID as provided across different tables
	for _, table := range tables {
		endpoint := fmt.Sprintf("%s/api/now/table/%s/%s", serviceNowURL, table, serviceNowID)
		if tryServiceNowDeletion(endpoint, fmt.Sprintf("table '%s' with full ID", table)) {
			success = true
			break
		}
	}

	// Second attempt - if ID has prefix, try without it across different tables
	if !success && strings.Contains(serviceNowID, "_") {
		parts := strings.SplitN(serviceNowID, "_", 2)
		if len(parts) == 2 {
			numericID := parts[1]
			for _, table := range tables {
				endpoint := fmt.Sprintf("%s/api/now/table/%s/%s", serviceNowURL, table, numericID)
				if tryServiceNowDeletion(endpoint, fmt.Sprintf("table '%s' with numeric ID", table)) {
					success = true
					break
				}
			}
		}
	}

	// Third attempt - try with the ServiceNow REST API v2
	if !success {
		v2Endpoint := fmt.Sprintf("%s/api/now/v2/table/task?sysparm_query=number=%s", serviceNowURL, serviceNowID)
		if tryServiceNowQueryAndDelete(v2Endpoint, serviceNowURL) {
			success = true
		}
	}

	if !success {
		log.Printf("[CRITICAL ERROR] All attempts to remove the record from ServiceNow database failed for ID: %s", serviceNowID)
	}
}

// Try a single ServiceNow deletion with full error reporting - no authentication
func tryServiceNowDeletion(endpoint string, attemptDesc string) bool {
	log.Printf("[ATTEMPT] Trying ServiceNow deletion with %s: %s", attemptDesc, endpoint)

	req, err := http.NewRequest("DELETE", endpoint, nil)
	if err != nil {
		log.Printf("[ERROR] Failed to create DELETE request for %s: %v", attemptDesc, err)
		return false
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add a custom header that some ServiceNow instances require
	req.Header.Set("X-HTTP-Method-Override", "DELETE")

	// Create HTTP client with increased timeout
	client := &http.Client{
		Timeout: 20 * time.Second,
	}

	// Execute the request with retry logic
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			// Exponential backoff for retries
			backoff := time.Duration(math.Pow(2, float64(i))) * time.Second
			log.Printf("[RETRY] Waiting %v before retry %d for %s", backoff, i+1, attemptDesc)
			time.Sleep(backoff)
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[ERROR] Attempt %d: Failed to execute DELETE request: %v", i+1, err)
			continue
		}

		body, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()

		// Detailed response logging for debugging
		log.Printf("[DEBUG] ServiceNow API response - Status: %d, Headers: %v, Body: %s",
			resp.StatusCode, resp.Header, string(body))

		// ServiceNow returns different status codes for successful deletion
		if resp.StatusCode >= 200 && resp.StatusCode < 300 || resp.StatusCode == 404 {
			// 200-299: Success, 404: Already deleted
			log.Printf("[SUCCESS] Successfully removed record from ServiceNow using %s", attemptDesc)
			return true
		} else {
			log.Printf("[ERROR] Attempt %d: Failed to remove record using %s. Status: %d, Response: %s",
				i+1, attemptDesc, resp.StatusCode, string(body))
		}
	}

	return false
}

// Query for the record first, then delete it by sys_id - no authentication
func tryServiceNowQueryAndDelete(queryEndpoint string, baseURL string) bool {
	log.Printf("[ATTEMPT] Trying ServiceNow query-then-delete approach: %s", queryEndpoint)

	req, err := http.NewRequest("GET", queryEndpoint, nil)
	if err != nil {
		log.Printf("[ERROR] Failed to create GET query request: %v", err)
		return false
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[ERROR] Failed to execute GET query request: %v", err)
		return false
	}

	body, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	// Check if we got a successful response
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[ERROR] Query failed. Status: %d, Response: %s", resp.StatusCode, string(body))
		return false
	}

	// Parse the response to extract sys_id
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("[ERROR] Failed to parse ServiceNow query response: %v", err)
		return false
	}

	// Navigate through the response structure to find the sys_id
	resultList, ok := result["result"].([]interface{})
	if !ok || len(resultList) == 0 {
		log.Printf("[ERROR] No matching records found in ServiceNow")
		return false
	}

	// Process each result and try to delete them
	for _, item := range resultList {
		record, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		sysID, ok := record["sys_id"].(string)
		if !ok || sysID == "" {
			continue
		}

		log.Printf("[INFO] Found matching record with sys_id: %s", sysID)

		// Try to delete using sys_id with different table names
		tables := []string{"task", "risk_task", "rm_risk"}
		for _, table := range tables {
			deleteEndpoint := fmt.Sprintf("%s/api/now/table/%s/%s", baseURL, table, sysID)
			if tryServiceNowDeletion(deleteEndpoint, fmt.Sprintf("sys_id deletion from '%s'", table)) {
				return true
			}
		}
	}

	return false
}

func handleProjects(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var projectList []interface{}
	for _, project := range MockDatabase.Projects {
		projectList = append(projectList, project)
	}

	json.NewEncoder(w).Encode(projectList)
}

func handleSearchIssues(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	jql := r.URL.Query().Get("jql")
	if jql == "" {
		http.Error(w, "Missing JQL parameter", http.StatusBadRequest)
		return
	}

	var results []JiraTicket

	// Parse JQL (e.g., "project=AUDIT AND customfield_servicenow_id=CTRL003")
	jqlParts := strings.Split(jql, " AND ")
	filters := make(map[string]string)

	for _, part := range jqlParts {
		if strings.Contains(part, "=") {
			partSplit := strings.SplitN(part, "=", 2)
			if len(partSplit) == 2 {
				key := strings.TrimSpace(partSplit[0])
				value := strings.Trim(strings.TrimSpace(partSplit[1]), "\"'")
				filters[key] = value
			}
		}
	}

	log.Printf("[JIRA MOCK] Search with filters: %v", filters)

	for _, ticket := range MockDatabase.Tickets {
		matches := true

		// Project filter
		if projectKey, ok := filters["project"]; ok {
			if !strings.HasPrefix(ticket.Key, projectKey) {
				matches = false
			}
		}

		// ServiceNow ID filter
		if snID, ok := filters["customfield_servicenow_id"]; ok {
			if ticket.Fields == nil || ticket.Fields["customfield_servicenow_id"] == nil ||
				ticket.Fields["customfield_servicenow_id"].(string) != snID {
				matches = false
			}
		}

		// Summary filter
		if summary, ok := filters["summary ~ "]; ok {
			if !strings.Contains(strings.ToLower(ticket.Summary), strings.ToLower(summary)) {
				matches = false
			}
		}

		// Status filter
		if status, ok := filters["status"]; ok {
			if !strings.EqualFold(ticket.Status, status) {
				matches = false
			}
		}

		if matches {
			// Add formatted fields to the ticket
			if ticket.Fields == nil {
				ticket.Fields = make(map[string]interface{})
			}

			// Add proper formatted fields for the API response
			ticket.Fields["summary"] = ticket.Summary
			ticket.Fields["description"] = ticket.Description
			ticket.Fields["duedate"] = ticket.DueDate
			ticket.Fields["assignee"] = map[string]interface{}{
				"name": ticket.Assignee,
			}
			ticket.Fields["status"] = map[string]interface{}{
				"name": ticket.Status,
			}
			ticket.Fields["priority"] = map[string]interface{}{
				"name": ticket.Priority,
			}

			results = append(results, ticket)
		}
	}

	log.Printf("[JIRA MOCK] Search JQL: %s, Found: %d issues", jql, len(results))

	response := map[string]interface{}{
		"issues":     results,
		"total":      len(results),
		"maxResults": 50,
		"startAt":    0,
	}
	json.NewEncoder(w).Encode(response)
}

// WEBHOOK HANDLERS

func handleReceiveWebhook(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Log the received webhook
	log.Printf("[JIRA MOCK] Received webhook: %v", payload)

	// Add to webhook log
	logEntry := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"type":      "incoming",
		"source":    "external",
		"payload":   payload,
	}
	MockDatabase.WebhookLog = append(MockDatabase.WebhookLog, logEntry)

	// Limit log size
	if len(MockDatabase.WebhookLog) > 100 {
		MockDatabase.WebhookLog = MockDatabase.WebhookLog[len(MockDatabase.WebhookLog)-100:]
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"received"}`))
}

// Complete overhaul of the handleServiceNowWebhook function to fix ServiceNow GRC integration
func handleServiceNowWebhook(w http.ResponseWriter, r *http.Request) {
	log.Printf("[JIRA MOCK] Received ServiceNow webhook request: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
	log.Printf("[JIRA MOCK] Headers: %v", r.Header)

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Content-Type", "application/json")

	// If the request is OPTIONS, return immediately with a 200 OK (CORS preflight)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	log.Printf("[JIRA MOCK] Received ServiceNow webhook with headers: %v", r.Header)

	// Read and save the request body for debugging
	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[JIRA MOCK] ERROR reading ServiceNow webhook body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Log the raw request body
	log.Printf("[JIRA MOCK] RAW ServiceNow webhook payload: %s", string(requestBody))

	// Restore the body for further processing
	r.Body = io.NopCloser(bytes.NewBuffer(requestBody))

	// Try to decode as a standard ServiceNowWebhookPayload
	var payload ServiceNowWebhookPayload
	if err := json.NewDecoder(bytes.NewBuffer(requestBody)).Decode(&payload); err != nil {
		log.Printf("[JIRA MOCK] WARNING: Failed to parse standard ServiceNow payload: %v", err)

		// If standard parsing fails, try to parse as a raw map for flexibility
		var rawPayload map[string]interface{}
		if err := json.NewDecoder(bytes.NewBuffer(requestBody)).Decode(&rawPayload); err != nil {
			log.Printf("[JIRA MOCK] ERROR: Failed to parse ServiceNow webhook as raw JSON: %v", err)
			http.Error(w, "Invalid JSON in request body", http.StatusBadRequest)
			return
		}

		// Log the raw payload structure
		log.Printf("[JIRA MOCK] ServiceNow raw payload structure received:")
		for key, value := range rawPayload {
			log.Printf("[JIRA MOCK]   %s: %v (%T)", key, value, value)
		}

		// Convert the raw payload to our expected format
		if sysID, ok := rawPayload["sys_id"].(string); ok {
			payload.SysID = sysID
		} else if id, ok := rawPayload["id"].(string); ok {
			payload.SysID = id
		}

		if tableName, ok := rawPayload["table_name"].(string); ok {
			payload.TableName = tableName
		} else if tableName, ok := rawPayload["table"].(string); ok {
			payload.TableName = tableName
		} else {
			// Default to risk table for GRC if not specified
			payload.TableName = "sn_risk_risk"
			log.Printf("[JIRA MOCK] No table name found, defaulting to sn_risk_risk")
		}

		if actionType, ok := rawPayload["action_type"].(string); ok {
			payload.ActionType = actionType
		} else if action, ok := rawPayload["action"].(string); ok {
			payload.ActionType = action
		} else {
			payload.ActionType = "insert" // Default to insert if not specified
			log.Printf("[JIRA MOCK] No action type found, defaulting to insert")
		}

		// Convert data section if present
		if data, ok := rawPayload["data"].(map[string]interface{}); ok {
			payload.Data = data
		} else {
			// If no data section, use the whole payload as data
			payload.Data = rawPayload
			log.Printf("[JIRA MOCK] No data section found, using entire payload as data")
		}
	}

	// Ensure we have the minimum required fields
	if payload.SysID == "" {
		if id, ok := payload.Data["sys_id"].(string); ok {
			payload.SysID = id
		} else if id, ok := payload.Data["id"].(string); ok {
			payload.SysID = id
		} else if id, ok := payload.Data["number"].(string); ok {
			payload.SysID = id
		} else {
			// Generate a random ID if none found
			payload.SysID = fmt.Sprintf("GRC-%d", time.Now().Unix())
			log.Printf("[JIRA MOCK] No sys_id found, generating one: %s", payload.SysID)
		}
	}

	// Ensure we have a valid action type
	if payload.ActionType == "" {
		payload.ActionType = "insert"
		log.Printf("[JIRA MOCK] No action_type found, defaulting to insert")
	}

	// Extract and log title/summary for better logging
	title := "Unknown"
	if t, ok := payload.Data["title"].(string); ok && t != "" {
		title = t
	} else if t, ok := payload.Data["short_description"].(string); ok && t != "" {
		title = t
	} else if t, ok := payload.Data["name"].(string); ok && t != "" {
		title = t
	}

	// Log what we've parsed from the payload
	log.Printf("[JIRA MOCK] ServiceNow webhook processed: SysID=%s, Table=%s, Action=%s, Title=%s",
		payload.SysID, payload.TableName, payload.ActionType, title)

	// Add to webhook log
	logEntry := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"type":      "incoming",
		"source":    "servicenow",
		"table":     payload.TableName,
		"action":    payload.ActionType,
		"sys_id":    payload.SysID,
		"title":     title,
	}
	MockDatabase.WebhookLog = append(MockDatabase.WebhookLog, logEntry)

	// Check if this webhook is for a Jira-initiated update
	isSyncFromJira := false
	if syncSource, ok := payload.Data["sync_source"].(string); ok && syncSource == "jira_initiated" {
		isSyncFromJira = true
		log.Printf("[JIRA MOCK] Received ServiceNow webhook for a Jira-initiated update, will not create/update ticket")
	}

	// Handle based on the action type
	switch payload.ActionType {
	case "insert", "created", "create":
		// Only create a ticket if not Jira-initiated
		if !isSyncFromJira {
			createJiraTicketFromServiceNow(payload)
		} else {
			log.Printf("[JIRA MOCK] Skipping ticket creation for ServiceNow item %s - originated from Jira", payload.SysID)
		}

	case "update", "updated", "modify", "modified":
		// Find the corresponding Jira ticket and update it if not Jira-initiated
		jiraKey, exists := MockDatabase.ServiceNowJiraMap[payload.SysID]
		if exists && !isSyncFromJira {
			updateJiraTicketFromServiceNow(jiraKey, payload)
		} else {
			log.Printf("[JIRA MOCK] Skipping update for ServiceNow item %s - originated from Jira or no matching ticket", payload.SysID)
		}

	case "delete", "deleted", "remove", "removed":
		// Find the corresponding Jira ticket and mark it
		jiraKey, exists := MockDatabase.ServiceNowJiraMap[payload.SysID]
		if exists {
			ticket, ticketExists := MockDatabase.Tickets[jiraKey]
			if ticketExists && !isSyncFromJira {
				// Mark as deleted in Jira but don't actually delete
				ticket.Status = "Closed"
				ticket.Resolution = "Won't Fix"
				ticket.Updated = time.Now().Format(time.RFC3339)

				if ticket.Fields == nil {
					ticket.Fields = make(map[string]interface{})
				}
				ticket.Fields["servicenow_deleted"] = true

				MockDatabase.Tickets[jiraKey] = ticket

				log.Printf("[JIRA MOCK] Marked ticket %s as closed due to ServiceNow deletion", jiraKey)
			} else if isSyncFromJira {
				log.Printf("[JIRA MOCK] Skipping deletion for ServiceNow item %s - originated from Jira", payload.SysID)
			}
		}
	}

	// Return success
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Webhook processed successfully",
	})
}

func triggerWebhook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	eventType := vars["event_type"]

	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	issueKey := "AUDIT-1"
	if key, ok := data["issue_key"].(string); ok && key != "" {
		issueKey = key
	}

	var webhookPayload map[string]interface{}
	switch eventType {
	case "issue_created", "jira:issue_created":
		webhookPayload = buildIssueWebhookPayload(issueKey, "created", data)
	case "issue_updated", "jira:issue_updated":
		webhookPayload = buildIssueWebhookPayload(issueKey, "updated", data)
	case "comment_created":
		webhookPayload = buildCommentWebhookPayload(issueKey, "created", data)
	case "comment_updated":
		webhookPayload = buildCommentWebhookPayload(issueKey, "updated", data)
	default:
		http.Error(w, "Unsupported event type", http.StatusBadRequest)
		return
	}

	webhookURL := r.URL.Query().Get("webhook_url")
	if webhookURL == "" {
		webhookURL = "http://localhost:8081/api/webhooks/jira"
	}

	jsonPayload, _ := json.Marshal(webhookPayload)
	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(jsonPayload))
	if err != nil {
		http.Error(w, fmt.Sprintf("Error sending webhook: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Log the outgoing webhook
	logEntry := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"type":      "outgoing",
		"target":    webhookURL,
		"event":     eventType,
		"issue_key": issueKey,
	}
	MockDatabase.WebhookLog = append(MockDatabase.WebhookLog, logEntry)

	if w != nil {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":     "success",
			"message":    fmt.Sprintf("Jira webhook sent to %s", webhookURL),
			"webhook_id": fmt.Sprintf("mock-jira-webhook-%d", time.Now().UnixNano()),
			"event_type": eventType,
			"issue_key":  issueKey,
		})
	}
}

///////////////////////////////////////////////////////////////////

func triggerStatusChangeWebhook(issueKey string, newStatus string, previousStatus string) {
	payload := buildIssueWebhookPayload(issueKey, "updated", map[string]interface{}{
		"status": newStatus,
	})

	// Dynamic changelog based on previous and new status
	var fromID, toID string
	switch previousStatus {
	case "To Do":
		fromID = "1"
	case "In Progress":
		fromID = "3"
	case "Done":
		fromID = "5"
	}
	switch newStatus {
	case "To Do":
		toID = "1"
	case "In Progress":
		toID = "3"
	case "Done":
		toID = "5"
	}

	payload["changelog"] = map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"field":      "status",
				"fieldtype":  "jira",
				"from":       fromID,
				"fromString": previousStatus,
				"to":         toID,
				"toString":   newStatus,
			},
		},
	}

	webhookURL := "http://localhost:3000/api/webhooks/jira"
	jsonPayload, _ := json.Marshal(payload)

	sendWebhookWithRetry(webhookURL, jsonPayload)

	// Log the outgoing webhook
	logEntry := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"type":      "outgoing",
		"target":    webhookURL,
		"event":     "jira:issue_updated",
		"issue_key": issueKey,
		"changes":   fmt.Sprintf("Status: %s → %s", previousStatus, newStatus),
	}
	MockDatabase.WebhookLog = append(MockDatabase.WebhookLog, logEntry)

	log.Printf("[JIRA MOCK] Status change webhook sent for issue %s (from %s to %s)",
		issueKey, previousStatus, newStatus)
}

func buildIssueWebhookPayload(issueKey string, action string, data map[string]interface{}) map[string]interface{} {
	ticket, exists := MockDatabase.Tickets[issueKey]

	summary := "Mock issue"
	description := "This is a mock issue for testing"
	status := "To Do"

	if exists {
		summary = ticket.Summary
		description = ticket.Description
		status = ticket.Status
	}

	if s, ok := data["summary"].(string); ok && s != "" {
		summary = s
	}
	if d, ok := data["description"].(string); ok && d != "" {
		description = d
	}
	if s, ok := data["status"].(string); ok && s != "" {
		status = s
	}

	issueFields := map[string]interface{}{
		"summary":     summary,
		"description": description,
		"status": map[string]interface{}{
			"id":   "3", // Default ID, updated in changelog
			"name": status,
		},
	}

	if snID, ok := data["servicenow_id"].(string); ok && snID != "" {
		issueFields["customfield_servicenow_id"] = snID
	} else if exists && ticket.Fields != nil && ticket.Fields["customfield_servicenow_id"] != nil {
		issueFields["customfield_servicenow_id"] = ticket.Fields["customfield_servicenow_id"]
	}

	return map[string]interface{}{
		"webhookEvent": "jira:issue_" + action,
		"issue": map[string]interface{}{
			"id":     issueKey,
			"key":    issueKey,
			"self":   fmt.Sprintf("http://localhost:4000/rest/api/2/issue/%s", issueKey),
			"fields": issueFields,
		},
		"user": map[string]interface{}{
			"name":         "mock-user",
			"displayName":  "Mock User",
			"emailAddress": "mock@example.com",
		},
	}
}

func buildCommentWebhookPayload(issueKey string, action string, data map[string]interface{}) map[string]interface{} {
	commentBody := "This is a mock comment"
	if body, ok := data["comment"].(string); ok && body != "" {
		commentBody = body
	}

	commentID := "12345"
	if id, ok := data["comment_id"].(string); ok && id != "" {
		commentID = id
	}

	author := "mock-user"
	authorDisplay := "Mock User"
	if authorName, ok := data["author"].(string); ok && authorName != "" {
		author = authorName
		authorDisplay = strings.Title(strings.Replace(authorName, ".", " ", -1))
	}

	issuePayload := buildIssueWebhookPayload(issueKey, "commented", data)
	issuePayload["webhookEvent"] = "comment_" + action
	issuePayload["comment"] = map[string]interface{}{
		"id":   commentID,
		"body": commentBody,
		"author": map[string]interface{}{
			"name":         author,
			"displayName":  authorDisplay,
			"emailAddress": author + "@example.com",
		},
		"created": time.Now().Format(time.RFC3339),
		"updated": time.Now().Format(time.RFC3339),
	}

	return issuePayload
}

func handleReset(w http.ResponseWriter, r *http.Request) {
	MockDatabase.Tickets = make(map[string]JiraTicket)
	MockDatabase.ServiceNowJiraMap = make(map[string]string)
	MockDatabase.WebhookLog = make([]map[string]interface{}, 0)

	// Reset counters but keep the ticketCounter map intact
	for key := range MockDatabase.TicketCounters {
		MockDatabase.TicketCounters[key] = 0
	}

	log.Printf("[JIRA MOCK] Database reset")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Database reset successfully",
	})
}
func handleAutoCreateConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "GET" {
		json.NewEncoder(w).Encode(MockDatabase.AutoCreateMappings)
		return
	}

	if r.Method == "POST" {
		var configUpdate map[string]bool
		if err := json.NewDecoder(r.Body).Decode(&configUpdate); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Update the auto-create mappings
		for tableName, enabled := range configUpdate {
			MockDatabase.AutoCreateMappings[tableName] = enabled
		}

		log.Printf("[JIRA MOCK] Updated auto-create mappings: %v", MockDatabase.AutoCreateMappings)

		json.NewEncoder(w).Encode(map[string]string{
			"status":  "success",
			"message": "Auto-create mappings updated",
		})
	}
}

func getWebhookLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	limit := 50 // Default limit
	if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
		fmt.Sscanf(limitParam, "%d", &limit)
		if limit <= 0 {
			limit = 50
		}
	}

	// Return the most recent logs up to the limit
	startIdx := 0
	if len(MockDatabase.WebhookLog) > limit {
		startIdx = len(MockDatabase.WebhookLog) - limit
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs":  MockDatabase.WebhookLog[startIdx:],
		"total": len(MockDatabase.WebhookLog),
		"limit": limit,
	})
}

func createJiraTicketFromServiceNow(payload ServiceNowWebhookPayload) {
	log.Printf("[JIRA MOCK] Starting ticket creation for ServiceNow item %s", payload.SysID)

	data := payload.Data

	// First, determine which Jira project to use based on ServiceNow table
	var projectKey string

	// Map ServiceNow table names to Jira projects
	switch {
	case strings.Contains(payload.TableName, "risk"):
		projectKey = "RISK"
		log.Printf("[JIRA MOCK] Mapping to RISK project")
	case strings.Contains(payload.TableName, "compliance"):
		projectKey = "COMP"
		log.Printf("[JIRA MOCK] Mapping to COMP project")
	case strings.Contains(payload.TableName, "incident"):
		projectKey = "INC"
		log.Printf("[JIRA MOCK] Mapping to INC project")
	case strings.Contains(payload.TableName, "audit"):
		projectKey = "AUDIT"
		log.Printf("[JIRA MOCK] Mapping to AUDIT project")
	case strings.Contains(payload.TableName, "vendor"):
		projectKey = "VEN"
		log.Printf("[JIRA MOCK] Mapping to VEN project")
	case strings.Contains(payload.TableName, "regulatory"):
		projectKey = "REG"
		log.Printf("[JIRA MOCK] Mapping to REG project")
	default:
		projectKey = "AUDIT"
		log.Printf("[JIRA MOCK] No specific mapping found, defaulting to AUDIT project")
	}

	// Extract common fields
	var summary string
	if title, ok := data["title"].(string); ok && title != "" {
		log.Printf("[JIRA MOCK] Using title field for summary: %s", title)
		summary = title
	} else if shortDesc, ok := data["short_description"].(string); ok && shortDesc != "" {
		log.Printf("[JIRA MOCK] Using short_description field for summary: %s", shortDesc)
		summary = shortDesc
	} else if name, ok := data["name"].(string); ok && name != "" {
		log.Printf("[JIRA MOCK] Using name field for summary: %s", name)
		summary = name
	} else {
		summary = fmt.Sprintf("ServiceNow %s: %s",
			strings.Replace(payload.TableName, "sn_", "", 1), payload.SysID)
		log.Printf("[JIRA MOCK] No title found, using default summary: %s", summary)
	}

	var description string
	if desc, ok := data["description"].(string); ok && desc != "" {
		description = desc
		log.Printf("[JIRA MOCK] Using description field: %s", description)
	} else if notes, ok := data["notes"].(string); ok && notes != "" {
		description = notes
		log.Printf("[JIRA MOCK] Using notes field for description: %s", description)
	} else if comments, ok := data["comments"].(string); ok && comments != "" {
		description = comments
		log.Printf("[JIRA MOCK] Using comments field for description: %s", description)
	} else {
		description = fmt.Sprintf("ServiceNow item imported from %s with ID: %s",
			payload.TableName, payload.SysID)
		log.Printf("[JIRA MOCK] No description found, using default: %s", description)
	}

	// Build fields map for Jira issue
	fields := map[string]interface{}{
		"project": map[string]string{
			"key": projectKey,
		},
		"issuetype": map[string]string{
			"name": "Task", // Default issue type
		},
		"summary":                   summary,
		"description":               description,
		"customfield_servicenow_id": payload.SysID,
	}

	// Add priority if available
	if severity, ok := data["severity"].(string); ok && severity != "" {
		var priority string
		switch strings.ToLower(severity) {
		case "critical", "high", "1":
			priority = "High"
		case "medium", "moderate", "2":
			priority = "Medium"
		case "low", "3":
			priority = "Low"
		default:
			priority = "Medium"
		}

		fields["priority"] = map[string]string{
			"name": priority,
		}
		log.Printf("[JIRA MOCK] Setting priority to %s based on severity %s", priority, severity)
	} else if priority, ok := data["priority"].(string); ok && priority != "" {
		var jiraPriority string
		switch strings.ToLower(priority) {
		case "critical", "high", "1":
			jiraPriority = "High"
		case "medium", "moderate", "2":
			jiraPriority = "Medium"
		case "low", "3":
			jiraPriority = "Low"
		default:
			jiraPriority = "Medium"
		}

		fields["priority"] = map[string]string{
			"name": jiraPriority,
		}
		log.Printf("[JIRA MOCK] Setting priority to %s based on priority %s", jiraPriority, priority)
	}

	// Add due date if available
	if dueDate, ok := data["due_date"].(string); ok && dueDate != "" {
		fields["duedate"] = dueDate
		log.Printf("[JIRA MOCK] Setting due date to %s", dueDate)
	}

	// Add assignee if available
	if assignee, ok := data["assigned_to"].(string); ok && assignee != "" {
		fields["assignee"] = map[string]string{
			"name": assignee,
		}
		log.Printf("[JIRA MOCK] Setting assignee to %s from assigned_to", assignee)
	} else if owner, ok := data["owner"].(string); ok && owner != "" {
		fields["assignee"] = map[string]string{
			"name": owner,
		}
		log.Printf("[JIRA MOCK] Setting assignee to %s from owner", owner)
	} else if assignedTo, ok := data["assigned_to_user"].(string); ok && assignedTo != "" {
		fields["assignee"] = map[string]string{
			"name": assignedTo,
		}
		log.Printf("[JIRA MOCK] Setting assignee to %s from assigned_to_user", assignedTo)
	}

	// Create direct issue via the Jira API
	log.Printf("[JIRA MOCK] Creating Jira ticket with fields: %+v", fields)

	// Create the request body
	requestBody := map[string]interface{}{
		"fields": fields,
	}

	// Convert to JSON
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		log.Printf("[JIRA MOCK] Error marshalling request body: %v", err)
		return
	}

	// Print the full JSON request for debugging
	log.Printf("[JIRA MOCK] Sending Jira API request: %s", string(jsonBody))

	// Make the API call to create the Jira issue
	resp, err := http.Post("http://localhost:4000/rest/api/2/issue",
		"application/json", bytes.NewBuffer(jsonBody))

	if err != nil {
		log.Printf("[JIRA MOCK] Error creating Jira ticket: %v", err)
		return
	}
	defer resp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[JIRA MOCK] Error reading response body: %v", err)
		return
	}

	// Log the response
	log.Printf("[JIRA MOCK] Jira API response status: %d", resp.StatusCode)
	log.Printf("[JIRA MOCK] Jira API response body: %s", string(respBody))

	// Parse the response
	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		log.Printf("[JIRA MOCK] Error parsing response: %v", err)
		return
	}

	// Process the response
	if key, ok := response["key"].(string); ok {
		log.Printf("[JIRA MOCK] Successfully created Jira ticket %s for ServiceNow item %s (%s)",
			key, payload.SysID, summary)

		// Map the ServiceNow ID to the Jira key
		MockDatabase.ServiceNowJiraMap[payload.SysID] = key

		// Create a JiraTicket struct to directly add to the DB
		// This ensures the ticket shows up even if API call had issues
		id := fmt.Sprintf("10%d", len(MockDatabase.Tickets)+1)

		ticket := JiraTicket{
			ID:          id,
			Key:         key,
			Self:        fmt.Sprintf("http://localhost:4000/rest/api/2/issue/%s", key),
			Summary:     summary,
			Description: description,
			Status:      "To Do",
			Priority:    fields["priority"].(map[string]string)["name"],
			Created:     time.Now().Format(time.RFC3339),
			Updated:     time.Now().Format(time.RFC3339),
			Fields:      fields,
			Comments:    []JiraComment{},
		}

		// Add the ticket directly to the database
		MockDatabase.Tickets[key] = ticket
		log.Printf("[JIRA MOCK] Added ticket to MockDatabase: %s", key)
	} else {
		log.Printf("[JIRA MOCK] Failed to create Jira ticket: %v", response)
	}

	// REMOVED: Don't call notifySlack here since ServiceNow already sent to Slack directly
	// go notifySlack(ticket, "created")
}
func updateJiraTicketFromServiceNow(jiraKey string, payload ServiceNowWebhookPayload) {
	data := payload.Data

	// Get the current ticket
	ticket, exists := MockDatabase.Tickets[jiraKey]
	if !exists {
		log.Printf("[JIRA MOCK] Cannot update Jira ticket %s: not found", jiraKey)
		return
	}

	// Start building the update
	updated := false
	fields := make(map[string]interface{})

	// Check for title/summary update
	if title, ok := data["title"].(string); ok && title != "" && title != ticket.Summary {
		fields["summary"] = title
		ticket.Summary = title
		updated = true
	} else if shortDesc, ok := data["short_description"].(string); ok && shortDesc != "" && shortDesc != ticket.Summary {
		fields["summary"] = shortDesc
		ticket.Summary = shortDesc
		updated = true
	}

	// Check for description update
	if desc, ok := data["description"].(string); ok && desc != ticket.Description {
		fields["description"] = desc
		ticket.Description = desc
		updated = true
	}

	// Check for status update
	if status, ok := data["status"].(string); ok && status != "" {
		// Map ServiceNow status to Jira status
		var jiraStatus string
		switch strings.ToLower(status) {
		case "open", "new", "pending":
			jiraStatus = "To Do"
		case "in progress", "active":
			jiraStatus = "In Progress"
		case "closed", "resolved", "complete", "completed":
			jiraStatus = "Done"
		default:
			// Keep the current status
			jiraStatus = ticket.Status
		}

		if jiraStatus != ticket.Status {
			fields["status"] = map[string]string{
				"name": jiraStatus,
			}

			previousStatus := ticket.Status
			ticket.Status = jiraStatus
			updated = true

			// If status has changed to Done, add a resolution
			if jiraStatus == "Done" {
				fields["resolution"] = map[string]string{
					"name": "Done",
				}
				ticket.Resolution = "Done"
			}

			// Trigger a status change webhook
			go triggerStatusChangeWebhook(jiraKey, jiraStatus, previousStatus)
		}
	}

	// Check for priority update
	if severity, ok := data["severity"].(string); ok && severity != "" {
		var priority string
		switch strings.ToLower(severity) {
		case "critical", "high":
			priority = "High"
		case "medium":
			priority = "Medium"
		case "low":
			priority = "Low"
		default:
			priority = "Medium"
		}

		if priority != ticket.Priority {
			fields["priority"] = map[string]string{
				"name": priority,
			}
			ticket.Priority = priority
			updated = true
		}
	}

	// Check for assignee update
	var newAssignee string
	if assignee, ok := data["assigned_to"].(string); ok && assignee != "" {
		newAssignee = assignee
	} else if owner, ok := data["owner"].(string); ok && owner != "" {
		newAssignee = owner
	}

	if newAssignee != "" && newAssignee != ticket.Assignee {
		fields["assignee"] = map[string]string{
			"name": newAssignee,
		}
		ticket.Assignee = newAssignee
		updated = true
	}

	// Check for due date update
	if dueDate, ok := data["due_date"].(string); ok && dueDate != "" && dueDate != ticket.DueDate {
		fields["duedate"] = dueDate
		ticket.DueDate = dueDate
		updated = true
	}

	// Update the database record
	if updated {
		ticket.Updated = time.Now().Format(time.RFC3339)
		MockDatabase.Tickets[jiraKey] = ticket

		// Store all fields in the ticket's Fields map
		if ticket.Fields == nil {
			ticket.Fields = make(map[string]interface{})
		}
		for k, v := range fields {
			ticket.Fields[k] = v
		}

		log.Printf("[JIRA MOCK] Updated Jira ticket %s from ServiceNow change", jiraKey)
	} else {
		log.Printf("[JIRA MOCK] No changes needed for Jira ticket %s", jiraKey)
	}
}

func notifyServiceNowOfStatusChange(serviceNowID string, newStatus string, previousStatus string) {
	// Map Jira status to ServiceNow status
	var snStatus string
	switch newStatus {
	case "To Do":
		snStatus = "Open"
	case "In Progress":
		snStatus = "In Progress"
	case "Done":
		snStatus = "Closed"
	default:
		snStatus = newStatus // Use the same status if no mapping exists
	}

	// Determine the ServiceNow table from the ID format
	var tableName string
	if strings.HasPrefix(serviceNowID, "RISK") {
		tableName = "sn_risk_risk"
	} else if strings.HasPrefix(serviceNowID, "INC") {
		tableName = "sn_si_incident"
	} else if strings.HasPrefix(serviceNowID, "TASK") {
		tableName = "sn_compliance_task"
	} else if strings.HasPrefix(serviceNowID, "TEST") {
		tableName = "sn_policy_control_test"
	} else if strings.HasPrefix(serviceNowID, "AUDIT") {
		tableName = "sn_audit_finding"
	} else if strings.HasPrefix(serviceNowID, "VR") {
		tableName = "sn_vendor_risk"
	} else if strings.HasPrefix(serviceNowID, "REG") {
		tableName = "sn_regulatory_change"
	} else {
		// Default table if we can't determine from the ID
		tableName = "sn_risk_risk"
	}

	// Prepare the update payload
	updateData := map[string]interface{}{
		"status":        snStatus,
		"updated_on":    time.Now().Format(time.RFC3339),
		"update_source": "jira",
	}

	jsonData, _ := json.Marshal(updateData)

	// Make the API call to update the ServiceNow item
	endpointURL := fmt.Sprintf("http://localhost:3000/api/now/table/%s/%s", tableName, serviceNowID)

	req, err := http.NewRequest("PATCH", endpointURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[JIRA MOCK] Error creating ServiceNow update request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[JIRA MOCK] Error sending status update to ServiceNow: %v", err)
		return
	}
	defer resp.Body.Close()

	// Log the response
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("[JIRA MOCK] Successfully updated ServiceNow item %s status to %s",
			serviceNowID, snStatus)
	} else {
		log.Printf("[JIRA MOCK] Failed to update ServiceNow item %s. Status: %d, Response: %s",
			serviceNowID, resp.StatusCode, string(respBody))
	}

	// Add to webhook log
	logEntry := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"type":      "outgoing",
		"target":    "servicenow",
		"action":    "update_status",
		"item_id":   serviceNowID,
		"changes":   fmt.Sprintf("Status: %s → %s", previousStatus, newStatus),
	}
	MockDatabase.WebhookLog = append(MockDatabase.WebhookLog, logEntry)
}

func testServiceNowConnectivity(w http.ResponseWriter, r *http.Request) {
	// If invoked as a handler, set up response
	if w != nil && r != nil {
		w.Header().Set("Content-Type", "application/json")
	}

	// Create a test request to ServiceNow
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", "http://localhost:3000/api/status", nil)
	if err != nil {
		log.Printf("[JIRA MOCK] Error creating ServiceNow connectivity test request: %v", err)

		if w != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": fmt.Sprintf("Error creating request: %v", err),
			})
		}
		return
	}

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[JIRA MOCK] ServiceNow connectivity test failed: %v", err)

		if w != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": fmt.Sprintf("Connection failed: %v", err),
			})
		}
		return
	}
	defer resp.Body.Close()

	// Read and log response
	body, _ := io.ReadAll(resp.Body)
	log.Printf("[JIRA MOCK] ServiceNow connectivity test result: %d, %s", resp.StatusCode, string(body))

	if w != nil {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "success",
				"message": "Successfully connected to ServiceNow",
			})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": fmt.Sprintf("ServiceNow returned status %d", resp.StatusCode),
			})
		}
	}
}

func addCommentToServiceNow(serviceNowID string, body string, author string) {
	// Determine the ServiceNow table from the ID format
	var tableName string
	if strings.HasPrefix(serviceNowID, "RISK") {
		tableName = "sn_risk_risk"
	} else if strings.HasPrefix(serviceNowID, "INC") {
		tableName = "sn_si_incident"
	} else if strings.HasPrefix(serviceNowID, "TASK") {
		tableName = "sn_compliance_task"
	} else if strings.HasPrefix(serviceNowID, "TEST") {
		tableName = "sn_policy_control_test"
	} else if strings.HasPrefix(serviceNowID, "AUDIT") {
		tableName = "sn_audit_finding"
	} else if strings.HasPrefix(serviceNowID, "VR") {
		tableName = "sn_vendor_risk"
	} else if strings.HasPrefix(serviceNowID, "REG") {
		tableName = "sn_regulatory_change"
	} else {
		// Default table if we can't determine from the ID
		tableName = "sn_risk_risk"
	}

	// Prepare the comment payload - for simplicity, we'll just update the record with a comments field
	updateData := map[string]interface{}{
		"comments":      fmt.Sprintf("[JIRA Comment from %s] %s", author, body),
		"updated_on":    time.Now().Format(time.RFC3339),
		"update_source": "jira",
	}

	jsonData, _ := json.Marshal(updateData)

	// Make the API call to update the ServiceNow item
	endpointURL := fmt.Sprintf("http://localhost:3000/api/now/table/%s/%s", tableName, serviceNowID)

	req, err := http.NewRequest("PATCH", endpointURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[JIRA MOCK] Error creating ServiceNow comment request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[JIRA MOCK] Error sending comment to ServiceNow: %v", err)
		return
	}
	defer resp.Body.Close()

	// Log the response
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("[JIRA MOCK] Successfully added comment to ServiceNow item %s",
			serviceNowID)
	} else {
		log.Printf("[JIRA MOCK] Failed to add comment to ServiceNow item %s. Status: %d, Response: %s",
			serviceNowID, resp.StatusCode, string(respBody))
	}

	// Add to webhook log
	logEntry := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"type":      "outgoing",
		"target":    "servicenow",
		"action":    "add_comment",
		"item_id":   serviceNowID,
		"author":    author,
	}
	MockDatabase.WebhookLog = append(MockDatabase.WebhookLog, logEntry)
}

func handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	html := `
        <!DOCTYPE html>
        <html lang="en">
        <head>
            <meta charset="UTF-8">
            <meta name="viewport" content="width=device-width, initial-scale=1.0">
            <title>Mock Jira UI</title>
            <style>
                :root {
                    --primary: #1976d2;
                    --primary-dark: #0d47a1;
                    --primary-light: #e3f2fd;
                    --text: #37474f;
                    --text-secondary: #78909c;
                    --bg: #f5f7fa;
                    --card-bg: #ffffff;
                    --border: #e0e4e8;
                    --success: #2e7d32;
                    --success-light: #edf7ed;
                    --error: #d32f2f;
                    --error-light: #fdecea;
                    --status-todo: #1976d2;
                    --status-progress: #7b1fa2;
                    --status-resolved: #388e3c;
                    --status-done: #546e7a;
                    --radius: 6px;
                    --shadow: 0 2px 6px rgba(0,0,0,0.04);
                    --shadow-hover: 0 4px 8px rgba(0,0,0,0.08);
                    --transition: all 0.2s ease;
                    --header-bg: #0d47a1;
                }
                
                * {
                    box-sizing: border-box;
                    margin: 0;
                    padding: 0;
                }
                
                body {
                    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, 'Open Sans', sans-serif;
                    line-height: 1.6;
                    color: var(--text);
                    background-color: var(--bg);
                    margin: 0;
                    padding: 0;
                }
                
                .container {
                    max-width: 1300px;
                    margin: 0 auto;
                    padding: 1.5rem;
                }
                
                header {
                    background-color: var(--header-bg);
                    color: white;
                    padding: 1.25rem;
                    border-radius: var(--radius);
                    margin-bottom: 1.5rem;
                    display: flex;
                    align-items: center;
                    box-shadow: var(--shadow);
                }
                
                h1, h2, h3 {
                    font-weight: 500;
                    margin-bottom: 0.75rem;
                    color: var(--text);
                }
                
                h1 {
                    font-size: 1.5rem;
                    color: white;
                    margin-bottom: 0;
                    letter-spacing: 0.02em;
                }
                
                h2 {
                    font-size: 1.25rem;
                    border-bottom: 1px solid var(--border);
                    padding-bottom: 0.5rem;
                    margin-bottom: 1rem;
                }
                
                h3 {
                    font-size: 1.125rem;
                    font-weight: 600;
                }
                
                .card {
                    background-color: var(--card-bg);
                    border-radius: var(--radius);
                    box-shadow: var(--shadow);
                    padding: 1.5rem;
                    margin-bottom: 1.25rem;
                    border: 1px solid var(--border);
                    transition: var(--transition);
                }
                
                .card:hover {
                    box-shadow: var(--shadow-hover);
                }
                
                .row {
                    display: flex;
                    flex-wrap: wrap;
                    margin: 0 -0.75rem;
                }
                
                .col {
                    flex: 1;
                    padding: 0 0.75rem;
                    min-width: 300px;
                }
                
                .form-group {
                    margin-bottom: 1.25rem;
                }
                
                label {
                    display: block;
                    margin-bottom: 0.5rem;
                    font-weight: 500;
                    font-size: 0.875rem;
                    color: var(--text);
                }
                
                input, select, textarea {
                    width: 100%;
                    padding: 0.625rem 0.75rem;
                    border: 1px solid var(--border);
                    border-radius: var(--radius);
                    background-color: var(--card-bg);
                    color: var(--text);
                    font-family: inherit;
                    font-size: 0.875rem;
                    transition: var(--transition);
                }
                
                input:focus, select:focus, textarea:focus {
                    outline: none;
                    border-color: var(--primary);
                    box-shadow: 0 0 0 2px rgba(25,118,210,0.2);
                }
                
                button {
                    background-color: var(--primary);
                    color: white;
                    border: none;
                    padding: 0.625rem 1rem;
                    border-radius: var(--radius);
                    cursor: pointer;
                    font-size: 0.875rem;
                    font-weight: 500;
                    transition: var(--transition);
                }
                
                button:hover {
                    background-color: var(--primary-dark);
                    transform: translateY(-1px);
                }
                
                .ticket-list {
                    list-style: none;
                    padding: 0;
                }
                
                .ticket-item {
                    background-color: var(--card-bg);
                    border-left: 4px solid var(--primary);
                    border-radius: var(--radius);
                    padding: 1rem 1.25rem;
                    margin-bottom: 0.75rem;
                    box-shadow: var(--shadow);
                    cursor: pointer;
                    transition: var(--transition);
                }
                
                .ticket-item:hover {
                    background-color: var(--primary-light);
                    transform: translateY(-2px);
                    box-shadow: var(--shadow-hover);
                }
                
                .ticket-header {
                    display: flex;
                    justify-content: space-between;
                    align-items: center;
                    margin-bottom: 0.625rem;
                }
                
                .ticket-key {
                    color: var(--primary);
                    font-weight: 600;
                    font-size: 0.875rem;
                    letter-spacing: 0.03em;
                }
                
                .ticket-status {
                    display: inline-block;
                    padding: 0.25rem 0.75rem;
                    border-radius: 12px;
                    font-size: 0.75rem;
                    font-weight: 500;
                    text-transform: uppercase;
                    letter-spacing: 0.03em;
                }
                
                .status-to-do, .status-open {
                    background-color: var(--status-todo);
                    color: white;
                }
                
                .status-in-progress, .status-inprogress {
                    background-color: var(--status-progress);
                    color: white;
                }
                
                .status-resolved {
                    background-color: var(--status-resolved);
                    color: white;
                }
                
                .status-closed, .status-done {
                    background-color: var(--status-done);
                    color: white;
                }
                
                .ticket-detail {
                    padding: 1.25rem;
                    border: 1px solid var(--border);
                    border-radius: var(--radius);
                    margin-top: 1.25rem;
                    background-color: var(--card-bg);
                }
                
                .tabs {
                    display: flex;
                    border-bottom: 1px solid var(--border);
                    margin-bottom: 1.5rem;
                    background-color: var(--card-bg);
                    border-radius: var(--radius) var(--radius) 0 0;
                    padding: 0 0.5rem;
                }
                
                .tab {
                    padding: 0.875rem 1.25rem;
                    cursor: pointer;
                    font-weight: 500;
                    color: var(--text-secondary);
                    border-bottom: 2px solid transparent;
                    transition: var(--transition);
                    position: relative;
                }
                
                .tab:hover {
                    color: var(--primary);
                }
                
                .tab.active {
                    color: var(--primary);
                    border-bottom: 2px solid var(--primary);
                    font-weight: 600;
                }
                
                .tab-content {
                    display: none;
                    animation: fadeIn 0.25s ease;
                }
                
                .tab-content.active {
                    display: block;
                }
                
                @keyframes fadeIn {
                    from { opacity: 0; }
                    to { opacity: 1; }
                }
                
                .comment {
                    padding: 1rem;
                    border-left: 3px solid var(--border);
                    margin-bottom: 1rem;
                    background-color: var(--bg);
                    border-radius: var(--radius);
                }
                
                .comment-author {
                    font-weight: 600;
                    font-size: 0.875rem;
                    color: var(--text);
                    margin-bottom: 0.375rem;
                }
                
                .comment-date {
                    color: var(--text-secondary);
                    font-size: 0.75rem;
                    margin-bottom: 0.75rem;
                }
                
                .webhook-log {
                    background-color: #f8f9fa;
                    border: 1px solid var(--border);
                    border-radius: var(--radius);
                    padding: 1rem;
                    height: 300px;
                    overflow-y: auto;
                    font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
                    font-size: 0.75rem;
                    line-height: 1.5;
                }
                
                .log-entry {
                    margin-bottom: 0.5rem;
                    padding: 0.5rem;
                    border-bottom: 1px solid #eee;
                }
                
                .transition-btn {
                    margin-right: 0.75rem;
                    margin-bottom: 0.75rem;
                    font-size: 0.8125rem;
                }
                
                .alert {
                    padding: 0.875rem 1.25rem;
                    margin-bottom: 1.25rem;
                    border-radius: var(--radius);
                    display: none;
                    animation: slideIn 0.3s ease;
                }
                
                @keyframes slideIn {
                    from { transform: translateY(-10px); opacity: 0; }
                    to { transform: translateY(0); opacity: 1; }
                }
                
                .alert-success {
                    background-color: var(--success-light);
                    border: 1px solid var(--success);
                    color: var(--success);
                }
                
                .alert-error {
                    background-color: var(--error-light);
                    border: 1px solid var(--error);
                    color: var(--error);
                }
                
                .refresh-btn {
                    margin-left: 0.625rem;
                    padding: 0.375rem 0.75rem;
                    font-size: 0.75rem;
                    background-color: var(--text-secondary);
                }
                
                .refresh-btn:hover {
                    background-color: var(--text);
                }
                
                .project-list {
                    list-style: none;
                    padding: 0;
                }
                
                .project-item {
                    background-color: var(--card-bg);
                    border-left: 3px solid var(--primary);
                    border-radius: var(--radius);
                    padding: 1rem 1.25rem;
                    margin-bottom: 0.75rem;
                    box-shadow: var(--shadow);
                    cursor: pointer;
                    transition: var(--transition);
                }
                
                .project-item:hover {
                    transform: translateY(-2px);
                    background-color: var(--primary-light);
                    box-shadow: var(--shadow-hover);
                }
                
                .project-header {
                    font-weight: 500;
                    color: var(--primary);
                    font-size: 0.9375rem;
                }
                
                .project-tickets {
                    padding: 0.75rem 0;
                }
                
                .project-tickets table {
                    width: 100%;
                    border-collapse: collapse;
                    border: 1px solid var(--border);
                    border-radius: var(--radius);
                    overflow: hidden;
                }
                
                .project-tickets th, .project-tickets td {
                    padding: 0.75rem 1rem;
                    text-align: left;
                    border-bottom: 1px solid var(--border);
                    font-size: 0.875rem;
                }
                
                .project-tickets th {
                    background-color: #f8f9fa;
                    font-weight: 600;
                    color: var(--text);
                    border-bottom: 2px solid var(--border);
                }
                
                .ticket-row {
                    cursor: pointer;
                    transition: var(--transition);
                }
                
                .ticket-row:hover {
                    background-color: var(--primary-light);
                }
                
                #searchButton {
                    margin-top: 0.5rem;
                }
                
                .search-container {
                    display: flex;
                    gap: 0.5rem;
                    align-items: center;
                }
                
                .search-container input {
                    flex: 1;
                }
                
                .dashboard-summary {
                    margin-bottom: 2rem;
                }
                
                .stats-cards {
                    display: grid;
                    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
                    gap: 1rem;
                    margin-bottom: 2rem;
                }
                
                .stat-card {
                    background-color: var(--card-bg);
                    border-radius: var(--radius);
                    padding: 1.25rem;
                    text-align: center;
                    box-shadow: var(--shadow);
                    border: 1px solid var(--border);
                }
                
                .stat-value {
                    font-size: 1.75rem;
                    font-weight: bold;
                    color: var(--primary);
                    margin-bottom: 0.5rem;
                }
                
                .stat-label {
                    font-size: 0.875rem;
                    color: var(--text-secondary);
                    text-transform: uppercase;
                    letter-spacing: 0.05em;
                }
                
                .btn-group {
                    display: flex;
                    gap: 0.75rem;
                }
                
                .section-header {
                    display: flex;
                    justify-content: space-between;
                    align-items: center;
                    margin-bottom: 1rem;
                }
            </style>
        </head>
        <body>
            <div class="container">
                <header>
                    <h1>Mock Jira Server</h1>
                </header>
                
                <div id="alertContainer"></div>
                
                <div class="tabs">
                    <div class="tab active" data-tab="tickets">Tickets</div>
                    <div class="tab" data-tab="projects">Projects</div>
                    <div class="tab" data-tab="create">Create Ticket</div>
                    <div class="tab" data-tab="webhooks">Webhooks</div>
                    <div class="tab" data-tab="admin">Admin</div>
                </div>
                
                <div id="tickets" class="tab-content active">
                    <!-- Dashboard stats overview -->
                    <div class="dashboard-summary">
                        <div class="stats-cards">
                            <div class="stat-card">
                                <div class="stat-value" id="total-tickets">--</div>
                                <div class="stat-label">Total Tickets</div>
                            </div>
                            <div class="stat-card">
                                <div class="stat-value" id="todo-tickets">--</div>
                                <div class="stat-label">To Do</div>
                            </div>
                            <div class="stat-card">
                                <div class="stat-value" id="progress-tickets">--</div>
                                <div class="stat-label">In Progress</div>
                            </div>
                            <div class="stat-card">
                                <div class="stat-value" id="done-tickets">--</div>
                                <div class="stat-label">Done</div>
                            </div>
                        </div>
                    </div>
                
                    <div class="card">
                        <div class="section-header">
                            <h2>Search Tickets</h2>
                            <button id="refreshTickets" class="refresh-btn">Refresh</button>
                        </div>
                        <div class="search-container">
                            <input type="text" id="searchInput" placeholder="Search by key, summary, or description">
                            <button id="searchButton">Search</button>
                        </div>
                    </div>
                    
                    <div class="row">
                        <div class="col">
                            <div class="card">
                                <h2>Tickets</h2>
                                <ul id="ticketList" class="ticket-list">
                                    <!-- Tickets will be loaded here -->
                                </ul>
                            </div>
                        </div>
                        
                        <div class="col">
                            <div class="card">
                                <h2>Ticket Details</h2>
                                <div id="ticketDetail">
                                    <p>Select a ticket to view details</p>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
                
                <div id="projects" class="tab-content">
                    <div class="card">
                        <h2>Jira Projects</h2>
                        <div class="row">
                            <div class="col">
                                <div id="project-list" class="project-list">
                                    <!-- Projects will be loaded here -->
                                </div>
                            </div>
                            <div class="col">
                                <div id="project-tickets" class="project-tickets">
                                    <h3>Project Tickets</h3>
                                    <p>Select a project to view tickets</p>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
                
                <div id="create" class="tab-content">
                    <div class="card">
                        <h2>Create New Ticket</h2>
                        <form id="createTicketForm" onsubmit="return false;">
                            <div class="form-group">
                                <label for="projectKey">Project</label>
                                <select id="projectKey" required>
                                    <option value="AUDIT">Audit Management (AUDIT)</option>
                                    <option value="RISK">Risk Management (RISK)</option>
                                    <option value="INC">Incident Management (INC)</option>
                                    <option value="COMP">Compliance Tasks (COMP)</option>
                                    <option value="VEN">Vendor Risk (VEN)</option>
                                    <option value="REG">Regulatory Changes (REG)</option>
                                </select>
                            </div>
                            <div class="form-group">
                                <label for="ticketType">Issue Type</label>
                                <select id="ticketType" required onchange="updateFormFields()">
                                    <option value="Task">Task</option>
                                    <option value="Epic">Epic</option>
                                    <option value="Subtask">Subtask</option>
                                </select>
                            </div>
                            
                            <!-- Fields for all issue types -->
                            <div class="form-group">
                                <label for="summary">Summary</label>
                                <input type="text" id="summary" required>
                            </div>
                            
                            <div class="form-group">
                                <label for="description">Description</label>
                                <textarea id="description" rows="5"></textarea>
                            </div>
                            
                            <!-- Epic-specific fields -->
                            <div id="epicFields" class="issue-type-fields" style="display:none;">
                                <div class="form-group">
                                    <label for="epicName">Epic Name</label>
                                    <input type="text" id="epicName">
                                </div>
                                <div class="form-group">
                                    <label for="epicColor">Epic Color</label>
                                    <select id="epicColor">
                                        <option value="green">Green</option>
                                        <option value="blue">Blue</option>
                                        <option value="red">Red</option>
                                        <option value="yellow">Yellow</option>
                                        <option value="purple">Purple</option>
                                    </select>
                                </div>
                            </div>
                            
                            <!-- Subtask-specific fields -->
                            <div id="subtaskFields" class="issue-type-fields" style="display:none;">
                                <div class="form-group">
                                    <label for="parentIssue">Parent Issue</label>
                                    <input type="text" id="parentIssue" placeholder="Enter parent issue key (e.g., AUDIT-1)">
                                </div>
                            </div>

                            <!-- Common fields that may vary by issue type -->
                            <div id="commonFields">
                                <div class="row">
                                    <div class="col">
                                        <div class="form-group">
                                            <label for="priority">Priority</label>
                                            <select id="priority">
                                                <option value="Highest">Highest</option>
                                                <option value="High">High</option>
                                                <option value="Medium" selected>Medium</option>
                                                <option value="Low">Low</option>
                                                <option value="Lowest">Lowest</option>
                                            </select>
                                        </div>
                                    </div>
                                    <div class="col">
                                        <div class="form-group">
                                            <label for="dueDate">Due Date</label>
                                            <input type="date" id="dueDate">
                                        </div>
                                    </div>
                                </div>
                                <div class="form-group">
                                    <label for="assignee">Assignee</label>
                                    <input type="text" id="assignee" placeholder="Enter assignee name">
                                </div>
                                <div class="form-group">
                                    <label for="serviceNowId">ServiceNow ID</label>
                                    <input type="text" id="serviceNowId" placeholder="Enter ServiceNow ID">
                                </div>
                            </div>
                            
                            <button type="button" id="createTicketBtn">Create Ticket</button>
                        </form>
                    </div>
                </div>
                
                <div id="webhooks" class="tab-content">
                    <div class="card">
                        <h2>Webhook Configuration</h2>
                        <div class="form-group">
                            <label for="webhookUrl">Webhook URL</label>
                            <input type="text" id="webhookUrl" placeholder="http://localhost:8081/api/webhooks/jira" value="http://localhost:8081/api/webhooks/jira">
                        </div>
                        <div class="form-group">
                            <label for="webhookIssueKey">Issue Key (optional)</label>
                            <input type="text" id="webhookIssueKey" placeholder="AUDIT-1" value="AUDIT-1">
                        </div>
                    </div>

                    <div class="card">
                        <h2>Trigger Webhook Events</h2>
                        <div class="row">
                            <div class="col">
                                <h3>Issue Events</h3>
                                <div class="form-group">
                                    <div class="btn-group">
                                        <button id="triggerIssueCreated" class="webhook-btn" data-event="issue_created">Issue Created</button>
                                        <button id="triggerIssueUpdated" class="webhook-btn" data-event="issue_updated">Issue Updated</button>
                                    </div>
                                </div>
                                <div class="form-group">
                                    <label for="issueWebhookData">Additional Data (JSON)</label>
                                    <textarea id="issueWebhookData" rows="5" placeholder='{"summary": "Custom issue summary", "description": "Custom description", "status": "In Progress", "servicenow_id": "INC123456"}'></textarea>
                                </div>
                            </div>
                            <div class="col">
                                <h3>Comment Events</h3>
                                <div class="form-group">
                                    <div class="btn-group">
                                        <button id="triggerCommentCreated" class="webhook-btn" data-event="comment_created">Comment Created</button>
                                        <button id="triggerCommentUpdated" class="webhook-btn" data-event="comment_updated">Comment Updated</button>
                                    </div>
                                </div>
                                <div class="form-group">
                                    <label for="commentWebhookData">Comment Data (JSON)</label>
                                    <textarea id="commentWebhookData" rows="5" placeholder='{"comment": "This is a test comment", "comment_id": "12345"}'></textarea>
                                </div>
                            </div>
                        </div>
                    </div>
                    
                    <div class="card">
                        <div class="section-header">
                            <h2>Webhook Event Log</h2>
                            <button id="refreshWebhookLog" class="refresh-btn">Refresh</button>
                        </div>
                        <div id="webhookLog" class="webhook-log">
                            <!-- Webhook logs will be displayed here -->
                        </div>
                    </div>
                </div>
                
                <div id="admin" class="tab-content">
                    <div class="card">
                        <h2>Admin Tools</h2>
                        <div class="form-group">
                            <label>Database Management</label>
                            <div class="btn-group">
                                <button id="resetDatabase">Reset Database</button>
                                <button id="testServicenow">Test ServiceNow Connection</button>
                            </div>
                        </div>
                        <div class="form-group">
                            <label>Server Status</label>
                            <div id="serverStatus" style="padding: 0.75rem; background-color: var(--bg); border-radius: var(--radius);">Checking...</div>
                        </div>
                    </div>
                </div>
            </div>

            <script>
                // Base URL for API requests
                const API_BASE_URL = '';
                
                // DOM elements
                const dom = {
                    alertContainer: document.getElementById('alertContainer'),
                    ticketList: document.getElementById('ticketList'),
                    ticketDetail: document.getElementById('ticketDetail'),
                    searchInput: document.getElementById('searchInput'),
                    searchButton: document.getElementById('searchButton'),
                    createTicketForm: document.getElementById('createTicketForm'),
                    createTicketBtn: document.getElementById('createTicketBtn'),
                    webhookLog: document.getElementById('webhookLog'),
                    refreshTickets: document.getElementById('refreshTickets'),
                    refreshWebhookLog: document.getElementById('refreshWebhookLog'),
                    totalTickets: document.getElementById('total-tickets'),
                    todoTickets: document.getElementById('todo-tickets'),
                    progressTickets: document.getElementById('progress-tickets'),
                    doneTickets: document.getElementById('done-tickets')
                };

                // Utility functions
                const utils = {
                    showAlert: function(message, type = 'success') {
                        const alert = document.createElement('div');
                        alert.className = 'alert alert-' + type;
                        alert.textContent = message;
                        alert.style.display = 'block';
                        
                        dom.alertContainer.innerHTML = '';
                        dom.alertContainer.appendChild(alert);
                        
                        setTimeout(() => {
                            if (dom.alertContainer.contains(alert)) {
                                alert.style.display = 'none';
                                setTimeout(() => {
                                    if (dom.alertContainer.contains(alert)) {
                                        dom.alertContainer.removeChild(alert);
                                    }
                                }, 300);
                            }
                        }, 5000);
                    },
                    formatDate: function(dateString) {
                        return !dateString ? '' : new Date(dateString).toLocaleString();
                    },
                    logWebhookEvent: function(message) {
                        const entry = document.createElement('div');
                        entry.className = 'log-entry';
                        entry.textContent = '[' + new Date().toLocaleTimeString() + '] ' + message;
                        dom.webhookLog.prepend(entry);
                    },
                    updateDashboardStats: function(tickets) {
                        const total = tickets.length;
                        let todo = 0;
                        let inProgress = 0;
                        let done = 0;
                        
                        tickets.forEach(ticket => {
                            if (ticket.status === 'To Do') {
                                todo++;
                            } else if (ticket.status === 'In Progress') {
                                inProgress++;
                            } else if (ticket.status === 'Done' || ticket.status === 'Closed' || ticket.status === 'Resolved') {
                                done++;
                            }
                        });
                        
                        dom.totalTickets.textContent = total;
                        dom.todoTickets.textContent = todo;
                        dom.progressTickets.textContent = inProgress;
                        dom.doneTickets.textContent = done;
                    }
                };

                // API functions
                const api = {
                    fetch: async function(endpoint, options = {}) {
                        try {
                            const response = await fetch(API_BASE_URL + endpoint, options);
                            if (!response.ok) throw new Error('API error: ' + response.status);
                            return await response.json();
                        } catch (error) {
                            console.error("API error for " + endpoint + ": " + error);
                            throw error;
                        }
                    },
                    triggerWebhook: async function(eventType, data) {
                        const webhookUrl = document.getElementById('webhookUrl').value.trim();
                        let url = "/trigger_webhook/" + eventType;

                        if (webhookUrl) url += '?webhook_url=' + encodeURIComponent(webhookUrl);
                        
                        try {
                            await api.fetch(url, {
                                method: 'POST',
                                headers: {'Content-Type': 'application/json'},
                                body: JSON.stringify(data)
                            });
                            utils.showAlert('Webhook triggered successfully');
                            utils.logWebhookEvent(eventType.replace("_", " ") + " webhook sent for issue " + data.issue_key);
                            this.getWebhookLogs();
                        } catch (error) {
                            utils.showAlert("Failed to trigger webhook: " + error.message, "error");
                        }
                    },
                    getWebhookLogs: async function() {
                        try {
                            const logs = await api.fetch('/api/webhook_logs');
                            dom.webhookLog.innerHTML = '';
                            
                            if (!logs.logs || logs.logs.length === 0) {
                                dom.webhookLog.innerHTML = '<div class="log-entry">No logs found</div>';
                                return;
                            }
                            
                            logs.logs.forEach(log => {
                                const entry = document.createElement('div');
                                entry.className = 'log-entry';
                                
                                // Format based on log type
                                let logText = '[' + new Date(log.timestamp).toLocaleTimeString() + '] ';
                                
                                if (log.type === 'incoming') {
                                    logText += 'RECEIVED: ';
                                    if (log.source === 'servicenow') {
                                        logText += 'ServiceNow ' + log.action + ' for ' + log.table + ' ID:' + log.sys_id;
                                    } else {
                                        logText += 'External webhook';
                                    }
                                } else {
                                    logText += 'SENT: ';
                                    if (log.target === 'servicenow') {
                                        logText += 'ServiceNow ' + log.action + ' for item ' + log.item_id;
                                    } else {
                                        logText += log.event + ' webhook for issue ' + log.issue_key;
                                    }
                                }
                                
                                entry.textContent = logText;
                                dom.webhookLog.appendChild(entry);
                            });
                        } catch (error) {
                            utils.showAlert("Failed to load webhook logs: " + error.message, "error");
                        }
                    },
                    testServiceNowConnection: async function() {
                        try {
                            const result = await api.fetch('/test_servicenow_connectivity');
                            if (result.status === 'success') {
                                utils.showAlert('Successfully connected to ServiceNow');
                            } else {
                                utils.showAlert('Failed to connect to ServiceNow: ' + result.message, 'error');
                            }
                        } catch (error) {
                            utils.showAlert('Failed to test ServiceNow connection: ' + error.message, 'error');
                        }
                    }
                };

                function updateFormFields() {
                    const ticketType = document.getElementById('ticketType').value;
                    const epicFields = document.getElementById('epicFields');
                    const subtaskFields = document.getElementById('subtaskFields');
                    
                    // Hide all issue-type-specific fields first
                    document.querySelectorAll('.issue-type-fields').forEach(el => {
                        el.style.display = 'none';
                    });
                    
                    // Show fields specific to the selected issue type
                    if (ticketType === 'Epic') {
                        epicFields.style.display = 'block';
                        document.getElementById('epicName').required = true;
                    } else if (ticketType === 'Subtask') {
                        subtaskFields.style.display = 'block';
                        document.getElementById('parentIssue').required = true;
                    }
                }

                // Core functionality
                const app = {
                    async loadTickets() {
                        try {
                            const tickets = await api.fetch('/rest/api/2/issue');
                            dom.ticketList.innerHTML = '';
                            
                            // Update the dashboard stats
                            utils.updateDashboardStats(tickets);
                            
                            if (!tickets || tickets.length === 0) {
                                dom.ticketList.innerHTML = '<li>No tickets found</li>';
                                return;
                            }
                            
                            tickets.forEach(ticket => {
                                const statusClass = "status-" + ticket.status.toLowerCase().replace(/ /g, "-");
                                
                                const html = 
                                    '<div class="ticket-header">' +
                                    '<span class="ticket-key">' + ticket.key + '</span>' +
                                    '<span class="ticket-status ' + statusClass + '">' + ticket.status + '</span>' +
                                    '</div>' +
                                    '<div>' + ticket.summary + '</div>';
                                
                                const li = document.createElement("li");
                                li.className = "ticket-item";
                                li.innerHTML = html;
                                
                                li.addEventListener("click", () => this.loadTicketDetails(ticket.key));
                                dom.ticketList.appendChild(li);
                            });
                        } catch (error) {
                            const errorMessage = "Failed to load tickets: " + error.message;
                            utils.showAlert(errorMessage, "error");
                        }
                    },
                    
                    async loadTicketDetails(key) {
                        try {
                            const issueURL = "/rest/api/2/issue/" + key;
                            const transitionsURL = "/rest/api/2/issue/" + key + "/transitions";
                            const commentURL = "/rest/api/2/issue/" + key + "/comment";
                            
                            const [ticket, transitionsData, commentsData] = await Promise.all([
                                api.fetch(issueURL),
                                api.fetch(transitionsURL),
                                api.fetch(commentURL)
                            ]);

                            // Extract issue type from fields
                            const issueType = ticket.fields?.issuetype?.name || "Unknown";
                            
                            // Build detail HTML
                            const statusClass = "status-" + ticket.status.toLowerCase().replace(/ /g, "-");
                            
                            let html = 
                                '<h3>' + ticket.summary + '</h3>' +
                                '<div>' +
                                '<strong>Key:</strong> ' + ticket.key +
                                '<br><strong>Type:</strong> ' + issueType +
                                '<br><strong>Status:</strong> <span class="ticket-status ' + statusClass + '">' + ticket.status + '</span>' +
                                '<br><strong>Created:</strong> ' + utils.formatDate(ticket.created) +
                                '<br><strong>Updated:</strong> ' + utils.formatDate(ticket.updated);
                            
                            if (ticket.assignee) {
                                html += '<br><strong>Assignee:</strong> ' + ticket.assignee;
                            }
                            if (ticket.priority) {
                                html += '<br><strong>Priority:</strong> ' + ticket.priority;
                            }
                            if (ticket.dueDate) {
                                html += '<br><strong>Due Date:</strong> ' + ticket.dueDate;
                            }
                            
                            // Special handling for ServiceNow ID
                            if (ticket.fields && ticket.fields.customfield_servicenow_id) {
                                html += '<br><strong>ServiceNow ID:</strong> ' + ticket.fields.customfield_servicenow_id;
                            }
                            
                            html += '</div>';
                            
                            // Description
                            if (ticket.description) {
                                html += '<div class="ticket-detail"><h4>Description</h4><p>' + ticket.description + '</p></div>';
                            }
                            
                            // Transitions
                            if (transitionsData && transitionsData.transitions && transitionsData.transitions.length > 0) {
                                html += '<div class="ticket-detail"><h4>Actions</h4><div class="btn-group">';
                                
                                transitionsData.transitions.forEach(transition => {
                                    html += '<button class="transition-btn" data-transition-id="' + transition.id + '" ' +
                                           'data-ticket-key="' + key + '" data-current-status="' + ticket.status + '" ' +
                                           'data-new-status="' + transition.to.name + '">' + transition.name + '</button>';
                                });
                                
                                html += '</div></div>';
                            }
                            
                            // Comments
                            if (commentsData && commentsData.comments && commentsData.comments.length > 0) {
                                html += '<div class="ticket-detail"><h4>Comments</h4><div>';
                                
                                commentsData.comments.forEach(comment => {
                                    html += 
                                        '<div class="comment">' +
                                        '<div class="comment-author">' + comment.author + '</div>' +
                                        '<div class="comment-date">' + utils.formatDate(comment.created) + '</div>' +
                                        '<div>' + comment.body + '</div>' +
                                        '</div>';
                                });
                                
                                html += '</div></div>';
                            }
                            
                            // Add comment form
                            html += 
                                '<div class="ticket-detail">' +
                                '<h4>Add Comment</h4>' +
                                '<div class="form-group">' +
                                '<textarea id="commentBody" rows="3" placeholder="Type your comment here..."></textarea>' +
                                '</div>' +
                                '<button id="addCommentBtn" data-ticket-key="' + key + '">Add Comment</button>' +
                                '</div>';
                            
                            dom.ticketDetail.innerHTML = html;
                            
                            // Add event listeners
                            document.querySelectorAll('.transition-btn').forEach(btn => {
                                btn.addEventListener('click', this.handleTransition.bind(this));
                            });
                            
                            const addCommentBtn = document.getElementById('addCommentBtn');
                            if (addCommentBtn) {
                                addCommentBtn.addEventListener('click', this.handleAddComment.bind(this));
                            }
                            
                        } catch (error) {
                            utils.showAlert("Failed to load ticket details: " + error.message, "error");
                        }
                    },
                    
                    async handleTransition(e) {
                        const btn = e.target;
                        const transitionId = btn.getAttribute('data-transition-id');
                        const ticketKey = btn.getAttribute('data-ticket-key');
                        const currentStatus = btn.getAttribute('data-current-status');
                        const newStatus = btn.getAttribute('data-new-status');
                        
                        try {
                            await api.fetch("/rest/api/2/issue/" + ticketKey + "/transitions", {
                                method: "POST",
                                headers: { "Content-Type": "application/json" },
                                body: JSON.stringify({"transition": {"id": transitionId}})
                            });
                            
                            utils.showAlert("Ticket " + ticketKey + " status changed successfully");
                            
                            // Trigger webhook
                            api.triggerWebhook('issue_updated', {
                                issue_key: ticketKey,
                                status: newStatus,
                                previous_status: currentStatus
                            });
                            
                            this.loadTickets();
                            this.loadTicketDetails(ticketKey);
                            
                        } catch (error) {
                            utils.showAlert("Failed to change ticket status: " + error.message, "error");
                        }
                    },
                    
                    async handleAddComment(e) {
                        const ticketKey = e.target.getAttribute('data-ticket-key');
                        const commentBody = document.getElementById('commentBody').value.trim();
                        
                        if (!commentBody) {
                            utils.showAlert('Comment body cannot be empty', 'error');
                            return;
                        }
                        
                        try {
                            const commentData = await api.fetch("/rest/api/2/issue/" + ticketKey + "/comment", {
                                method: "POST",
                                headers: { "Content-Type": "application/json" },
                                body: JSON.stringify({ "body": commentBody })
                            });
                            
                            // Trigger comment webhook
                            api.triggerWebhook('comment_created', {
                                issue_key: ticketKey,
                                comment: commentBody,
                                comment_id: commentData.id || "12345"
                            });
                            
                            utils.showAlert('Comment added successfully');
                            document.getElementById('commentBody').value = '';
                            this.loadTicketDetails(ticketKey);
                            
                        } catch (error) {
                            utils.showAlert("Failed to add comment: " + error.message, "error");
                        }
                    },
                    
                    async handleSearch() {
                        const searchTerm = dom.searchInput.value.trim();
                        
                        if (!searchTerm) {
                            this.loadTickets();
                            return;
                        }
                        
                        const jql = 'summary ~ "' + searchTerm + '"';
                        
                        try {
                            const data = await api.fetch("/rest/api/2/search?jql=" + encodeURIComponent(jql));
                            dom.ticketList.innerHTML = '';
                            
                            if (!data.issues || data.issues.length === 0) {
                                dom.ticketList.innerHTML = '<li>No matching tickets found</li>';
                                return;
                            }
                            
                            data.issues.forEach(ticket => {
                                const statusClass = 'status-' + ticket.fields.status.name.toLowerCase().replace(/ /g, '-');
                                const li = document.createElement('li');
                                li.className = 'ticket-item';
                                li.innerHTML = 
                                    '<div class="ticket-header">' +
                                    '<span class="ticket-key">' + ticket.key + '</span>' +
                                    '<span class="ticket-status ' + statusClass + '">' + ticket.fields.status.name + '</span>' +
                                    '</div>' +
                                    '<div>' + ticket.fields.summary + '</div>';
                                
                                li.addEventListener('click', () => this.loadTicketDetails(ticket.key));
                                dom.ticketList.appendChild(li);
                            });
                        } catch (error) {
                            utils.showAlert("Search failed: " + error.message, "error");
                        }
                    },
                    
                    async handleCreateTicket(e) {
                        e.preventDefault();
                        console.log("Create ticket form submitted");
                        
                        const ticketType = document.getElementById('ticketType').value;
                        const summary = document.getElementById('summary').value.trim();
                        const description = document.getElementById('description').value.trim();
                        const priority = document.getElementById('priority').value;
                        const dueDate = document.getElementById('dueDate').value;
                        const assignee = document.getElementById('assignee').value.trim();
                        const serviceNowId = document.getElementById('serviceNowId').value.trim();
                        
                        console.log("Form values:", { ticketType, summary, description });

                        if (!summary) {
                            utils.showAlert('Summary is required', 'error');
                            return;
                        }
                        
                        // Validate type-specific required fields
                        if (ticketType === 'Epic' && !document.getElementById('epicName').value.trim()) {
                            utils.showAlert('Epic Name is required for Epic issue type', 'error');
                            return;
                        }
                        
                        if (ticketType === 'Subtask' && !document.getElementById('parentIssue').value.trim()) {
                            utils.showAlert('Parent Issue is required for Subtask issue type', 'error');
                            return;
                        }
                        
                        try {
                            const requestBody = {
                                fields: {
                                    project: {
                                        key: document.getElementById('projectKey').value // This uses the selected project
                                    },
                                    issuetype: {
                                        name: ticketType
                                    },
                                    summary,
                                    description,
                                    priority: {name: priority}
                                }
                            };

                            console.log("Sending request to create ticket:", requestBody);
                            
                            // Add type-specific fields
                            if (ticketType === 'Epic') {
                                requestBody.fields.epicName = document.getElementById('epicName').value.trim();
                                requestBody.fields.epicColor = document.getElementById('epicColor').value;
                            } else if (ticketType === 'Subtask') {
                                requestBody.fields.parent = {
                                    key: document.getElementById('parentIssue').value.trim()
                                };
                            }
                            
                            if (dueDate) requestBody.fields.duedate = dueDate;
                            if (assignee) requestBody.fields.assignee = {name: assignee};
                            if (serviceNowId) requestBody.fields.customfield_servicenow_id = serviceNowId;
                            
                            const result = await api.fetch('/rest/api/2/issue', {
                                method: 'POST',
                                headers: {'Content-Type': 'application/json'},
                                body: JSON.stringify(requestBody)
                            });

                            console.log("Ticket created:", result);
                            
                            // Trigger webhook with the appropriate structure based on issue type
                            const webhookData = {
                                issue_key: result.key,
                                summary,
                                description,
                                status: "To Do",
                                issuetype: ticketType
                            };
                            
                            if (serviceNowId) webhookData.servicenow_id = serviceNowId;
                            
                            // Add type-specific fields to webhook payload
                            if (ticketType === 'Epic') {
                                webhookData.epicName = document.getElementById('epicName').value.trim();
                                webhookData.epicColor = document.getElementById('epicColor').value;
                            } else if (ticketType === 'Subtask') {
                                webhookData.parent = document.getElementById('parentIssue').value.trim();
                            }
                            
                            api.triggerWebhook('issue_created', webhookData);
                            
                            utils.showAlert("Ticket " + result.key + " created successfully");
                            dom.createTicketForm.reset();
                            
                            document.querySelector('.tab[data-tab="tickets"]').click();
                            this.loadTickets();
                            
                        } catch (error) {
                            console.error("Error creating ticket:", error);
                            utils.showAlert("Failed to create ticket: " + error.message, "error");
                        }
                    },
                    
                    setupEventListeners() {
                        // Search
                        dom.searchButton.addEventListener('click', () => this.handleSearch());
                        dom.searchInput.addEventListener('keyup', e => {
                            if (e.key === 'Enter') this.handleSearch();
                        });
                        
                        // Create ticket - use direct button click handler
                        const createTicketBtn = document.getElementById('createTicketBtn');
                        if (createTicketBtn) {
                            console.log("Found create ticket button, attaching click handler");
                            createTicketBtn.addEventListener('click', e => {
                                console.log("Create ticket button clicked");
                                this.handleCreateTicket(e);
                            });
                        } else {
                            console.error('Create ticket button not found');
                        }
                        
                        // Refresh buttons
                        dom.refreshTickets.addEventListener('click', () => this.loadTickets());
                        if (dom.refreshWebhookLog) {
                            dom.refreshWebhookLog.addEventListener('click', () => api.getWebhookLogs());
                        }
                        
                        // Webhook buttons
                        document.querySelectorAll('.webhook-btn').forEach(btn => {
                            btn.addEventListener('click', e => {
                                const eventType = btn.getAttribute('data-event');
                                const issueKey = document.getElementById('webhookIssueKey').value.trim();
                                
                                let requestData = {issue_key: issueKey};
                                
                                try {
                                    if (eventType.includes('issue')) {
                                        const customData = document.getElementById('issueWebhookData').value.trim();
                                        if (customData) Object.assign(requestData, JSON.parse(customData));
                                    } else if (eventType.includes('comment')) {
                                        const customData = document.getElementById('commentWebhookData').value.trim();
                                        if (customData) Object.assign(requestData, JSON.parse(customData));
                                    }
                                    
                                    api.triggerWebhook(eventType, requestData);
                                } catch (error) {
                                    utils.showAlert("Invalid JSON data: " + error.message, "error");
                                }
                            });
                        });
                        
                        // Admin buttons
                        const resetDatabaseBtn = document.getElementById('resetDatabase');
                        if (resetDatabaseBtn) {
                            resetDatabaseBtn.addEventListener('click', () => {
                                if (confirm('Are you sure you want to reset the database? All tickets will be lost.')) {
                                    api.fetch('/reset', {method: 'POST'})
                                        .then(() => {
                                            utils.showAlert('Database reset successfully');
                                            this.loadTickets();
                                        })
                                        .catch(error => {
                                            utils.showAlert("Failed to reset database: " + error.message, "error");
                                        });
                                }
                            });
                        }
                        
                        // ServiceNow test button
                        const testServicenowBtn = document.getElementById('testServicenow');
                        if (testServicenowBtn) {
                            testServicenowBtn.addEventListener('click', () => {
                                api.testServiceNowConnection();
                            });
                        }
                        
                        // Tab navigation
                        document.querySelectorAll('.tab').forEach(tab => {
                            tab.addEventListener('click', () => {
                                const tabId = tab.getAttribute('data-tab');
                                
                                // Update active tab
                                document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
                                tab.classList.add('active');
                                
                                // Update active tab content
                                document.querySelectorAll('.tab-content').forEach(content => content.classList.remove('active'));
                                document.getElementById(tabId).classList.add('active');
                                
                                // Load data for the tab if needed
                                if (tabId === 'webhooks') {
                                    api.getWebhookLogs();
                                } else if (tabId === 'projects') {
                                    loadProjects();
                                }
                            });
                        });
                    },
                    
                    checkServerStatus() {
                        const statusElement = document.getElementById('serverStatus');
                        try {
                            fetch(API_BASE_URL + '/health')
                                .then(response => response.json())
                                .then(data => {
                                    statusElement.textContent = "Server running - Version: " + data.version + " | DB Status: " + data.database;
                                })
                                .catch(error => {
                                    statusElement.textContent = 'Server Error - Unable to connect';
                                    statusElement.style.color = 'red';
                                });
                        } catch (error) {
                            statusElement.textContent = 'Server Error - Unable to connect';
                            statusElement.style.color = 'red';
                        }
                    },
                    
                    init() {
                        this.setupEventListeners();
                        this.loadTickets();
                        this.checkServerStatus();
                        
                        // Call updateFormFields on init to set initial state
                        updateFormFields();

                        // Load projects on init
                        if (document.getElementById('project-list')) {
                            loadProjects();
                        }
                        
                        // Auto-check for webhooks log on init
                        if (document.getElementById('webhookLog')) {
                            api.getWebhookLogs();
                        }
                    }
                };
                
                // Projects functionality
                function loadProjects() {
                    fetch('/rest/api/2/project')
                        .then(response => response.json())
                        .then(projects => {
                            const projectList = document.getElementById('project-list');
                            projectList.innerHTML = '';
                            
                            projects.forEach(project => {
                                const div = document.createElement('div');
                                div.className = 'project-item';
                                div.innerHTML = "<div class=\"project-header\">" + project.key + " - " + project.name + "</div>";
                                div.setAttribute('data-project-key', project.key);
                                
                                div.addEventListener('click', function() {
                                    const projectKey = this.getAttribute('data-project-key');
                                    loadProjectTickets(projectKey);
                                    
                                    // Highlight the selected project
                                    document.querySelectorAll('.project-item').forEach(item => {
                                        item.style.backgroundColor = '';
                                    });
                                    this.style.backgroundColor = '#e3f2fd';
                                });
                                
                                projectList.appendChild(div);
                            });
                        })
                        .catch(error => {
                            console.error('Error loading projects:', error);
                            document.getElementById('project-list').innerHTML = '<div>Error loading projects</div>';
                        });
                }

                function loadProjectTickets(projectKey) {
                    const projectTicketsContainer = document.getElementById('project-tickets');
                    projectTicketsContainer.innerHTML = "<h3>" + projectKey + " Tickets</h3><div>Loading tickets...</div>";
                    
                    fetch("/rest/api/2/search?jql=project=" + encodeURIComponent(projectKey))
                        .then(response => response.json())
                        .then(data => {
                            let html = [
                                "<h3>" + projectKey + " Tickets</h3>",
                                "<table>",
                                "    <thead>",
                                "        <tr>",
                                "            <th>Key</th>",
                                "            <th>Summary</th>",
                                "            <th>Status</th>",
                                "            <th>Created</th>",
                                "        </tr>",
                                "    </thead>",
                                "    <tbody>"
                            ].join("\n");
                            
                            if (data.issues && data.issues.length > 0) {
                                data.issues.forEach(issue => {
                                    html += [
                                        "<tr class='ticket-row' data-ticket-key='" + issue.key + "'>",
                                        "    <td>" + issue.key + "</td>",
                                        "    <td>" + issue.fields.summary + "</td>",
                                        "    <td>" + issue.fields.status.name + "</td>",
                                        "    <td>" + new Date(issue.fields.created).toLocaleDateString() + "</td>",
                                        "</tr>"
                                    ].join("\n");
                                });
                            } else {
                                html += [
                                    "<tr>",
                                    "    <td colspan='4'>No tickets found for this project</td>",
                                    "</tr>"
                                ].join("\n");
                            }
                            
                            html += [
                                "    </tbody>",
                                "</table>"
                            ].join("\n");
                            
                            projectTicketsContainer.innerHTML = html;
                            
                            // Add click event to ticket rows
                            document.querySelectorAll('.ticket-row').forEach(row => {
                                row.addEventListener('click', function() {
                                    const ticketKey = this.getAttribute('data-ticket-key');
                                    
                                    // Switch to tickets tab and load the ticket details
                                    document.querySelector('.tab[data-tab="tickets"]').click();
                                    app.loadTicketDetails(ticketKey);
                                });
                                
                                // Add hover style
                                row.style.cursor = 'pointer';
                                row.addEventListener('mouseover', function() {
                                    this.style.backgroundColor = '#f5f5f5';
                                });
                                row.addEventListener('mouseout', function() {
                                    this.style.backgroundColor = '';
                                });
                            });
                        })
                        .catch(error => {
                            console.error('Error loading project tickets:', error);
                            projectTicketsContainer.innerHTML = "<h3>" + projectKey + " Tickets</h3><div>Error loading tickets</div>";
                        });
                }
                
                // Initialize the application
                document.addEventListener('DOMContentLoaded', () => {
                    app.init();
                });
            </script>
        </body>
        </html>
    `
	w.Write([]byte(html))
}
