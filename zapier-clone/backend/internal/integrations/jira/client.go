package jira

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client provides methods to interact with the Jira API
type Client struct {
	BaseURL    string
	Email      string
	APIToken   string
	HTTPClient *http.Client
	ProjectKey string
}

// NewClient creates a new Jira client
func NewClient(baseURL, email, apiToken, projectKey string) *Client {
	return &Client{
		BaseURL:    baseURL,
		Email:      email,
		APIToken:   apiToken,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		ProjectKey: projectKey,
	}
}

// To handle HTTP requests
func (c *Client) makeRequest(method, endpoint string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("error marshaling request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonData)
	}

	url := fmt.Sprintf("%s/%s", c.BaseURL, endpoint)
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.Email, c.APIToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errorResp ErrorResponse
		if err := json.Unmarshal(respBody, &errorResp); err != nil {
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
		}
		errorResp.StatusCode = resp.StatusCode
		return nil, &errorResp
	}

	return respBody, nil
}

// CreateIssue creates a new issue in Jira
func (c *Client) CreateIssue(ticket *Ticket) (*Ticket, error) {
	// Convert ticket to API format
	fields := map[string]interface{}{
		"project":     map[string]string{"key": ticket.Project},
		"issuetype":   map[string]string{"name": ticket.IssueType},
		"summary":     ticket.Summary,
		"description": ticket.Description,
	}

	if ticket.Priority != "" {
		fields["priority"] = map[string]string{"name": ticket.Priority}
	}

	if !ticket.DueDate.IsZero() {
		fields["duedate"] = ticket.DueDate.Format("2006-01-02")
	}

	if len(ticket.Labels) > 0 {
		fields["labels"] = ticket.Labels
	}

	if ticket.IssueType == "Epic" && ticket.Epic != nil {
		// Set the Epic Name field - the exact field may vary depending on your Jira setup
		// Common field names include:
		// - "customfield_10011" (typical for Jira Cloud)
		// - "customfield_10000" or similar for Jira Server
		fields["customfield_10011"] = ticket.Epic.Name

		// Some Jira instances may support Epic color:
		// fields["customfield_10010"] = ticket.Epic.Color
	}

	// Add components if any are specified
	if len(ticket.Components) > 0 {
		components := make([]map[string]string, len(ticket.Components))
		for i, component := range ticket.Components {
			components[i] = map[string]string{"name": component}
		}
		fields["components"] = components
	}

	// Add any custom fields
	if len(ticket.Fields) > 0 {
		for key, value := range ticket.Fields {
			fields[key] = value
		}
	}

	requestBody := map[string]interface{}{
		"fields": fields,
	}
	// Make the API request
	resp, err := c.makeRequest("POST", "issue", requestBody)
	if err != nil {
		return nil, fmt.Errorf("error creating Jira issue: %w", err)
	}

	// Parse the response
	var result struct {
		ID   string `json:"id"`
		Key  string `json:"key"`
		Self string `json:"self"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("error parsing Jira response: %w", err)
	}

	// Return the created ticket
	createdTicket := &Ticket{
		ID:          result.ID,
		Key:         result.Key,
		Self:        result.Self,
		Project:     ticket.Project,
		IssueType:   ticket.IssueType,
		Summary:     ticket.Summary,
		Description: ticket.Description,
		Priority:    ticket.Priority,
		DueDate:     ticket.DueDate,
		Labels:      ticket.Labels,
		Fields:      ticket.Fields,
	}

	return createdTicket, nil
}

// CreateSubtask creates a subtask for an existing issue
func (c *Client) CreateSubtask(parentKey, summary, description string) (string, error) {
	data := map[string]interface{}{
		"fields": map[string]interface{}{
			"project": map[string]string{
				"key": c.ProjectKey,
			},
			"parent": map[string]string{
				"key": parentKey,
			},
			"issuetype": map[string]string{
				"name": "Sub-task",
			},
			"summary":     summary,
			"description": description,
		},
	}

	resp, err := c.makeRequest("POST", "issue", data)
	if err != nil {
		return "", fmt.Errorf("error creating Jira subtask: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("error unmarshaling response: %w", err)
	}

	issueKey, ok := result["key"].(string)
	if !ok {
		return "", fmt.Errorf("issue key not found in response")
	}

	return issueKey, nil
}

// UpdateIssue updates an existing Jira issue with new values
func (c *Client) UpdateIssue(issueKey string, update *TicketUpdate) error {
	// Build the update request based on the update object
	updateRequest := UpdateTicketRequest{
		Fields: make(map[string]interface{}),
	}

	// Handle status change through transitions
	if update.Status != "" {
		// First get available transitions
		transitions, err := c.getTransitions(issueKey)
		if err != nil {
			return fmt.Errorf("error getting available transitions: %w", err)
		}

		// Find the transition ID for the target status
		var transitionID string
		for _, t := range transitions {
			if strings.EqualFold(t.To.Name, update.Status) {
				transitionID = t.ID
				break
			}
		}

		if transitionID == "" {
			return fmt.Errorf("no transition found for status %s", update.Status)
		}

		// Make the transition request
		transitionReq := TransitionRequest{}
		transitionReq.Transition.ID = transitionID

		if update.Resolution != "" {
			if transitionReq.Fields == nil {
				transitionReq.Fields = make(map[string]interface{})
			}
			transitionReq.Fields["resolution"] = map[string]string{"name": update.Resolution}
		}

		_, err = c.makeRequest("POST", fmt.Sprintf("issue/%s/transitions", issueKey), transitionReq)
		if err != nil {
			return fmt.Errorf("error transitioning issue: %w", err)
		}
	}

	// Add other field updates
	if update.Summary != "" {
		updateRequest.Fields["summary"] = update.Summary
	}

	if update.Description != "" {
		updateRequest.Fields["description"] = update.Description
	}

	if update.Priority != "" {
		updateRequest.Fields["priority"] = map[string]string{"name": update.Priority}
	}

	if update.DueDate != "" {
		updateRequest.Fields["duedate"] = update.DueDate
	}

	if update.Assignee != "" {
		updateRequest.Fields["assignee"] = map[string]string{"name": update.Assignee}
	}

	// Add any custom fields from the Fields map
	if update.Fields != nil {
		for key, value := range update.Fields {
			updateRequest.Fields[key] = value
		}
	}

	// Only make the update request if we have fields to update
	if len(updateRequest.Fields) > 0 {
		_, err := c.makeRequest("PUT", fmt.Sprintf("issue/%s", issueKey), updateRequest)
		if err != nil {
			return fmt.Errorf("error updating issue: %w", err)
		}
	}

	// Add comment if provided
	if update.Comment != "" {
		commentReq := CommentRequest{
			Body: update.Comment,
		}

		_, err := c.makeRequest("POST", fmt.Sprintf("issue/%s/comment", issueKey), commentReq)
		if err != nil {
			return fmt.Errorf("error adding comment to issue: %w", err)
		}
	}

	return nil
}

// Helper method to get available transitions for an issue
func (c *Client) getTransitions(issueKey string) ([]Transition, error) {
	resp, err := c.makeRequest("GET", fmt.Sprintf("issue/%s/transitions", issueKey), nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Transitions []Transition `json:"transitions"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result.Transitions, nil
}

// AddComment adds a comment to an issue
func (c *Client) AddComment(issueKey, comment string) error {
	data := map[string]interface{}{
		"body": comment,
	}

	_, err := c.makeRequest("POST", fmt.Sprintf("issue/%s/comment", issueKey), data)
	if err != nil {
		return fmt.Errorf("error adding comment to Jira issue: %w", err)
	}

	return nil
}

// GetIssue gets issue details by key
func (c *Client) GetIssue(issueKey string) (map[string]interface{}, error) {
	resp, err := c.makeRequest("GET", fmt.Sprintf("issue/%s", issueKey), nil)
	if err != nil {
		return nil, fmt.Errorf("error getting Jira issue: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %w", err)
	}

	return result, nil
}

// TransitionIssue changes the status of an issue
func (c *Client) TransitionIssue(issueKey string, transitionID string) error {
	data := map[string]interface{}{
		"transition": map[string]string{
			"id": transitionID,
		},
	}

	_, err := c.makeRequest("POST", fmt.Sprintf("issue/%s/transitions", issueKey), data)
	if err != nil {
		return fmt.Errorf("error transitioning Jira issue: %w", err)
	}

	return nil
}
