package common

import (
	"context"
	"encoding/json"
	"time"
)

// Connection represents a connection to a third-party service
type Connection struct {
	ID         int64
	UserID     int64
	Name       string
	Service    string
	Status     string
	AuthType   string
	AuthData   map[string]interface{}
	Metadata   map[string]interface{}
	LastUsedAt time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// AuthData represents authentication data for a service
type AuthData struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	APIKey       string    `json:"api_key,omitempty"`
	APISecret    string    `json:"api_secret,omitempty"`
	// Additional fields may be added as needed for specific services
}

// TriggerType represents a type of workflow trigger
type TriggerType struct {
	ID           string
	Service      string
	Name         string
	Description  string
	InputSchema  json.RawMessage
	OutputSchema json.RawMessage
}

// ActionType represents a type of workflow action
type ActionType struct {
	ID           string
	Service      string
	Name         string
	Description  string
	InputSchema  json.RawMessage
	OutputSchema json.RawMessage
}

// TriggerConfig holds configuration for a trigger
type TriggerConfig struct {
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config"`
}

// ActionConfig holds configuration for an action
type ActionConfig struct {
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config"`
}

// DataMapping represents a mapping from a source field to a target field
type DataMapping struct {
	SourceService string `json:"source_service"`
	SourceField   string `json:"source_field"`
	TargetService string `json:"target_service"`
	TargetField   string `json:"target_field"`
	Transformer   string `json:"transformer,omitempty"`
}

// TriggerData represents data coming from a trigger
type TriggerData struct {
	Type    string                 `json:"type"`
	Service string                 `json:"service"`
	Data    map[string]interface{} `json:"data"`
}

// ActionData represents data for an action
type ActionData struct {
	Type    string                 `json:"type"`
	Service string                 `json:"service"`
	Data    map[string]interface{} `json:"data"`
}

// AuthHandler defines authentication functionality for a service
type AuthHandler interface {
	// GetAuthURL returns the URL to redirect the user to for OAuth
	GetAuthURL(state string) string

	// HandleCallback processes the OAuth callback
	HandleCallback(ctx context.Context, code string) (*AuthData, error)

	// RefreshToken refreshes an expired OAuth token
	RefreshToken(ctx context.Context, authData *AuthData) (*AuthData, error)

	// ValidateAuth validates authentication credentials
	ValidateAuth(ctx context.Context, authData *AuthData) error
}

// TriggerHandler defines functionality for a service's triggers
type TriggerHandler interface {
	// GetTriggerTypes returns all available trigger types for this service
	GetTriggerTypes() ([]TriggerType, error)

	// ValidateTriggerConfig validates trigger configuration
	ValidateTriggerConfig(triggerType string, config json.RawMessage) error

	// ExecuteTrigger executes a trigger manually (for testing)
	ExecuteTrigger(ctx context.Context, connection *Connection, triggerType string, config json.RawMessage) (*TriggerData, error)
}

// ActionHandler defines functionality for a service's actions
type ActionHandler interface {
	// GetActionTypes returns all available action types for this service
	GetActionTypes() ([]ActionType, error)

	// ValidateActionConfig validates action configuration
	ValidateActionConfig(actionType string, config json.RawMessage) error

	// ExecuteAction executes an action
	ExecuteAction(ctx context.Context, connection *Connection, actionType string, config json.RawMessage, inputData map[string]interface{}) (*ActionData, error)
}

// ServiceProvider is the interface that all service integrations must implement
type ServiceProvider interface {
	// GetService returns the service identifier
	GetService() string

	// GetAuthHandler returns the authentication handler
	GetAuthHandler() AuthHandler

	// GetTriggerHandler returns the trigger handler
	GetTriggerHandler() TriggerHandler

	// GetActionHandler returns the action handler
	GetActionHandler() ActionHandler

	// ValidateConnection validates a connection
	ValidateConnection(ctx context.Context, connection *Connection) error

	// GetConnectionDetails returns details about a connection
	GetConnectionDetails(ctx context.Context, connection *Connection) (map[string]interface{}, error)
}
