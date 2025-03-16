package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/auditcue/integration-framework/internal/integrations/common"
)

// TriggerType constants
const (
	TriggerIssueCreated       = "issue_created"
	TriggerIssueUpdated       = "issue_updated"
	TriggerIssueStatusChanged = "issue_status_changed"
	TriggerIssueCommented     = "issue_commented"
	TriggerScheduledJQLSearch = "scheduled_jql_search"
)

// TriggerHandler implements the TriggerHandler interface for Jira
type TriggerHandler struct {
	client *JiraClient
}

// NewTriggerHandler creates a new trigger handler for Jira
func NewTriggerHandler(client *JiraClient) *TriggerHandler {
	return &TriggerHandler{
		client: client,
	}
}

// GetTriggerTypes returns all available trigger types for Jira
func (h *TriggerHandler) GetTriggerTypes() ([]common.TriggerType, error) {
	return []common.TriggerType{
		{
			ID:           TriggerIssueCreated,
			Service:      ServiceName,
			Name:         "Issue Created",
			Description:  "Triggered when a new issue is created in Jira",
			InputSchema:  json.RawMessage(`{"type":"object","required":["site_name","project_key"],"properties":{"site_name":{"type":"string","description":"Jira site name"},"project_key":{"type":"string","description":"Project key to monitor"},"issue_type":{"type":"string","description":"Optional issue type to filter (ID)"}}}`),
			OutputSchema: json.RawMessage(`{"type":"object","properties":{"issue":{"type":"object"},"project":{"type":"object"},"user":{"type":"object"}}}`),
		},
		{
			ID:           TriggerIssueUpdated,
			Service:      ServiceName,
			Name:         "Issue Updated",
			Description:  "Triggered when an issue is updated in Jira",
			InputSchema:  json.RawMessage(`{"type":"object","required":["site_name","project_key"],"properties":{"site_name":{"type":"string","description":"Jira site name"},"project_key":{"type":"string","description":"Project key to monitor"},"issue_type":{"type":"string","description":"Optional issue type to filter (ID)"},"fields":{"type":"array","items":{"type":"string"},"description":"Fields to monitor for changes"}}}`),
			OutputSchema: json.RawMessage(`{"type":"object","properties":{"issue":{"type":"object"},"project":{"type":"object"},"user":{"type":"object"},"changelog":{"type":"object"}}}`),
		},
		{
			ID:           TriggerIssueStatusChanged,
			Service:      ServiceName,
			Name:         "Issue Status Changed",
			Description:  "Triggered when an issue's status changes in Jira",
			InputSchema:  json.RawMessage(`{"type":"object","required":["site_name","project_key"],"properties":{"site_name":{"type":"string","description":"Jira site name"},"project_key":{"type":"string","description":"Project key to monitor"},"from_status":{"type":"string","description":"Optional from status ID"},"to_status":{"type":"string","description":"Optional to status ID"}}}`),
			OutputSchema: json.RawMessage(`{"type":"object","properties":{"issue":{"type":"object"},"project":{"type":"object"},"user":{"type":"object"},"from_status":{"type":"object"},"to_status":{"type":"object"}}}`),
		},
		{
			ID:           TriggerIssueCommented,
			Service:      ServiceName,
			Name:         "Issue Commented",
			Description:  "Triggered when a comment is added to an issue in Jira",
			InputSchema:  json.RawMessage(`{"type":"object","required":["site_name","project_key"],"properties":{"site_name":{"type":"string","description":"Jira site name"},"project_key":{"type":"string","description":"Project key to monitor"}}}`),
			OutputSchema: json.RawMessage(`{"type":"object","properties":{"issue":{"type":"object"},"project":{"type":"object"},"user":{"type":"object"},"comment":{"type":"object"}}}`),
		},
		{
			ID:           TriggerScheduledJQLSearch,
			Service:      ServiceName,
			Name:         "Scheduled JQL Search",
			Description:  "Searches for issues using JQL on a schedule",
			InputSchema:  json.RawMessage(`{"type":"object","required":["site_name","jql","schedule"],"properties":{"site_name":{"type":"string","description":"Jira site name"},"jql":{"type":"string","description":"JQL query"},"schedule":{"type":"string","description":"Cron expression for schedule"},"max_results":{"type":"integer","description":"Maximum number of results to return","default":10}}}`),
			OutputSchema: json.RawMessage(`{"type":"object","properties":{"issues":{"type":"array","items":{"type":"object"}},"total":{"type":"integer"},"timestamp":{"type":"string","format":"date-time"}}}`),
		},
	}, nil
}

// ValidateTriggerConfig validates trigger configuration
func (h *TriggerHandler) ValidateTriggerConfig(triggerType string, config json.RawMessage) error {
	// Simple validation - just try to unmarshal the config to ensure it's valid JSON
	var configMap map[string]interface{}
	if err := json.Unmarshal(config, &configMap); err != nil {
		return fmt.Errorf("invalid trigger configuration: %w", err)
	}

	// Check for required fields based on trigger type
	switch triggerType {
	case TriggerIssueCreated, TriggerIssueUpdated, TriggerIssueStatusChanged, TriggerIssueCommented:
		required := []string{"site_name", "project_key"}
		for _, field := range required {
			if _, ok := configMap[field]; !ok {
				return fmt.Errorf("missing required field: %s", field)
			}
		}
	case TriggerScheduledJQLSearch:
		required := []string{"site_name", "jql", "schedule"}
		for _, field := range required {
			if _, ok := configMap[field]; !ok {
				return fmt.Errorf("missing required field: %s", field)
			}
		}

		// Validate cron expression
		schedule, ok := configMap["schedule"].(string)
		if !ok {
			return fmt.Errorf("schedule must be a string")
		}
		if !isValidCronExpression(schedule) {
			return fmt.Errorf("invalid cron expression: %s", schedule)
		}
	default:
		return fmt.Errorf("unknown trigger type: %s", triggerType)
	}

	return nil
}

// ExecuteTrigger executes a trigger manually (for testing)
func (h *TriggerHandler) ExecuteTrigger(ctx context.Context, connection *common.Connection, triggerType string, config json.RawMessage) (*common.TriggerData, error) {
	// Extract auth data from connection
	var authData common.AuthData
	authDataBytes, err := json.Marshal(connection.AuthData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal auth data: %w", err)
	}
	if err := json.Unmarshal(authDataBytes, &authData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal auth data: %w", err)
	}

	// Parse trigger config
	var configMap map[string]interface{}
	if err := json.Unmarshal(config, &configMap); err != nil {
		return nil, fmt.Errorf("failed to parse trigger config: %w", err)
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

	// Execute the appropriate trigger
	switch triggerType {
	case TriggerScheduledJQLSearch:
		return h.executeScheduledJQLSearch(ctx, authData.AccessToken, cloudID, configMap)
	case TriggerIssueCreated, TriggerIssueUpdated, TriggerIssueStatusChanged, TriggerIssueCommented:
		return nil, fmt.Errorf("webhook-based triggers cannot be manually executed")
	default:
		return nil, fmt.Errorf("unknown trigger type: %s", triggerType)
	}
}

// executeScheduledJQLSearch executes the scheduled JQL search trigger
func (h *TriggerHandler) executeScheduledJQLSearch(ctx context.Context, accessToken, cloudID string, config map[string]interface{}) (*common.TriggerData, error) {
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
	fields := []string{"summary", "status", "assignee", "priority", "created", "updated", "issuetype", "project"}

	// Search for issues
	result, err := h.client.searchIssues(ctx, accessToken, cloudID, jql, fields, maxResults)
	if err != nil {
		return nil, fmt.Errorf("failed to search issues: %w", err)
	}

	// Add timestamp to result
	result["timestamp"] = time.Now().Format(time.RFC3339)

	return &common.TriggerData{
		Type:    TriggerScheduledJQLSearch,
		Service: ServiceName,
		Data:    result,
	}, nil
}

// isValidCronExpression validates a cron expression (simple check for now)
func isValidCronExpression(cron string) bool {
	// Just a simple check for now
	// A more comprehensive check would use a proper cron library
	parts := strings.Fields(cron)
	return len(parts) == 5 || len(parts) == 6
}

// HandleWebhook processes an incoming webhook from Jira
func (h *TriggerHandler) HandleWebhook(payload []byte) (*common.TriggerData, error) {
	// Parse the webhook payload
	var webhookData map[string]interface{}
	if err := json.Unmarshal(payload, &webhookData); err != nil {
		return nil, fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	// Determine the webhook event type
	webhookEvent, ok := webhookData["webhookEvent"].(string)
	if !ok {
		return nil, fmt.Errorf("webhook payload does not contain webhookEvent field")
	}

	// Map webhook event to trigger type
	var triggerType string
	switch webhookEvent {
	case "jira:issue_created":
		triggerType = TriggerIssueCreated
	case "jira:issue_updated":
		// Check if status changed
		changelog, hasChangelog := webhookData["changelog"].(map[string]interface{})
		if hasChangelog {
			items, hasItems := changelog["items"].([]interface{})
			if hasItems {
				for _, item := range items {
					itemMap, isMap := item.(map[string]interface{})
					if isMap && itemMap["field"] == "status" {
						triggerType = TriggerIssueStatusChanged
						break
					}
				}
			}
		}
		if triggerType == "" {
			triggerType = TriggerIssueUpdated
		}
	case "comment_created":
		triggerType = TriggerIssueCommented
	default:
		return nil, fmt.Errorf("unsupported webhook event: %s", webhookEvent)
	}

	return &common.TriggerData{
		Type:    triggerType,
		Service: ServiceName,
		Data:    webhookData,
	}, nil
}
