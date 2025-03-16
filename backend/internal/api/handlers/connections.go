package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/auditcue/integration-framework/internal/db"
	"github.com/auditcue/integration-framework/internal/integrations/common"
	"github.com/auditcue/integration-framework/internal/workflow"
	"github.com/auditcue/integration-framework/pkg/logger"
	"github.com/gin-gonic/gin"
)

// ConnectionHandler handles connection-related API endpoints
type ConnectionHandler struct {
	db     *db.Database
	engine *workflow.Engine
	logger *logger.Logger
	states map[string]string // Map of state tokens to user IDs
}

// NewConnectionHandler creates a new connection handler
func NewConnectionHandler(db *db.Database, engine *workflow.Engine, logger *logger.Logger) *ConnectionHandler {
	return &ConnectionHandler{
		db:     db,
		engine: engine,
		logger: logger,
		states: make(map[string]string),
	}
}

// Connection represents a connection to a third-party service
type Connection struct {
	ID         int64                  `json:"id"`
	UserID     int64                  `json:"user_id"`
	Name       string                 `json:"name"`
	Service    string                 `json:"service"`
	Status     string                 `json:"status"`
	AuthType   string                 `json:"auth_type"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	LastUsedAt time.Time              `json:"last_used_at,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

// ConnectionRequest represents a request to create or update a connection
type ConnectionRequest struct {
	Name     string                 `json:"name" binding:"required"`
	Service  string                 `json:"service" binding:"required"`
	AuthType string                 `json:"auth_type" binding:"required"`
	AuthData map[string]interface{} `json:"auth_data"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ListConnections lists all connections for the authenticated user
func (h *ConnectionHandler) ListConnections(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Query connections
	rows, err := h.db.DB().Query(`
		SELECT id, user_id, name, service, status, auth_type, metadata, last_used_at, created_at, updated_at
		FROM connections
		WHERE user_id = ?
		ORDER BY last_used_at DESC
	`, userID)
	if err != nil {
		h.logger.Error("Failed to query connections", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve connections"})
		return
	}
	defer rows.Close()

	// Iterate through rows
	var connections []Connection
	for rows.Next() {
		var conn Connection
		var metadataJSON sql.NullString
		var lastUsedAt sql.NullTime

		err := rows.Scan(
			&conn.ID,
			&conn.UserID,
			&conn.Name,
			&conn.Service,
			&conn.Status,
			&conn.AuthType,
			&metadataJSON,
			&lastUsedAt,
			&conn.CreatedAt,
			&conn.UpdatedAt,
		)
		if err != nil {
			h.logger.Error("Failed to scan connection row", "error", err)
			continue
		}

		// Parse metadata JSON
		conn.Metadata = make(map[string]interface{})
		if metadataJSON.Valid && metadataJSON.String != "" {
			if err := json.Unmarshal([]byte(metadataJSON.String), &conn.Metadata); err != nil {
				h.logger.Error("Failed to parse metadata JSON", "error", err)
			}
		}

		// Set last used time
		if lastUsedAt.Valid {
			conn.LastUsedAt = lastUsedAt.Time
		}

		connections = append(connections, conn)
	}

	c.JSON(http.StatusOK, gin.H{"connections": connections})
}

// GetConnection gets a connection by ID
func (h *ConnectionHandler) GetConnection(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get connection ID from path
	connectionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid connection ID"})
		return
	}

	// Query the connection
	var conn Connection
	var metadataJSON sql.NullString
	var lastUsedAt sql.NullTime

	err = h.db.DB().QueryRow(`
		SELECT id, user_id, name, service, status, auth_type, metadata, last_used_at, created_at, updated_at
		FROM connections
		WHERE id = ? AND user_id = ?
	`, connectionID, userID).Scan(
		&conn.ID,
		&conn.UserID,
		&conn.Name,
		&conn.Service,
		&conn.Status,
		&conn.AuthType,
		&metadataJSON,
		&lastUsedAt,
		&conn.CreatedAt,
		&conn.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Connection not found"})
			return
		}
		h.logger.Error("Failed to get connection", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve connection"})
		return
	}

	// Parse metadata JSON
	conn.Metadata = make(map[string]interface{})
	if metadataJSON.Valid && metadataJSON.String != "" {
		if err := json.Unmarshal([]byte(metadataJSON.String), &conn.Metadata); err != nil {
			h.logger.Error("Failed to parse metadata JSON", "error", err)
		}
	}

	// Set last used time
	if lastUsedAt.Valid {
		conn.LastUsedAt = lastUsedAt.Time
	}

	// Get connection details from the service provider
	provider, err := h.engine.GetServiceProvider(conn.Service)
	if err != nil {
		h.logger.Error("Failed to get service provider", "error", err, "service", conn.Service)
		c.JSON(http.StatusOK, conn)
		return
	}

	// Get the authData for the connection
	var authData map[string]interface{}
	err = h.db.DB().QueryRow(`
		SELECT auth_data FROM connections WHERE id = ?
	`, connectionID).Scan(&authData)
	if err != nil {
		h.logger.Error("Failed to get auth data", "error", err)
		c.JSON(http.StatusOK, conn)
		return
	}

	// Create a common.Connection from the connection data
	commonConn := &common.Connection{
		ID:       conn.ID,
		UserID:   conn.UserID,
		Name:     conn.Name,
		Service:  conn.Service,
		Status:   conn.Status,
		AuthType: conn.AuthType,
		AuthData: authData,
		Metadata: conn.Metadata,
	}

	// Get connection details
	details, err := provider.GetConnectionDetails(c.Request.Context(), commonConn)
	if err != nil {
		h.logger.Error("Failed to get connection details", "error", err)
		c.JSON(http.StatusOK, conn)
		return
	}

	// Add details to the connection
	conn.Metadata = details

	c.JSON(http.StatusOK, conn)
}

// CreateConnection creates a new connection
func (h *ConnectionHandler) CreateConnection(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse request
	var req ConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate service
	provider, err := h.engine.GetServiceProvider(req.Service)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unsupported service: %s", req.Service)})
		return
	}

	// Validate auth type
	if req.AuthType != "oauth" && req.AuthType != "api_key" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported auth type"})
		return
	}

	// For API key auth, validate auth data
	if req.AuthType == "api_key" && len(req.AuthData) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Auth data is required for API key authentication"})
		return
	}

	// Marshal auth data and metadata to JSON
	authDataJSON, err := json.Marshal(req.AuthData)
	if err != nil {
		h.logger.Error("Failed to marshal auth data", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process auth data"})
		return
	}

	var metadataJSON []byte
	if len(req.Metadata) > 0 {
		metadataJSON, err = json.Marshal(req.Metadata)
		if err != nil {
			h.logger.Error("Failed to marshal metadata", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process metadata"})
			return
		}
	}

	// Create connection record
	var connectionID int64
	err = h.db.DB().QueryRow(`
		INSERT INTO connections (user_id, name, service, status, auth_type, auth_data, metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`, userID, req.Name, req.Service, "pending", req.AuthType, authDataJSON, metadataJSON, time.Now(), time.Now()).Scan(&connectionID)
	if err != nil {
		h.logger.Error("Failed to create connection", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create connection"})
		return
	}

	// For OAuth authentication, we'll return the connection ID
	// The client will need to complete the OAuth flow
	if req.AuthType == "oauth" {
		c.JSON(http.StatusCreated, gin.H{
			"id":      connectionID,
			"message": "Connection created. Please complete the OAuth flow.",
		})
		return
	}

	// For API key authentication, validate the connection
	commonConn := &common.Connection{
		ID:       connectionID,
		UserID:   userID.(int64),
		Name:     req.Name,
		Service:  req.Service,
		Status:   "pending",
		AuthType: req.AuthType,
		AuthData: req.AuthData,
		Metadata: req.Metadata,
	}

	if err := provider.ValidateConnection(c.Request.Context(), commonConn); err != nil {
		h.logger.Error("Failed to validate connection", "error", err)

		// Update the connection status to failed
		_, updateErr := h.db.DB().Exec(`
			UPDATE connections SET status = ? WHERE id = ?
		`, "failed", connectionID)
		if updateErr != nil {
			h.logger.Error("Failed to update connection status", "error", updateErr)
		}

		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to validate connection: %v", err)})
		return
	}

	// Update the connection status to active
	_, err = h.db.DB().Exec(`
		UPDATE connections SET status = ? WHERE id = ?
	`, "active", connectionID)
	if err != nil {
		h.logger.Error("Failed to update connection status", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update connection status"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":      connectionID,
		"message": "Connection created and validated successfully.",
	})
}

// UpdateConnection updates an existing connection
func (h *ConnectionHandler) UpdateConnection(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get connection ID from path
	connectionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid connection ID"})
		return
	}

	// Parse request
	var req ConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if connection exists and belongs to the user
	var count int
	err = h.db.DB().QueryRow(`
		SELECT COUNT(*) FROM connections WHERE id = ? AND user_id = ?
	`, connectionID, userID).Scan(&count)
	if err != nil {
		h.logger.Error("Failed to check connection", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify connection"})
		return
	}

	if count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Connection not found"})
		return
	}

	// Validate service
	provider, err := h.engine.GetServiceProvider(req.Service)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unsupported service: %s", req.Service)})
		return
	}

	// Get existing auth data and metadata
	var existingAuthData, existingMetadata string
	err = h.db.DB().QueryRow(`
		SELECT auth_data, metadata FROM connections WHERE id = ?
	`, connectionID).Scan(&existingAuthData, &existingMetadata)
	if err != nil {
		h.logger.Error("Failed to get existing connection data", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve connection data"})
		return
	}

	// Parse existing auth data and metadata
	var authData map[string]interface{}
	if err := json.Unmarshal([]byte(existingAuthData), &authData); err != nil {
		h.logger.Error("Failed to parse auth data", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process connection data"})
		return
	}

	var metadata map[string]interface{}
	if existingMetadata != "" {
		if err := json.Unmarshal([]byte(existingMetadata), &metadata); err != nil {
			h.logger.Error("Failed to parse metadata", "error", err)
			metadata = make(map[string]interface{})
		}
	} else {
		metadata = make(map[string]interface{})
	}

	// Update auth data if provided
	if len(req.AuthData) > 0 {
		for k, v := range req.AuthData {
			authData[k] = v
		}
	}

	// Update metadata if provided
	if len(req.Metadata) > 0 {
		for k, v := range req.Metadata {
			metadata[k] = v
		}
	}

	// Marshal updated auth data and metadata
	authDataJSON, err := json.Marshal(authData)
	if err != nil {
		h.logger.Error("Failed to marshal auth data", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process auth data"})
		return
	}

	var metadataJSON []byte
	if len(metadata) > 0 {
		metadataJSON, err = json.Marshal(metadata)
		if err != nil {
			h.logger.Error("Failed to marshal metadata", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process metadata"})
			return
		}
	}

	// Initialize status as "pending"
	status := "pending"

	// For API key authentication, validate the connection
	if req.AuthType == "api_key" {
		commonConn := &common.Connection{
			ID:       connectionID,
			UserID:   userID.(int64),
			Name:     req.Name,
			Service:  req.Service,
			Status:   status,
			AuthType: req.AuthType,
			AuthData: authData,
			Metadata: metadata,
		}

		if err := provider.ValidateConnection(c.Request.Context(), commonConn); err != nil {
			h.logger.Error("Failed to validate connection", "error", err)
			status = "failed"
		} else {
			status = "active"
		}
	}

	// Update the connection
	_, err = h.db.DB().Exec(`
		UPDATE connections
		SET name = ?, service = ?, status = ?, auth_type = ?, auth_data = ?, metadata = ?, updated_at = ?
		WHERE id = ?
	`, req.Name, req.Service, status, req.AuthType, authDataJSON, metadataJSON, time.Now(), connectionID)
	if err != nil {
		h.logger.Error("Failed to update connection", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update connection"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Connection updated successfully.",
		"status":  status,
	})
}

// DeleteConnection deletes a connection
func (h *ConnectionHandler) DeleteConnection(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get connection ID from path
	connectionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid connection ID"})
		return
	}

	// Check if the connection exists and belongs to the user
	var count int
	err = h.db.DB().QueryRow(`
		SELECT COUNT(*) FROM connections WHERE id = ? AND user_id = ?
	`, connectionID, userID).Scan(&count)
	if err != nil {
		h.logger.Error("Failed to check connection", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify connection"})
		return
	}

	if count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Connection not found"})
		return
	}

	// Check if any active workflows are using this connection
	var workflowCount int
	err = h.db.DB().QueryRow(`
		SELECT COUNT(*) FROM workflows
		WHERE user_id = ? AND status = 'active' AND
		(trigger_service IN (SELECT service FROM connections WHERE id = ?) OR
		 id IN (SELECT workflow_id FROM workflow_actions WHERE action_service IN (SELECT service FROM connections WHERE id = ?)))
	`, userID, connectionID, connectionID).Scan(&workflowCount)
	if err != nil {
		h.logger.Error("Failed to check workflows using connection", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify usage"})
		return
	}

	if workflowCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete connection - it is used by active workflows"})
		return
	}

	// Delete the connection
	_, err = h.db.DB().Exec(`DELETE FROM connections WHERE id = ?`, connectionID)
	if err != nil {
		h.logger.Error("Failed to delete connection", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete connection"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Connection deleted successfully"})
}

// TestConnection tests a connection
func (h *ConnectionHandler) TestConnection(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get connection ID from path
	connectionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid connection ID"})
		return
	}

	// Get the connection
	var service, authType string
	var authDataJSON string
	err = h.db.DB().QueryRow(`
		SELECT service, auth_type, auth_data
		FROM connections
		WHERE id = ? AND user_id = ?
	`, connectionID, userID).Scan(&service, &authType, &authDataJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Connection not found"})
			return
		}
		h.logger.Error("Failed to get connection", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve connection"})
		return
	}

	// Parse auth data
	var authData map[string]interface{}
	if err := json.Unmarshal([]byte(authDataJSON), &authData); err != nil {
		h.logger.Error("Failed to parse auth data", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process connection data"})
		return
	}

	// Get the service provider
	provider, err := h.engine.GetServiceProvider(service)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unsupported service: %s", service)})
		return
	}

	// Create common.Connection
	commonConn := &common.Connection{
		ID:       connectionID,
		UserID:   userID.(int64),
		Service:  service,
		AuthType: authType,
		AuthData: authData,
	}

	// Validate the connection
	err = provider.ValidateConnection(c.Request.Context(), commonConn)
	if err != nil {
		h.logger.Error("Connection validation failed", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"valid":   false,
			"message": fmt.Sprintf("Connection failed: %v", err),
		})
		return
	}

	// Update the connection status
	_, err = h.db.DB().Exec(`
		UPDATE connections
		SET status = 'active', last_used_at = ?, updated_at = ?
		WHERE id = ?
	`, time.Now(), time.Now(), connectionID)
	if err != nil {
		h.logger.Error("Failed to update connection status", "error", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":   true,
		"message": "Connection is valid",
	})
}

// GetOAuthURL returns the OAuth authorization URL for a service
func (h *ConnectionHandler) GetOAuthURL(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get service from path
	service := c.Param("service")

	// Get the service provider
	provider, err := h.engine.GetServiceProvider(service)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unsupported service: %s", service)})
		return
	}

	// Get auth handler
	authHandler := provider.GetAuthHandler()

	// Generate a random state token
	state, err := generateRandomString(32)
	if err != nil {
		h.logger.Error("Failed to generate state token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare OAuth flow"})
		return
	}

	// Store the state token with the user ID
	h.states[state] = fmt.Sprintf("%d", userID)

	// Get the OAuth URL
	url := authHandler.GetAuthURL(state)

	c.JSON(http.StatusOK, gin.H{
		"url":   url,
		"state": state,
	})
}

// OAuthCallback handles the OAuth callback
func (h *ConnectionHandler) OAuthCallback(c *gin.Context) {
	// Get service from path
	service := c.Param("service")

	// Get code and state from query parameters
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing code or state parameter"})
		return
	}

	// Verify state token
	userIDStr, exists := h.states[state]
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid state token"})
		return
	}

	// Clean up the state token
	delete(h.states, state)

	// Convert user ID to int64
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		h.logger.Error("Failed to parse user ID", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process OAuth callback"})
		return
	}

	// Get the service provider
	provider, err := h.engine.GetServiceProvider(service)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unsupported service: %s", service)})
		return
	}

	// Get auth handler
	authHandler := provider.GetAuthHandler()

	// Handle the callback
	authData, err := authHandler.HandleCallback(c.Request.Context(), code)
	if err != nil {
		h.logger.Error("Failed to handle OAuth callback", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to complete OAuth flow: %v", err)})
		return
	}

	// Find the pending connection for this service and user
	var connectionID int64
	err = h.db.DB().QueryRow(`
		SELECT id FROM connections
		WHERE user_id = ? AND service = ? AND status = 'pending' AND auth_type = 'oauth'
		ORDER BY created_at DESC
		LIMIT 1
	`, userID, service).Scan(&connectionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "No pending connection found"})
			return
		}
		h.logger.Error("Failed to find pending connection", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve connection"})
		return
	}

	// Marshal auth data to JSON
	authDataJSON, err := json.Marshal(authData)
	if err != nil {
		h.logger.Error("Failed to marshal auth data", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process auth data"})
		return
	}

	// Update the connection with the auth data
	_, err = h.db.DB().Exec(`
		UPDATE connections
		SET auth_data = ?, status = 'active', last_used_at = ?, updated_at = ?
		WHERE id = ?
	`, authDataJSON, time.Now(), time.Now(), connectionID)
	if err != nil {
		h.logger.Error("Failed to update connection", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update connection"})
		return
	}

	// Redirect to a success page or return success response
	c.JSON(http.StatusOK, gin.H{
		"message": "OAuth flow completed successfully",
		"id":      connectionID,
	})
}

// ListIntegrations lists all available integration services
func (h *ConnectionHandler) ListIntegrations(c *gin.Context) {
	// List all registered service providers
	var services []string
	for _, provider := range h.engine.GetAllServiceProviders() {
		services = append(services, provider.GetService())
	}

	c.JSON(http.StatusOK, gin.H{"services": services})
}

// GetIntegrationDetails returns details about an integration service
func (h *ConnectionHandler) GetIntegrationDetails(c *gin.Context) {
	// Get service from path
	service := c.Param("service")

	// Get the service provider
	provider, err := h.engine.GetServiceProvider(service)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unsupported service: %s", service)})
		return
	}

	// Get trigger and action types
	triggerHandler := provider.GetTriggerHandler()
	actionHandler := provider.GetActionHandler()

	triggers, err := triggerHandler.GetTriggerTypes()
	if err != nil {
		h.logger.Error("Failed to get trigger types", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve triggers"})
		return
	}

	actions, err := actionHandler.GetActionTypes()
	if err != nil {
		h.logger.Error("Failed to get action types", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve actions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"service":  service,
		"triggers": triggers,
		"actions":  actions,
	})
}

// ListTriggers lists all triggers for a service
func (h *ConnectionHandler) ListTriggers(c *gin.Context) {
	// Get service from path
	service := c.Param("service")

	// Get the service provider
	provider, err := h.engine.GetServiceProvider(service)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unsupported service: %s", service)})
		return
	}

	// Get trigger types
	triggerHandler := provider.GetTriggerHandler()
	triggers, err := triggerHandler.GetTriggerTypes()
	if err != nil {
		h.logger.Error("Failed to get trigger types", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve triggers"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"triggers": triggers})
}

// ListActions lists all actions for a service
func (h *ConnectionHandler) ListActions(c *gin.Context) {
	// Get service from path
	service := c.Param("service")

	// Get the service provider
	provider, err := h.engine.GetServiceProvider(service)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unsupported service: %s", service)})
		return
	}

	// Get action types
	actionHandler := provider.GetActionHandler()
	actions, err := actionHandler.GetActionTypes()
	if err != nil {
		h.logger.Error("Failed to get action types", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve actions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"actions": actions})
}

// generateRandomString generates a random string of the specified length
func generateRandomString(length int) (string, error) {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b)[:length], nil
}
