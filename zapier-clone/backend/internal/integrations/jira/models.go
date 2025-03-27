package jira

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Ticket represents a Jira issue
type Ticket struct {
	ID          string                 `json:"id,omitempty"`
	Key         string                 `json:"key,omitempty"`
	Self        string                 `json:"self,omitempty"`
	Project     string                 `json:"project"`
	IssueType   string                 `json:"issuetype"`
	Summary     string                 `json:"summary"`
	Description string                 `json:"description"`
	Priority    string                 `json:"priority,omitempty"`
	Status      string                 `json:"status,omitempty"`
	Assignee    string                 `json:"assignee,omitempty"`
	Parent      string                 `json:"parent,omitempty"`
	Reporter    string                 `json:"reporter,omitempty"`
	Created     time.Time              `json:"created,omitempty"`
	Updated     time.Time              `json:"updated,omitempty"`
	DueDate     time.Time              `json:"duedate,omitempty"`
	Labels      []string               `json:"labels,omitempty"`
	Epic        *EpicDetails           `json:"epic,omitempty"`
	Components  []string               `json:"components,omitempty"`
	Fields      map[string]interface{} `json:"fields,omitempty"`
}

// EpicDetails contains Epic-specific fields
type EpicDetails struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type TicketUpdate struct {
	Status      string                 `json:"status,omitempty"`
	Resolution  string                 `json:"resolution,omitempty"`
	Comment     string                 `json:"comment,omitempty"`
	Assignee    string                 `json:"assignee,omitempty"`
	Priority    string                 `json:"priority,omitempty"`
	DueDate     string                 `json:"dueDate,omitempty"`
	Summary     string                 `json:"summary,omitempty"`
	Description string                 `json:"description,omitempty"`
	Fields      map[string]interface{} `json:"fields,omitempty"`
}

// Comment represents a comment on a Jira issue
type Comment struct {
	ID         string    `json:"id,omitempty"`
	Body       string    `json:"body"`
	Author     string    `json:"author,omitempty"`
	Created    time.Time `json:"created,omitempty"`
	Updated    time.Time `json:"updated,omitempty"`
	Visibility string    `json:"visibility,omitempty"`
}

// User represents a Jira user
type User struct {
	ID           string `json:"accountId,omitempty"`
	EmailAddress string `json:"emailAddress,omitempty"`
	DisplayName  string `json:"displayName,omitempty"`
	Active       bool   `json:"active,omitempty"`
	TimeZone     string `json:"timeZone,omitempty"`
}

// Project represents a Jira project
type Project struct {
	ID         string            `json:"id,omitempty"`
	Key        string            `json:"key"`
	Name       string            `json:"name"`
	AvatarUrls map[string]string `json:"avatarUrls,omitempty"`
	URL        string            `json:"url,omitempty"`
}

// IssueType represents a Jira issue type
type IssueType struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IconURL     string `json:"iconUrl,omitempty"`
}

// Status represents a Jira issue status
type Status struct {
	ID          string         `json:"id,omitempty"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Category    StatusCategory `json:"statusCategory,omitempty"`
}

// StatusCategory represents a Jira status category
type StatusCategory struct {
	ID        int    `json:"id,omitempty"`
	Key       string `json:"key,omitempty"`
	Name      string `json:"name,omitempty"`
	ColorName string `json:"colorName,omitempty"`
}

// Priority represents a Jira priority
type Priority struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name"`
	IconURL string `json:"iconUrl,omitempty"`
}

// Transition represents a Jira workflow transition
type Transition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	To   Status `json:"to,omitempty"`
}

// Attachment represents a file attachment on a Jira issue
type Attachment struct {
	ID           string    `json:"id,omitempty"`
	Filename     string    `json:"filename"`
	Author       string    `json:"author,omitempty"`
	Created      time.Time `json:"created,omitempty"`
	Size         int       `json:"size,omitempty"`
	MimeType     string    `json:"mimeType,omitempty"`
	ContentURL   string    `json:"content,omitempty"`
	ThumbnailURL string    `json:"thumbnail,omitempty"`
}

// Resolution represents a Jira resolution
type Resolution struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Component represents a Jira component
type Component struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Lead        string `json:"lead,omitempty"`
}

// Version represents a Jira version
type Version struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Released    bool   `json:"released,omitempty"`
	Archived    bool   `json:"archived,omitempty"`
	ReleaseDate string `json:"releaseDate,omitempty"`
	Overdue     bool   `json:"overdue,omitempty"`
}

// CustomField represents a custom field in Jira
type CustomField struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	SearchKey   string `json:"searchKey,omitempty"`
}

// SearchResult represents the result of a Jira search
type SearchResult struct {
	StartAt    int      `json:"startAt"`
	MaxResults int      `json:"maxResults"`
	Total      int      `json:"total"`
	Issues     []Ticket `json:"issues"`
}

// CreateTicketRequest represents the request to create a new Jira issue
type CreateTicketRequest struct {
	Fields struct {
		Project struct {
			Key string `json:"key"`
		} `json:"project"`
		Summary     string `json:"summary"`
		Description string `json:"description"`
		IssueType   struct {
			Name string `json:"name"`
		} `json:"issuetype"`
		Priority struct {
			Name string `json:"name,omitempty"`
		} `json:"priority,omitempty"`
		Labels       []string               `json:"labels,omitempty"`
		Components   []map[string]string    `json:"components,omitempty"`
		DueDate      string                 `json:"duedate,omitempty"`
		Assignee     map[string]string      `json:"assignee,omitempty"`
		CustomFields map[string]interface{} `json:"-"`
	} `json:"fields"`
}

// MarshalJSON implements a custom JSON marshaler for CreateTicketRequest
// to properly handle custom fields
func (c CreateTicketRequest) MarshalJSON() ([]byte, error) {
	type Alias CreateTicketRequest
	data, err := json.Marshal(struct{ Alias }{Alias: Alias(c)})
	if err != nil {
		return nil, err
	}

	if len(c.Fields.CustomFields) == 0 {
		return data, nil
	}

	// Parse the JSON into a map
	var objMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &objMap); err != nil {
		return nil, err
	}

	// Get the fields object
	var fieldsMap map[string]json.RawMessage
	if err := json.Unmarshal(objMap["fields"], &fieldsMap); err != nil {
		return nil, err
	}

	// Add all custom fields to the fieldsMap
	for key, value := range c.Fields.CustomFields {
		customFieldData, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		fieldsMap[key] = customFieldData
	}

	// Re-marshal the modified fieldsMap
	modifiedFields, err := json.Marshal(fieldsMap)
	if err != nil {
		return nil, err
	}
	objMap["fields"] = modifiedFields

	// Re-marshal the complete object
	return json.Marshal(objMap)
}

// UpdateTicketRequest represents the request to update a Jira issue
type UpdateTicketRequest struct {
	Fields          map[string]interface{}   `json:"fields,omitempty"`
	Update          map[string]interface{}   `json:"update,omitempty"`
	HistoryMetadata map[string]interface{}   `json:"historyMetadata,omitempty"`
	Properties      []map[string]interface{} `json:"properties,omitempty"`
}

// TransitionRequest represents the request to transition a Jira issue
type TransitionRequest struct {
	Transition struct {
		ID string `json:"id"`
	} `json:"transition"`
	Fields          map[string]interface{} `json:"fields,omitempty"`
	Update          map[string]interface{} `json:"update,omitempty"`
	HistoryMetadata map[string]interface{} `json:"historyMetadata,omitempty"`
}

// CommentRequest represents the request to add a comment to a Jira issue
type CommentRequest struct {
	Body       string `json:"body"`
	Visibility struct {
		Type  string `json:"type,omitempty"`
		Value string `json:"value,omitempty"`
	} `json:"visibility,omitempty"`
}

// WebhookEvent represents a Jira webhook event
type WebhookEvent struct {
	WebhookEvent string            `json:"webhookEvent"`
	Issue        *WebhookIssue     `json:"issue,omitempty"`
	Comment      *WebhookComment   `json:"comment,omitempty"`
	User         *WebhookUser      `json:"user,omitempty"`
	Changelog    *WebhookChangelog `json:"changelog,omitempty"`
	Timestamp    int64             `json:"timestamp"`
}

// WebhookIssue represents the issue field in a Jira webhook event
type WebhookIssue struct {
	ID     string             `json:"id"`
	Key    string             `json:"key"`
	Fields WebhookIssueFields `json:"fields"`
	Self   string             `json:"self"`
}

// WebhookIssueFields represents the issue fields in a Jira webhook event
type WebhookIssueFields struct {
	Summary     string             `json:"summary"`
	Description string             `json:"description"`
	Status      *WebhookStatus     `json:"status"`
	Resolution  *WebhookResolution `json:"resolution"`
	Assignee    *WebhookUser       `json:"assignee"`
	Reporter    *WebhookUser       `json:"reporter"`
	Priority    *WebhookPriority   `json:"priority"`
	IssueType   *WebhookIssueType  `json:"issuetype"`
	// Add any custom fields you access
	CustomFields map[string]interface{} `json:"-"`
}

// UnmarshalJSON is a custom unmarshaller for WebhookIssueFields
// to handle custom fields that start with "customfield_"
func (f *WebhookIssueFields) UnmarshalJSON(data []byte) error {
	// Use a temporary structure to decode standard fields
	type tempFields WebhookIssueFields
	var standardFields tempFields

	if err := json.Unmarshal(data, &standardFields); err != nil {
		return err
	}

	// Copy standard fields to the original struct
	*f = WebhookIssueFields(standardFields)

	// Parse custom fields
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawFields); err != nil {
		return err
	}

	// Initialize custom fields map
	f.CustomFields = make(map[string]interface{})

	// Extract custom fields (those starting with "customfield_")
	for key, value := range rawFields {
		if strings.HasPrefix(key, "customfield_") {
			var fieldValue interface{}
			if err := json.Unmarshal(value, &fieldValue); err != nil {
				return err
			}
			f.CustomFields[key] = fieldValue
		}
	}

	return nil
}

// WebhookStatus represents a status in a Jira webhook event
type WebhookStatus struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Self        string `json:"self"`
}

// WebhookResolution represents a resolution in a Jira webhook event
type WebhookResolution struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Self        string `json:"self"`
}

// WebhookUser represents a user in a Jira webhook event
type WebhookUser struct {
	Self         string `json:"self"`
	Name         string `json:"name"`
	Key          string `json:"key"`
	EmailAddress string `json:"emailAddress"`
	DisplayName  string `json:"displayName"`
	Active       bool   `json:"active"`
}

// WebhookComment represents a comment in a Jira webhook event
type WebhookComment struct {
	ID      string       `json:"id"`
	Body    string       `json:"body"`
	Author  *WebhookUser `json:"author"`
	Created string       `json:"created"`
	Updated string       `json:"updated"`
	Self    string       `json:"self"`
}

// WebhookPriority represents a priority in a Jira webhook event
type WebhookPriority struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Self string `json:"self"`
}

// WebhookIssueType represents an issue type in a Jira webhook event
type WebhookIssueType struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Self        string `json:"self"`
}

// WebhookChangelog represents a changelog in a Jira webhook event
type WebhookChangelog struct {
	ID    string                 `json:"id"`
	Items []WebhookChangelogItem `json:"items"`
}

// WebhookChangelogItem represents a changelog item in a Jira webhook event
type WebhookChangelogItem struct {
	Field      string `json:"field"`
	FromString string `json:"fromString"`
	ToString   string `json:"toString"`
	From       string `json:"from"`
	To         string `json:"to"`
}

// ErrorResponse represents an error response from Jira API
type ErrorResponse struct {
	ErrorMessages []string          `json:"errorMessages"`
	Errors        map[string]string `json:"errors"`
	StatusCode    int               `json:"-"`
}

// Error implements the error interface for ErrorResponse
func (e *ErrorResponse) Error() string {
	var msg string
	if len(e.ErrorMessages) > 0 {
		msg = strings.Join(e.ErrorMessages, "; ")
	} else if len(e.Errors) > 0 {
		errorStrings := make([]string, 0, len(e.Errors))
		for key, val := range e.Errors {
			errorStrings = append(errorStrings, fmt.Sprintf("%s: %s", key, val))
		}
		msg = strings.Join(errorStrings, "; ")
	} else {
		msg = "unknown error"
	}
	return fmt.Sprintf("Jira API error (status %d): %s", e.StatusCode, msg)
}

// NewEmptyRiskJiraMapping creates a new mapping store with empty mappings
func NewEmptyRiskJiraMapping() *RiskJiraMapping {
	return &RiskJiraMapping{
		RiskIDToJiraKey: make(map[string]string),
		JiraKeyToRiskID: make(map[string]string),
		filePath:        "", // No persistence
	}
}

type IncidentJiraMapping struct {
	IncidentIDToJiraKey map[string]string `json:"incident_id_to_jira_key"`
	JiraKeyToIncidentID map[string]string `json:"jira_key_to_incident_id"`
	filePath            string
}

// NewIncidentJiraMapping creates a new mapping store and loads existing mappings
func NewIncidentJiraMapping(dataDir string) (*IncidentJiraMapping, error) {
	filePath := filepath.Join(dataDir, "incident_jira_mapping.json")

	// Create the mapping
	mapping := &IncidentJiraMapping{
		IncidentIDToJiraKey: make(map[string]string),
		JiraKeyToIncidentID: make(map[string]string),
		filePath:            filePath,
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Try to load existing mappings
	file, err := os.Open(filePath)
	if os.IsNotExist(err) {
		// File doesn't exist yet, that's ok
		return mapping, nil
	} else if err != nil {
		return nil, fmt.Errorf("error opening mapping file: %w", err)
	}
	defer file.Close()

	// Decode the JSON data
	if err := json.NewDecoder(file).Decode(mapping); err != nil {
		return nil, fmt.Errorf("error decoding mapping data: %w", err)
	}

	return mapping, nil
}

// SaveMapping saves the current mappings to disk
func (m *IncidentJiraMapping) SaveMapping() error {
	if m.filePath == "" {
		return nil // No persistence
	}

	file, err := os.Create(m.filePath)
	if err != nil {
		return fmt.Errorf("error creating mapping file: %w", err)
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(m); err != nil {
		return fmt.Errorf("error encoding mapping data: %w", err)
	}

	return nil
}

// AddMapping adds a new mapping between ServiceNow incident ID and Jira key
func (m *IncidentJiraMapping) AddMapping(incidentID, jiraKey string) error {
	m.IncidentIDToJiraKey[incidentID] = jiraKey
	m.JiraKeyToIncidentID[jiraKey] = incidentID
	return m.SaveMapping()
}

// GetJiraKeyFromIncidentID gets the Jira key for a given ServiceNow incident ID
func (m *IncidentJiraMapping) GetJiraKeyFromIncidentID(incidentID string) (string, bool) {
	jiraKey, exists := m.IncidentIDToJiraKey[incidentID]
	return jiraKey, exists
}

// GetIncidentIDFromJiraKey gets the ServiceNow incident ID for a given Jira key
func (m *IncidentJiraMapping) GetIncidentIDFromJiraKey(jiraKey string) (string, bool) {
	incidentID, exists := m.JiraKeyToIncidentID[jiraKey]
	return incidentID, exists
}
