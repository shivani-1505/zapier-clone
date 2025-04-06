// backend/internal/reporting/dashboard.go
package reporting

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/jira"
	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/servicenow"
	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/slack"
)

// DashboardData represents the dashboard metrics
type DashboardData struct {
	OpenRisks           int `json:"open_risks"`
	CompletedControls   int `json:"completed_controls"`
	OpenRegulatoryTasks int `json:"open_regulatory_tasks"`
	OverdueItems        int `json:"overdue_items"`
	ComplianceScore     int `json:"compliance_score"` // calculated from existing metrics
}

// GetDashboardData fetches current dashboard metrics from Jira
func GetDashboardData(jiraClient *jira.Client) (*DashboardData, error) {
	if jiraClient == nil {
		return nil, fmt.Errorf("jira client is nil")
	}

	// Try to connect to the mock server instead of the real Atlassian instance
	baseURL := "http://localhost:3001" // Use the mock server instead

	// Log the request URL
	requestURL := fmt.Sprintf("%s/rest/api/2/search?jql=project=%s", baseURL, jiraClient.ProjectKey)
	log.Printf("Making request to: %s", requestURL)

	// Create a proper URL
	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %w", err)
	}

	// Create the request
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error making request: %v", err)
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("Bad status: %d - %s", resp.StatusCode, string(bodyBytes))
		return nil, fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	// Read and parse the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	// Parse JSON response - this structure matches what your mock server returns
	var jiraResponse struct {
		Issues []struct {
			Key    string `json:"key"`
			Fields struct {
				Status struct {
					Name string `json:"name"`
				} `json:"status"`
				DueDate      string                 `json:"duedate"`
				Labels       []string               `json:"labels,omitempty"`
				Description  string                 `json:"description"`
				CustomFields map[string]interface{} `json:"-"`
			} `json:"fields"`
		} `json:"issues"`
		Total      int `json:"total"`
		MaxResults int `json:"maxResults"`
		StartAt    int `json:"startAt"`
	}

	if err := json.Unmarshal(bodyBytes, &jiraResponse); err != nil {
		return nil, fmt.Errorf("error parsing JSON response: %w", err)
	}

	// Calculate dashboard metrics from the actual data
	data := &DashboardData{
		OpenRisks:           0,
		CompletedControls:   0,
		OpenRegulatoryTasks: 0,
		OverdueItems:        0,
		ComplianceScore:     0,
	}

	now := time.Now()
	totalItems := 0
	totalCompleted := 0

	// Process the issues to calculate metrics
	for _, issue := range jiraResponse.Issues {
		totalItems++

		// Check for completed controls
		if issue.Fields.Status.Name == "Done" || issue.Fields.Status.Name == "Closed" {
			totalCompleted++
			data.CompletedControls++
		}

		// Count open risks - look for "risk" label
		for _, label := range issue.Fields.Labels {
			if label == "risk" && issue.Fields.Status.Name != "Done" && issue.Fields.Status.Name != "Closed" {
				data.OpenRisks++
				break
			}
		}

		// Count regulatory tasks - look for "regulatory" label or text in description
		for _, label := range issue.Fields.Labels {
			if (label == "regulatory" || strings.Contains(strings.ToLower(issue.Fields.Description), "regulatory")) &&
				issue.Fields.Status.Name != "Done" && issue.Fields.Status.Name != "Closed" {
				data.OpenRegulatoryTasks++
				break
			}
		}

		// Check for overdue items
		if issue.Fields.DueDate != "" {
			dueDate, err := time.Parse("2006-01-02", issue.Fields.DueDate)
			if err == nil && now.After(dueDate) && issue.Fields.Status.Name != "Done" && issue.Fields.Status.Name != "Closed" {
				data.OverdueItems++
			}
		}
	}

	// Calculate compliance score (if there are items)
	if totalItems > 0 {
		data.ComplianceScore = (totalCompleted * 100) / totalItems
	} else {
		data.ComplianceScore = 100 // Default to 100% if no items
	}

	log.Printf("Processed %d Jira issues for dashboard metrics", len(jiraResponse.Issues))
	return data, nil
}

// UpdateDashboard periodically updates dashboard metrics
func UpdateDashboard(serviceNowClient *servicenow.Client, jiraClient *jira.Client) {
	ticker := time.NewTicker(5 * time.Minute) // Update every 5 minutes
	defer ticker.Stop()

	for {
		data, err := GetDashboardData(jiraClient)
		if err != nil {
			log.Printf("Error updating dashboard: %v", err)
		} else {
			log.Printf("Dashboard update: OpenRisks=%d, CompletedControls=%d, OpenRegulatoryTasks=%d, OverdueItems=%d, ComplianceScore=%d%%",
				data.OpenRisks, data.CompletedControls, data.OpenRegulatoryTasks, data.OverdueItems, data.ComplianceScore)
		}
		<-ticker.C
	}
}

// CheckOverdueTasks periodically checks for overdue tasks and sends reminders
func CheckOverdueTasks(jiraClient *jira.Client, slackClient *slack.Client, auditChannel string) {
	ticker := time.NewTicker(1 * time.Hour) // Check every hour
	defer ticker.Stop()

	for {
		// Query Jira for tasks that are not done and have a due date
		jql := fmt.Sprintf("project=%s AND status!=Done AND duedate IS NOT EMPTY AND customfield_servicenow_id IS NOT EMPTY", jiraClient.ProjectKey)
		resp, err := jiraClient.SearchIssues(jql)

		if err != nil {
			log.Printf("Error fetching Jira issues for overdue check: %v", err)
			<-ticker.C
			continue
		}

		now := time.Now()

		for _, issue := range resp.Issues {
			// Skip if no due date or due date is in the future
			if issue.DueDate.IsZero() || now.Before(issue.DueDate) {
				continue
			}

			// Issue is overdue
			assignee := "Unassigned"
			if issue.Assignee != "" {
				assignee = issue.Assignee
			}

			// Get ServiceNow ID
			snID := ""
			if issue.Fields != nil && issue.Fields["customfield_servicenow_id"] != nil {
				snID = issue.Fields["customfield_servicenow_id"].(string)
			}

			// Format the Jira link
			jiraLink := fmt.Sprintf("%s/browse/%s", jiraClient.BaseURL, issue.Key)

			// Create Slack message
			message := slack.Message{
				Channel: auditChannel,
				Text: fmt.Sprintf("Reminder: Task %s (ServiceNow: %s) is overdue (Due: %s). Assigned to: %s.",
					issue.Key, snID, issue.DueDate.Format("Jan 2, 2006"), assignee),
				Blocks: []slack.Block{
					{
						Type: "section",
						Text: slack.NewTextObject("mrkdwn", fmt.Sprintf("*Overdue Task Reminder*\n*Task:* <%s|%s>\n*ServiceNow:* %s\n*Due:* %s\n*Assignee:* %s",
							jiraLink, issue.Key, snID, issue.DueDate.Format("Jan 2, 2006"), assignee), false),
					},
				},
			}

			// Send the message
			_, err := slackClient.PostMessage(auditChannel, message)
			if err != nil {
				log.Printf("Error sending overdue reminder for %s: %v", issue.Key, err)
			} else {
				log.Printf("Sent overdue reminder for task: %s (ServiceNow: %s)", issue.Key, snID)
			}
		}

		<-ticker.C
	}
}
