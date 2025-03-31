package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
    "os"
    "io/ioutil"
    "math"

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

	r.HandleFunc("/", handleUI).Methods("GET")

	// Start server
	port := "5000"
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

		// Either remove the unused variable or use it somewhere
		// Option 1: Simply remove this code block if not needed
		/*
			issueType := "Task"
			if it, ok := fields["issuetype"].(map[string]interface{}); ok {
				if name, ok := it["name"].(string); ok {
					issueType = name
				}
			}
		*/

		// Option 2: If we need to keep track of the issue type, store it in a field
		// This ensures the variable is used and not just declared
		// fields["issue_type_stored"] = issueType

        ticketType := r.FormValue("ticketType")
            if ticketType == "" {
               ticketType = "Task" // Default to Task if not specified
        }

		ticket := JiraTicket{
			ID:          id,
			Key:         key,
			Self:        fmt.Sprintf("http://localhost:5000/rest/api/2/issue/%s", key),
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
		}

		MockDatabase.Tickets[key] = ticket

		// Log the creation
		log.Printf("[JIRA MOCK] Created issue: %s - %s with ServiceNow ID: %s",
			key, summary, serviceNowID)

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
	w.Header().Set("Content-Type", "application/json")

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

	// Handle based on the action type
	switch payload.ActionType {
	case "insert", "created", "create":
		// Always create a ticket for GRC items, regardless of mapping
		createJiraTicketFromServiceNow(payload)

	case "update", "updated", "modify", "modified":
		// Find the corresponding Jira ticket and update it
		jiraKey, exists := MockDatabase.ServiceNowJiraMap[payload.SysID]
		if exists {
			updateJiraTicketFromServiceNow(jiraKey, payload)
		} else {
			log.Printf("[JIRA MOCK] No matching Jira ticket found for ServiceNow item %s, creating new ticket",
				payload.SysID)
			// If update comes in for an item we don't have, create it
			createJiraTicketFromServiceNow(payload)
		}

	case "delete", "deleted", "remove", "removed":
		// Find the corresponding Jira ticket and mark it
		jiraKey, exists := MockDatabase.ServiceNowJiraMap[payload.SysID]
		if exists {
			ticket, ticketExists := MockDatabase.Tickets[jiraKey]
			if ticketExists {
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

	webhookURL := "http://localhost:8081/api/webhooks/jira"
	jsonPayload, _ := json.Marshal(payload)
	resp, err := http.Post(webhookURL, "application/json", strings.NewReader(string(jsonPayload)))
	if err != nil {
		log.Printf("[JIRA MOCK] Error sending status change webhook: %v", err)
		return
	}
	defer resp.Body.Close()

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
			"self":   fmt.Sprintf("http://localhost:5000/rest/api/2/issue/%s", issueKey),
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
	resp, err := http.Post("http://localhost:5000/rest/api/2/issue",
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
			Self:        fmt.Sprintf("http://localhost:5000/rest/api/2/issue/%s", key),
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
                .refresh-btn { margin-left: 10px; }
                .project-list { list-style: none; padding: 0; }
                .project-item { background-color: white; border-left: 4px solid #0052cc; border-radius: 3px; padding: 15px; margin-bottom: 10px; box-shadow: 0 1px 2px rgba(0,0,0,0.1); cursor: pointer; }
                .project-item:hover { background-color: #f8f9fa; }
                .project-header { font-weight: bold; color: #0052cc; font-size: 16px; }
                .project-tickets { padding: 10px; }
                .project-tickets table { width: 100%; border-collapse: collapse; }
                .project-tickets th, .project-tickets td { padding: 8px; text-align: left; border-bottom: 1px solid #ddd; }
                .project-tickets th { background-color: #f2f2f2; }
                .ticket-row { cursor: pointer; }
                .ticket-row:hover { background-color: #f5f5f5; }
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
                    <div class="card">
                        <h2>Search Tickets <button id="refreshTickets" class="refresh-btn">Refresh</button></h2>
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
        <form id="createTicketForm">
            <div class="form-group">
                <label for="ticketType">Issue Type</label>
                <select id="ticketType" required>
                    <option value="Task">Task</option>
                    <option value="Epic">Epic</option>
                    <option value="Subtask">Subtask</option>
                </select>
            </div>
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
                        <h3>Webhook Event Log <button id="refreshWebhookLog" class="refresh-btn">Refresh</button></h3>
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
                                <button id="testServicenow">Test ServiceNow Connection</button>
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
                    webhookLog: document.getElementById('webhookLog'),
                    refreshTickets: document.getElementById('refreshTickets'),
                    refreshWebhookLog: document.getElementById('refreshWebhookLog')
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

                // Core functionality
                const app = {
                    async loadTickets() {
                        try {
                            const tickets = await api.fetch('/rest/api/2/issue');
                            dom.ticketList.innerHTML = '';
                            
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
                                '<br><strong>Type:</strong> ' + issueType + // ✅ Added Issue Type
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
                                html += '<div class="ticket-detail"><h4>Actions</h4><div>';
                                
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
                                    project: {
                                        key: "AUDIT"
                                    },
                                    issuetype: {
                                        name: "Task"
                                    },
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
    
    // Fix: Remove fmt.Sprintf and use proper URL construction
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