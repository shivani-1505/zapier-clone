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

	"github.com/gorilla/mux"
)

const (
	ServiceNowURL           = "http://localhost:3000"
	JiraURL                 = "http://localhost:3001"
	SlackURL                = "http://localhost:3002"
	GRCURL                  = "http://localhost:8080"
	JiraProjectKey          = "AUDIT"
	SlackAuditChannel       = "C54321"
	DashboardUpdateInterval = 300 * time.Second
	OverdueCheckInterval    = 60 * time.Second
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

type ControlTest struct {
	SysID       string `json:"sys_id"`
	Number      string `json:"number"`
	Description string `json:"description"`
	ControlName string `json:"control_name"`
	DueDate     string `json:"due_date"`
	TestStatus  string `json:"test_status"`
	Results     string `json:"results"`
	AssignedTo  string `json:"assigned_to"`
}

type VendorRisk struct {
	SysID        string `json:"sys_id"`
	Number       string `json:"number"`
	Description  string `json:"description"`
	RiskName     string `json:"risk_name"`
	State        string `json:"state"`
	AssignedTo   string `json:"assigned_to"`
	Resolution   string `json:"resolution"`
	SysUpdatedOn string `json:"sys_updated_on"`
}

type RegulatoryChange struct {
	SysID       string `json:"sys_id"`
	Number      string `json:"number"`
	Description string `json:"description"`
	ChangeName  string `json:"change_name"`
	Effective   string `json:"effective_date"`
	AssignedTo  string `json:"assigned_to"`
}

type JiraIssue struct {
	ID     string                 `json:"id"`
	Key    string                 `json:"key"`
	Self   string                 `json:"self"`
	Fields map[string]interface{} `json:"fields,omitempty"`
}

type SlackMessage struct {
	Channel string        `json:"channel"`
	Text    string        `json:"text"`
	Blocks  []interface{} `json:"blocks,omitempty"`
}

type DashboardData struct {
	OpenRisks           int `json:"open_risks"`
	CompletedControls   int `json:"completed_controls"`
	OpenRegulatoryTasks int `json:"open_regulatory_tasks"`
}

func main() {
	log.Println("Initializing GRC integration server...")

	r := mux.NewRouter()
	r.HandleFunc("/api/webhooks/servicenow", handleServiceNowWebhook).Methods("POST")
	r.HandleFunc("/api/webhooks/jira", handleJiraWebhook).Methods("POST")
	r.HandleFunc("/api/dashboard", handleDashboard).Methods("GET")

	go func() {
		log.Println("Starting overdue task checker...")
		checkOverdueTasks()
	}()

	// Optional: Remove updateDashboard if real-time querying is sufficient
	go func() {
		log.Println("Starting dashboard updater...")
		updateDashboard()
	}()

	log.Println("Starting GRC integration server on :8080...")
	err := http.ListenAndServe(":8080", r)
	if err != nil {
		log.Printf("Failed to start server on :8080: %v", err)
		log.Println("Attempting to start on :8081...")
		err = http.ListenAndServe(":8081", r)
		if err != nil {
			log.Fatalf("Failed to start server on :8081: %v", err)
		}
		log.Println("Server running on :8081")
	}
}

func handleServiceNowWebhook(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		SysID      string                 `json:"sys_id"`
		TableName  string                 `json:"table_name"`
		ActionType string                 `json:"action_type"`
		Data       map[string]interface{} `json:"data"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("Error decoding ServiceNow webhook: %v", err)
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	log.Printf("Received ServiceNow webhook: %s/%s, action: %s", payload.TableName, payload.SysID, payload.ActionType)

	switch payload.TableName {
	case "sn_policy_control_test":
		if payload.ActionType == "inserted" {
			control := ControlTest{
				SysID:       payload.SysID,
				Number:      payload.Data["number"].(string),
				Description: payload.Data["description"].(string),
				ControlName: payload.Data["control_name"].(string),
				DueDate:     payload.Data["due_date"].(string),
				AssignedTo:  payload.Data["assigned_to"].(string),
			}

			jiraIssue := map[string]interface{}{
				"fields": map[string]interface{}{
					"project":                   map[string]string{"key": JiraProjectKey},
					"summary":                   fmt.Sprintf("Test Control: %s", control.ControlName),
					"description":               control.Description,
					"issuetype":                 map[string]string{"name": "Task"},
					"duedate":                   control.DueDate,
					"assignee":                  map[string]string{"name": control.AssignedTo},
					"customfield_servicenow_id": control.SysID,
				},
			}

			if err := createJiraIssue(jiraIssue, "control test", control.SysID); err != nil {
				http.Error(w, fmt.Sprintf("Failed to create Jira issue: %v", err), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}

	case "sn_vendor_risk":
		if payload.ActionType == "inserted" {
			risk := VendorRisk{
				SysID:       payload.SysID,
				Number:      payload.Data["number"].(string),
				Description: payload.Data["description"].(string),
				RiskName:    payload.Data["risk_name"].(string),
				AssignedTo:  payload.Data["assigned_to"].(string),
				State:       payload.Data["state"].(string),
			}

			jiraIssue := map[string]interface{}{
				"fields": map[string]interface{}{
					"project":                   map[string]string{"key": JiraProjectKey},
					"summary":                   fmt.Sprintf("Vendor Risk: %s", risk.RiskName),
					"description":               fmt.Sprintf("Request updated report for vendor risk: %s\nDetails: %s", risk.RiskName, risk.Description),
					"issuetype":                 map[string]string{"name": "Task"},
					"assignee":                  map[string]string{"name": risk.AssignedTo},
					"customfield_servicenow_id": risk.SysID,
				},
			}

			if err := createJiraIssue(jiraIssue, "vendor risk", risk.SysID); err != nil {
				http.Error(w, fmt.Sprintf("Failed to create Jira issue: %v", err), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}

	case "sn_regulatory_change":
		if payload.ActionType == "inserted" {
			regChange := RegulatoryChange{
				SysID:       payload.SysID,
				Number:      payload.Data["number"].(string),
				Description: payload.Data["description"].(string),
				ChangeName:  payload.Data["change_name"].(string),
				Effective:   payload.Data["effective_date"].(string),
				AssignedTo:  payload.Data["assigned_to"].(string),
			}

			epic := map[string]interface{}{
				"fields": map[string]interface{}{
					"project":                   map[string]string{"key": JiraProjectKey},
					"summary":                   fmt.Sprintf("Regulatory Change: %s", regChange.ChangeName),
					"description":               regChange.Description,
					"issuetype":                 map[string]string{"name": "Epic"},
					"customfield_servicenow_id": regChange.SysID,
				},
			}

			epicKey, err := createJiraIssueWithKey(epic, "regulatory change epic", regChange.SysID)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to create Jira epic: %v", err), http.StatusInternalServerError)
				return
			}

			tasks := []map[string]interface{}{
				{
					"fields": map[string]interface{}{
						"project":                   map[string]string{"key": JiraProjectKey},
						"summary":                   "Update privacy policy",
						"description":               "Update privacy policy due to " + regChange.ChangeName,
						"issuetype":                 map[string]string{"name": "Task"},
						"duedate":                   regChange.Effective,
						"assignee":                  map[string]string{"name": regChange.AssignedTo},
						"customfield_servicenow_id": regChange.SysID,
						"parent":                    map[string]string{"key": epicKey},
					},
				},
				{
					"fields": map[string]interface{}{
						"project":                   map[string]string{"key": JiraProjectKey},
						"summary":                   "Train staff",
						"description":               "Train staff on " + regChange.ChangeName,
						"issuetype":                 map[string]string{"name": "Task"},
						"duedate":                   regChange.Effective,
						"assignee":                  map[string]string{"name": regChange.AssignedTo},
						"customfield_servicenow_id": regChange.SysID,
						"parent":                    map[string]string{"key": epicKey},
					},
				},
			}

			for _, task := range tasks {
				if err := createJiraIssue(task, "regulatory change task", regChange.SysID); err != nil {
					log.Printf("Failed to create subtask under epic %s: %v", epicKey, err)
				}
			}
			w.WriteHeader(http.StatusOK)
		}
	}
}

func handleJiraWebhook(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		WebhookEvent string                 `json:"webhookEvent"`
		Issue        map[string]interface{} `json:"issue"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("Error decoding Jira webhook: %v", err)
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if payload.WebhookEvent != "jira:issue_updated" {
		w.WriteHeader(http.StatusOK)
		return
	}

	issueKey, _ := payload.Issue["key"].(string)
	fields, ok := payload.Issue["fields"].(map[string]interface{})
	if !ok || issueKey == "" {
		log.Printf("Invalid issue format for key: %s", issueKey)
		http.Error(w, "Invalid issue data", http.StatusBadRequest)
		return
	}

	snID, ok := fields["customfield_servicenow_id"].(string)
	if !ok || snID == "" {
		log.Printf("No ServiceNow ID in issue %s", issueKey)
		w.WriteHeader(http.StatusOK)
		return
	}

	table := determineServiceNowTable(snID)
	if table == "" {
		log.Printf("No matching ServiceNow table for %s (issue: %s), assuming sn_regulatory_change for REG001", snID, issueKey)
		if strings.HasPrefix(snID, "REG") {
			table = "sn_regulatory_change"
		} else {
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	status, ok := fields["status"].(map[string]interface{})["name"].(string)
	if !ok {
		log.Printf("No status found in issue %s", issueKey)
		w.WriteHeader(http.StatusOK)
		return
	}
	description, _ := fields["description"].(string)

	updatePayload := map[string]interface{}{
		"sys_updated_on": time.Now().Format(time.RFC3339),
	}
	switch table {
	case "sn_vendor_risk":
		if status == "Done" {
			updatePayload["state"] = "Resolved"
		} else {
			updatePayload["state"] = status
		}
		updatePayload["resolution"] = description
	case "sn_regulatory_change":
		updatePayload["state"] = status
		updatePayload["description"] = description
	case "sn_policy_control_test":
		updatePayload["test_status"] = status
		updatePayload["results"] = description
	}

	if err := updateServiceNow(table, snID, updatePayload); err != nil {
		log.Printf("Failed to update ServiceNow %s/%s: %v", table, snID, err)
		http.Error(w, fmt.Sprintf("ServiceNow update failed: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Updated ServiceNow %s/%s: status=%s", table, snID, status)
	w.WriteHeader(http.StatusOK)
}

func checkOverdueTasks() {
	for {
		time.Sleep(OverdueCheckInterval)

		resp, err := httpClient.Get(JiraURL + "/rest/api/2/search?jql=project=" + JiraProjectKey + "+AND+status!=Done+AND+customfield_servicenow_id+IS+NOT+EMPTY")
		if err != nil {
			log.Printf("Error fetching Jira issues: %v", err)
			continue
		}
		defer resp.Body.Close()

		var responseData struct {
			Issues []map[string]interface{} `json:"issues"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&responseData); err != nil {
			log.Printf("Error decoding Jira issues response: %v", err)
			continue
		}

		for _, issue := range responseData.Issues {
			fields := issue["fields"].(map[string]interface{})
			dueDateStr, ok := fields["duedate"].(string)
			if !ok || dueDateStr == "" {
				continue
			}
			dueDate, err := time.Parse("2006-01-02", dueDateStr)
			if err != nil {
				log.Printf("Error parsing due date for issue %s: %v", issue["key"], err)
				continue
			}

			if time.Now().After(dueDate) {
				assignee := "Unassigned"
				if assigneeField, ok := fields["assignee"].(map[string]interface{}); ok && assigneeField["name"] != nil {
					assignee = assigneeField["name"].(string)
				}
				snID := fields["customfield_servicenow_id"].(string)
				key := issue["key"].(string)
				jiraLink := fmt.Sprintf("%s/rest/api/2/issue/%s", JiraURL, key)

				slackMsg := SlackMessage{
					Channel: SlackAuditChannel,
					Text:    fmt.Sprintf("Reminder: Task %s (ServiceNow: %s) is overdue (Due: %s). Assigned to: %s. View: %s", key, snID, dueDateStr, assignee, jiraLink),
					Blocks: []interface{}{
						map[string]interface{}{
							"type": "section",
							"text": map[string]interface{}{
								"type": "mrkdwn",
								"text": fmt.Sprintf("*Overdue Task Reminder*\n*Task:* <%s|%s>\n*ServiceNow:* %s\n*Due:* %s\n*Assignee:* %s", jiraLink, key, snID, dueDateStr, assignee),
							},
						},
					},
				}

				jsonPayload, err := json.Marshal(slackMsg)
				if err != nil {
					log.Printf("Error marshaling Slack message: %v", err)
					continue
				}
				resp, err := httpClient.Post(SlackURL+"/api/chat.postMessage", "application/json", bytes.NewBuffer(jsonPayload))
				if err != nil {
					log.Printf("Error sending Slack reminder for %s: %v", key, err)
				} else {
					resp.Body.Close()
					log.Printf("Sent Slack reminder for overdue task: %s (ServiceNow: %s)", key, snID)
				}
			}
		}
	}
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	resp, err := httpClient.Get(JiraURL + "/rest/api/2/search?jql=project=" + JiraProjectKey + "+AND+customfield_servicenow_id+IS+NOT+EMPTY")
	if err != nil {
		log.Printf("Error fetching Jira issues for dashboard: %v", err)
		http.Error(w, fmt.Sprintf("Failed to fetch dashboard data: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var responseData struct {
		Issues []map[string]interface{} `json:"issues"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&responseData); err != nil {
		log.Printf("Error decoding Jira issues for dashboard: %v", err)
		http.Error(w, fmt.Sprintf("Failed to decode dashboard data: %v", err), http.StatusInternalServerError)
		return
	}

	data := DashboardData{
		OpenRisks:           0,
		CompletedControls:   0,
		OpenRegulatoryTasks: 0,
	}

	for _, issue := range responseData.Issues {
		fields := issue["fields"].(map[string]interface{})
		issueType := fields["issuetype"].(map[string]interface{})["name"].(string)
		status := fields["status"].(map[string]interface{})["name"].(string)
		snID := fields["customfield_servicenow_id"].(string)
		fmt.Printf(snID)
		summary := fields["summary"].(string)

		switch {
		case strings.Contains(summary, "Vendor Risk"):
			if status != "Done" {
				data.OpenRisks++
			}
		case strings.Contains(summary, "Test Control"):
			// Changed to count all "Test Control" issues, not just "Done"
			data.CompletedControls++
		case issueType == "Epic" || strings.Contains(summary, "Regulatory Change"):
			if status != "Done" {
				data.OpenRegulatoryTasks++
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding dashboard response: %v", err)
		http.Error(w, fmt.Sprintf("Error encoding response: %v", err), http.StatusInternalServerError)
	}
}

func updateDashboard() {
	for {
		time.Sleep(DashboardUpdateInterval)

		resp, err := httpClient.Get(GRCURL + "/api/dashboard")
		if err != nil {
			log.Printf("Error fetching dashboard data: %v", err)
			continue
		}
		defer resp.Body.Close()

		var data DashboardData
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			log.Printf("Error decoding dashboard data: %v", err)
			continue
		}

		log.Printf("Dashboard update: OpenRisks=%d, CompletedControls=%d, OpenRegulatoryTasks=%d", data.OpenRisks, data.CompletedControls, data.OpenRegulatoryTasks)
	}
}

func createJiraIssue(issue map[string]interface{}, context, sysID string) error {
	jsonPayload, err := json.Marshal(issue)
	if err != nil {
		log.Printf("Error marshaling Jira issue for %s %s: %v", context, sysID, err)
		return err
	}

	resp, err := httpClient.Post(JiraURL+"/rest/api/2/issue", "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Printf("Error creating Jira issue for %s %s: %v", context, sysID, err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Jira issue creation failed for %s %s: %d, %s", context, sysID, resp.StatusCode, string(body))
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var issueResp JiraIssue
	if err := json.NewDecoder(resp.Body).Decode(&issueResp); err != nil {
		log.Printf("Error decoding Jira response for %s %s: %v", context, sysID, err)
		return err
	}
	log.Printf("Created Jira issue: %s for %s: %s", issueResp.Key, context, sysID)
	return nil
}

func createJiraIssueWithKey(issue map[string]interface{}, context, sysID string) (string, error) {
	if err := createJiraIssue(issue, context, sysID); err != nil {
		return "", err
	}
	resp, err := httpClient.Get(JiraURL + "/rest/api/2/search?jql=project=" + JiraProjectKey + "+AND+customfield_servicenow_id=" + sysID)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var searchResp struct {
		Issues []JiraIssue `json:"issues"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return "", err
	}
	if len(searchResp.Issues) > 0 {
		return searchResp.Issues[0].Key, nil
	}
	return "", fmt.Errorf("issue not found")
}

func determineServiceNowTable(snID string) string {
	for _, table := range []string{"sn_vendor_risk", "sn_regulatory_change", "sn_policy_control_test"} {
		resp, err := httpClient.Get(ServiceNowURL + "/api/now/table/" + table + "/" + snID)
		if err != nil {
			log.Printf("Error checking table %s for %s: %v", table, snID, err)
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			log.Printf("Found table %s for %s", table, snID)
			return table
		}
		log.Printf("Table %s not found for %s: %d", table, snID, resp.StatusCode)
	}
	return ""
}

func updateServiceNow(table, snID string, payload map[string]interface{}) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload: %v", err)
	}

	req, err := http.NewRequest("PATCH", ServiceNowURL+"/api/now/table/"+table+"/"+snID, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
