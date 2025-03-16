package jira

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/auditcue/integration-framework/internal/integrations/common"
)

// ActionType constants
const (
	ActionCreateIssue     = "create_issue"
	ActionUpdateIssue     = "update_issue"
	ActionAddComment      = "add_comment"
	ActionTransitionIssue = "transition_issue"
	ActionSearchIssues    = "search_issues"
)

// ActionHandler implements the ActionHandler interface for Jira
type ActionHandler struct {
	client *JiraClient
}

// NewActionHandler creates a new action handler for Jira
func NewActionHandler(client *JiraClient) *ActionHandler {
	return &ActionHandler{
		client: client,
	}
}

// GetActionTypes returns all available action types for Jira
func (h *ActionHandler) GetActionTypes() ([]common.ActionType, error) {
	return []common.ActionType{
		{
			ID:           ActionCreateIssue,
			Service:      ServiceName,
			Name:         "Create Issue",
			Description:  "Creates a new issue in Jira",
			InputSchema:  json.RawMessage(`{"type":"object","required":["site_name","project_key","issue_type_id","summary"],"properties":{"site_name":{"type":"string","description":"Jira site name"},"project_key":{"type":"string","description":"Project key"},"issue_type_id":{"type":"string","description":"Issue type ID"},"summary":{"type":"string","description":"Issue summary"},"description":{"type":"string","description":"Issue description"},"priority":{"type":"string","description":"Priority ID"},"labels":{"type":"array","items":{"type":"string"},"description":"Issue labels"},"custom_fields":{"type":"object","description":"Custom fields"}}}`),
			OutputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"},"key":{"type":"string"},"self":{"type":"string"}}}`),
		},
		{
			ID:           ActionUpdateIssue,
			Service:      ServiceName,
			Name:         "Update Issue",
			Description:  "Updates an existing issue in Jira",
			InputSchema:  json.RawMessage(`{"type":"object","required":["site_name","issue_key"],"properties":{"site_name":{"type":"string","description":"Jira site name"},"issue_key":{"type":"string","description":"Issue key"},"summary":{"type":"string","description":"Issue summary"},"description":{"type":"string","description":"Issue description"},"priority":{"type":"string","description":"Priority ID"},"labels":{"type":"array","items":{"type":"string"},"description":"Issue labels"},"custom_fields":{"type":"object","description":"Custom fields"}}}`),
			OutputSchema: json.RawMessage(`{"type":"object","properties":{"success":{"type":"boolean"}}}`),
		},
		{
			ID:           ActionAddComment,
			Service:      ServiceName,
			Name:         "Add Comment",
			Description:  "Adds a comment to an issue",
			InputSchema:  json.RawMessage(`{"type":"object","required":["site_name","issue_key","body"],"properties":{"site_name":{"type":"string","description":"Jira site name"},"issue_key":{"type":"string","description":"Issue key"},"body":{"type":"string","description":"Comment body"}}}`),
			OutputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"},"self":{"type":"string"}}}`),
		},
		{
			ID:           ActionTransitionIssue,
			Service:      ServiceName,
			Name:         "Transition Issue",
			Description:  "Moves an issue through a workflow transition",
			InputSchema:  json.RawMessage(`{"type":"object","required":["site_name","issue_key","transition_id"],"properties":{"site_name":{"type":"string","description":"Jira site name"},"issue_key":{"type":"string","description":"Issue key"},"transition_id":{"type":"string","description":"Transition ID"},"comment":{"type":"string","description":"Transition comment"}}}`),
			OutputSchema: json.RawMessage(`{"type":"object","properties":{"success":{"type":"boolean"}}}`),
		},
		{
			ID:           ActionSearchIssues,
			Service:      ServiceName,
			Name:         "Search Issues",
			Description:  "Searches for issues using JQL",
			InputSchema:  json.RawMessage(`{"type":"object","required":["site_name","jql"],"properties":{"site_name":{"type":"string","description":"Jira site name"},"jql":{"type":"string","description":"JQL query"},"max_results":{"type":"integer","description":"Maximum number of results to return","default":10},"fields":{"type":"array","items":{"type":"string"},"description":"Fields to include in results"}}}`),
			OutputSchema: json.RawMessage(`{"type":"object","properties":{"issues":{"type":"array","items":{"type":"object"}},"total":{"type":"integer"},"max_results":{"type":"integer"},"start_at":{"type":"integer"}}}`),
		},
	}, nil
}

// ValidateActionConfig validates action configuration
func (h *ActionHandler) ValidateActionConfig(actionType string, config json.RawMessage) error {
	// Simple validation - just try to unmarshal the config to ensure it's valid JSON
	var configMap map[string]interface{}
	if err := json.Unmarshal(config, &configMap); err != nil {
		return fmt.Errorf("invalid action configuration: %w", err)
	}

	// Check for required fields based on action type
	switch actionType {
	case ActionCreateIssue:
		required := []string{"site_name", "project_key", "issue_type_id", "summary"}
		for _, field := range required {
			if _, ok := configMap[field]; !ok {
				return fmt.Errorf("missing required field: %s", field)
			}
		}
	case ActionUpdateIssue:
		required := []string{"site_name", "issue_key"}
		for _, field := range required {
			if _, ok := configMap[field]; !ok {
				return fmt.Errorf("missing required field: %s", field)
			}
		}
	case ActionAddComment:
		required := []string{"site_name", "issue_key", "body"}
		for _, field := range required {
			if _, ok := configMap[field]; !ok {
				return fmt.Errorf("missing required field: %s", field)
			}
		}
	case ActionTransitionIssue:
		required := []string{"site_name", "issue_key", "transition_id"}
		for _, field := range required {
			if _, ok := configMap[field]; !ok {
				return fmt.Errorf("missing required field: %s", field)
			}
		}
	case ActionSearchIssues:
		required := []string{"site_name", "jql"}
		for _, field := range required {
			if _, ok := configMap[field]; !ok {
				return fmt.Errorf("missing required field: %s", field)
			}
		}
	default:
		return fmt.Errorf("unknown action type: %s", actionType)
	}

	return nil
}

// ExecuteAction executes a Jira action
func (h *ActionHandler) ExecuteAction(ctx context.Context, connection *common.Connection, actionType string, config json.RawMessage, inputData map[string]interface{}) (*common.ActionData, error) {
	// Extract auth data from connection
	var authData common.AuthData
	authDataBytes, err := json.Marshal(connection.AuthData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal auth data: %w", err)
	}
	if err := json.Unmarshal(authDataBytes, &authData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal auth data: %w", err)
	}

	// Parse action config
	var configMap map[string]interface{}
	if err := json.Unmarshal(config, &configMap); err != nil {
		return nil, fmt.Errorf("failed to parse action config: %w", err)
	}

	// Merge config with input data - input data takes precedence
	for k, v := range inputData {
		configMap[k] = v
	}

	// Get site name from config
	siteName, ok := configMap["site_name"].(string)
	if !ok {
		return nil, fmt.Errorf("site_name is required and must be a string")
	}

	// Get cloud ID for the site
	cloudID, err := h.client.getCloudID(ctx, authData.AccessToken, siteName)
	if err != nil {
		return nil, fmt.Errorf("failed to get cloud ID: %w", err)
	}

	// Execute the appropriate action
	switch actionType {
	case ActionCreateIssue:
		return h.executeCreateIssue(ctx, authData.AccessToken, cloudID, configMap)
	case ActionUpdateIssue:
		return h.executeUpdateIssue(ctx, authData.AccessToken, cloudID, configMap)
	case ActionAddComment:
		return h.executeAddComment(ctx, authData.AccessToken, cloudID, configMap)
	case ActionTransitionIssue:
		return h.executeTransitionIssue(ctx, authData.AccessToken, cloudID, configMap)
	case ActionSearchIssues:
		return h.executeSearchIssues(ctx, authData.AccessToken, cloudID, configMap)
	default:
		return nil, fmt.Errorf("unknown action type: %s", actionType)
	}
}

// executeCreateIssue executes the create issue action
func (h *ActionHandler) executeCreateIssue(ctx context.Context, accessToken, cloudID string, config map[string]interface{}) (*common.ActionData, error) {
	// Prepare issue data
	issueData := map[string]interface{}{
		"fields": map[string]interface{}{
			"project": map[string]interface{}{
				"key": config["project_key"],
			},
			"issuetype": map[string]interface{}{
				"id": config["issue_type_id"],
			},
			"summary": config["summary"],
		},
	}

	// Add optional fields
	fields := issueData["fields"].(map[string]interface{})

	if description, ok := config["description"].(string); ok && description != "" {
		fields["description"] = map[string]interface{}{
			"type":    "doc",
			"version": 1,
			"content": []map[string]interface{}{
				{
					"type": "paragraph",
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": description,
						},
					},
				},
			},
		}
	}

	if priority, ok := config["priority"].(string); ok && priority != "" {
		fields["priority"] = map[string]interface{}{
			"id": priority,
		}
	}

	if labels, ok := config["labels"].([]interface{}); ok && len(labels) > 0 {
		fields["labels"] = labels
	}

	// Add custom fields
	if customFields, ok := config["custom_fields"].(map[string]interface{}); ok {
		for field, value := range customFields {
			fields[field] = value
		}
	}

	// Create the issue
	result, err := h.client.createIssue(ctx, accessToken, cloudID, issueData)
	if err != nil {
		return nil, fmt.Errorf("failed to create issue: %w", err)
	}

	return &common.ActionData{
		Type:    ActionCreateIssue,
		Service: ServiceName,
		Data:    result,
	}, nil
}

// executeUpdateIssue executes the update issue action
func (h *ActionHandler) executeUpdateIssue(ctx context.Context, accessToken, cloudID string, config map[string]interface{}) (*common.ActionData, error) {
	issueKey, ok := config["issue_key"].(string)
	if !ok {
		return nil, fmt.Errorf("issue_key is required and must be a string")
	}

	// Prepare update data
	updateData := map[string]interface{}{
		"fields": map[string]interface{}{},
	}
	fields := updateData["fields"].(map[string]interface{})

	if summary, ok := config["summary"].(string); ok && summary != "" {
		fields["summary"] = summary
	}

	if description, ok := config["description"].(string); ok && description != "" {
		fields["description"] = map[string]interface{}{
			"type":    "doc",
			"version": 1,
			"content": []map[string]interface{}{
				{
					"type": "paragraph",
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": description,
						},
					},
				},
			},
		}
	}

	if priority, ok := config["priority"].(string); ok && priority != "" {
		fields["priority"] = map[string]interface{}{
			"id": priority,
		}
	}

	if labels, ok := config["labels"].([]interface{}); ok {
		fields["labels"] = labels
	}

	// Add custom fields
	if customFields, ok := config["custom_fields"].(map[string]interface{}); ok {
		for field, value := range customFields {
			fields[field] = value
		}
	}

	// Update the issue
	err := h.client.updateIssue(ctx, accessToken, cloudID, issueKey, updateData)
	if err != nil {
		return nil, fmt.Errorf("failed to update issue: %w", err)
	}

	return &common.ActionData{
		Type:    ActionUpdateIssue,
		Service: ServiceName,
		Data: map[string]interface{}{
			"success":   true,
			"issue_key": issueKey,
		},
	}, nil
}

// executeAddComment executes the add comment action
func (h *ActionHandler) executeAddComment(ctx context.Context, accessToken, cloudID string, config map[string]interface{}) (*common.ActionData, error) {
	issueKey, ok := config["issue_key"].(string)
	if !ok {
		return nil, fmt.Errorf("issue_key is required and must be a string")
	}

	body, ok := config["body"].(string)
	if !ok {
		return nil, fmt.Errorf("body is required and must be a string")
	}

	// Prepare comment data
	commentData := map[string]interface{}{
		"body": map[string]interface{}{
			"type":    "doc",
			"version": 1,
			"content": []map[string]interface{}{
				{
					"type": "paragraph",
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": body,
						},
					},
				},
			},
		},
	}

	// Add the comment
	result, err := h.client.addComment(ctx, accessToken, cloudID, issueKey, commentData)
	if err != nil {
		return nil, fmt.Errorf("failed to add comment: %w", err)
	}

	return &common.ActionData{
		Type:    ActionAddComment,
		Service: ServiceName,
		Data:    result,
	}, nil
}

// executeTransitionIssue executes the transition issue action
func (h *ActionHandler) executeTransitionIssue(ctx context.Context, accessToken, cloudID string, config map[string]interface{}) (*common.ActionData, error) {
	issueKey, ok := config["issue_key"].(string)
	if !ok {
		return nil, fmt.Errorf("issue_key is required and must be a string")
	}

	transitionID, ok := config["transition_id"].(string)
	if !ok {
		return nil, fmt.Errorf("transition_id is required and must be a string")
	}

	// Prepare transition data
	transitionData := map[string]interface{}{
		"transition": map[string]interface{}{
			"id": transitionID,
		},
	}

	// Add comment if provided
	if comment, ok := config["comment"].(string); ok && comment != "" {
		transitionData["update"] = map[string]interface{}{
			"comment": []map[string]interface{}{
				{
					"add": map[string]interface{}{
						"body": map[string]interface{}{
							"type":    "doc",
							"version": 1,
							"content": []map[string]interface{}{
								{
									"type": "paragraph",
									"content": []map[string]interface{}{
										{
											"type": "text",
											"text": comment,
										},
									},
								},
							},
						},
					},
				},
			},
		}
	}

	// Perform the transition
	err := h.client.transitionIssue(ctx, accessToken, cloudID, issueKey, transitionData)
	if err != nil {
		return nil, fmt.Errorf("failed to transition issue: %w", err)
	}

	return &common.ActionData{
		Type:    ActionTransitionIssue,
		Service: ServiceName,
		Data: map[string]interface{}{
			"success":       true,
			"issue_key":     issueKey,
			"transition_id": transitionID,
		},
	}, nil
}

// executeSearchIssues executes the search issues action
func (h *ActionHandler) executeSearchIssues(ctx context.Context, accessToken, cloudID string, config map[string]interface{}) (*common.ActionData, error) {
	jql, ok := config["jql"].(string)
	if !ok {
		return nil, fmt.Errorf("jql is required and must be a string")
	}

	// Default max results
	maxResults := 10
	if max, ok := config["max_results"].(float64); ok {
		maxResults = int(max)
	}

	// Default fields
	fields := []string{"summary", "status", "assignee", "priority"}
	if customFields, ok := config["fields"].([]interface{}); ok && len(customFields) > 0 {
		fields = make([]string, 0, len(customFields))
		for _, field := range customFields {
			if f, ok := field.(string); ok {
				fields = append(fields, f)
			}
		}
	}

	// Search for issues
	result, err := h.client.searchIssues(ctx, accessToken, cloudID, jql, fields, maxResults)
	if err != nil {
		return nil, fmt.Errorf("failed to search issues: %w", err)
	}

	return &common.ActionData{
		Type:    ActionSearchIssues,
		Service: ServiceName,
		Data:    result,
	}, nil
}
