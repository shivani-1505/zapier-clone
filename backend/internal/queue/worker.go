package queue

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/auditcue/integration-framework/internal/workflow"
	"github.com/auditcue/integration-framework/pkg/logger"
)

// Worker represents a worker that processes jobs from the queue
type Worker struct {
	id        int
	queue     *Queue
	engine    *workflow.Engine
	db        *sql.DB
	logger    *logger.Logger
	stop      chan struct{}
	wg        sync.WaitGroup
	timeout   time.Duration
	isRunning bool
}

// Workers represents a group of workers
type Workers struct {
	workers []*Worker
	logger  *logger.Logger
}

// NewWorker creates a new worker
func NewWorker(id int, queue *Queue, engine *workflow.Engine, db *sql.DB, logger *logger.Logger, timeout time.Duration) *Worker {
	return &Worker{
		id:        id,
		queue:     queue,
		engine:    engine,
		db:        db,
		logger:    logger.WithContext("worker_id", id),
		stop:      make(chan struct{}),
		timeout:   timeout,
		isRunning: false,
	}
}

// Start starts the worker
func (w *Worker) Start() {
	if w.isRunning {
		return
	}

	w.isRunning = true
	w.wg.Add(1)

	go func() {
		defer w.wg.Done()
		w.logger.Info("Worker started")

		for {
			select {
			case <-w.stop:
				w.logger.Info("Worker stopped")
				return
			default:
				// Try to get and process a job
				job, err := w.queue.Dequeue()
				if err != nil {
					if err != ErrQueueEmpty {
						w.logger.Error("Failed to dequeue job", "error", err)
					}
					time.Sleep(1 * time.Second)
					continue
				}

				// Process the job
				w.processJob(job)
			}
		}
	}()
}

// Stop stops the worker
func (w *Worker) Stop() {
	if !w.isRunning {
		return
	}

	close(w.stop)
	w.wg.Wait()
	w.isRunning = false
}

// processJob processes a job
func (w *Worker) processJob(job *Job) {
	w.logger.Info("Processing job", "job_id", job.ID, "job_type", job.Type)

	// Mark job as running
	job.SetRunning()
	if err := w.queue.Update(job); err != nil {
		w.logger.Error("Failed to update job status", "job_id", job.ID, "error", err)
		return
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), w.timeout)
	defer cancel()

	// Process the job based on its type
	var err error
	switch job.Type {
	case JobWorkflowExecution:
		err = w.processWorkflowExecution(ctx, job)
	case JobWebhookDelivery:
		err = w.processWebhookDelivery(ctx, job)
	case JobScheduledTrigger:
		err = w.processScheduledTrigger(ctx, job)
	case JobConnectionRefresh:
		err = w.processConnectionRefresh(ctx, job)
	default:
		err = fmt.Errorf("unknown job type: %s", job.Type)
	}

	// Update job status based on processing result
	if err != nil {
		w.logger.Error("Job processing failed", "job_id", job.ID, "error", err)
		job.SetFailed(err)
	} else {
		w.logger.Info("Job completed successfully", "job_id", job.ID)
		job.SetCompleted()
	}

	// Update the job in the queue
	if updateErr := w.queue.Update(job); updateErr != nil {
		w.logger.Error("Failed to update job status after processing", "job_id", job.ID, "error", updateErr)
	}
}

// processWorkflowExecution processes a workflow execution job
func (w *Worker) processWorkflowExecution(ctx context.Context, job *Job) error {
	payload := job.Payload

	// Check if this is a continuation of an existing workflow execution
	if payload.WorkflowExecutionID > 0 {
		// TODO: Handle continuation of a workflow execution
		// This would involve executing a specific action in a workflow
		return fmt.Errorf("workflow execution continuation not implemented yet")
	}

	// Execute the workflow
	execution, err := w.engine.ExecuteWorkflow(ctx, payload.WorkflowID, payload.TriggerData)
	if err != nil {
		return fmt.Errorf("failed to execute workflow: %w", err)
	}

	// Update the job payload with the execution ID
	job.Payload.WorkflowExecutionID = execution.ID
	return nil
}

// processWebhookDelivery processes a webhook delivery job
func (w *Worker) processWebhookDelivery(ctx context.Context, job *Job) error {
	payload := job.Payload

	// Create an HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, "POST", payload.WebhookURL, bytes.NewBuffer(payload.WebhookPayload))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for key, value := range payload.WebhookHeaders {
		req.Header.Set(key, value)
	}

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("webhook delivery failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// processScheduledTrigger processes a scheduled trigger job
func (w *Worker) processScheduledTrigger(ctx context.Context, job *Job) error {
	payload := job.Payload

	// Get the service provider for the trigger
	provider, err := w.engine.GetServiceProvider(payload.TriggerService)
	if err != nil {
		return fmt.Errorf("service provider not found: %w", err)
	}

	// Get the trigger handler
	triggerHandler := provider.GetTriggerHandler()

	// Get connection for the trigger
	connection, err := w.getConnectionForTrigger(ctx, payload.UserID, payload.TriggerService)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}

	// Execute the trigger
	triggerData, err := triggerHandler.ExecuteTrigger(ctx, connection, payload.TriggerID, payload.TriggerConfig)
	if err != nil {
		return fmt.Errorf("failed to execute trigger: %w", err)
	}

	// Marshal trigger data
	triggerDataJSON, err := json.Marshal(triggerData.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal trigger data: %w", err)
	}

	// Create a workflow execution job
	executionJob := NewWorkflowExecutionJob(payload.UserID, payload.WorkflowID, triggerDataJSON)
	if err := w.queue.Enqueue(executionJob); err != nil {
		return fmt.Errorf("failed to enqueue workflow execution job: %w", err)
	}

	// Update the next trigger time in the scheduled_triggers table
	schedule, ok := payload.TriggerConfig["schedule"].(string)
	if !ok {
		return fmt.Errorf("invalid schedule in trigger config")
	}

	nextTrigger := calculateNextTriggerTime(schedule)
	_, err = w.db.ExecContext(ctx, `
		UPDATE scheduled_triggers
		SET last_triggered_at = ?, next_trigger_at = ?
		WHERE workflow_id = ?
	`, time.Now(), nextTrigger, payload.WorkflowID)
	if err != nil {
		return fmt.Errorf("failed to update scheduled trigger: %w", err)
	}

	return nil
}

// processConnectionRefresh processes a connection refresh job
func (w *Worker) processConnectionRefresh(ctx context.Context, job *Job) error {
	payload := job.Payload

	// Get the connection details
	var service, authType string
	var authDataJSON string
	err := w.db.QueryRowContext(ctx, `
		SELECT service, auth_type, auth_data
		FROM connections
		WHERE id = ? AND user_id = ?
	`, payload.ConnectionID, payload.UserID).Scan(&service, &authType, &authDataJSON)
	if err != nil {
		return fmt.Errorf("failed to get connection details: %w", err)
	}

	// Skip if not an OAuth connection
	if authType != "oauth" {
		return nil
	}

	// Parse auth data
	var authData common.AuthData
	if err := json.Unmarshal([]byte(authDataJSON), &authData); err != nil {
		return fmt.Errorf("failed to parse auth data: %w", err)
	}

	// Check if token needs refresh
	if time.Now().Before(authData.ExpiresAt) {
		// Token is still valid
		return nil
	}

	// Get the service provider
	provider, err := w.engine.GetServiceProvider(service)
	if err != nil {
		return fmt.Errorf("service provider not found: %w", err)
	}

	// Get the auth handler
	authHandler := provider.GetAuthHandler()

	// Refresh the token
	refreshedAuth, err := authHandler.RefreshToken(ctx, &authData)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}

	// Update the connection with the refreshed token
	refreshedAuthJSON, err := json.Marshal(refreshedAuth)
	if err != nil {
		return fmt.Errorf("failed to marshal refreshed auth data: %w", err)
	}

	_, err = w.db.ExecContext(ctx, `
		UPDATE connections
		SET auth_data = ?, updated_at = ?
		WHERE id = ?
	`, refreshedAuthJSON, time.Now(), payload.ConnectionID)
	if err != nil {
		return fmt.Errorf("failed to update connection: %w", err)
	}

	return nil
}

// getConnectionForTrigger retrieves a connection for a trigger
func (w *Worker) getConnectionForTrigger(ctx context.Context, userID int64, service string) (*common.Connection, error) {
	var connection common.Connection
	var authDataJSON, metadataJSON string

	err := w.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, service, status, auth_type, auth_data, metadata, created_at, updated_at
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
		&connection.CreatedAt,
		&connection.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}

	// Parse auth data
	if err := json.Unmarshal([]byte(authDataJSON), &connection.AuthData); err != nil {
		return nil, fmt.Errorf("failed to parse auth data: %w", err)
	}

	// Parse metadata
	if metadataJSON != "" {
		if err := json.Unmarshal([]byte(metadataJSON), &connection.Metadata); err != nil {
			return nil, fmt.Errorf("failed to parse metadata: %w", err)
		}
	} else {
		connection.Metadata = make(map[string]interface{})
	}

	return &connection, nil
}

// StartWorkers starts a group of workers
func StartWorkers(queue *Queue, count int, db *sql.DB, logger *logger.Logger) *Workers {
	logger.Info("Starting workers", "count", count)

	workers := make([]*Worker, count)
	for i := 0; i < count; i++ {
		worker := NewWorker(i, queue, engine, db, logger, 5*time.Minute)
		worker.Start()
		workers[i] = worker
	}

	return &Workers{
		workers: workers,
		logger:  logger,
	}
}

// Stop stops all workers
func (w *Workers) Stop() {
	w.logger.Info("Stopping workers")

	for _, worker := range w.workers {
		worker.Stop()
	}
}
