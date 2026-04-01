package jobs

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/milan604/core-lab/pkg/apperr"
)

// Store abstracts job persistence and queue state transitions.
type Store interface {
	Create(context.Context, Job) (Job, error)
	Get(context.Context, string) (Job, bool, error)
	List(context.Context, JobFilter) ([]Job, error)
	ClaimReady(context.Context, time.Time, int) ([]Job, error)
	MarkSucceeded(context.Context, string, jsonRawResult, time.Time) (Job, error)
	MarkRetry(context.Context, string, string, time.Time, time.Time) (Job, error)
	MarkFailed(context.Context, string, string, time.Time) (Job, error)
	Cancel(context.Context, string, time.Time) (Job, error)
	Retry(context.Context, string, time.Time) (Job, error)
	PruneTerminalBefore(context.Context, time.Time) (int, error)
	Stats(context.Context) (storeStats, error)
}

type jsonRawResult = []byte

type storeStats struct {
	JobsStored int
	Totals     map[Status]int
	Queues     map[string]QueueStats
}

// MemoryStore is an in-memory Store implementation suitable for embedded
// workers, local development, and single-process job servers.
type MemoryStore struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

// NewMemoryStore returns a new in-memory job store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		jobs: make(map[string]*Job),
	}
}

// Create inserts a new job.
func (s *MemoryStore) Create(_ context.Context, job Job) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.jobs[job.ID]; exists {
		return Job{}, apperr.New(apperr.ErrorCodeInvalidInput).
			WithMessage("job id already exists").
			AddSuggestion("id", "provide a unique job id")
	}

	jobCopy := job.clone()
	s.jobs[job.ID] = &jobCopy
	return jobCopy.clone(), nil
}

// Get returns a job by id.
func (s *MemoryStore) Get(_ context.Context, id string) (Job, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, ok := s.jobs[id]
	if !ok {
		return Job{}, false, nil
	}
	return job.clone(), true, nil
}

// List returns jobs that match the filter.
func (s *MemoryStore) List(_ context.Context, filter JobFilter) ([]Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	statuses := make(map[Status]struct{}, len(filter.Statuses))
	for _, status := range filter.Statuses {
		statuses[status] = struct{}{}
	}

	result := make([]Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		if filter.Queue != "" && job.Queue != filter.Queue {
			continue
		}
		if filter.Type != "" && job.Type != filter.Type {
			continue
		}
		if len(statuses) > 0 {
			if _, ok := statuses[job.Status]; !ok {
				continue
			}
		}
		result = append(result, job.clone())
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].ID < result[j].ID
		}
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= len(result) {
		return []Job{}, nil
	}
	result = result[offset:]

	limit := filter.Limit
	if limit <= 0 || limit > len(result) {
		return result, nil
	}
	return result[:limit], nil
}

// ClaimReady transitions scheduled jobs when due and claims queued jobs for workers.
func (s *MemoryStore) ClaimReady(_ context.Context, now time.Time, limit int) ([]Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		return nil, nil
	}

	type candidate struct {
		id string
		at time.Time
	}

	candidates := make([]candidate, 0, len(s.jobs))
	for _, job := range s.jobs {
		if job.Status == StatusScheduled && !job.AvailableAt.After(now) {
			job.Status = StatusQueued
			job.UpdatedAt = now
		}
		if job.Status == StatusQueued && !job.AvailableAt.After(now) {
			candidates = append(candidates, candidate{id: job.ID, at: job.AvailableAt})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].at.Equal(candidates[j].at) {
			return candidates[i].id < candidates[j].id
		}
		return candidates[i].at.Before(candidates[j].at)
	})

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	claimed := make([]Job, 0, len(candidates))
	for _, candidate := range candidates {
		job := s.jobs[candidate.id]
		if job == nil || job.Status != StatusQueued {
			continue
		}
		startedAt := now
		job.Status = StatusRunning
		job.Attempt++
		job.StartedAt = &startedAt
		job.UpdatedAt = now
		claimed = append(claimed, job.clone())
	}
	return claimed, nil
}

// MarkSucceeded marks a job as completed successfully.
func (s *MemoryStore) MarkSucceeded(_ context.Context, id string, result jsonRawResult, now time.Time) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return Job{}, notFoundJob(id)
	}
	completedAt := now
	job.Status = StatusSucceeded
	job.Result = append(jsonRawResult(nil), result...)
	job.LastError = ""
	job.CompletedAt = &completedAt
	job.UpdatedAt = now
	return job.clone(), nil
}

// MarkRetry reschedules a failed attempt for another run.
func (s *MemoryStore) MarkRetry(_ context.Context, id, reason string, runAt, now time.Time) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return Job{}, notFoundJob(id)
	}
	job.Status = StatusScheduled
	job.LastError = reason
	job.AvailableAt = runAt
	job.CompletedAt = nil
	job.UpdatedAt = now
	return job.clone(), nil
}

// MarkFailed marks a job as terminally failed.
func (s *MemoryStore) MarkFailed(_ context.Context, id, reason string, now time.Time) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return Job{}, notFoundJob(id)
	}
	completedAt := now
	job.Status = StatusFailed
	job.LastError = reason
	job.CompletedAt = &completedAt
	job.UpdatedAt = now
	return job.clone(), nil
}

// Cancel cancels a queued or scheduled job.
func (s *MemoryStore) Cancel(_ context.Context, id string, now time.Time) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return Job{}, notFoundJob(id)
	}
	if job.Status == StatusRunning {
		return Job{}, apperr.New(apperr.ErrorCodeInvalidInput).
			WithMessage("running jobs cannot be canceled")
	}
	if job.Status.IsTerminal() {
		return Job{}, apperr.New(apperr.ErrorCodeInvalidInput).
			WithMessage("job is already in a terminal state")
	}
	completedAt := now
	job.Status = StatusCanceled
	job.CompletedAt = &completedAt
	job.UpdatedAt = now
	return job.clone(), nil
}

// Retry resets a failed or canceled job and queues it again immediately.
func (s *MemoryStore) Retry(_ context.Context, id string, now time.Time) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return Job{}, notFoundJob(id)
	}
	if !(job.Status == StatusFailed || job.Status == StatusCanceled) {
		return Job{}, apperr.New(apperr.ErrorCodeInvalidInput).
			WithMessage("only failed or canceled jobs can be retried")
	}
	job.Status = StatusQueued
	job.Attempt = 0
	job.LastError = ""
	job.Result = nil
	job.AvailableAt = now
	job.StartedAt = nil
	job.CompletedAt = nil
	job.UpdatedAt = now
	return job.clone(), nil
}

// PruneTerminalBefore deletes finished jobs older than the cutoff.
func (s *MemoryStore) PruneTerminalBefore(_ context.Context, cutoff time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	removed := 0
	for id, job := range s.jobs {
		if !job.Status.IsTerminal() || job.CompletedAt == nil {
			continue
		}
		if job.CompletedAt.Before(cutoff) {
			delete(s.jobs, id)
			removed++
		}
	}
	return removed, nil
}

// Stats returns an aggregate snapshot of the store state.
func (s *MemoryStore) Stats(_ context.Context) (storeStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := storeStats{
		JobsStored: len(s.jobs),
		Totals:     make(map[Status]int),
		Queues:     make(map[string]QueueStats),
	}
	for _, job := range s.jobs {
		stats.Totals[job.Status]++
		queueStats := stats.Queues[job.Queue]
		queueStats.Name = job.Queue
		if queueStats.Totals == nil {
			queueStats.Totals = make(map[Status]int)
		}
		queueStats.Totals[job.Status]++
		queueStats.JobsTotal++
		stats.Queues[job.Queue] = queueStats
	}
	return stats, nil
}

func notFoundJob(id string) error {
	return apperr.New(apperr.ErrorCodeNotFound).
		WithMessage("job not found").
		AddSuggestion("job_id", id)
}

func isAppError(err error) bool {
	var appErr *apperr.AppError
	return errors.As(err, &appErr)
}
