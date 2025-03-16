package queue

import (
	"encoding/json"
	"fmt"
	"time"
)

// JobType represents the type of job
type JobType string

const (
	// JobWorkflowExecution represents a workflow execution job
	JobWorkflowExecution JobType = "workflow_execution"

	// JobWebhookDelivery represents a webhook delivery job
	JobWebhookDelivery JobType = "webhook_delivery"

	// JobScheduledTrigger represents a scheduled trigger job
	JobScheduledTrigger JobType = "scheduled_trigger"

	// JobConnectionRefresh represents a connection token refresh job
	JobConnectionRefresh JobType = "connection_refresh"
)

// JobStatus represents the status of a job
type JobStatus string

const (
	// JobStatusPending indicates the job is waiting to be processed
	JobStatusPending JobStatus = "pending"

	// JobStatusRunning indicates the job is currently being processed
	JobStatusRunning JobStatus = "running"

	// JobStatusCompleted indicates the job completed successfully
	JobStatusCompleted JobStatus = "completed"

	// JobStatusFailed indicates the job failed
	JobStatusFailed JobStatus = "failed"

	// JobStatusRetrying indicates the job failed and is scheduled for retry
	JobStatusRetrying JobStatus = "retrying"
)

// Job represents a unit of work to be processed asynchronously
type Job struct {
	ID          string     `json:"id"`
	Type        JobType    `json:"type"`
	Status      JobStatus  `json:"status"`
	Payload     JobPayload `json:"payload"`
	RetryCount  int        `json:"retry_count"`
	MaxRetries  int        `json:"max_retries"`
	Error       string     `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	NextRetryAt *time.Time `json:"next_retry_at,omitempty"`
}

// JobPayload represents the payload of a job
type JobPayload struct {
	// Common fields
	UserID int64 `json:"user_id,omitempty"`

	// WorkflowExecution job fields
	WorkflowID          int64           `json:"workflow_id,omitempty"`
	WorkflowExecutionID int64           `json:"workflow_execution_id,omitempty"`
	TriggerData         json.RawMessage `json:"trigger_data,omitempty"`

	// WebhookDelivery job fields
	WebhookURL     string            `json:"webhook_url,omitempty"`
	WebhookPayload json.RawMessage   `json:"webhook_payload,omitempty"`
	WebhookHeaders map[string]string `json:"webhook_headers,omitempty"`

	// ScheduledTrigger job fields
	TriggerID     string          `json:"trigger_id,omitempty"`
	TriggerConfig json.RawMessage `json:"trigger_config,omitempty"`

	// ConnectionRefresh job fields
	ConnectionID int64 `json:"connection_id,omitempty"`
}

// NewJob creates a new job
func NewJob(jobType JobType, payload JobPayload) *Job {
	now := time.Now()
	return &Job{
		ID:         generateJobID(),
		Type:       jobType,
		Status:     JobStatusPending,
		Payload:    payload,
		RetryCount: 0,
		MaxRetries: 3, // Default max retries
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// NewWorkflowExecutionJob creates a new workflow execution job
func NewWorkflowExecutionJob(userID, workflowID int64, triggerData json.RawMessage) *Job {
	return NewJob(JobWorkflowExecution, JobPayload{
		UserID:      userID,
		WorkflowID:  workflowID,
		TriggerData: triggerData,
	})
}

// NewWebhookDeliveryJob creates a new webhook delivery job
func NewWebhookDeliveryJob(userID int64, webhookURL string, payload json.RawMessage, headers map[string]string) *Job {
	return NewJob(JobWebhookDelivery, JobPayload{
		UserID:         userID,
		WebhookURL:     webhookURL,
		WebhookPayload: payload,
		WebhookHeaders: headers,
	})
}

// NewScheduledTriggerJob creates a new scheduled trigger job
func NewScheduledTriggerJob(userID, workflowID int64, triggerID string, triggerConfig json.RawMessage) *Job {
	return NewJob(JobScheduledTrigger, JobPayload{
		UserID:        userID,
		WorkflowID:    workflowID,
		TriggerID:     triggerID,
		TriggerConfig: triggerConfig,
	})
}

// NewConnectionRefreshJob creates a new connection refresh job
func NewConnectionRefreshJob(userID, connectionID int64) *Job {
	return NewJob(JobConnectionRefresh, JobPayload{
		UserID:       userID,
		ConnectionID: connectionID,
	})
}

// SetCompleted marks the job as completed
func (j *Job) SetCompleted() {
	now := time.Now()
	j.Status = JobStatusCompleted
	j.CompletedAt = &now
	j.UpdatedAt = now
}

// SetFailed marks the job as failed
func (j *Job) SetFailed(err error) {
	now := time.Now()

	if j.RetryCount < j.MaxRetries {
		j.Status = JobStatusRetrying
		j.RetryCount++

		// Calculate next retry time with exponential backoff
		retryDelay := time.Duration(1<<uint(j.RetryCount-1)) * time.Minute
		nextRetry := now.Add(retryDelay)
		j.NextRetryAt = &nextRetry
	} else {
		j.Status = JobStatusFailed
		j.CompletedAt = &now
	}

	if err != nil {
		j.Error = err.Error()
	}

	j.UpdatedAt = now
}

// SetRunning marks the job as running
func (j *Job) SetRunning() {
	now := time.Now()
	j.Status = JobStatusRunning
	j.StartedAt = &now
	j.UpdatedAt = now
}

// IsReady returns true if the job is ready to be processed
func (j *Job) IsReady() bool {
	if j.Status == JobStatusPending {
		return true
	}

	if j.Status == JobStatusRetrying && j.NextRetryAt != nil {
		return time.Now().After(*j.NextRetryAt)
	}

	return false
}

// generateJobID generates a unique job ID
func generateJobID() string {
	// In a real implementation, use a proper UUID library
	return fmt.Sprintf("job_%d", time.Now().UnixNano())
}
