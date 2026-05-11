package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/milan604/core-lab/pkg/apperr"
	"github.com/milan604/core-lab/pkg/auth"
	"github.com/milan604/core-lab/pkg/logger"
	coretenant "github.com/milan604/core-lab/pkg/tenant"
)

// Manager runs the background worker pool and coordinates job execution.
type Manager struct {
	cfg      Config
	log      logger.LogManager
	store    Store
	metrics  *metrics
	handlers map[string]*handlerRegistration

	workCh chan Job

	mu        sync.RWMutex
	startedAt time.Time
	running   bool
	cancel    context.CancelFunc
	wg        sync.WaitGroup

	activeWorkers atomic.Int64
}

// NewManager returns a manager backed by the provided store. If store is nil,
// a MemoryStore is used.
func NewManager(cfg Config, store Store) (*Manager, error) {
	cfg = normalizeConfig(cfg)
	logMgr := cfg.Logger
	if logMgr == nil {
		logMgr = logger.MustNewDefaultLogger()
	}

	if store == nil {
		store = NewMemoryStore()
	}

	metrics, err := newMetrics(cfg.Name, cfg.Registerer)
	if err != nil {
		return nil, err
	}

	return &Manager{
		cfg:      cfg,
		log:      logMgr,
		store:    store,
		metrics:  metrics,
		handlers: make(map[string]*handlerRegistration),
		workCh:   make(chan Job, cfg.QueueBuffer),
	}, nil
}

// RegisterHandler registers a job handler and its defaults.
func (m *Manager) RegisterHandler(jobType string, handler HandlerFunc, opts ...HandlerOption) error {
	if jobType == "" {
		return apperr.New(apperr.ErrorCodeInvalidInput).
			WithMessage("job type is required")
	}
	if handler == nil {
		return apperr.New(apperr.ErrorCodeInvalidInput).
			WithMessage("job handler is required")
	}

	reg := &handlerRegistration{
		HandlerInfo: HandlerInfo{
			Type:        jobType,
			Queue:       m.cfg.DefaultQueue,
			MaxAttempts: m.cfg.DefaultMaxAttempts,
			Timeout:     Duration(m.cfg.DefaultTimeout),
		},
		handler: handler,
	}
	for _, opt := range opts {
		opt(reg)
	}
	if reg.Queue == "" {
		reg.Queue = m.cfg.DefaultQueue
	}
	if reg.MaxAttempts <= 0 {
		reg.MaxAttempts = m.cfg.DefaultMaxAttempts
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[jobType] = reg
	return nil
}

// Start launches the dispatcher, janitor, and worker goroutines.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	runtimeCtx, cancel := context.WithCancel(context.Background())
	if ctx != nil {
		parent := ctx
		go func() {
			select {
			case <-parent.Done():
				cancel()
			case <-runtimeCtx.Done():
			}
		}()
	}

	m.running = true
	m.startedAt = time.Now().UTC()
	m.cancel = cancel

	m.wg.Add(1)
	go m.runDispatcher(runtimeCtx)

	if m.cfg.Retention > 0 && m.cfg.CleanupInterval > 0 {
		m.wg.Add(1)
		go m.runJanitor(runtimeCtx)
	}

	for i := 0; i < m.cfg.Workers; i++ {
		m.wg.Add(1)
		go m.runWorker(runtimeCtx, i+1)
	}

	m.log.InfoF("job manager started name=%s workers=%d", m.cfg.Name, m.cfg.Workers)
	return nil
}

// Stop stops the worker runtime and waits for goroutines to exit.
func (m *Manager) Stop(_ context.Context) error {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		if closer, ok := m.store.(interface{ Close() error }); ok {
			return closer.Close()
		}
		return nil
	}
	cancel := m.cancel
	m.running = false
	m.cancel = nil
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	m.wg.Wait()
	m.log.InfoF("job manager stopped name=%s", m.cfg.Name)

	var stopErr error
	if closer, ok := m.store.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			stopErr = errors.Join(stopErr, err)
		}
	}
	return stopErr
}

// Enqueue inserts a new job into the store.
func (m *Manager) Enqueue(ctx context.Context, req EnqueueRequest) (Job, error) {
	if req.Type == "" {
		return Job{}, apperr.New(apperr.ErrorCodeInvalidInput).
			WithMessage("job type is required").
			AddSuggestion("type", "provide a registered job type")
	}

	handler, ok := m.getHandler(req.Type)
	if !ok && !m.cfg.AllowEnqueueWithoutHandler {
		return Job{}, apperr.New(apperr.ErrorCodeInvalidInput).
			WithMessage("job type is not registered").
			AddSuggestion("type", req.Type)
	}

	now := time.Now().UTC()
	queue := req.Queue
	if queue == "" && ok {
		queue = handler.Queue
	}
	if queue == "" {
		queue = m.cfg.DefaultQueue
	}

	maxAttempts := req.MaxAttempts
	if maxAttempts <= 0 && ok {
		maxAttempts = handler.MaxAttempts
	}
	if maxAttempts <= 0 {
		maxAttempts = m.cfg.DefaultMaxAttempts
	}

	timeout := req.Timeout
	if timeout.Duration() <= 0 && ok {
		timeout = handler.Timeout
	}
	if timeout.Duration() <= 0 {
		timeout = Duration(m.cfg.DefaultTimeout)
	}

	availableAt := now
	status := StatusQueued
	if req.RunAfter != nil && req.RunAfter.After(now) {
		availableAt = req.RunAfter.UTC()
		status = StatusScheduled
	}

	job := Job{
		ID:          normalizeJobID(req.ID),
		Type:        req.Type,
		Queue:       queue,
		Status:      status,
		Payload:     append([]byte(nil), req.Payload...),
		Metadata:    cloneMetadata(req.Metadata),
		MaxAttempts: maxAttempts,
		Timeout:     timeout,
		CreatedAt:   now,
		UpdatedAt:   now,
		AvailableAt: availableAt,
	}
	if job.Metadata == nil {
		job.Metadata = make(map[string]string)
	}
	if claims, ok := auth.ClaimsFromContext(ctx); ok {
		ctx = auth.ContextWithClaims(ctx, claims)
	}
	if requestID, ok := ctx.Value(logger.RequestIDKey).(string); ok && strings.TrimSpace(requestID) != "" {
		ctx = auth.ContextWithCorrelationID(ctx, requestID)
	}
	if requestContext, ok := coretenant.RequestContextFromContext(ctx); ok {
		job.Metadata = coretenant.MergeMetadata(job.Metadata, requestContext)
	}
	if _, exists := job.Metadata["job_manager"]; !exists && m.cfg.Name != "" {
		job.Metadata["job_manager"] = m.cfg.Name
	}
	if len(job.Metadata) == 0 {
		job.Metadata = nil
	}

	created, err := m.store.Create(ctx, job)
	if err != nil {
		return Job{}, err
	}
	m.syncStoredGauge(ctx)
	if m.metrics != nil {
		m.metrics.enqueued.WithLabelValues(created.Queue, created.Type).Inc()
	}
	return created, nil
}

// GetJob returns a job by id.
func (m *Manager) GetJob(ctx context.Context, id string) (Job, error) {
	job, ok, err := m.store.Get(ctx, id)
	if err != nil {
		return Job{}, err
	}
	if !ok {
		return Job{}, notFoundJob(id)
	}
	return job, nil
}

// ListJobs returns jobs matching the filter.
func (m *Manager) ListJobs(ctx context.Context, filter JobFilter) ([]Job, error) {
	return m.store.List(ctx, filter)
}

// CancelJob cancels a queued or scheduled job.
func (m *Manager) CancelJob(ctx context.Context, id string) (Job, error) {
	job, err := m.store.Cancel(ctx, id, time.Now().UTC())
	if err != nil {
		return Job{}, err
	}
	return job, nil
}

// RetryJob requeues a failed or canceled job.
func (m *Manager) RetryJob(ctx context.Context, id string) (Job, error) {
	job, err := m.store.Retry(ctx, id, time.Now().UTC())
	if err != nil {
		return Job{}, err
	}
	return job, nil
}

// Handlers returns the registered handler metadata sorted by type.
func (m *Manager) Handlers() []HandlerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	handlers := make([]HandlerInfo, 0, len(m.handlers))
	for _, reg := range m.handlers {
		handlers = append(handlers, reg.HandlerInfo)
	}
	sort.Slice(handlers, func(i, j int) bool {
		return handlers[i].Type < handlers[j].Type
	})
	return handlers
}

// Health returns a lightweight runtime snapshot.
func (m *Manager) Health() HealthSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return HealthSnapshot{
		Manager:           m.cfg.Name,
		Running:           m.running,
		StartedAt:         m.startedAt,
		ConfiguredWorkers: m.cfg.Workers,
		ActiveWorkers:     m.activeWorkers.Load(),
	}
}

// Stats returns a detailed runtime and queue snapshot.
func (m *Manager) Stats(ctx context.Context) (StatsSnapshot, error) {
	storeStats, err := m.store.Stats(ctx)
	if err != nil {
		return StatsSnapshot{}, err
	}

	m.mu.RLock()
	startedAt := m.startedAt
	running := m.running
	m.mu.RUnlock()

	uptime := int64(0)
	if running && !startedAt.IsZero() {
		uptime = int64(time.Since(startedAt).Seconds())
	}

	return StatsSnapshot{
		Manager:            m.cfg.Name,
		StartedAt:          startedAt,
		UptimeSeconds:      uptime,
		JobsStored:         storeStats.JobsStored,
		Totals:             storeStats.Totals,
		Queues:             storeStats.Queues,
		ConfiguredWorkers:  m.cfg.Workers,
		ActiveWorkers:      m.activeWorkers.Load(),
		RegisteredHandlers: m.Handlers(),
	}, nil
}

func (m *Manager) runDispatcher(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(m.cfg.ClaimInterval)
	defer ticker.Stop()

	for {
		if err := m.dispatch(ctx); err != nil && !isContextDone(ctx.Err()) {
			m.log.WarnF("job dispatcher scan failed: %v", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (m *Manager) dispatch(ctx context.Context) error {
	availableSlots := cap(m.workCh) - len(m.workCh)
	if availableSlots <= 0 {
		return nil
	}

	claimFilter := ClaimFilter{Types: m.handlerTypes()}
	if len(claimFilter.Types) == 0 {
		return nil
	}

	claimed, err := m.store.ClaimReady(ctx, time.Now().UTC(), availableSlots, claimFilter)
	if err != nil {
		return err
	}
	for _, job := range claimed {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case m.workCh <- job:
		}
	}
	return nil
}

func (m *Manager) runWorker(ctx context.Context, workerID int) {
	defer m.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case job := <-m.workCh:
			m.activeWorkers.Add(1)
			if m.metrics != nil {
				m.metrics.runningWorkers.Inc()
			}

			if err := m.processJob(ctx, workerID, job); err != nil && !isContextDone(err) {
				m.log.WarnF("job processing failed worker=%d job_id=%s error=%v", workerID, job.ID, err)
			}

			m.activeWorkers.Add(-1)
			if m.metrics != nil {
				m.metrics.runningWorkers.Dec()
			}
		}
	}
}

func (m *Manager) processJob(ctx context.Context, workerID int, job Job) error {
	handler, ok := m.getHandler(job.Type)
	if !ok {
		return m.finalizeFailure(ctx, job, fmt.Errorf("no handler registered for job type %s", job.Type))
	}

	execCtx := ctx
	cancel := func() {}
	if timeout := job.Timeout.Duration(); timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	m.log.InfoF("processing job worker=%d job_id=%s type=%s queue=%s attempt=%d", workerID, job.ID, job.Type, job.Queue, job.Attempt)
	result, err := handler.handler(execCtx, job)
	if err != nil {
		return m.finalizeFailure(ctx, job, err)
	}

	rawResult, err := marshalResult(result)
	if err != nil {
		return m.finalizeFailure(ctx, job, err)
	}

	if _, err := m.store.MarkSucceeded(ctx, job.ID, rawResult, time.Now().UTC()); err != nil {
		return err
	}
	if m.metrics != nil {
		m.metrics.processed.WithLabelValues(job.Queue, job.Type, string(StatusSucceeded)).Inc()
	}
	m.syncStoredGauge(ctx)
	return nil
}

func (m *Manager) finalizeFailure(ctx context.Context, job Job, err error) error {
	now := time.Now().UTC()
	reason := err.Error()

	if job.Attempt < job.MaxAttempts {
		delay := m.retryDelayFor(job.Attempt)
		if _, storeErr := m.store.MarkRetry(ctx, job.ID, reason, now.Add(delay), now); storeErr != nil {
			return storeErr
		}
		m.log.WarnF("job scheduled for retry job_id=%s type=%s attempt=%d max_attempts=%d error=%v", job.ID, job.Type, job.Attempt, job.MaxAttempts, err)
		return nil
	}

	if _, storeErr := m.store.MarkFailed(ctx, job.ID, reason, now); storeErr != nil {
		return storeErr
	}
	if m.metrics != nil {
		m.metrics.processed.WithLabelValues(job.Queue, job.Type, string(StatusFailed)).Inc()
	}
	m.syncStoredGauge(ctx)
	return nil
}

func (m *Manager) runJanitor(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(m.cfg.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().UTC().Add(-m.cfg.Retention)
			removed, err := m.store.PruneTerminalBefore(ctx, cutoff)
			if err != nil {
				m.log.WarnF("job janitor failed: %v", err)
				continue
			}
			if removed > 0 {
				m.log.InfoF("job janitor pruned terminal jobs removed=%d", removed)
				m.syncStoredGauge(ctx)
			}
		}
	}
}

func (m *Manager) retryDelayFor(attempt int) time.Duration {
	if attempt <= 0 {
		return m.cfg.RetryBaseDelay
	}
	delay := m.cfg.RetryBaseDelay
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= m.cfg.RetryMaxDelay {
			return m.cfg.RetryMaxDelay
		}
	}
	if delay > m.cfg.RetryMaxDelay {
		return m.cfg.RetryMaxDelay
	}
	return delay
}

func (m *Manager) getHandler(jobType string) (*handlerRegistration, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	reg, ok := m.handlers[jobType]
	return reg, ok
}

func (m *Manager) handlerTypes() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	types := make([]string, 0, len(m.handlers))
	for jobType := range m.handlers {
		types = append(types, jobType)
	}
	sort.Strings(types)
	return types
}

func (m *Manager) syncStoredGauge(ctx context.Context) {
	if m.metrics == nil {
		return
	}
	stats, err := m.store.Stats(ctx)
	if err != nil {
		return
	}
	m.metrics.stored.Set(float64(stats.JobsStored))
}

func normalizeConfig(cfg Config) Config {
	defaults := DefaultConfig()

	if cfg.Name == "" {
		cfg.Name = defaults.Name
	}
	if cfg.DefaultQueue == "" {
		cfg.DefaultQueue = defaults.DefaultQueue
	}
	if cfg.Workers <= 0 {
		cfg.Workers = defaults.Workers
	}
	if cfg.QueueBuffer <= 0 {
		cfg.QueueBuffer = defaults.QueueBuffer
	}
	if cfg.ClaimInterval <= 0 {
		cfg.ClaimInterval = defaults.ClaimInterval
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = defaults.CleanupInterval
	}
	if cfg.Retention <= 0 {
		cfg.Retention = defaults.Retention
	}
	if cfg.DefaultMaxAttempts <= 0 {
		cfg.DefaultMaxAttempts = defaults.DefaultMaxAttempts
	}
	if cfg.DefaultTimeout <= 0 {
		cfg.DefaultTimeout = defaults.DefaultTimeout
	}
	if cfg.RetryBaseDelay <= 0 {
		cfg.RetryBaseDelay = defaults.RetryBaseDelay
	}
	if cfg.RetryMaxDelay <= 0 {
		cfg.RetryMaxDelay = defaults.RetryMaxDelay
	}
	return cfg
}

func marshalResult(result any) ([]byte, error) {
	if result == nil {
		return nil, nil
	}
	return json.Marshal(result)
}

func cloneMetadata(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func isContextDone(err error) bool {
	return err == context.Canceled || err == context.DeadlineExceeded
}
