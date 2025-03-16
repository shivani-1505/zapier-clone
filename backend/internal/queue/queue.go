package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/auditcue/integration-framework/internal/config"
	"github.com/go-redis/redis/v8"
)

// Queue errors
var (
	ErrQueueEmpty    = errors.New("queue is empty")
	ErrJobNotFound   = errors.New("job not found")
	ErrInvalidDriver = errors.New("invalid queue driver")
)

// Queue interface for job queueing
type Queue interface {
	// Enqueue adds a job to the queue
	Enqueue(job *Job) error

	// Dequeue removes and returns the next job from the queue
	Dequeue() (*Job, error)

	// Get retrieves a job by ID
	Get(id string) (*Job, error)

	// Update updates a job in the queue
	Update(job *Job) error

	// Delete removes a job from the queue
	Delete(id string) error

	// Close closes the queue connection
	Close() error
}

// RedisQueue implements Queue using Redis
type RedisQueue struct {
	client         *redis.Client
	pendingQueue   string
	processingSet  string
	completedSet   string
	failedSet      string
	jobsHash       string
	retryingZSet   string
	jobTTL         time.Duration
	completedTTL   time.Duration
	processingLock time.Duration
}

// NewQueue creates a new queue based on the driver in config
func NewQueue(cfg config.QueueConfig) (Queue, error) {
	switch strings.ToLower(cfg.Driver) {
	case "redis":
		return NewRedisQueue(cfg)
	// Add other queue drivers here
	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidDriver, cfg.Driver)
	}
}

// NewRedisQueue creates a new Redis-backed queue
func NewRedisQueue(cfg config.QueueConfig) (Queue, error) {
	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Address,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisQueue{
		client:         client,
		pendingQueue:   "auditcue:queue:pending",
		processingSet:  "auditcue:queue:processing",
		completedSet:   "auditcue:queue:completed",
		failedSet:      "auditcue:queue:failed",
		jobsHash:       "auditcue:queue:jobs",
		retryingZSet:   "auditcue:queue:retrying",
		jobTTL:         7 * 24 * time.Hour, // 7 days
		completedTTL:   24 * time.Hour,     // 1 day
		processingLock: 60 * time.Minute,   // 60 minutes
	}, nil
}

// Enqueue adds a job to the queue
func (q *RedisQueue) Enqueue(job *Job) error {
	ctx := context.Background()

	// Serialize the job
	jobBytes, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	// Start a Redis transaction
	txf := func(tx *redis.Tx) error {
		// Save the job data in the jobs hash
		if err := tx.HSet(ctx, q.jobsHash, job.ID, jobBytes).Err(); err != nil {
			return err
		}

		// Add job to the pending queue
		if err := tx.LPush(ctx, q.pendingQueue, job.ID).Err(); err != nil {
			return err
		}

		// Set expiration on the job data
		return tx.Expire(ctx, q.jobsHash, q.jobTTL).Err()
	}

	// Execute the transaction
	err = q.client.Watch(ctx, txf, q.jobsHash, q.pendingQueue)
	if err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	return nil
}

// Dequeue removes and returns the next job from the queue
func (q *RedisQueue) Dequeue() (*Job, error) {
	ctx := context.Background()

	// First, check for retryable jobs that are ready
	now := time.Now().Unix()

	// Get job IDs ready for retry (score <= current time)
	retryIDs, err := q.client.ZRangeByScore(ctx, q.retryingZSet, &redis.ZRangeBy{
		Min:   "0",
		Max:   fmt.Sprintf("%d", now),
		Count: 1,
	}).Result()

	var jobID string

	if err != nil {
		return nil, fmt.Errorf("failed to check retrying jobs: %w", err)
	}

	// If we have a retryable job, use it
	if len(retryIDs) > 0 {
		jobID = retryIDs[0]

		// Remove it from the retry set
		if err := q.client.ZRem(ctx, q.retryingZSet, jobID).Err(); err != nil {
			return nil, fmt.Errorf("failed to remove job from retry set: %w", err)
		}
	} else {
		// Otherwise, try to get a job from the pending queue
		jobID, err = q.client.RPop(ctx, q.pendingQueue).Result()
		if err != nil {
			if err == redis.Nil {
				return nil, ErrQueueEmpty
			}
			return nil, fmt.Errorf("failed to pop job from queue: %w", err)
		}
	}

	// Start a Redis transaction
	txf := func(tx *redis.Tx) error {
		// Get job data
		jobBytes, err := tx.HGet(ctx, q.jobsHash, jobID).Result()
		if err != nil {
			if err == redis.Nil {
				return ErrJobNotFound
			}
			return err
		}

		// Add job to processing set with expiration
		if err := tx.SAdd(ctx, q.processingSet, jobID).Err(); err != nil {
			return err
		}

		// Set expiration on processing set
		return tx.Expire(ctx, q.processingSet, q.processingLock).Err()
	}

	// Execute the transaction
	err = q.client.Watch(ctx, txf, q.jobsHash, q.processingSet)
	if err != nil {
		return nil, fmt.Errorf("failed to process dequeued job: %w", err)
	}

	// Get the job data
	jobBytes, err := q.client.HGet(ctx, q.jobsHash, jobID).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("failed to get job data: %w", err)
	}

	// Deserialize the job
	var job Job
	if err := json.Unmarshal([]byte(jobBytes), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	return &job, nil
}

// Get retrieves a job by ID
func (q *RedisQueue) Get(id string) (*Job, error) {
	ctx := context.Background()

	// Get job data from hash
	jobBytes, err := q.client.HGet(ctx, q.jobsHash, id).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("failed to get job data: %w", err)
	}

	// Deserialize the job
	var job Job
	if err := json.Unmarshal([]byte(jobBytes), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	return &job, nil
}

// Update updates a job in the queue
func (q *RedisQueue) Update(job *Job) error {
	ctx := context.Background()

	// Serialize the job
	jobBytes, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	// Start a Redis transaction
	txf := func(tx *redis.Tx) error {
		// Update job data in hash
		if err := tx.HSet(ctx, q.jobsHash, job.ID, jobBytes).Err(); err != nil {
			return err
		}

		// Update job status sets
		switch job.Status {
		case JobStatusCompleted:
			// Remove from processing set
			if err := tx.SRem(ctx, q.processingSet, job.ID).Err(); err != nil {
				return err
			}

			// Add to completed set
			if err := tx.SAdd(ctx, q.completedSet, job.ID).Err(); err != nil {
				return err
			}

			// Set expiration on completed jobs
			if err := tx.Expire(ctx, q.completedSet, q.completedTTL).Err(); err != nil {
				return err
			}

		case JobStatusFailed:
			// Remove from processing set
			if err := tx.SRem(ctx, q.processingSet, job.ID).Err(); err != nil {
				return err
			}

			// Add to failed set
			if err := tx.SAdd(ctx, q.failedSet, job.ID).Err(); err != nil {
				return err
			}

		case JobStatusRetrying:
			// Remove from processing set
			if err := tx.SRem(ctx, q.processingSet, job.ID).Err(); err != nil {
				return err
			}

			// If job has a next retry time, add to retrying zset
			if job.NextRetryAt != nil {
				score := float64(job.NextRetryAt.Unix())
				if err := tx.ZAdd(ctx, q.retryingZSet, &redis.Z{
					Score:  score,
					Member: job.ID,
				}).Err(); err != nil {
					return err
				}
			}
		}

		// Set expiration on job data
		return tx.Expire(ctx, q.jobsHash, q.jobTTL).Err()
	}

	// Execute the transaction
	err = q.client.Watch(ctx, txf, q.jobsHash, q.processingSet, q.completedSet, q.failedSet, q.retryingZSet)
	if err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	return nil
}

// Delete removes a job from the queue
func (q *RedisQueue) Delete(id string) error {
	ctx := context.Background()

	// Start a Redis transaction
	txf := func(tx *redis.Tx) error {
		// Remove job from all sets
		if err := tx.SRem(ctx, q.processingSet, id).Err(); err != nil {
			return err
		}

		if err := tx.SRem(ctx, q.completedSet, id).Err(); err != nil {
			return err
		}

		if err := tx.SRem(ctx, q.failedSet, id).Err(); err != nil {
			return err
		}

		if err := tx.ZRem(ctx, q.retryingZSet, id).Err(); err != nil {
			return err
		}

		// Remove job data from hash
		return tx.HDel(ctx, q.jobsHash, id).Err()
	}

	// Execute the transaction
	err := q.client.Watch(ctx, txf, q.jobsHash, q.processingSet, q.completedSet, q.failedSet, q.retryingZSet)
	if err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}

	return nil
}

// Close closes the Redis connection
func (q *RedisQueue) Close() error {
	return q.client.Close()
}

// Recovery performs recovery of stuck jobs
// This should be called during service startup
func (q *RedisQueue) Recovery() error {
	ctx := context.Background()

	// Get all jobs in the processing set
	processingJobs, err := q.client.SMembers(ctx, q.processingSet).Result()
	if err != nil {
		return fmt.Errorf("failed to get processing jobs: %w", err)
	}

	// Start a Redis transaction
	txf := func(tx *redis.Tx) error {
		// Move all processing jobs back to the pending queue
		for _, jobID := range processingJobs {
			// Add job back to the beginning of the pending queue
			if err := tx.RPush(ctx, q.pendingQueue, jobID).Err(); err != nil {
				return err
			}

			// Remove job from processing set
			if err := tx.SRem(ctx, q.processingSet, jobID).Err(); err != nil {
				return err
			}

			// Update job status to pending
			jobBytes, err := tx.HGet(ctx, q.jobsHash, jobID).Result()
			if err != nil {
				if err == redis.Nil {
					continue
				}
				return err
			}

			var job Job
			if err := json.Unmarshal([]byte(jobBytes), &job); err != nil {
				return err
			}

			job.Status = JobStatusPending
			job.UpdatedAt = time.Now()

			updatedJobBytes, err := json.Marshal(job)
			if err != nil {
				return err
			}

			if err := tx.HSet(ctx, q.jobsHash, jobID, updatedJobBytes).Err(); err != nil {
				return err
			}
		}

		return nil
	}

	// Execute the transaction
	if len(processingJobs) > 0 {
		err = q.client.Watch(ctx, txf, q.jobsHash, q.pendingQueue, q.processingSet)
		if err != nil {
			return fmt.Errorf("failed to recover processing jobs: %w", err)
		}
	}

	return nil
}
