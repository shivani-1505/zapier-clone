package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/auditcue/integration-framework/internal/config"
	"github.com/auditcue/integration-framework/internal/integrations/common"
)

// ServiceName is the name of this service
const ServiceName = "jira"

// JiraClient represents a client for the Jira API
type JiraClient struct {
	config     config.JiraConfig
	httpClient *http.Client
}

// JiraProvider implements the ServiceProvider interface for Jira
type JiraProvider struct {
	config         config.JiraConfig
	authHandler    *AuthHandler
	triggerHandler *TriggerHandler
	actionHandler  *ActionHandler
}

// NewJiraProvider creates a new Jira service provider
func NewJiraProvider(config config.JiraConfig) *JiraProvider {
	client := &JiraClient{
		config:     config,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}

	authHandler := NewAuthHandler(config)
	triggerHandler := NewTriggerHandler(client)
	actionHandler := NewActionHandler(client)

	return &JiraProvider{
		config:         config,
		authHandler:    authHandler,
		triggerHandler: triggerHandler,
		actionHandler:  actionHandler,
	}
}

// GetService returns the service identifier
func (p *JiraProvider) GetService() string {
	return ServiceName
}

// GetAuthHandler returns the authentication handler
func (p *JiraProvider) GetAuthHandler() common.AuthHandler {
	return p.authHandler
}

// GetTriggerHandler returns the trigger handler
func (p *JiraProvider) GetTriggerHandler() common.TriggerHandler {
	return p.triggerHandler
}

// GetActionHandler returns the action handler
func (p *JiraProvider) GetActionHandler() common.ActionHandler {
	return p.actionHandler
}

// ValidateConnection validates a connection to Jira
func (p *JiraProvider) ValidateConnection(ctx context.Context, connection *common.Connection) error {
	// Extract the auth data
	var authData common.AuthData
	authDataBytes, err := json.Marshal(connection.AuthData)
	if err != nil {
		return fmt.Errorf("failed to marshal auth data: %w", err)
	}
	if err := json.Unmarshal(authDataBytes, &authData); err != nil {
		return fmt.Errorf("failed to unmarshal auth data: %w", err)
	}

	// Verify the access token is still valid
	return p.authHandler.ValidateAuth(ctx, &authData)
}

// GetConnectionDetails returns details about a Jira connection
func (p *JiraProvider) GetConnectionDetails(ctx context.Context, connection *common.Connection) (map[string]interface{}, error) {
	// Extract the auth data
	var authData common.AuthData
	authDataBytes, err := json.Marshal(connection.AuthData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal auth data: %w", err)
	}
	if err := json.Unmarshal(authDataBytes, &authData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal auth data: %w", err)
	}

	// Make a request to get user information
	client := &JiraClient{
		config:     p.config,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}

	// Make a request to get accessible resources
	resources, err := client.getAccessibleResources(ctx, authData.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get accessible resources: %w", err)
	}

	// For now, we'll just return the list of resources
	return map[string]interface{}{
		"resources": resources,
	}, nil
}

// NewClient creates a new Jira API client with the given access token
func (c *JiraClient) WithToken(accessToken string) *JiraClient {
	return &JiraClient{
		config:     c.config,
		httpClient: c.httpClient,
	}
}

// buildAPIURL builds a full URL for a given API path
func (c *JiraClient) buildAPIURL(baseURL, path string) string {
	return fmt.Sprintf("%s%s", baseURL, path)
}

// sendRequest sends an authenticated request to the Jira API
func (c *JiraClient) sendRequest(ctx context.Context, method, url string, body interface{}, accessToken string) ([]byte, error) {
	var reqBody []byte
	var err error

	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// getAccessibleResources gets the list of Jira resources accessible with the token
func (c *JiraClient) getAccessibleResources(ctx context.Context, accessToken string) ([]map[string]interface{}, error) {
	url := c.buildAPIURL(c.config.APIURL, "/oauth/token/accessible-resources")

	respBytes, err := c.sendRequest(ctx, http.MethodGet, url, nil, accessToken)
	if err != nil {
		return nil, err
	}

	var resources []map[string]interface{}
	if err := json.Unmarshal(respBytes, &resources); err != nil {
		return nil, fmt.Errorf("failed to unmarshal accessible resources: %w", err)
	}

	return resources, nil
}

// getCloudID retrieves the cloud ID for a given site name
func (c *JiraClient) getCloudID(ctx context.Context, accessToken, siteName string) (string, error) {
	resources, err := c.getAccessibleResources(ctx, accessToken)
	if err != nil {
		return "", err
	}

	// Find the cloud ID that matches the site name
	for _, resource := range resources {
		name, ok := resource["name"].(string)
		if ok && name == siteName {
			cloudID, ok := resource["id"].(string)
			if ok {
				return cloudID, nil
			}
		}
	}

	return "", fmt.Errorf("no cloud ID found for site: %s", siteName)
}

// createIssue creates a new issue in Jira
func (c *JiraClient) createIssue(ctx context.Context, accessToken, cloudID string, issueData map[string]interface{}) (map[string]interface{}, error) {
	url := c.buildAPIURL(c.config.APIURL, fmt.Sprintf("/ex/jira/%s/rest/api/3/issue", cloudID))

	respBytes, err := c.sendRequest(ctx, http.MethodPost, url, issueData, accessToken)
	if err != nil {
		return nil, err
	}

	var responseData map[string]interface{}
	if err := json.Unmarshal(respBytes, &responseData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal issue response: %w", err)
	}

	return responseData, nil
}

// getIssue gets an issue from Jira
func (c *JiraClient) getIssue(ctx context.Context, accessToken, cloudID, issueKey string) (map[string]interface{}, error) {
	url := c.buildAPIURL(c.config.APIURL, fmt.Sprintf("/ex/jira/%s/rest/api/3/issue/%s", cloudID, issueKey))

	respBytes, err := c.sendRequest(ctx, http.MethodGet, url, nil, accessToken)
	if err != nil {
		return nil, err
	}

	var issueData map[string]interface{}
	if err := json.Unmarshal(respBytes, &issueData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal issue data: %w", err)
	}

	return issueData, nil
}

// updateIssue updates an existing issue in Jira
func (c *JiraClient) updateIssue(ctx context.Context, accessToken, cloudID, issueKey string, updateData map[string]interface{}) error {
	url := c.buildAPIURL(c.config.APIURL, fmt.Sprintf("/ex/jira/%s/rest/api/3/issue/%s", cloudID, issueKey))

	_, err := c.sendRequest(ctx, http.MethodPut, url, updateData, accessToken)
	return err
}

// searchIssues searches for issues in Jira using JQL
func (c *JiraClient) searchIssues(ctx context.Context, accessToken, cloudID, jql string, fields []string, maxResults int) (map[string]interface{}, error) {
	url := c.buildAPIURL(c.config.APIURL, fmt.Sprintf("/ex/jira/%s/rest/api/3/search", cloudID))

	requestBody := map[string]interface{}{
		"jql":        jql,
		"fields":     fields,
		"maxResults": maxResults,
	}

	respBytes, err := c.sendRequest(ctx, http.MethodPost, url, requestBody, accessToken)
	if err != nil {
		return nil, err
	}

	var searchResults map[string]interface{}
	if err := json.Unmarshal(respBytes, &searchResults); err != nil {
		return nil, fmt.Errorf("failed to unmarshal search results: %w", err)
	}

	return searchResults, nil
}

// addComment adds a comment to an issue
func (c *JiraClient) addComment(ctx context.Context, accessToken, cloudID, issueKey string, commentData map[string]interface{}) (map[string]interface{}, error) {
	url := c.buildAPIURL(c.config.APIURL, fmt.Sprintf("/ex/jira/%s/rest/api/3/issue/%s/comment", cloudID, issueKey))

	respBytes, err := c.sendRequest(ctx, http.MethodPost, url, commentData, accessToken)
	if err != nil {
		return nil, err
	}

	var responseData map[string]interface{}
	if err := json.Unmarshal(respBytes, &responseData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal comment response: %w", err)
	}

	return responseData, nil
}

// getProjects gets the list of projects in Jira
func (c *JiraClient) getProjects(ctx context.Context, accessToken, cloudID string) ([]map[string]interface{}, error) {
	url := c.buildAPIURL(c.config.APIURL, fmt.Sprintf("/ex/jira/%s/rest/api/3/project", cloudID))

	respBytes, err := c.sendRequest(ctx, http.MethodGet, url, nil, accessToken)
	if err != nil {
		return nil, err
	}

	var projects []map[string]interface{}
	if err := json.Unmarshal(respBytes, &projects); err != nil {
		return nil, fmt.Errorf("failed to unmarshal projects: %w", err)
	}

	return projects, nil
}

// getIssueTypes gets the list of issue types for a project
func (c *JiraClient) getIssueTypes(ctx context.Context, accessToken, cloudID, projectKey string) ([]map[string]interface{}, error) {
	url := c.buildAPIURL(c.config.APIURL, fmt.Sprintf("/ex/jira/%s/rest/api/3/project/%s", cloudID, projectKey))

	respBytes, err := c.sendRequest(ctx, http.MethodGet, url, nil, accessToken)
	if err != nil {
		return nil, err
	}

	var projectData map[string]interface{}
	if err := json.Unmarshal(respBytes, &projectData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal project data: %w", err)
	}

	issueTypes, ok := projectData["issueTypes"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to extract issue types from project data")
	}

	var result []map[string]interface{}
	for _, it := range issueTypes {
		if issueType, ok := it.(map[string]interface{}); ok {
			result = append(result, issueType)
		}
	}

	return result, nil
}

// getTransitions gets the available transitions for an issue
func (c *JiraClient) getTransitions(ctx context.Context, accessToken, cloudID, issueKey string) ([]map[string]interface{}, error) {
	url := c.buildAPIURL(c.config.APIURL, fmt.Sprintf("/ex/jira/%s/rest/api/3/issue/%s/transitions", cloudID, issueKey))

	respBytes, err := c.sendRequest(ctx, http.MethodGet, url, nil, accessToken)
	if err != nil {
		return nil, err
	}

	var transitionsData map[string]interface{}
	if err := json.Unmarshal(respBytes, &transitionsData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal transitions data: %w", err)
	}

	transitions, ok := transitionsData["transitions"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to extract transitions from response")
	}

	var result []map[string]interface{}
	for _, t := range transitions {
		if transition, ok := t.(map[string]interface{}); ok {
			result = append(result, transition)
		}
	}

	return result, nil
}

// transitionIssue performs a transition on an issue
func (c *JiraClient) transitionIssue(ctx context.Context, accessToken, cloudID, issueKey string, transitionData map[string]interface{}) error {
	url := c.buildAPIURL(c.config.APIURL, fmt.Sprintf("/ex/jira/%s/rest/api/3/issue/%s/transitions", cloudID, issueKey))

	_, err := c.sendRequest(ctx, http.MethodPost, url, transitionData, accessToken)
	return err
}
