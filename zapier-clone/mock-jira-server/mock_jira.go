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

	// Webhook receiver (this would be an endpoint in your application)
	r.HandleFunc("/api/webhooks/jira", handleReceiveWebhook).Methods("POST")

	// Webhook trigger (special endpoint to simulate sending webhooks to your app)
	r.HandleFunc("/trigger_webhook/{event_type}", triggerWebhook).Methods("POST")

	// Health check and UI
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")

	r.HandleFunc("/", handleUI).Methods("GET")

	// Start server
	port := "3001" // Different from ServiceNow mock
	fmt.Printf("Starting mock Jira server on port %s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

// Jira API handler implementations

func handleIssues(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		// Return all issues
		tickets := MockDatabase["tickets"].(map[string]JiraTicket)
		var results []JiraTicket
		for _, ticket := range tickets {
			results = append(results, ticket)
		}
		json.NewEncoder(w).Encode(results)

	case "POST":
		// Create a new issue
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

		// Generate ID and key
		id := fmt.Sprintf("10%d", len(MockDatabase["tickets"].(map[string]JiraTicket))+1)
		key := fmt.Sprintf("AUDIT-%d", len(MockDatabase["tickets"].(map[string]JiraTicket))+1)

		// Extract data from fields
		summary := ""
		if sum, ok := fields["summary"].(string); ok {
			summary = sum
		}

		description := ""
		if desc, ok := fields["description"].(string); ok {
			description = desc
		}

		// Create the ticket
		ticket := JiraTicket{
			ID:          id,
			Key:         key,
			Self:        fmt.Sprintf("http://localhost:3001/rest/api/2/issue/%s", key),
			Summary:     summary,
			Description: description,
			Status:      "To Do",
			Created:     time.Now().Format(time.RFC3339),
			Updated:     time.Now().Format(time.RFC3339),
			Comments:    []JiraComment{},
		}

		// Check for custom fields for ServiceNow mapping
		if customFields, ok := fields["customfield_servicenow_id"]; ok {
			if snID, ok := customFields.(string); ok && snID != "" {
				ServiceNowJiraMapping[snID] = key
			}
		}

		// Save to database
		tickets := MockDatabase["tickets"].(map[string]JiraTicket)
		tickets[key] = ticket
		MockDatabase["tickets"] = tickets

		// Return success with key
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
		}

		ticket.Updated = time.Now().Format(time.RFC3339)
		tickets[key] = ticket
		MockDatabase["tickets"] = tickets

		w.WriteHeader(http.StatusNoContent)

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
		// Return comments
		response := map[string]interface{}{
			"comments": ticket.Comments,
			"total":    len(ticket.Comments),
		}
		json.NewEncoder(w).Encode(response)

	case "POST":
		// Add comment
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
		// Return available transitions
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
		// Process transition
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

		// Process transition
		switch transitionID {
		case "21": // To Do -> In Progress
			ticket.Status = "In Progress"
		case "31": // In Progress -> Done
			ticket.Status = "Done"
			// Check for resolution
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

		// Send webhook notification about the status change
		go triggerStatusChangeWebhook(key, ticket.Status)
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

func handleReceiveWebhook(w http.ResponseWriter, r *http.Request) {
	// This simulates your application's webhook endpoint for receiving Jira events
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Log the received webhook
	fmt.Printf("Received webhook from Jira: %v\n", payload)

	// Return success
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"received"}`))
}

func triggerWebhook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	eventType := vars["event_type"]

	// Parse the data from the request
	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get issue key if provided, or use default
	issueKey := "AUDIT-1"
	if key, ok := data["issue_key"].(string); ok && key != "" {
		issueKey = key
	}

	// Build webhook payload based on event type
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

	// Get the webhook URL from the query parameter or use default
	webhookURL := r.URL.Query().Get("webhook_url")
	if webhookURL == "" {
		webhookURL = "http://localhost:8080/api/webhooks/jira"
	}

	// Send the webhook
	jsonPayload, _ := json.Marshal(webhookPayload)
	resp, err := http.Post(webhookURL, "application/json", strings.NewReader(string(jsonPayload)))
	if err != nil {
		http.Error(w, fmt.Sprintf("Error sending webhook: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Return status
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":     "success",
		"message":    fmt.Sprintf("Jira webhook sent to %s", webhookURL),
		"webhook_id": fmt.Sprintf("mock-jira-webhook-%d", time.Now().UnixNano()),
		"event_type": eventType,
		"issue_key":  issueKey,
	})
}

func triggerStatusChangeWebhook(issueKey string, newStatus string) {
	// Build payload for status change
	payload := buildIssueWebhookPayload(issueKey, "updated", map[string]interface{}{
		"status": newStatus,
	})

	// Add changelog info
	payload["changelog"] = map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"field":      "status",
				"fieldtype":  "jira",
				"from":       "3",
				"fromString": "In Progress", // This is just a placeholder
				"to":         "5",
				"toString":   newStatus,
			},
		},
	}

	// Get default webhook URL
	webhookURL := "http://localhost:8080/api/webhooks/jira"

	// Send the webhook
	jsonPayload, _ := json.Marshal(payload)
	resp, err := http.Post(webhookURL, "application/json", strings.NewReader(string(jsonPayload)))
	if err != nil {
		fmt.Printf("Error sending status change webhook: %v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Status change webhook sent for issue %s (new status: %s)\n", issueKey, newStatus)
}

func buildIssueWebhookPayload(issueKey string, action string, data map[string]interface{}) map[string]interface{} {
	tickets := MockDatabase["tickets"].(map[string]JiraTicket)
	ticket, exists := tickets[issueKey]

	// Use default fields if the ticket doesn't exist
	summary := "Mock issue"
	description := "This is a mock issue for testing"
	status := "To Do"

	if exists {
		summary = ticket.Summary
		description = ticket.Description
		status = ticket.Status
	}

	// Override with provided data
	if s, ok := data["summary"].(string); ok && s != "" {
		summary = s
	}
	if d, ok := data["description"].(string); ok && d != "" {
		description = d
	}
	if s, ok := data["status"].(string); ok && s != "" {
		status = s
	}

	// Build the issue fields
	issueFields := map[string]interface{}{
		"summary":     summary,
		"description": description,
		"status": map[string]interface{}{
			"id":   "3", // Just a placeholder ID
			"name": status,
		},
	}

	// Add ServiceNow ID if provided
	if snID, ok := data["servicenow_id"].(string); ok && snID != "" {
		issueFields["customfield_servicenow_id"] = snID
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

func handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`
        <!DOCTYPE html>
        <html>
        <head>
            <title>Mock Jira Server</title>
            <style>
                body { font-family: Arial, sans-serif; margin: 40px; }
                h1, h2 { color: #333; }
                .section { margin: 20px 0; padding: 10px; background: #f5f5f5; border-radius: 5px; }
                .endpoint { margin: 10px 0; padding: 10px; background: #fff; border: 1px solid #ddd; border-radius: 3px; }
                .btn { display: inline-block; padding: 8px 16px; background: #0066cc; color: white; 
                    text-decoration: none; border-radius: 4px; margin-right: 10px; }
            </style>
        </head>
        <body>
            <h1>Mock Jira Server</h1>
            <div class="section">
                <h2>Trigger Jira Webhooks</h2>
                <div class="endpoint">
                    <h3>Issue Created</h3>
                    <form id="issueCreateForm">
                        <p>
                            <label>Summary: <input type="text" name="summary" value="New Test Issue"></label>
                        </p>
                        <p>
                            <label>Description: <input type="text" name="description" value="This is a test issue"></label>
                        </p>
                        <p>
                            <label>ServiceNow ID: <input type="text" name="servicenow_id" value=""></label>
                        </p>
                        <p>
                            <label>Webhook URL: <input type="text" name="webhook_url" value="http://localhost:8080/api/webhooks/jira"></label>
                        </p>
                        <p><button type="submit" class="btn">Send Webhook</button></p>
                    </form>
                </div>
                
                <div class="endpoint">
                    <h3>Issue Updated</h3>
                    <form id="issueUpdateForm">
                        <p>
                            <label>Issue Key: <input type="text" name="issue_key" value="AUDIT-1"></label>
                        </p>
                        <p>
                            <label>Status: 
                                <select name="status">
                                    <option value="To Do">To Do</option>
                                    <option value="In Progress">In Progress</option>
                                    <option value="Done">Done</option>
                                </select>
                            </label>
                        </p>
                        <p>
                            <label>Webhook URL: <input type="text" name="webhook_url" value="http://localhost:8080/api/webhooks/jira"></label>
                        </p>
                        <p><button type="submit" class="btn">Send Webhook</button></p>
                    </form>
                </div>
                
                <div class="endpoint">
                    <h3>Comment Created</h3>
                    <form id="commentCreateForm">
                        <p>
                            <label>Issue Key: <input type="text" name="issue_key" value="AUDIT-1"></label>
                        </p>
                        <p>
                            <label>Comment: <input type="text" name="comment" value="This is a test comment"></label>
                        </p>
                        <p>
                            <label>Webhook URL: <input type="text" name="webhook_url" value="http://localhost:8080/api/webhooks/jira"></label>
                        </p>
                        <p><button type="submit" class="btn">Send Webhook</button></p>
                    </form>
                </div>
            </div>
            
            <div class="section">
                <h2>API Endpoints</h2>
                <div class="endpoint">
                    <p>Use these endpoints to test your Jira integration:</p>
                    <ul>
                        <li><strong>Issues API:</strong> http://localhost:3001/rest/api/2/issue</li>
                        <li><strong>Issue by Key:</strong> http://localhost:3001/rest/api/2/issue/{key}</li>
                        <li><strong>Comments API:</strong> http://localhost:3001/rest/api/2/issue/{key}/comment</li>
                        <li><strong>Transitions API:</strong> http://localhost:3001/rest/api/2/issue/{key}/transitions</li>
                        <li><strong>Projects API:</strong> http://localhost:3001/rest/api/2/project</li>
                        <li><strong>Webhook Trigger:</strong> http://localhost:3001/trigger_webhook/{event_type}</li>
                    </ul>
                </div>
            </div>
            
            <script>
                // Handle form submissions
                document.getElementById('issueCreateForm').addEventListener('submit', function(e) {
                    e.preventDefault();
                    const data = {
                        summary: this.elements.summary.value,
                        description: this.elements.description.value,
                        servicenow_id: this.elements.servicenow_id.value,
                    };
                    
                    const webhookURL = this.elements.webhook_url.value;
                    const url = '/trigger_webhook/issue_created' + (webhookURL ? '?webhook_url=' + encodeURIComponent(webhookURL) : '');
                    
                    fetch(url, {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'application/json',
                        },
                        body: JSON.stringify(data)
                    })
                    .then(response => response.json())
                    .then(data => {
                        alert('Webhook sent successfully: ' + JSON.stringify(data));
                    })
                    .catch(error => {
                        alert('Error sending webhook: ' + error);
                    });
                });
                
                document.getElementById('issueUpdateForm').addEventListener('submit', function(e) {
                    e.preventDefault();
                    const data = {
                        issue_key: this.elements.issue_key.value,
                        status: this.elements.status.value,
                    };
                    
                    const webhookURL = this.elements.webhook_url.value;
                    const url = '/trigger_webhook/issue_updated' + (webhookURL ? '?webhook_url=' + encodeURIComponent(webhookURL) : '');
                    
                    fetch(url, {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'application/json',
                        },
                        body: JSON.stringify(data)
                    })
                    .then(response => response.json())
                    .then(data => {
                        alert('Webhook sent successfully: ' + JSON.stringify(data));
                    })
                    .catch(error => {
                        alert('Error sending webhook: ' + error);
                    });
                });
                
                document.getElementById('commentCreateForm').addEventListener('submit', function(e) {
                    e.preventDefault();
                    const data = {
                        issue_key: this.elements.issue_key.value,
                        comment: this.elements.comment.value,
                    };
                    
                    const webhookURL = this.elements.webhook_url.value;
                    const url = '/trigger_webhook/comment_created' + (webhookURL ? '?webhook_url=' + encodeURIComponent(webhookURL) : '');
                    
                    fetch(url, {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'application/json',
                        },
                        body: JSON.stringify(data)
                    })
                    .then(response => response.json())
                    .then(data => {
                        alert('Webhook sent successfully: ' + JSON.stringify(data));
                    })
                    .catch(error => {
                        alert('Error sending webhook: ' + error);
                    });
                });
            </script>
        </body>
        </html>
    `))
}
