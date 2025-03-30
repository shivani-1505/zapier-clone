package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// JiraTicket represents a Jira issue in our mock system
type JiraTicket struct {
	ID          string                 `json:"id"`
	Key         string                 `json:"key"`
	Self        string                 `json:"self"`
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

// MockDatabase holds our mock Jira data
var MockDatabase = map[string]interface{}{
	"tickets": map[string]JiraTicket{},
	"projects": map[string]interface{}{
		"AUDIT": map[string]interface{}{
			"key":  "AUDIT",
			"name": "Audit Management",
		},
	},
}

// ServiceNowJiraMapping maps ServiceNow IDs to Jira ticket keys
var ServiceNowJiraMapping = map[string]string{}

func main() {
	r := mux.NewRouter()

	// Jira REST API endpoints
	r.HandleFunc("/rest/api/2/issue", handleIssues).Methods("GET", "POST")
	r.HandleFunc("/rest/api/2/issue/{key}", handleIssueByKey).Methods("GET", "PUT", "DELETE")
	r.HandleFunc("/rest/api/2/issue/{key}/comment", handleComments).Methods("GET", "POST")
	r.HandleFunc("/rest/api/2/issue/{key}/transitions", handleTransitions).Methods("GET", "POST")
	r.HandleFunc("/rest/api/2/project", handleProjects).Methods("GET")
	r.HandleFunc("/rest/api/2/search", handleSearchIssues).Methods("GET")

	// Webhook receiver
	r.HandleFunc("/api/webhooks/jira", handleReceiveWebhook).Methods("POST")

	// Webhook trigger
	r.HandleFunc("/trigger_webhook/{event_type}", triggerWebhook).Methods("POST")

	// Reset endpoint
	r.HandleFunc("/reset", handleReset).Methods("POST")

	// Health check and UI
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")

	r.HandleFunc("/", handleUI).Methods("GET")

	// Start server
	port := "5000"
	fmt.Printf("Starting mock Jira server on port %s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func handleIssues(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		tickets := MockDatabase["tickets"].(map[string]JiraTicket)
		var results []JiraTicket
		for _, ticket := range tickets {
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

		tickets := MockDatabase["tickets"].(map[string]JiraTicket)
		id := fmt.Sprintf("10%d", len(tickets)+1)
		keyNum := len(tickets) + 1
		var key string
		for {
			key = fmt.Sprintf("AUDIT-%d", keyNum)
			if _, exists := tickets[key]; !exists {
				break
			}
			keyNum++
		}

		summary := ""
		if sum, ok := fields["summary"].(string); ok {
			summary = sum
		}
		description := ""
		if desc, ok := fields["description"].(string); ok {
			description = desc
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

		ticket := JiraTicket{
			ID:          id,
			Key:         key,
			Self:        fmt.Sprintf("http://localhost:3001/rest/api/2/issue/%s", key),
			Summary:     summary,
			Description: description,
			Status:      "To Do",
			Created:     time.Now().Format(time.RFC3339),
			Updated:     time.Now().Format(time.RFC3339),
			DueDate:     dueDate,
			Assignee:    assignee,
			Fields:      fields,
			Comments:    []JiraComment{},
		}

		if serviceNowID != "" {
			ServiceNowJiraMapping[serviceNowID] = key
		}

		tickets[key] = ticket
		MockDatabase["tickets"] = tickets
		log.Printf("Created Jira issue: %s with customfield_servicenow_id: %v", key, ticket.Fields["customfield_servicenow_id"])

		json.NewEncoder(w).Encode(map[string]string{
			"id":   id,
			"key":  key,
			"self": ticket.Self,
		})
	}
}

func handleIssueByKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	key := vars["key"]

	tickets := MockDatabase["tickets"].(map[string]JiraTicket)
	ticket, exists := tickets[key]
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
				ticket.Fields[k] = v
			}
		}

		previousStatus := ticket.Status // Store previous status for changelog
		ticket.Updated = time.Now().Format(time.RFC3339)
		tickets[key] = ticket
		MockDatabase["tickets"] = tickets
		log.Printf("Updated Jira issue: %s to status: %s", key, ticket.Status)

		w.WriteHeader(http.StatusNoContent)
		go triggerStatusChangeWebhook(key, ticket.Status, previousStatus)

	case "DELETE":
		delete(tickets, key)
		MockDatabase["tickets"] = tickets
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleComments(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	key := vars["key"]

	tickets := MockDatabase["tickets"].(map[string]JiraTicket)
	ticket, exists := tickets[key]
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

		comment := JiraComment{
			ID:      fmt.Sprintf("comment-%d", len(ticket.Comments)+1),
			Body:    body,
			Author:  "mock-user",
			Created: time.Now().Format(time.RFC3339),
		}

		ticket.Comments = append(ticket.Comments, comment)
		tickets[key] = ticket
		MockDatabase["tickets"] = tickets

		json.NewEncoder(w).Encode(comment)
	}
}

func handleTransitions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	key := vars["key"]

	tickets := MockDatabase["tickets"].(map[string]JiraTicket)
	ticket, exists := tickets[key]
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
		default:
			http.Error(w, "Invalid transition ID", http.StatusBadRequest)
			return
		}

		ticket.Updated = time.Now().Format(time.RFC3339)
		tickets[key] = ticket
		MockDatabase["tickets"] = tickets

		w.WriteHeader(http.StatusNoContent)
		go triggerStatusChangeWebhook(key, ticket.Status, previousStatus)
	}
}

func handleProjects(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	projects := MockDatabase["projects"].(map[string]interface{})
	var projectList []interface{}
	for _, project := range projects {
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

	tickets := MockDatabase["tickets"].(map[string]JiraTicket)
	var results []JiraTicket

	// Parse JQL (e.g., "project=AUDIT AND customfield_servicenow_id=ctrl003")
	jqlParts := strings.Split(jql, " AND ")
	projectFilter := ""
	customFieldFilter := ""
	for _, part := range jqlParts {
		if strings.HasPrefix(part, "project=") {
			projectFilter = strings.TrimPrefix(part, "project=")
		}
		if strings.HasPrefix(part, "customfield_servicenow_id=") {
			customFieldFilter = strings.TrimPrefix(part, "customfield_servicenow_id=")
		}
	}

	for _, ticket := range tickets {
		matchesProject := projectFilter == "" || strings.HasPrefix(ticket.Key, projectFilter)
		matchesCustomField := customFieldFilter == "" || (ticket.Fields["customfield_servicenow_id"] != nil && ticket.Fields["customfield_servicenow_id"].(string) == customFieldFilter)
		if matchesProject && matchesCustomField {
			if ticket.Fields == nil {
				ticket.Fields = make(map[string]interface{})
			}
			ticket.Fields["duedate"] = ticket.DueDate
			ticket.Fields["assignee"] = map[string]interface{}{
				"name": ticket.Assignee,
			}
			ticket.Fields["status"] = map[string]interface{}{
				"name": ticket.Status,
			}
			results = append(results, ticket)
		}
	}

	log.Printf("Search JQL: %s, Found: %d issues", jql, len(results))

	response := map[string]interface{}{
		"issues":     results,
		"total":      len(results),
		"maxResults": 50,
		"startAt":    0,
	}
	json.NewEncoder(w).Encode(response)
}

func handleReceiveWebhook(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("Received webhook from Jira: %v", payload)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"received"}`))
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
	resp, err := http.Post(webhookURL, "application/json", strings.NewReader(string(jsonPayload)))
	if err != nil {
		http.Error(w, fmt.Sprintf("Error sending webhook: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":     "success",
		"message":    fmt.Sprintf("Jira webhook sent to %s", webhookURL),
		"webhook_id": fmt.Sprintf("mock-jira-webhook-%d", time.Now().UnixNano()),
		"event_type": eventType,
		"issue_key":  issueKey,
	})
}

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

	webhookURL := "http://localhost:8081/api/webhooks/jira"
	jsonPayload, _ := json.Marshal(payload)
	resp, err := http.Post(webhookURL, "application/json", strings.NewReader(string(jsonPayload)))
	if err != nil {
		log.Printf("Error sending status change webhook: %v", err)
		return
	}
	defer resp.Body.Close()

	log.Printf("Status change webhook sent for issue %s (from %s to %s)", issueKey, previousStatus, newStatus)
}

func buildIssueWebhookPayload(issueKey string, action string, data map[string]interface{}) map[string]interface{} {
	tickets := MockDatabase["tickets"].(map[string]JiraTicket)
	ticket, exists := tickets[issueKey]

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
	} else if exists && ticket.Fields["customfield_servicenow_id"] != nil {
		issueFields["customfield_servicenow_id"] = ticket.Fields["customfield_servicenow_id"]
	}

	return map[string]interface{}{
		"webhookEvent": "jira:issue_" + action,
		"issue": map[string]interface{}{
			"id":     issueKey,
			"key":    issueKey,
			"self":   fmt.Sprintf("http://localhost:3001/rest/api/2/issue/%s", issueKey),
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

	issuePayload := buildIssueWebhookPayload(issueKey, "commented", data)
	issuePayload["webhookEvent"] = "comment_" + action
	issuePayload["comment"] = map[string]interface{}{
		"id":   commentID,
		"body": commentBody,
		"author": map[string]interface{}{
			"name":         "mock-user",
			"displayName":  "Mock User",
			"emailAddress": "mock@example.com",
		},
		"created": time.Now().Format(time.RFC3339),
		"updated": time.Now().Format(time.RFC3339),
	}

	return issuePayload
}

func handleReset(w http.ResponseWriter, r *http.Request) {
	MockDatabase["tickets"] = map[string]JiraTicket{}
	ServiceNowJiraMapping = map[string]string{}
	log.Printf("Mock Jira database reset")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"reset"}`))
}

func handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`
        <!DOCTYPE html>
        <html lang="en">
        <head>
            <meta charset="UTF-8">
            <meta name="viewport" content="width=device-width, initial-scale=1.0">
            <title>Mock Jira UI</title>
            <style>
                body { font-family: Arial, sans-serif; margin: 0; padding: 20px; background-color: #f4f5f7; }
                .container { max-width: 1200px; margin: 0 auto; }
                header { background-color: #0052cc; color: white; padding: 15px; border-radius: 5px; margin-bottom: 20px; }
                h1, h2, h3 { margin: 0; }
                .card { background-color: white; border-radius: 5px; box-shadow: 0 1px 3px rgba(0,0,0,0.12); padding: 20px; margin-bottom: 20px; }
                .row { display: flex; flex-wrap: wrap; margin: 0 -10px; }
                .col { flex: 1; padding: 0 10px; min-width: 300px; }
                .form-group { margin-bottom: 15px; }
                label { display: block; margin-bottom: 5px; font-weight: bold; }
                input, select, textarea { width: 100%; padding: 8px; border: 1px solid #ddd; border-radius: 4px; box-sizing: border-box; }
                button { background-color: #0052cc; color: white; border: none; padding: 10px 15px; border-radius: 4px; cursor: pointer; }
                button:hover { background-color: #0043a6; }
                .ticket-list { list-style: none; padding: 0; }
                .ticket-item { background-color: white; border-left: 4px solid #0052cc; border-radius: 3px; padding: 15px; margin-bottom: 10px; box-shadow: 0 1px 2px rgba(0,0,0,0.1); cursor: pointer; }
                .ticket-item:hover { background-color: #f8f9fa; }
                .ticket-header { display: flex; justify-content: space-between; margin-bottom: 10px; }
                .ticket-key { color: #0052cc; font-weight: bold; }
                .ticket-status { display: inline-block; padding: 3px 8px; border-radius: 10px; font-size: 12px; font-weight: bold; }
                .status-to-do, .status-open { background-color: #0052cc; color: white; }
                .status-in-progress, .status-inprogress { background-color: #0065ff; color: white; }
                .status-resolved { background-color: #36b37e; color: white; }
                .status-closed, .status-done { background-color: #6b778c; color: white; }
                .ticket-detail { padding: 10px; border: 1px solid #ddd; border-radius: 4px; margin-top: 15px; }
                .tabs { display: flex; border-bottom: 1px solid #ddd; margin-bottom: 15px; }
                .tab { padding: 10px 15px; cursor: pointer; border-bottom: 2px solid transparent; }
                .tab.active { border-bottom: 2px solid #0052cc; font-weight: bold; }
                .tab-content { display: none; }
                .tab-content.active { display: block; }
                .comment { padding: 10px; border-left: 3px solid #ddd; margin-bottom: 10px; }
                .comment-author { font-weight: bold; }
                .comment-date { color: #6b778c; font-size: 12px; }
                .webhook-log { background-color: #f4f5f7; border: 1px solid #ddd; border-radius: 4px; padding: 10px; height: 300px; overflow-y: auto; font-family: monospace; font-size: 12px; }
                .log-entry { margin-bottom: 5px; padding: 5px; border-bottom: 1px solid #eee; }
                .transition-btn { margin-right: 5px; margin-bottom: 5px; }
                .alert { padding: 10px; margin-bottom: 15px; border-radius: 4px; display: none; }
                .alert-success { background-color: #e3fcef; border: 1px solid #36b37e; color: #006644; }
                .alert-error { background-color: #ffebe6; border: 1px solid #ff5630; color: #de350b; }
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
                    <div class="tab" data-tab="create">Create Ticket</div>
                    <div class="tab" data-tab="webhooks">Webhooks</div>
                    <div class="tab" data-tab="admin">Admin</div>
                </div>
                
                <div id="tickets" class="tab-content active">
                    <div class="card">
                        <h2>Search Tickets</h2>
                        <div class="form-group">
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
                
                <div id="create" class="tab-content">
                    <div class="card">
                        <h2>Create New Ticket</h2>
                        <form id="createTicketForm">
                            <div class="form-group">
                                <label for="summary">Summary</label>
                                <input type="text" id="summary" required>
                            </div>
                            <div class="form-group">
                                <label for="description">Description</label>
                                <textarea id="description" rows="5"></textarea>
                            </div>
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
                            <button type="submit">Create Ticket</button>
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
                                    <button id="triggerIssueCreated" class="webhook-btn" data-event="issue_created">Issue Created</button>
                                    <button id="triggerIssueUpdated" class="webhook-btn" data-event="issue_updated">Issue Updated</button>
                                </div>
                                <div class="form-group">
                                    <label for="issueWebhookData">Additional Data (JSON)</label>
                                    <textarea id="issueWebhookData" rows="5" placeholder='{"summary": "Custom issue summary", "description": "Custom description", "status": "In Progress", "servicenow_id": "INC123456"}'></textarea>
                                </div>
                            </div>
                            <div class="col">
                                <h3>Comment Events</h3>
                                <div class="form-group">
                                    <button id="triggerCommentCreated" class="webhook-btn" data-event="comment_created">Comment Created</button>
                                    <button id="triggerCommentUpdated" class="webhook-btn" data-event="comment_updated">Comment Updated</button>
                                </div>
                                <div class="form-group">
                                    <label for="commentWebhookData">Comment Data (JSON)</label>
                                    <textarea id="commentWebhookData" rows="5" placeholder='{"comment": "This is a test comment", "comment_id": "12345"}'></textarea>
                                </div>
                            </div>
                        </div>
                    </div>
                    
                    <div class="card">
                        <h3>Webhook Event Log</h3>
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
                            <div>
                                <button id="resetDatabase">Reset Database</button>
                                <button id="importData">Import Data</button>
                                <button id="exportData">Export Data</button>
                            </div>
                        </div>
                        <div class="form-group">
                            <label>Server Status</label>
                            <div id="serverStatus">Checking...</div>
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
                    webhookLog: document.getElementById('webhookLog')
                };

                // Utility functions
                const utils = {
                    showAlert: function(message, type = 'success') {
                        const alert = document.createElement('div');
                        alert.className = 'alert alert-' + type;
                        alert.textContent = message;
                        dom.alertContainer.appendChild(alert);
                        alert.style.display = 'block';
                        setTimeout(() => alert.remove(), 5000);
                    },
                    formatDate: function(dateString) {
                        return !dateString ? '' : new Date(dateString).toLocaleString();
                    },
                    logWebhookEvent: function(message) {
                        const entry = document.createElement('div');
                        entry.className = 'log-entry';
                        entry.textContent = '[' + new Date().toLocaleTimeString() + '] ' + message;
                        dom.webhookLog.prepend(entry);
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
                        } catch (error) {
                            utils.showAlert("Failed to trigger webhook: " + error.message, "error");
                        }
                    }
                };

                // Core functionality
                const app = {
                    async loadTickets() {
                        try {
                            const tickets = await api.fetch('/rest/api/2/issue');
                            dom.ticketList.innerHTML = '';
                            
                            if (tickets.length === 0) {
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
                            
                            // Build detail HTML
                            const statusClass = "status-" + ticket.status.toLowerCase().replace(/ /g, "-");
                            
                            let html = 
                                '<h3>' + ticket.summary + '</h3>' +
                                '<div>' +
                                '<strong>Key:</strong> ' + ticket.key +
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
                            if (ticket.fields && ticket.fields.customfield_servicenow_id) {
                                html += '<br><strong>ServiceNow ID:</strong> ' + ticket.fields.customfield_servicenow_id;
                            }
                            
                            html += '</div>';
                            
                            // Description
                            if (ticket.description) {
                                html += '<div class="ticket-detail"><h4>Description</h4><p>' + ticket.description + '</p></div>';
                            }
                            
                            // Transitions
                            if (transitionsData.transitions && transitionsData.transitions.length > 0) {
                                html += '<div class="ticket-detail"><h4>Actions</h4><div>';
                                
                                transitionsData.transitions.forEach(transition => {
                                    html += '<button class="transition-btn" data-transition-id="' + transition.id + '" ' +
                                           'data-ticket-key="' + key + '" data-current-status="' + ticket.status + '" ' +
                                           'data-new-status="' + transition.name + '">' + transition.name + '</button>';
                                });
                                
                                html += '</div></div>';
                            }
                            
                            // Comments
                            if (commentsData.comments && commentsData.comments.length > 0) {
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
                        
                        const jql = 'project=AUDIT AND summary ~ "' + searchTerm + '"';
                        
                        try {
                            const data = await api.fetch("/rest/api/2/search?jql=" + encodeURIComponent(jql));
                            dom.ticketList.innerHTML = '';
                            
                            if (!data.issues || data.issues.length === 0) {
                                dom.ticketList.innerHTML = '<li>No matching tickets found</li>';
                                return;
                            }
                            
                            data.issues.forEach(ticket => {
                                const statusClass = 'status-' + ticket.status.toLowerCase().replace(/ /g, '-');
                                const li = document.createElement('li');
                                li.className = 'ticket-item';
                                li.innerHTML = 
                                    '<div class="ticket-header">' +
                                    '<span class="ticket-key">' + ticket.key + '</span>' +
                                    '<span class="ticket-status ' + statusClass + '">' + ticket.status + '</span>' +
                                    '</div>' +
                                    '<div>' + ticket.summary + '</div>';
                                
                                li.addEventListener('click', () => this.loadTicketDetails(ticket.key));
                                dom.ticketList.appendChild(li);
                            });
                        } catch (error) {
                            utils.showAlert("Search failed: " + error.message, "error");
                        }
                    },
                    
                    async handleCreateTicket(e) {
                        e.preventDefault();
                        
                        const summary = document.getElementById('summary').value.trim();
                        const description = document.getElementById('description').value.trim();
                        const priority = document.getElementById('priority').value;
                        const dueDate = document.getElementById('dueDate').value;
                        const assignee = document.getElementById('assignee').value.trim();
                        const serviceNowId = document.getElementById('serviceNowId').value.trim();
                        
                        if (!summary) {
                            utils.showAlert('Summary is required', 'error');
                            return;
                        }
                        
                        try {
                            const requestBody = {
                                fields: {
                                    summary,
                                    description,
                                    priority: {name: priority}
                                }
                            };
                            
                            if (dueDate) requestBody.fields.duedate = dueDate;
                            if (assignee) requestBody.fields.assignee = {name: assignee};
                            if (serviceNowId) requestBody.fields.customfield_servicenow_id = serviceNowId;
                            
                            const result = await api.fetch('/rest/api/2/issue', {
                                method: 'POST',
                                headers: {'Content-Type': 'application/json'},
                                body: JSON.stringify(requestBody)
                            });
                            
                            // Trigger webhook
                            api.triggerWebhook('issue_created', {
                                issue_key: result.key,
                                summary,
                                description,
                                status: "To Do",
                                servicenow_id: serviceNowId
                            });
                            
                            utils.showAlert("Ticket " + result.key + " created successfully");
                            dom.createTicketForm.reset();
                            
                            document.querySelector('.tab[data-tab="tickets"]').click();
                            this.loadTickets();
                            
                        } catch (error) {
                            utils.showAlert("Failed to create ticket: " + error.message, "error");
                        }
                    },
                    
                    setupEventListeners() {
                        // Search
                        dom.searchButton.addEventListener('click', () => this.handleSearch());
                        dom.searchInput.addEventListener('keyup', e => {
                            if (e.key === 'Enter') this.handleSearch();
                        });
                        
                        // Create ticket
                        dom.createTicketForm.addEventListener('submit', e => this.handleCreateTicket(e));
                        
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
                        
                        // Export data
                        const exportDataBtn = document.getElementById('exportData');
                        if (exportDataBtn) {
                            exportDataBtn.addEventListener('click', () => {
                                api.fetch('/export')
                                    .then(data => {
                                        const blob = new Blob([JSON.stringify(data, null, 2)], {type: 'application/json'});
                                        const url = URL.createObjectURL(blob);
                                        const a = document.createElement('a');
                                        a.href = url;
                                        const now = new Date();
                                        a.download = "jira_data_export_" + now.getFullYear() + "-" + 
                                                   String(now.getMonth() + 1).padStart(2, '0') + "-" + 
                                                   String(now.getDate()).padStart(2, '0') + ".json";
                                        document.body.appendChild(a);
                                        a.click();
                                        document.body.removeChild(a);
                                        URL.revokeObjectURL(url);
                                        utils.showAlert('Data exported successfully');
                                    })
                                    .catch(error => {
                                        utils.showAlert("Failed to export data: " + error.message, "error");
                                    });
                            });
                        }
                        
                        // Import data
document.getElementById('importData')?.addEventListener('click', () => {
    const fileInput = document.createElement('input');
    fileInput.type = 'file';
    fileInput.accept = 'application/json';
    
    fileInput.addEventListener('change', e => {
        const file = e.target.files[0];
        if (!file) return;
        
        const reader = new FileReader();
        reader.onload = event => {
            try {
                const data = JSON.parse(event.target.result);
                
                api.fetch('/import', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify(data)
                })
                .then(() => {
                    utils.showAlert('Data imported successfully');
                    this.loadTickets();
                })
                .catch(error => {
                    utils.showAlert(fmt.Sprintf("Failed to import data: %s", error.Error()), "error")
                });
            } catch (error) {
                utils.showAlert('Invalid JSON file', 'error');
            }
        };
        reader.readAsText(file);
    });
    
    fileInput.click();
});

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
    });
});

// Check server status
this.checkServerStatus();
},

async checkServerStatus() {
    const statusElement = document.getElementById('serverStatus');
    try {
    const response = await fetch(fmt.Sprintf("%s/health", API_BASE_URL));
    const data = await response.json();
    statusElement.textContent = fmt.Sprintf("Server running - Version: %s | DB Status: %s", data.version, data.database);
}
    catch (error) {
        statusElement.textContent = 'Server Error - Unable to connect';
        statusElement.style.color = 'red';
    }
},

init() {
    this.setupEventListeners();
    this.loadTickets();
}
};

// Initialize the application
document.addEventListener('DOMContentLoaded', () => {
    app.init();
});
                        
</script>
</body>
</html>
`))
}
