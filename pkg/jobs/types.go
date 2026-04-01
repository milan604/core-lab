// Package jobs provides a reusable background job runtime for services that
// need in-process workers, delayed execution, retry policies, runtime stats,
// and optional HTTP administration endpoints.
package jobs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/milan604/core-lab/pkg/logger"
)

// Status represents the lifecycle state of a job.
type Status string

const (
	StatusScheduled Status = "scheduled"
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCanceled  Status = "canceled"
)

// IsTerminal reports whether the status is a terminal state.
func (s Status) IsTerminal() bool {
	switch s {
	case StatusSucceeded, StatusFailed, StatusCanceled:
		return true
	default:
		return false
	}
}

// Duration marshals time.Duration values as strings in APIs while still
// supporting numeric JSON input when needed.
type Duration time.Duration

// Duration returns the underlying time.Duration.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

func (d Duration) String() string {
	return time.Duration(d).String()
}

// MarshalJSON writes durations as strings like "30s".
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON accepts either a duration string or a numeric duration.
func (d *Duration) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*d = 0
		return nil
	}

	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		if asString == "" {
			*d = 0
			return nil
		}
		value, err := time.ParseDuration(asString)
		if err != nil {
			return err
		}
		*d = Duration(value)
		return nil
	}

	var asNumber int64
	if err := json.Unmarshal(data, &asNumber); err != nil {
		return err
	}
	*d = Duration(time.Duration(asNumber))
	return nil
}

// Job represents a queued or processed background task.
type Job struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Queue       string            `json:"queue"`
	Status      Status            `json:"status"`
	Payload     json.RawMessage   `json:"payload,omitempty"`
	Result      json.RawMessage   `json:"result,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Attempt     int               `json:"attempt"`
	MaxAttempts int               `json:"max_attempts"`
	Timeout     Duration          `json:"timeout"`
	LastError   string            `json:"last_error,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	AvailableAt time.Time         `json:"available_at"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
}

func (j Job) clone() Job {
	clone := j
	if len(j.Payload) > 0 {
		clone.Payload = append(json.RawMessage(nil), j.Payload...)
	}
	if len(j.Result) > 0 {
		clone.Result = append(json.RawMessage(nil), j.Result...)
	}
	if j.Metadata != nil {
		clone.Metadata = make(map[string]string, len(j.Metadata))
		for k, v := range j.Metadata {
			clone.Metadata[k] = v
		}
	}
	if j.StartedAt != nil {
		startedAt := *j.StartedAt
		clone.StartedAt = &startedAt
	}
	if j.CompletedAt != nil {
		completedAt := *j.CompletedAt
		clone.CompletedAt = &completedAt
	}
	return clone
}

// EnqueueRequest describes a job to be pushed into the background runtime.
type EnqueueRequest struct {
	ID          string            `json:"id,omitempty"`
	Type        string            `json:"type"`
	Queue       string            `json:"queue,omitempty"`
	Payload     json.RawMessage   `json:"payload,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	MaxAttempts int               `json:"max_attempts,omitempty"`
	Timeout     Duration          `json:"timeout,omitempty"`
	RunAfter    *time.Time        `json:"run_after,omitempty"`
}

// HandlerFunc processes a single job.
type HandlerFunc func(context.Context, Job) (any, error)

// HandlerInfo describes a registered handler and its defaults.
type HandlerInfo struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Queue       string   `json:"queue"`
	MaxAttempts int      `json:"max_attempts"`
	Timeout     Duration `json:"timeout"`
}

type handlerRegistration struct {
	HandlerInfo
	handler HandlerFunc
}

// HandlerOption configures handler registration defaults.
type HandlerOption func(*handlerRegistration)

// WithHandlerDescription adds a human-readable description for admin APIs.
func WithHandlerDescription(description string) HandlerOption {
	return func(reg *handlerRegistration) {
		reg.Description = description
	}
}

// WithHandlerQueue sets the default queue for a handler.
func WithHandlerQueue(queue string) HandlerOption {
	return func(reg *handlerRegistration) {
		reg.Queue = queue
	}
}

// WithHandlerMaxAttempts sets the default retry budget for a handler.
func WithHandlerMaxAttempts(maxAttempts int) HandlerOption {
	return func(reg *handlerRegistration) {
		reg.MaxAttempts = maxAttempts
	}
}

// WithHandlerTimeout sets the default timeout for a handler.
func WithHandlerTimeout(timeout time.Duration) HandlerOption {
	return func(reg *handlerRegistration) {
		reg.Timeout = Duration(timeout)
	}
}

// JobFilter constrains list APIs.
type JobFilter struct {
	Queue    string
	Type     string
	Statuses []Status
	Limit    int
	Offset   int
}

// ClaimFilter constrains which jobs a manager is allowed to claim from a
// shared store. This prevents one service from stealing another service's
// jobs when multiple managers share the same backend.
type ClaimFilter struct {
	Queues []string
	Types  []string
}

// QueueStats describes aggregate queue state.
type QueueStats struct {
	Name      string         `json:"name"`
	Totals    map[Status]int `json:"totals"`
	JobsTotal int            `json:"jobs_total"`
}

// StatsSnapshot describes the current manager and store state.
type StatsSnapshot struct {
	Manager            string                `json:"manager"`
	StartedAt          time.Time             `json:"started_at"`
	UptimeSeconds      int64                 `json:"uptime_seconds"`
	JobsStored         int                   `json:"jobs_stored"`
	Totals             map[Status]int        `json:"totals"`
	Queues             map[string]QueueStats `json:"queues"`
	ConfiguredWorkers  int                   `json:"configured_workers"`
	ActiveWorkers      int64                 `json:"active_workers"`
	RegisteredHandlers []HandlerInfo         `json:"registered_handlers"`
}

// HealthSnapshot is a lightweight readiness view for job runtimes.
type HealthSnapshot struct {
	Manager           string    `json:"manager"`
	Running           bool      `json:"running"`
	StartedAt         time.Time `json:"started_at,omitempty"`
	ConfiguredWorkers int       `json:"configured_workers"`
	ActiveWorkers     int64     `json:"active_workers"`
}

// Config controls the manager runtime.
type Config struct {
	Name                       string
	DefaultQueue               string
	Workers                    int
	QueueBuffer                int
	ClaimInterval              time.Duration
	CleanupInterval            time.Duration
	Retention                  time.Duration
	DefaultMaxAttempts         int
	DefaultTimeout             time.Duration
	RetryBaseDelay             time.Duration
	RetryMaxDelay              time.Duration
	AllowEnqueueWithoutHandler bool
	Logger                     logger.LogManager
	Registerer                 prometheus.Registerer
}

// DefaultConfig returns production-safe defaults for a job manager.
func DefaultConfig() Config {
	return Config{
		Name:                       "jobs",
		DefaultQueue:               "default",
		Workers:                    4,
		QueueBuffer:                256,
		ClaimInterval:              250 * time.Millisecond,
		CleanupInterval:            5 * time.Minute,
		Retention:                  24 * time.Hour,
		DefaultMaxAttempts:         3,
		DefaultTimeout:             30 * time.Second,
		RetryBaseDelay:             1 * time.Second,
		RetryMaxDelay:              30 * time.Second,
		AllowEnqueueWithoutHandler: false,
	}
}

func normalizeJobID(id string) string {
	if id != "" {
		return id
	}
	return uuid.NewString()
}
