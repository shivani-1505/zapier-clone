package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/auditcue/integration-framework/internal/db"
	"github.com/auditcue/integration-framework/internal/integrations/common"
	"github.com/auditcue/integration-framework/internal/queue"
	"github.com/auditcue/integration-framework/internal/workflow"
	"github.com/auditcue/integration-framework/pkg/logger"
	"github.com/gin-gonic/gin"
)

// WorkflowHandler handles workflow-related API endpoints
type WorkflowHandler struct {
	db       *db.Database
	engine   *workflow.Engine
	jobQueue *queue.Queue
	logger   *logger.Logger
}

// NewWorkflowHandler creates a new workflow handler
func NewWorkflowHandler(db *db.Database, engine *workflow.Engine, jobQueue *queue.Queue, logger *logger.Logger) *WorkflowHandler {
	return &WorkflowHandler{
		db:       db,
		engine:   engine,
		jobQueue: jobQueue,
		logger:   logger,
	}
}

// WorkflowResponse represents a workflow in API responses
type WorkflowResponse struct {
	ID             int64                  `json:"id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	Status         string                 `json:"status"`
	TriggerService string                 `json:"trigger_service"`
	TriggerID      string                 `json:"trigger_id"`
	TriggerConfig  map[string]interface{} `json:"trigger_config"`
	Actions        []ActionResponse       `json:"actions"`
	DataMappings   []DataMappingResponse  `json:"data_mappings"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// ActionResponse represents a workflow action in API responses
type ActionResponse struct {
	ID            int64                  `json:"id"`
	ActionService string                 `json:"action_service"`
	ActionID      string                 `json:"action_id"`
	ActionConfig  map[string]interface{} `json:"action_config"`
	Position      int                    `json:"position"`
}

// DataMappingResponse represents a data mapping in API responses
type DataMappingResponse struct {
	ID            int64  `json:"id"`
	SourceService string `json:"source_service"`
	SourceField   string `json:"source_field"`
	TargetService string `json:"target_service"`
	TargetField   string `json:"target_field"`
	Transformer   string `json:"transformer,omitempty"`
}

// CreateWorkflowRequest represents a request to create a workflow
type CreateWorkflowRequest struct {
	Name           string                 `json:"name" binding:"required"`
	Description    string                 `json:"description"`
	TriggerService string                 `json:"trigger_service" binding:"required"`
	TriggerID      string                 `json:"trigger_id" binding:"required"`
	TriggerConfig  map[string]interface{} `json:"trigger_config" binding:"required"`
}

// UpdateWorkflowRequest represents a request to update a workflow
type UpdateWorkflowRequest struct {
	Name           string                 `json:"name,omitempty"`
	Description    string                 `json:"description,omitempty"`
	TriggerService string                 `json:"trigger_service,omitempty"`
	TriggerID      string                 `json:"trigger_id,omitempty"`
	TriggerConfig  map[string]interface{} `json:"trigger_config,omitempty"`
}

// AddActionRequest represents a request to add an action to a workflow
type AddActionRequest struct {
	ActionService string                 `json:"action_service" binding:"required"`
	ActionID      string                 `json:"action_id" binding:"required"`
	ActionConfig  map[string]interface{} `json:"action_config" binding:"required"`
	Position      int                    `json:"position"`
}

// UpdateActionRequest represents a request to update a workflow action
type UpdateActionRequest struct {
	ActionService string                 `json:"action_service,omitempty"`
	ActionID      string                 `json:"action_id,omitempty"`
	ActionConfig  map[string]interface{} `json:"action_config,omitempty"`
	Position      int                    `json:"position,omitempty"`
}

// ReorderActionsRequest represents a request to reorder actions in a workflow
type ReorderActionsRequest struct {
	ActionIDs []int64 `json:"action_ids" binding:"required"`
}

// AddDataMappingRequest represents a request to add a data mapping to a workflow
type AddDataMappingRequest struct {
	SourceService string `json:"source_service" binding:"required"`
	SourceField   string `json:"source_field" binding:"required"`
	TargetService string `json:"target_service" binding:"required"`
	TargetField   string `json:"target_field" binding:"required"`
	Transformer   string `json:"transformer,omitempty"`
}

// UpdateDataMappingRequest represents a request to update a data mapping
type UpdateDataMappingRequest struct {
	SourceService string `json:"source_service,omitempty"`
	SourceField   string `json:"source_field,omitempty"`
	TargetService string `json:"target_service,omitempty"`
	TargetField   string `json:"target_field,omitempty"`
	Transformer   string `json:"transformer,omitempty"`
}

// TestWorkflowRequest represents a request to test a workflow
type TestWorkflowRequest struct {
	TriggerData map[string]interface{} `json:"trigger_data" binding:"required"`
}

// ListWorkflows lists all workflows for the authenticated user
func (h *WorkflowHandler) ListWorkflows(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Query workflows
	rows, err := h.db.DB().Query(`
		SELECT id, name, description, status, trigger_service, trigger_id, trigger_config, created_at, updated_at
		FROM workflows
		WHERE user_id = ?
		ORDER BY updated_at DESC
	`, userID)
	if err != nil {
		h.logger.Error("Failed to query workflows", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve workflows"})
		return
	}
	defer rows.Close()

	// Iterate through rows
	var workflows []WorkflowResponse
	for rows.Next() {
		var workflow WorkflowResponse
		var triggerConfigJSON []byte

		err := rows.Scan(
			&workflow.ID,
			&workflow.Name,
			&workflow.Description,
			&workflow.Status,
			&workflow.TriggerService,
			&workflow.TriggerID,
			&triggerConfigJSON,
			&workflow.CreatedAt,
			&workflow.UpdatedAt,
		)
		if err != nil {
			h.logger.Error("Failed to scan workflow row", "error", err)
			continue
		}

		// Parse trigger config JSON
		if err := json.Unmarshal(triggerConfigJSON, &workflow.TriggerConfig); err != nil {
			h.logger.Error("Failed to parse trigger config", "error", err)
			workflow.TriggerConfig = make(map[string]interface{})
		}

		// Get actions for this workflow
		actionRows, err := h.db.DB().Query(`
			SELECT id, action_service, action_id, action_config, position
			FROM workflow_actions
			WHERE workflow_id = ?
			ORDER BY position
		`, workflow.ID)
		if err != nil {
			h.logger.Error("Failed to query workflow actions", "error", err)
			continue
		}

		workflow.Actions = []ActionResponse{}
		for actionRows.Next() {
			var action ActionResponse
			var actionConfigJSON []byte

			err := actionRows.Scan(
				&action.ID,
				&action.ActionService,
				&action.ActionID,
				&actionConfigJSON,
				&action.Position,
			)
			if err != nil {
				h.logger.Error("Failed to scan action row", "error", err)
				continue
			}

			// Parse action config JSON
			if err := json.Unmarshal(actionConfigJSON, &action.ActionConfig); err != nil {
				h.logger.Error("Failed to parse action config", "error", err)
				action.ActionConfig = make(map[string]interface{})
			}

			workflow.Actions = append(workflow.Actions, action)
		}
		actionRows.Close()

		// Get data mappings for this workflow
		mappingRows, err := h.db.DB().Query(`
			SELECT id, source_service, source_field, target_service, target_field, transformer
			FROM workflow_data_mappings
			WHERE workflow_id = ?
		`, workflow.ID)
		if err != nil {
			h.logger.Error("Failed to query data mappings", "error", err)
			continue
		}

		workflow.DataMappings = []DataMappingResponse{}
		for mappingRows.Next() {
			var mapping DataMappingResponse
			var transformer sql.NullString

			err := mappingRows.Scan(
				&mapping.ID,
				&mapping.SourceService,
				&mapping.SourceField,
				&mapping.TargetService,
				&mapping.TargetField,
				&transformer,
			)
			if err != nil {
				h.logger.Error("Failed to scan mapping row", "error", err)
				continue
			}

			if transformer.Valid {
				mapping.Transformer = transformer.String
			}

			workflow.DataMappings = append(workflow.DataMappings, mapping)
		}
		mappingRows.Close()

		workflows = append(workflows, workflow)
	}

	c.JSON(http.StatusOK, gin.H{"workflows": workflows})
}

// GetWorkflow gets a workflow by ID
func (h *WorkflowHandler) GetWorkflow(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get workflow ID from path
	workflowID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid workflow ID"})
		return
	}

	// Query the workflow
	var workflow WorkflowResponse
	var triggerConfigJSON []byte

	err = h.db.DB().QueryRow(`
		SELECT id, name, description, status, trigger_service, trigger_id, trigger_config, created_at, updated_at
		FROM workflows
		WHERE id = ? AND user_id = ?
	`, workflowID, userID).Scan(
		&workflow.ID,
		&workflow.Name,
		&workflow.Description,
		&workflow.Status,
		&workflow.TriggerService,
		&workflow.TriggerID,
		&triggerConfigJSON,
		&workflow.CreatedAt,
		&workflow.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to get workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve workflow"})
		return
	}

	// Parse trigger config JSON
	if err := json.Unmarshal(triggerConfigJSON, &workflow.TriggerConfig); err != nil {
		h.logger.Error("Failed to parse trigger config", "error", err)
		workflow.TriggerConfig = make(map[string]interface{})
	}

	// Get actions for this workflow
	actionRows, err := h.db.DB().Query(`
		SELECT id, action_service, action_id, action_config, position
		FROM workflow_actions
		WHERE workflow_id = ?
		ORDER BY position
	`, workflow.ID)
	if err != nil {
		h.logger.Error("Failed to query workflow actions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve workflow actions"})
		return
	}
	defer actionRows.Close()

	workflow.Actions = []ActionResponse{}
	for actionRows.Next() {
		var action ActionResponse
		var actionConfigJSON []byte

		err := actionRows.Scan(
			&action.ID,
			&action.ActionService,
			&action.ActionID,
			&actionConfigJSON,
			&action.Position,
		)
		if err != nil {
			h.logger.Error("Failed to scan action row", "error", err)
			continue
		}

		// Parse action config JSON
		if err := json.Unmarshal(actionConfigJSON, &action.ActionConfig); err != nil {
			h.logger.Error("Failed to parse action config", "error", err)
			action.ActionConfig = make(map[string]interface{})
		}

		workflow.Actions = append(workflow.Actions, action)
	}

	// Get data mappings for this workflow
	mappingRows, err := h.db.DB().Query(`
		SELECT id, source_service, source_field, target_service, target_field, transformer
		FROM workflow_data_mappings
		WHERE workflow_id = ?
	`, workflow.ID)
	if err != nil {
		h.logger.Error("Failed to query data mappings", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve data mappings"})
		return
	}
	defer mappingRows.Close()

	workflow.DataMappings = []DataMappingResponse{}
	for mappingRows.Next() {
		var mapping DataMappingResponse
		var transformer sql.NullString

		err := mappingRows.Scan(
			&mapping.ID,
			&mapping.SourceService,
			&mapping.SourceField,
			&mapping.TargetService,
			&mapping.TargetField,
			&transformer,
		)
		if err != nil {
			h.logger.Error("Failed to scan mapping row", "error", err)
			continue
		}

		if transformer.Valid {
			mapping.Transformer = transformer.String
		}

		workflow.DataMappings = append(workflow.DataMappings, mapping)
	}

	c.JSON(http.StatusOK, workflow)
}

// CreateWorkflow creates a new workflow
func (h *WorkflowHandler) CreateWorkflow(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse request
	var req CreateWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate trigger service and ID
	provider, err := h.engine.GetServiceProvider(req.TriggerService)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unsupported trigger service: %s", req.TriggerService)})
		return
	}

	triggerHandler := provider.GetTriggerHandler()
	triggerTypes, err := triggerHandler.GetTriggerTypes()
	if err != nil {
		h.logger.Error("Failed to get trigger types", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate trigger"})
		return
	}

	// Check if trigger ID is valid
	triggerValid := false
	for _, t := range triggerTypes {
		if t.ID == req.TriggerID {
			triggerValid = true
			break
		}
	}

	if !triggerValid {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unsupported trigger type: %s", req.TriggerID)})
		return
	}

	// Validate trigger config
	triggerConfigJSON, err := json.Marshal(req.TriggerConfig)
	if err != nil {
		h.logger.Error("Failed to marshal trigger config", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process trigger configuration"})
		return
	}

	if err := triggerHandler.ValidateTriggerConfig(req.TriggerID, triggerConfigJSON); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid trigger configuration: %v", err)})
		return
	}

	// Create workflow record
	var workflowID int64
	err = h.db.DB().QueryRow(`
		INSERT INTO workflows (user_id, name, description, status, trigger_service, trigger_id, trigger_config, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`, userID, req.Name, req.Description, "inactive", req.TriggerService, req.TriggerID, triggerConfigJSON, time.Now(), time.Now()).Scan(&workflowID)
	if err != nil {
		h.logger.Error("Failed to create workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create workflow"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":      workflowID,
		"message": "Workflow created successfully",
	})
}

// UpdateWorkflow updates an existing workflow
func (h *WorkflowHandler) UpdateWorkflow(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get workflow ID from path
	workflowID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid workflow ID"})
		return
	}

	// Check if workflow exists and belongs to the user
	var workflowStatus string
	err = h.db.DB().QueryRow(`
		SELECT status FROM workflows WHERE id = ? AND user_id = ?
	`, workflowID, userID).Scan(&workflowStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to check workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify workflow"})
		return
	}

	// Check if workflow is active
	if workflowStatus == "active" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot update an active workflow. Deactivate it first."})
		return
	}

	// Parse request
	var req UpdateWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get current workflow data
	var currentTriggerService, currentTriggerID string
	var currentTriggerConfigJSON []byte
	if req.TriggerService != "" || req.TriggerID != "" || req.TriggerConfig != nil {
		err = h.db.DB().QueryRow(`
			SELECT trigger_service, trigger_id, trigger_config FROM workflows WHERE id = ?
		`, workflowID).Scan(&currentTriggerService, &currentTriggerID, &currentTriggerConfigJSON)
		if err != nil {
			h.logger.Error("Failed to get current workflow data", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve workflow data"})
			return
		}
	}

	// Validate trigger service and ID
	triggerService := currentTriggerService
	if req.TriggerService != "" {
		triggerService = req.TriggerService
	}

	triggerID := currentTriggerID
	if req.TriggerID != "" {
		triggerID = req.TriggerID
	}

	// Validate trigger service
	provider, err := h.engine.GetServiceProvider(triggerService)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unsupported trigger service: %s", triggerService)})
		return
	}

	triggerHandler := provider.GetTriggerHandler()
	triggerTypes, err := triggerHandler.GetTriggerTypes()
	if err != nil {
		h.logger.Error("Failed to get trigger types", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate trigger"})
		return
	}

	// Check if trigger ID is valid
	triggerValid := false
	for _, t := range triggerTypes {
		if t.ID == triggerID {
			triggerValid = true
			break
		}
	}

	if !triggerValid {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unsupported trigger type: %s", triggerID)})
		return
	}

	// Prepare trigger config
	var triggerConfigJSON []byte
	if req.TriggerConfig != nil {
		triggerConfigJSON, err = json.Marshal(req.TriggerConfig)
		if err != nil {
			h.logger.Error("Failed to marshal trigger config", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process trigger configuration"})
			return
		}
	} else {
		triggerConfigJSON = currentTriggerConfigJSON
	}

	// Validate trigger config
	if err := triggerHandler.ValidateTriggerConfig(triggerID, triggerConfigJSON); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid trigger configuration: %v", err)})
		return
	}

	// Build the update query
	query := "UPDATE workflows SET updated_at = ?"
	args := []interface{}{time.Now()}

	if req.Name != "" {
		query += ", name = ?"
		args = append(args, req.Name)
	}

	if req.Description != "" {
		query += ", description = ?"
		args = append(args, req.Description)
	}

	if req.TriggerService != "" {
		query += ", trigger_service = ?"
		args = append(args, req.TriggerService)
	}

	if req.TriggerID != "" {
		query += ", trigger_id = ?"
		args = append(args, req.TriggerID)
	}

	if req.TriggerConfig != nil {
		query += ", trigger_config = ?"
		args = append(args, triggerConfigJSON)
	}

	query += " WHERE id = ?"
	args = append(args, workflowID)

	// Execute the update
	_, err = h.db.DB().Exec(query, args...)
	if err != nil {
		h.logger.Error("Failed to update workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update workflow"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Workflow updated successfully",
	})
}

// DeleteWorkflow deletes a workflow
func (h *WorkflowHandler) DeleteWorkflow(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get workflow ID from path
	workflowID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid workflow ID"})
		return
	}

	// Check if workflow exists and belongs to the user
	var count int
	err = h.db.DB().QueryRow(`
		SELECT COUNT(*) FROM workflows WHERE id = ? AND user_id = ?
	`, workflowID, userID).Scan(&count)
	if err != nil {
		h.logger.Error("Failed to check workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify workflow"})
		return
	}

	if count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
		return
	}

	// Begin a transaction
	tx, err := h.db.DB().Begin()
	if err != nil {
		h.logger.Error("Failed to begin transaction", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete workflow"})
		return
	}
	defer tx.Rollback()

	// Delete workflow actions
	_, err = tx.Exec("DELETE FROM workflow_actions WHERE workflow_id = ?", workflowID)
	if err != nil {
		h.logger.Error("Failed to delete workflow actions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete workflow"})
		return
	}

	// Delete data mappings
	_, err = tx.Exec("DELETE FROM workflow_data_mappings WHERE workflow_id = ?", workflowID)
	if err != nil {
		h.logger.Error("Failed to delete data mappings", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete workflow"})
		return
	}

	// Delete webhook triggers
	_, err = tx.Exec("DELETE FROM webhook_triggers WHERE workflow_id = ?", workflowID)
	if err != nil {
		h.logger.Error("Failed to delete webhook triggers", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete workflow"})
		return
	}

	// Delete scheduled triggers
	_, err = tx.Exec("DELETE FROM scheduled_triggers WHERE workflow_id = ?", workflowID)
	if err != nil {
		h.logger.Error("Failed to delete scheduled triggers", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete workflow"})
		return
	}

	// Delete workflow executions and action executions
	rows, err := tx.Query("SELECT id FROM workflow_executions WHERE workflow_id = ?", workflowID)
	if err != nil {
		h.logger.Error("Failed to query workflow executions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete workflow"})
		return
	}

	var executionIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			h.logger.Error("Failed to scan execution ID", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete workflow"})
			return
		}
		executionIDs = append(executionIDs, id)
	}
	rows.Close()

	// Delete action executions for each workflow execution
	for _, execID := range executionIDs {
		_, err = tx.Exec("DELETE FROM workflow_action_executions WHERE workflow_execution_id = ?", execID)
		if err != nil {
			h.logger.Error("Failed to delete action executions", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete workflow"})
			return
		}
	}

	// Delete workflow executions
	_, err = tx.Exec("DELETE FROM workflow_executions WHERE workflow_id = ?", workflowID)
	if err != nil {
		h.logger.Error("Failed to delete workflow executions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete workflow"})
		return
	}

	// Delete the workflow
	_, err = tx.Exec("DELETE FROM workflows WHERE id = ?", workflowID)
	if err != nil {
		h.logger.Error("Failed to delete workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete workflow"})
		return
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		h.logger.Error("Failed to commit transaction", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete workflow"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Workflow deleted successfully",
	})
}

// ActivateWorkflow activates a workflow
func (h *WorkflowHandler) ActivateWorkflow(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get workflow ID from path
	workflowID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid workflow ID"})
		return
	}

	// Get the workflow
	w, err := h.engine.GetWorkflow(c.Request.Context(), workflowID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Workflow not found: %v", err)})
		return
	}

	// Check if the workflow belongs to the user
	if w.UserID != userID.(int64) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Check if the workflow has at least one action
	if len(w.Actions) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Workflow must have at least one action"})
		return
	}

	// Validate the workflow
	if err := h.engine.ValidateWorkflow(c.Request.Context(), w); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid workflow: %v", err)})
		return
	}

	// Update workflow status
	_, err = h.db.DB().Exec(`
		UPDATE workflows SET status = 'active', updated_at = ? WHERE id = ?
	`, time.Now(), workflowID)
	if err != nil {
		h.logger.Error("Failed to activate workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to activate workflow"})
		return
	}

	// Handle trigger registration based on trigger type
	triggerConfigJSON, err := json.Marshal(w.TriggerConfig)
	if err != nil {
		h.logger.Error("Failed to marshal trigger config", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process trigger configuration"})
		return
	}

	var triggerConfig map[string]interface{}
	if err := json.Unmarshal(triggerConfigJSON, &triggerConfig); err != nil {
		h.logger.Error("Failed to unmarshal trigger config", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process trigger configuration"})
		return
	}

	// If it's a scheduled trigger, register it in the scheduled_triggers table
	if w.TriggerID == "scheduled_jql_search" || strings.HasPrefix(w.TriggerID, "scheduled_") {
		schedule, ok := triggerConfig["schedule"].(string)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Schedule is required for scheduled triggers"})
			return
		}

		// Calculate next trigger time
		nextTrigger := calculateNextTriggerTime(schedule)

		// Insert or update scheduled trigger
		_, err = h.db.DB().Exec(`
			INSERT INTO scheduled_triggers (workflow_id, schedule, next_trigger_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (workflow_id) DO UPDATE SET
				schedule = excluded.schedule,
				next_trigger_at = excluded.next_trigger_at,
				updated_at = excluded.updated_at
		`, workflowID, schedule, nextTrigger, time.Now(), time.Now())
		if err != nil {
			h.logger.Error("Failed to register scheduled trigger", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register trigger"})
			return
		}
	}

	// If it's a webhook trigger, register it in the webhook_triggers table
	if strings.HasPrefix(w.TriggerID, "webhook_") || strings.Contains(w.TriggerID, "created") || strings.Contains(w.TriggerID, "updated") || strings.Contains(w.TriggerID, "changed") || strings.Contains(w.TriggerID, "commented") {
		// Generate webhook token
		webhookToken, err := generateRandomString(32)
		if err != nil {
			h.logger.Error("Failed to generate webhook token", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register webhook"})
			return
		}

		// Generate webhook secret
		webhookSecret, err := generateRandomString(32)
		if err != nil {
			h.logger.Error("Failed to generate webhook secret", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register webhook"})
			return
		}

		// Build webhook URL
		webhookURL := fmt.Sprintf("/api/v1/webhooks/%s", webhookToken)

		// Insert or update webhook trigger
		_, err = h.db.DB().Exec(`
			INSERT INTO webhook_triggers (workflow_id, webhook_url, secret, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (workflow_id) DO UPDATE SET
				webhook_url = excluded.webhook_url,
				secret = excluded.secret,
				updated_at = excluded.updated_at
		`, workflowID, webhookURL, webhookSecret, time.Now(), time.Now())
		if err != nil {
			h.logger.Error("Failed to register webhook trigger", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register webhook"})
			return
		}

		// Return webhook URL in the response
		c.JSON(http.StatusOK, gin.H{
			"message":        "Workflow activated successfully",
			"webhook_url":    webhookURL,
			"webhook_secret": webhookSecret,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Workflow activated successfully",
	})
}

// DeactivateWorkflow deactivates a workflow
func (h *WorkflowHandler) DeactivateWorkflow(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get workflow ID from path
	workflowID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid workflow ID"})
		return
	}

	// Check if workflow exists and belongs to the user
	var count int
	err = h.db.DB().QueryRow(`
		SELECT COUNT(*) FROM workflows WHERE id = ? AND user_id = ?
	`, workflowID, userID).Scan(&count)
	if err != nil {
		h.logger.Error("Failed to check workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify workflow"})
		return
	}

	if count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
		return
	}

	// Update workflow status
	_, err = h.db.DB().Exec(`
		UPDATE workflows SET status = 'inactive', updated_at = ? WHERE id = ?
	`, time.Now(), workflowID)
	if err != nil {
		h.logger.Error("Failed to deactivate workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deactivate workflow"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Workflow deactivated successfully",
	})
}

// AddAction adds an action to a workflow
func (h *WorkflowHandler) AddAction(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get workflow ID from path
	workflowID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid workflow ID"})
		return
	}

	// Check if workflow exists and belongs to the user
	var workflowStatus string
	err = h.db.DB().QueryRow(`
		SELECT status FROM workflows WHERE id = ? AND user_id = ?
	`, workflowID, userID).Scan(&workflowStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to check workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify workflow"})
		return
	}

	// Check if workflow is active
	if workflowStatus == "active" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot modify an active workflow. Deactivate it first."})
		return
	}

	// Parse request
	var req AddActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate action service and ID
	provider, err := h.engine.GetServiceProvider(req.ActionService)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unsupported action service: %s", req.ActionService)})
		return
	}

	actionHandler := provider.GetActionHandler()
	actionTypes, err := actionHandler.GetActionTypes()
	if err != nil {
		h.logger.Error("Failed to get action types", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate action"})
		return
	}

	// Check if action ID is valid
	actionValid := false
	for _, a := range actionTypes {
		if a.ID == req.ActionID {
			actionValid = true
			break
		}
	}

	if !actionValid {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unsupported action type: %s", req.ActionID)})
		return
	}

	// Validate action config
	actionConfigJSON, err := json.Marshal(req.ActionConfig)
	if err != nil {
		h.logger.Error("Failed to marshal action config", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process action configuration"})
		return
	}

	if err := actionHandler.ValidateActionConfig(req.ActionID, actionConfigJSON); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid action configuration: %v", err)})
		return
	}

	// Determine position
	position := req.Position
	if position <= 0 {
		// Get the highest position
		var maxPosition sql.NullInt64
		err = h.db.DB().QueryRow(`
			SELECT MAX(position) FROM workflow_actions WHERE workflow_id = ?
		`, workflowID).Scan(&maxPosition)
		if err != nil {
			h.logger.Error("Failed to get max position", "error", err)
			position = 1
		} else if maxPosition.Valid {
			position = int(maxPosition.Int64) + 1
		} else {
			position = 1
		}
	} else {
		// Shift all actions with position >= requested position
		_, err = h.db.DB().Exec(`
			UPDATE workflow_actions
			SET position = position + 1
			WHERE workflow_id = ? AND position >= ?
		`, workflowID, position)
		if err != nil {
			h.logger.Error("Failed to shift action positions", "error", err)
		}
	}

	// Create action record
	var actionID int64
	err = h.db.DB().QueryRow(`
		INSERT INTO workflow_actions (workflow_id, action_service, action_id, action_config, position, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`, workflowID, req.ActionService, req.ActionID, actionConfigJSON, position, time.Now(), time.Now()).Scan(&actionID)
	if err != nil {
		h.logger.Error("Failed to create action", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add action"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":       actionID,
		"position": position,
		"message":  "Action added successfully",
	})
}

// UpdateAction updates an action in a workflow
func (h *WorkflowHandler) UpdateAction(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get workflow ID from path
	workflowID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid workflow ID"})
		return
	}

	// Get action ID from path
	actionID, err := strconv.ParseInt(c.Param("actionId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action ID"})
		return
	}

	// Check if workflow exists and belongs to the user
	var workflowStatus string
	err = h.db.DB().QueryRow(`
		SELECT status FROM workflows WHERE id = ? AND user_id = ?
	`, workflowID, userID).Scan(&workflowStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to check workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify workflow"})
		return
	}

	// Check if workflow is active
	if workflowStatus == "active" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot modify an active workflow. Deactivate it first."})
		return
	}

	// Check if action exists and belongs to the workflow
	var currentActionService, currentActionID string
	var currentActionConfigJSON []byte
	var currentPosition int
	err = h.db.DB().QueryRow(`
		SELECT action_service, action_id, action_config, position
		FROM workflow_actions
		WHERE id = ? AND workflow_id = ?
	`, actionID, workflowID).Scan(&currentActionService, &currentActionID, &currentActionConfigJSON, &currentPosition)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Action not found"})
			return
		}
		h.logger.Error("Failed to get action", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve action"})
		return
	}

	// Parse request
	var req UpdateActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Determine which fields to update
	actionService := currentActionService
	if req.ActionService != "" {
		actionService = req.ActionService
	}

	actionID := currentActionID
	if req.ActionID != "" {
		actionID = req.ActionID
	}

	var actionConfigJSON []byte
	if req.ActionConfig != nil {
		// Marshal new action config
		actionConfigJSON, err = json.Marshal(req.ActionConfig)
		if err != nil {
			h.logger.Error("Failed to marshal action config", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process action configuration"})
			return
		}
	} else {
		actionConfigJSON = currentActionConfigJSON
	}

	// Validate action service and ID
	provider, err := h.engine.GetServiceProvider(actionService)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unsupported action service: %s", actionService)})
		return
	}

	actionHandler := provider.GetActionHandler()
	actionTypes, err := actionHandler.GetActionTypes()
	if err != nil {
		h.logger.Error("Failed to get action types", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate action"})
		return
	}

	// Check if action ID is valid
	actionValid := false
	for _, a := range actionTypes {
		if a.ID == actionID {
			actionValid = true
			break
		}
	}

	if !actionValid {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unsupported action type: %s", actionID)})
		return
	}

	// Validate action config
	if err := actionHandler.ValidateActionConfig(actionID, actionConfigJSON); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid action configuration: %v", err)})
		return
	}

	// Handle position change if requested
	if req.Position > 0 && req.Position != currentPosition {
		// Begin transaction
		tx, err := h.db.DB().Begin()
		if err != nil {
			h.logger.Error("Failed to begin transaction", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update action"})
			return
		}
		defer tx.Rollback()

		// If moving up (to a lower position number)
		if req.Position < currentPosition {
			_, err = tx.Exec(`
				UPDATE workflow_actions
				SET position = position + 1
				WHERE workflow_id = ? AND position >= ? AND position < ?
			`, workflowID, req.Position, currentPosition)
		} else {
			// If moving down (to a higher position number)
			_, err = tx.Exec(`
				UPDATE workflow_actions
				SET position = position - 1
				WHERE workflow_id = ? AND position > ? AND position <= ?
			`, workflowID, currentPosition, req.Position)
		}

		if err != nil {
			h.logger.Error("Failed to shift action positions", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update action position"})
			return
		}

		// Update the action
		_, err = tx.Exec(`
			UPDATE workflow_actions
			SET action_service = ?, action_id = ?, action_config = ?, position = ?, updated_at = ?
			WHERE id = ?
		`, actionService, actionID, actionConfigJSON, req.Position, time.Now(), actionID)
		if err != nil {
			h.logger.Error("Failed to update action", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update action"})
			return
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			h.logger.Error("Failed to commit transaction", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update action"})
			return
		}
	} else {
		// Just update the action without changing position
		_, err = h.db.DB().Exec(`
			UPDATE workflow_actions
			SET action_service = ?, action_id = ?, action_config = ?, updated_at = ?
			WHERE id = ?
		`, actionService, actionID, actionConfigJSON, time.Now(), actionID)
		if err != nil {
			h.logger.Error("Failed to update action", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update action"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Action updated successfully",
	})
}

// DeleteAction deletes an action from a workflow
func (h *WorkflowHandler) DeleteAction(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get workflow ID from path
	workflowID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid workflow ID"})
		return
	}

	// Get action ID from path
	actionID, err := strconv.ParseInt(c.Param("actionId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action ID"})
		return
	}

	// Check if workflow exists and belongs to the user
	var workflowStatus string
	err = h.db.DB().QueryRow(`
		SELECT status FROM workflows WHERE id = ? AND user_id = ?
	`, workflowID, userID).Scan(&workflowStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to check workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify workflow"})
		return
	}

	// Check if workflow is active
	if workflowStatus == "active" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot modify an active workflow. Deactivate it first."})
		return
	}

	// Check if action exists and belongs to the workflow
	var position int
	err = h.db.DB().QueryRow(`
		SELECT position FROM workflow_actions WHERE id = ? AND workflow_id = ?
	`, actionID, workflowID).Scan(&position)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Action not found"})
			return
		}
		h.logger.Error("Failed to get action position", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve action"})
		return
	}

	// Begin transaction
	tx, err := h.db.DB().Begin()
	if err != nil {
		h.logger.Error("Failed to begin transaction", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete action"})
		return
	}
	defer tx.Rollback()

	// Delete the action
	_, err = tx.Exec("DELETE FROM workflow_actions WHERE id = ?", actionID)
	if err != nil {
		h.logger.Error("Failed to delete action", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete action"})
		return
	}

	// Shift positions of actions after the deleted one
	_, err = tx.Exec(`
		UPDATE workflow_actions
		SET position = position - 1
		WHERE workflow_id = ? AND position > ?
	`, workflowID, position)
	if err != nil {
		h.logger.Error("Failed to shift action positions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update action positions"})
		return
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		h.logger.Error("Failed to commit transaction", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete action"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Action deleted successfully",
	})
}

// ReorderActions reorders actions in a workflow
func (h *WorkflowHandler) ReorderActions(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get workflow ID from path
	workflowID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid workflow ID"})
		return
	}

	// Check if workflow exists and belongs to the user
	var workflowStatus string
	err = h.db.DB().QueryRow(`
		SELECT status FROM workflows WHERE id = ? AND user_id = ?
	`, workflowID, userID).Scan(&workflowStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to check workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify workflow"})
		return
	}

	// Check if workflow is active
	if workflowStatus == "active" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot modify an active workflow. Deactivate it first."})
		return
	}

	// Parse request
	var req ReorderActionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify all action IDs belong to the workflow
	var count int
	err = h.db.DB().QueryRow(`
		SELECT COUNT(*) FROM workflow_actions
		WHERE workflow_id = ? AND id IN (`+createPlaceholders(len(req.ActionIDs))+`)
	`, append([]interface{}{workflowID}, intsToInterfaces(req.ActionIDs)...)...).Scan(&count)
	if err != nil {
		h.logger.Error("Failed to verify action IDs", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify actions"})
		return
	}

	if count != len(req.ActionIDs) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Some action IDs are invalid or do not belong to this workflow"})
		return
	}

	// Begin transaction
	tx, err := h.db.DB().Begin()
	if err != nil {
		h.logger.Error("Failed to begin transaction", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reorder actions"})
		return
	}
	defer tx.Rollback()

	// Update positions
	for i, actionID := range req.ActionIDs {
		_, err = tx.Exec(`
			UPDATE workflow_actions
			SET position = ?, updated_at = ?
			WHERE id = ?
		`, i+1, time.Now(), actionID)
		if err != nil {
			h.logger.Error("Failed to update action position", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reorder actions"})
			return
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		h.logger.Error("Failed to commit transaction", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reorder actions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Actions reordered successfully",
	})
}

// AddDataMapping adds a data mapping to a workflow
func (h *WorkflowHandler) AddDataMapping(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get workflow ID from path
	workflowID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid workflow ID"})
		return
	}

	// Check if workflow exists and belongs to the user
	var workflowStatus string
	err = h.db.DB().QueryRow(`
		SELECT status FROM workflows WHERE id = ? AND user_id = ?
	`, workflowID, userID).Scan(&workflowStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to check workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify workflow"})
		return
	}

	// Check if workflow is active
	if workflowStatus == "active" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot modify an active workflow. Deactivate it first."})
		return
	}

	// Parse request
	var req AddDataMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate transformer if specified
	if req.Transformer != "" {
		if !h.engine.HasDataTransformer(req.Transformer) {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unknown transformer: %s", req.Transformer)})
			return
		}
	}

	// Create data mapping record
	var mappingID int64
	var transformer sql.NullString
	if req.Transformer != "" {
		transformer.String = req.Transformer
		transformer.Valid = true
	}

	err = h.db.DB().QueryRow(`
		INSERT INTO workflow_data_mappings (workflow_id, source_service, source_field, target_service, target_field, transformer, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`, workflowID, req.SourceService, req.SourceField, req.TargetService, req.TargetField, transformer, time.Now(), time.Now()).Scan(&mappingID)
	if err != nil {
		h.logger.Error("Failed to create data mapping", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add data mapping"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":      mappingID,
		"message": "Data mapping added successfully",
	})
}

// UpdateDataMapping updates a data mapping
func (h *WorkflowHandler) UpdateDataMapping(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get workflow ID from path
	workflowID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid workflow ID"})
		return
	}

	// Get mapping ID from path
	mappingID, err := strconv.ParseInt(c.Param("mappingId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid mapping ID"})
		return
	}

	// Check if workflow exists and belongs to the user
	var workflowStatus string
	err = h.db.DB().QueryRow(`
		SELECT status FROM workflows WHERE id = ? AND user_id = ?
	`, workflowID, userID).Scan(&workflowStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to check workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify workflow"})
		return
	}

	// Check if workflow is active
	if workflowStatus == "active" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot modify an active workflow. Deactivate it first."})
		return
	}

	// Check if mapping exists and belongs to the workflow
	var count int
	err = h.db.DB().QueryRow(`
		SELECT COUNT(*) FROM workflow_data_mappings WHERE id = ? AND workflow_id = ?
	`, mappingID, workflowID).Scan(&count)
	if err != nil {
		h.logger.Error("Failed to check mapping", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify mapping"})
		return
	}

	if count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Data mapping not found"})
		return
	}

	// Parse request
	var req UpdateDataMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate transformer if specified
	if req.Transformer != "" {
		if !h.engine.HasDataTransformer(req.Transformer) {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unknown transformer: %s", req.Transformer)})
			return
		}
	}

	// Build the update query
	query := "UPDATE workflow_data_mappings SET updated_at = ?"
	args := []interface{}{time.Now()}

	if req.SourceService != "" {
		query += ", source_service = ?"
		args = append(args, req.SourceService)
	}

	if req.SourceField != "" {
		query += ", source_field = ?"
		args = append(args, req.SourceField)
	}

	if req.TargetService != "" {
		query += ", target_service = ?"
		args = append(args, req.TargetService)
	}

	if req.TargetField != "" {
		query += ", target_field = ?"
		args = append(args, req.TargetField)
	}

	if req.Transformer != "" {
		query += ", transformer = ?"
		args = append(args, req.Transformer)
	}

	query += " WHERE id = ?"
	args = append(args, mappingID)

	// Execute the update
	_, err = h.db.DB().Exec(query, args...)
	if err != nil {
		h.logger.Error("Failed to update data mapping", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update data mapping"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Data mapping updated successfully",
	})
}

// DeleteDataMapping deletes a data mapping
func (h *WorkflowHandler) DeleteDataMapping(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get workflow ID from path
	workflowID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid workflow ID"})
		return
	}

	// Get mapping ID from path
	mappingID, err := strconv.ParseInt(c.Param("mappingId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid mapping ID"})
		return
	}

	// Check if workflow exists and belongs to the user
	var workflowStatus string
	err = h.db.DB().QueryRow(`
		SELECT status FROM workflows WHERE id = ? AND user_id = ?
	`, workflowID, userID).Scan(&workflowStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to check workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify workflow"})
		return
	}

	// Check if workflow is active
	if workflowStatus == "active" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot modify an active workflow. Deactivate it first."})
		return
	}

	// Check if mapping exists and belongs to the workflow
	var count int
	err = h.db.DB().QueryRow(`
		SELECT COUNT(*) FROM workflow_data_mappings WHERE id = ? AND workflow_id = ?
	`, mappingID, workflowID).Scan(&count)
	if err != nil {
		h.logger.Error("Failed to check mapping", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify mapping"})
		return
	}

	if count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Data mapping not found"})
		return
	}

	// Delete the mapping
	_, err = h.db.DB().Exec("DELETE FROM workflow_data_mappings WHERE id = ?", mappingID)
	if err != nil {
		h.logger.Error("Failed to delete data mapping", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete data mapping"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Data mapping deleted successfully",
	})
}

// TestWorkflow tests a workflow with sample trigger data
func (h *WorkflowHandler) TestWorkflow(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get workflow ID from path
	workflowID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid workflow ID"})
		return
	}

	// Check if workflow exists and belongs to the user
	var count int
	err = h.db.DB().QueryRow(`
		SELECT COUNT(*) FROM workflows WHERE id = ? AND user_id = ?
	`, workflowID, userID).Scan(&count)
	if err != nil {
		h.logger.Error("Failed to check workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify workflow"})
		return
	}

	if count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
		return
	}

	// Parse request
	var req TestWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Marshal trigger data
	triggerDataJSON, err := json.Marshal(req.TriggerData)
	if err != nil {
		h.logger.Error("Failed to marshal trigger data", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process trigger data"})
		return
	}

	// Execute the workflow
	execution, err := h.engine.ExecuteWorkflow(c.Request.Context(), workflowID, triggerDataJSON)
	if err != nil {
		h.logger.Error("Failed to execute workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to execute workflow: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"execution_id": execution.ID,
		"message":      "Workflow execution started",
	})
}

// HandleWebhook handles incoming webhooks
func (h *WorkflowHandler) HandleWebhook(c *gin.Context) {
	// Get webhook token from path
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook token"})
		return
	}

	// Get the webhook trigger record
	var workflowID int64
	var secret string
	err := h.db.DB().QueryRow(`
		SELECT workflow_id, secret FROM webhook_triggers WHERE webhook_url = ?
	`, fmt.Sprintf("/api/v1/webhooks/%s", token)).Scan(&workflowID, &secret)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Webhook not found"})
			return
		}
		h.logger.Error("Failed to get webhook trigger", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process webhook"})
		return
	}

	// Get the workflow
	var triggerService, triggerID string
	var status string
	err = h.db.DB().QueryRow(`
		SELECT trigger_service, trigger_id, status FROM workflows WHERE id = ?
	`, workflowID).Scan(&triggerService, &triggerID, &status)
	if err != nil {
		h.logger.Error("Failed to get workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process webhook"})
		return
	}

	// Check if workflow is active
	if status != "active" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Workflow is not active"})
		return
	}

	// Verify the webhook signature if available
	signature := c.GetHeader("X-Webhook-Signature")
	if signature != "" {
		if !verifyWebhookSignature(signature, secret, c.Request) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid webhook signature"})
			return
		}
	}

	// Read the request body
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		h.logger.Error("Failed to read webhook body", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read webhook data"})
		return
	}

	// Reset the request body for future middleware
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(body))

	// Get the service provider
	provider, err := h.engine.GetServiceProvider(triggerService)
	if err != nil {
		h.logger.Error("Failed to get service provider", "error", err, "service", triggerService)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process webhook"})
		return
	}

	// Process the webhook with the appropriate trigger handler
	triggerHandler := provider.GetTriggerHandler()

	// Some trigger handlers might have a specific webhook handling method
	var triggerData *common.TriggerData
	if webhookHandler, ok := triggerHandler.(interface {
		HandleWebhook(payload []byte) (*common.TriggerData, error)
	}); ok {
		triggerData, err = webhookHandler.HandleWebhook(body)
		if err != nil {
			h.logger.Error("Failed to handle webhook", "error", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to process webhook: %v", err)})
			return
		}
	} else {
		// Parse the webhook body as JSON
		var webhookData map[string]interface{}
		if err := json.Unmarshal(body, &webhookData); err != nil {
			h.logger.Error("Failed to parse webhook body", "error", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook data format"})
			return
		}

		// Create a generic trigger data
		triggerData = &common.TriggerData{
			Type:    triggerID,
			Service: triggerService,
			Data:    webhookData,
		}
	}

	// Marshal trigger data
	triggerDataJSON, err := json.Marshal(triggerData.Data)
	if err != nil {
		h.logger.Error("Failed to marshal trigger data", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process trigger data"})
		return
	}

	// Execute the workflow
	execution, err := h.engine.ExecuteWorkflow(c.Request.Context(), workflowID, triggerDataJSON)
	if err != nil {
		h.logger.Error("Failed to execute workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to execute workflow: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"execution_id": execution.ID,
		"message":      "Webhook processed successfully",
	})
}

// Helper functions

// createPlaceholders creates a string of SQL placeholders (?, ?, ...)
func createPlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	placeholders := make([]string, n)
	for i := range placeholders {
		placeholders[i] = "?"
	}
	return strings.Join(placeholders, ",")
}

// intsToInterfaces converts a slice of int64 to a slice of interface{}
func intsToInterfaces(ints []int64) []interface{} {
	interfaces := make([]interface{}, len(ints))
	for i, v := range ints {
		interfaces[i] = v
	}
	return interfaces
}

// calculateNextTriggerTime calculates the next trigger time for a cron expression
func calculateNextTriggerTime(cronExpr string) time.Time {
	// This is a simplified placeholder implementation
	// In a real application, use a proper cron library to calculate the next run time
	return time.Now().Add(5 * time.Minute)
}

// verifyWebhookSignature verifies the webhook signature
func verifyWebhookSignature(signature, secret string, r *http.Request) bool {
	// This is a simplified placeholder implementation
	// In a real application, implement proper signature verification based on the provider
	return true
}
