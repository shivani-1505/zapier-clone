package workflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/auditcue/integration-framework/internal/integrations/common"
	"github.com/auditcue/integration-framework/pkg/logger"
)

// Engine represents the workflow execution engine
type Engine struct {
	db               *sql.DB
	logger           *logger.Logger
	serviceProviders map[string]common.ServiceProvider
	dataTransformers map[string]DataTransformer
}

// Workflow represents a workflow
type Workflow struct {
	ID             int64
	UserID         int64
	Name           string
	Description    string
	Status         string
	TriggerService string
	TriggerID      string
	TriggerConfig  json.RawMessage
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Actions        []WorkflowAction
	DataMappings   []common.DataMapping
}

// WorkflowAction represents an action in a workflow
type WorkflowAction struct {
	ID            int64
	WorkflowID    int64
	ActionService string
	ActionID      string
	ActionConfig  json.RawMessage
	Position      int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// WorkflowExecution represents an execution of a workflow
type WorkflowExecution struct {
	ID          int64
	WorkflowID  int64
	Status      string
	TriggerData json.RawMessage
	StartedAt   time.Time
	CompletedAt *time.Time
	Error       *string
	Actions     []WorkflowActionExecution
}

// WorkflowActionExecution represents an execution of a workflow action
type WorkflowActionExecution struct {
	ID                  int64
	WorkflowExecutionID int64
	WorkflowActionID    int64
	Status              string
	InputData           json.RawMessage
	OutputData          json.RawMessage
	StartedAt           time.Time
	CompletedAt         *time.Time
	Error               *string
}

// DataTransformer is a function that transforms data
type DataTransformer func(input interface{}) (interface{}, error)

// ExecutionStatus represents the status of an execution
type ExecutionStatus string

const (
	StatusPending   ExecutionStatus = "pending"
	StatusRunning   ExecutionStatus = "running"
	StatusCompleted ExecutionStatus = "completed"
	StatusFailed    ExecutionStatus = "failed"
)

// NewEngine creates a new workflow engine
func NewEngine(db *sql.DB, logger *logger.Logger) *Engine {
	return &Engine{
		db:               db,
		logger:           logger,
		serviceProviders: make(map[string]common.ServiceProvider),
		dataTransformers: make(map[string]DataTransformer),
	}
}

// RegisterServiceProvider registers a service provider
func (e *Engine) RegisterServiceProvider(provider common.ServiceProvider) {
	serviceName := provider.GetService()
	e.serviceProviders[serviceName] = provider
	e.logger.Info("Registered service provider", "service", serviceName)
}

// RegisterDataTransformer registers a data transformer
func (e *Engine) RegisterDataTransformer(name string, transformer DataTransformer) {
	e.dataTransformers[name] = transformer
	e.logger.Info("Registered data transformer", "name", name)
}

// GetServiceProvider returns a service provider by name
func (e *Engine) GetServiceProvider(name string) (common.ServiceProvider, error) {
	provider, ok := e.serviceProviders[name]
	if !ok {
		return nil, fmt.Errorf("service provider not found: %s", name)
	}
	return provider, nil
}

// GetWorkflow retrieves a workflow by ID
func (e *Engine) GetWorkflow(ctx context.Context, id int64) (*Workflow, error) {
	// Get the workflow
	workflow := &Workflow{}
	err := e.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, description, status, trigger_service, trigger_id, trigger_config, created_at, updated_at
		FROM workflows
		WHERE id = ?
	`, id).Scan(
		&workflow.ID,
		&workflow.UserID,
		&workflow.Name,
		&workflow.Description,
		&workflow.Status,
		&workflow.TriggerService,
		&workflow.TriggerID,
		&workflow.TriggerConfig,
		&workflow.CreatedAt,
		&workflow.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("workflow not found: %d", id)
		}
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	// Get the workflow actions
	rows, err := e.db.QueryContext(ctx, `
		SELECT id, workflow_id, action_service, action_id, action_config, position, created_at, updated_at
		FROM workflow_actions
		WHERE workflow_id = ?
		ORDER BY position
	`, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow actions: %w", err)
	}
	defer rows.Close()

	workflow.Actions = []WorkflowAction{}
	for rows.Next() {
		var action WorkflowAction
		err := rows.Scan(
			&action.ID,
			&action.WorkflowID,
			&action.ActionService,
			&action.ActionID,
			&action.ActionConfig,
			&action.Position,
			&action.CreatedAt,
			&action.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan workflow action: %w", err)
		}
		workflow.Actions = append(workflow.Actions, action)
	}

	// Get the data mappings
	mappingRows, err := e.db.QueryContext(ctx, `
		SELECT source_service, source_field, target_service, target_field, transformer
		FROM workflow_data_mappings
		WHERE workflow_id = ?
	`, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get data mappings: %w", err)
	}
	defer mappingRows.Close()

	workflow.DataMappings = []common.DataMapping{}
	for mappingRows.Next() {
		var mapping common.DataMapping
		var transformer sql.NullString
		err := mappingRows.Scan(
			&mapping.SourceService,
			&mapping.SourceField,
			&mapping.TargetService,
			&mapping.TargetField,
			&transformer,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan data mapping: %w", err)
		}
		if transformer.Valid {
			mapping.Transformer = transformer.String
		}
		workflow.DataMappings = append(workflow.DataMappings, mapping)
	}

	return workflow, nil
}

// ValidateWorkflow validates a workflow
func (e *Engine) ValidateWorkflow(ctx context.Context, workflow *Workflow) error {
	// Check if trigger service exists
	triggerProvider, err := e.GetServiceProvider(workflow.TriggerService)
	if err != nil {
		return fmt.Errorf("invalid trigger service: %w", err)
	}

	// Validate trigger configuration
	triggerHandler := triggerProvider.GetTriggerHandler()
	if err := triggerHandler.ValidateTriggerConfig(workflow.TriggerID, workflow.TriggerConfig); err != nil {
		return fmt.Errorf("invalid trigger configuration: %w", err)
	}

	// Validate each action
	for _, action := range workflow.Actions {
		actionProvider, err := e.GetServiceProvider(action.ActionService)
		if err != nil {
			return fmt.Errorf("invalid action service: %w", err)
		}

		actionHandler := actionProvider.GetActionHandler()
		if err := actionHandler.ValidateActionConfig(action.ActionID, action.ActionConfig); err != nil {
			return fmt.Errorf("invalid action configuration: %w", err)
		}
	}

	// Validate data mappings
	for _, mapping := range workflow.DataMappings {
		if mapping.Transformer != "" {
			if _, ok := e.dataTransformers[mapping.Transformer]; !ok {
				return fmt.Errorf("unknown data transformer: %s", mapping.Transformer)
			}
		}
	}

	return nil
}

// ExecuteWorkflow executes a workflow with the given trigger data
func (e *Engine) ExecuteWorkflow(ctx context.Context, workflowID int64, triggerData json.RawMessage) (*WorkflowExecution, error) {
	// Get the workflow
	workflow, err := e.GetWorkflow(ctx, workflowID)
	if err != nil {
		return nil, err
	}

	// Validate workflow
	if err := e.ValidateWorkflow(ctx, workflow); err != nil {
		return nil, err
	}

	// Start a transaction
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create a workflow execution record
	var executionID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO workflow_executions (workflow_id, status, trigger_data, started_at)
		VALUES (?, ?, ?, ?)
		RETURNING id
	`, workflowID, StatusRunning, triggerData, time.Now()).Scan(&executionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow execution: %w", err)
	}

	// Commit the transaction to ensure the execution is recorded
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Execute the workflow in a separate goroutine
	go e.runWorkflowExecution(context.Background(), workflow, executionID, triggerData)

	// Return the execution record
	execution := &WorkflowExecution{
		ID:          executionID,
		WorkflowID:  workflowID,
		Status:      string(StatusRunning),
		TriggerData: triggerData,
		StartedAt:   time.Now(),
		Actions:     []WorkflowActionExecution{},
	}

	return execution, nil
}

// runWorkflowExecution executes a workflow in the background
func (e *Engine) runWorkflowExecution(ctx context.Context, workflow *Workflow, executionID int64, triggerData json.RawMessage) {
	var data map[string]interface{}
	if err := json.Unmarshal(triggerData, &data); err != nil {
		e.completeExecution(ctx, executionID, StatusFailed, fmt.Sprintf("Failed to parse trigger data: %v", err))
		return
	}

	// Process each action in sequence
	for _, action := range workflow.Actions {
		// Apply data mappings
		inputData, err := e.applyDataMappings(workflow.TriggerService, data, action.ActionService, workflow.DataMappings)
		if err != nil {
			e.completeExecution(ctx, executionID, StatusFailed, fmt.Sprintf("Failed to apply data mappings: %v", err))
			return
		}

		// Create action execution record
		actionExecID, err := e.createActionExecution(ctx, executionID, action.ID, inputData)
		if err != nil {
			e.completeExecution(ctx, executionID, StatusFailed, fmt.Sprintf("Failed to create action execution record: %v", err))
			return
		}

		// Get the connection for the action service
		connection, err := e.getConnectionForAction(ctx, workflow.UserID, action.ActionService)
		if err != nil {
			e.completeActionExecution(ctx, actionExecID, StatusFailed, nil, fmt.Sprintf("Failed to get connection: %v", err))
			e.completeExecution(ctx, executionID, StatusFailed, fmt.Sprintf("Failed to get connection: %v", err))
			return
		}

		// Execute the action
		actionProvider, err := e.GetServiceProvider(action.ActionService)
		if err != nil {
			e.completeActionExecution(ctx, actionExecID, StatusFailed, nil, fmt.Sprintf("Failed to get service provider: %v", err))
			e.completeExecution(ctx, executionID, StatusFailed, fmt.Sprintf("Failed to get service provider: %v", err))
			return
		}

		actionHandler := actionProvider.GetActionHandler()
		actionData, err := actionHandler.ExecuteAction(ctx, connection, action.ActionID, action.ActionConfig, inputData)
		if err != nil {
			e.completeActionExecution(ctx, actionExecID, StatusFailed, nil, fmt.Sprintf("Failed to execute action: %v", err))
			e.completeExecution(ctx, executionID, StatusFailed, fmt.Sprintf("Failed to execute action: %v", err))
			return
		}

		// Update action execution record
		outputJSON, err := json.Marshal(actionData.Data)
		if err != nil {
			e.completeActionExecution(ctx, actionExecID, StatusFailed, nil, fmt.Sprintf("Failed to marshal action output: %v", err))
			e.completeExecution(ctx, executionID, StatusFailed, fmt.Sprintf("Failed to marshal action output: %v", err))
			return
		}

		err = e.completeActionExecution(ctx, actionExecID, StatusCompleted, outputJSON, "")
		if err != nil {
			e.completeExecution(ctx, executionID, StatusFailed, fmt.Sprintf("Failed to complete action execution: %v", err))
			return
		}

		// Update data for next action
		data = actionData.Data
	}

	// Complete the workflow execution
	e.completeExecution(ctx, executionID, StatusCompleted, "")
}

// completeExecution updates a workflow execution as completed
func (e *Engine) completeExecution(ctx context.Context, executionID int64, status ExecutionStatus, errorMsg string) error {
	var err error
	if errorMsg != "" {
		_, err = e.db.ExecContext(ctx, `
			UPDATE workflow_executions
			SET status = ?, completed_at = ?, error = ?
			WHERE id = ?
		`, status, time.Now(), errorMsg, executionID)
	} else {
		_, err = e.db.ExecContext(ctx, `
			UPDATE workflow_executions
			SET status = ?, completed_at = ?
			WHERE id = ?
		`, status, time.Now(), executionID)
	}

	if err != nil {
		e.logger.Error("Failed to complete workflow execution", "executionID", executionID, "error", err)
		return err
	}

	return nil
}

// createActionExecution creates a record for an action execution
func (e *Engine) createActionExecution(ctx context.Context, executionID int64, actionID int64, inputData map[string]interface{}) (int64, error) {
	inputJSON, err := json.Marshal(inputData)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal input data: %w", err)
	}

	var actionExecID int64
	err = e.db.QueryRowContext(ctx, `
		INSERT INTO workflow_action_executions (workflow_execution_id, workflow_action_id, status, input_data, started_at)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id
	`, executionID, actionID, StatusRunning, inputJSON, time.Now()).Scan(&actionExecID)
	if err != nil {
		return 0, fmt.Errorf("failed to create action execution: %w", err)
	}

	return actionExecID, nil
}

// completeActionExecution updates an action execution as completed
func (e *Engine) completeActionExecution(ctx context.Context, actionExecID int64, status ExecutionStatus, outputData json.RawMessage, errorMsg string) error {
	var err error
	if errorMsg != "" {
		_, err = e.db.ExecContext(ctx, `
			UPDATE workflow_action_executions
			SET status = ?, output_data = ?, completed_at = ?, error = ?
			WHERE id = ?
		`, status, outputData, time.Now(), errorMsg, actionExecID)
	} else {
		_, err = e.db.ExecContext(ctx, `
			UPDATE workflow_action_executions
			SET status = ?, output_data = ?, completed_at = ?
			WHERE id = ?
		`, status, outputData, time.Now(), actionExecID)
	}

	if err != nil {
		e.logger.Error("Failed to complete action execution", "actionExecID", actionExecID, "error", err)
		return err
	}

	return nil
}

// applyDataMappings applies data mappings to prepare input for an action
func (e *Engine) applyDataMappings(sourceService string, sourceData map[string]interface{}, targetService string, mappings []common.DataMapping) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for _, mapping := range mappings {
		if mapping.SourceService == sourceService && mapping.TargetService == targetService {
			// Get the source value using the source field
			sourceValue, ok := sourceData[mapping.SourceField]
			if !ok {
				continue // Skip if field doesn't exist in source data
			}

			// Apply transformer if specified
			if mapping.Transformer != "" {
				transformer, ok := e.dataTransformers[mapping.Transformer]
				if !ok {
					return nil, fmt.Errorf("unknown transformer: %s", mapping.Transformer)
				}

				transformedValue, err := transformer(sourceValue)
				if err != nil {
					return nil, fmt.Errorf("failed to apply transformer: %w", err)
				}
				sourceValue = transformedValue
			}

			// Set the value in the result
			result[mapping.TargetField] = sourceValue
		}
	}

	return result, nil
}

// getConnectionForAction retrieves a connection for the given service
func (e *Engine) getConnectionForAction(ctx context.Context, userID int64, service string) (*common.Connection, error) {
	var connection common.Connection
	var authDataJSON, metadataJSON string

	err := e.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, service, status, auth_type, auth_data, metadata, last_used_at, created_at, updated_at
		FROM connections
		WHERE user_id = ? AND service = ? AND status = 'active'
		ORDER BY last_used_at DESC
		LIMIT 1
	`, userID, service).Scan(
		&connection.ID,
		&connection.UserID,
		&connection.Name,
		&connection.Service,
		&connection.Status,
		&connection.AuthType,
		&authDataJSON,
		&metadataJSON,
		&connection.LastUsedAt,
		&connection.CreatedAt,
		&connection.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("no active connection found for service: %s", service)
		}
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}

	// Parse auth data JSON
	if err := json.Unmarshal([]byte(authDataJSON), &connection.AuthData); err != nil {
		return nil, fmt.Errorf("failed to parse auth data: %w", err)
	}

	// Parse metadata JSON
	if metadataJSON != "" {
		if err := json.Unmarshal([]byte(metadataJSON), &connection.Metadata); err != nil {
			return nil, fmt.Errorf("failed to parse metadata: %w", err)
		}
	} else {
		connection.Metadata = make(map[string]interface{})
	}

	// Update last used timestamp
	_, err = e.db.ExecContext(ctx, `
		UPDATE connections
		SET last_used_at = ?
		WHERE id = ?
	`, time.Now(), connection.ID)
	if err != nil {
		e.logger.Error("Failed to update connection last_used_at", "connectionID", connection.ID, "error", err)
	}

	return &connection, nil
}
