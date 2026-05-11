package outbox

import (
	"context"
	"fmt"
	"strings"
	"time"

	coreevents "github.com/milan604/core-lab/pkg/events"
	"github.com/milan604/core-lab/pkg/logger"
)

const (
	defaultPollInterval   = 2 * time.Second
	defaultLeaseDuration  = 30 * time.Second
	defaultPublishTimeout = 10 * time.Second
	defaultMinRetry       = 5 * time.Second
	defaultMaxRetry       = 5 * time.Minute
	defaultBatchSize      = 25
)

// Record is the durable outbox payload persisted by authoritative services.
type Record struct {
	ID           string
	Topic        string
	Envelope     coreevents.Envelope
	Attempts     int
	AvailableAt  time.Time
	ClaimedBy    string
	ClaimedUntil time.Time
	CreatedAt    time.Time
}

// Store owns claiming and state transitions for durable outbox records.
type Store interface {
	ClaimBatch(ctx context.Context, consumer string, limit int, leaseDuration time.Duration, now time.Time) ([]Record, error)
	MarkDelivered(ctx context.Context, recordID string, deliveredAt time.Time) error
	MarkFailed(ctx context.Context, recordID string, nextAttemptAt time.Time, lastError string, failedAt time.Time) error
}

// Publisher emits a durable outbox record to the shared event transport.
type Publisher interface {
	Publish(ctx context.Context, topic string, envelope coreevents.Envelope) error
}

type Clock func() time.Time

type ProcessorOptions struct {
	Name            string
	PollInterval    time.Duration
	LeaseDuration   time.Duration
	PublishTimeout  time.Duration
	MinRetryBackoff time.Duration
	MaxRetryBackoff time.Duration
	BatchSize       int
	Logger          logger.LogManager
	Clock           Clock
}

// Processor runs a safe polling loop for durable business-event delivery.
type Processor struct {
	name           string
	store          Store
	publisher      Publisher
	logger         logger.LogManager
	clock          Clock
	pollInterval   time.Duration
	leaseDuration  time.Duration
	publishTimeout time.Duration
	minRetry       time.Duration
	maxRetry       time.Duration
	batchSize      int
}

func NewProcessor(store Store, publisher Publisher, opts ProcessorOptions) *Processor {
	if opts.Name == "" {
		opts.Name = "platform-event-outbox"
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = defaultPollInterval
	}
	if opts.LeaseDuration <= 0 {
		opts.LeaseDuration = defaultLeaseDuration
	}
	if opts.PublishTimeout <= 0 {
		opts.PublishTimeout = defaultPublishTimeout
	}
	if opts.MinRetryBackoff <= 0 {
		opts.MinRetryBackoff = defaultMinRetry
	}
	if opts.MaxRetryBackoff <= 0 {
		opts.MaxRetryBackoff = defaultMaxRetry
	}
	if opts.MaxRetryBackoff < opts.MinRetryBackoff {
		opts.MaxRetryBackoff = opts.MinRetryBackoff
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = defaultBatchSize
	}
	if opts.Clock == nil {
		opts.Clock = func() time.Time { return time.Now().UTC() }
	}

	return &Processor{
		name:           strings.TrimSpace(opts.Name),
		store:          store,
		publisher:      publisher,
		logger:         opts.Logger,
		clock:          opts.Clock,
		pollInterval:   opts.PollInterval,
		leaseDuration:  opts.LeaseDuration,
		publishTimeout: opts.PublishTimeout,
		minRetry:       opts.MinRetryBackoff,
		maxRetry:       opts.MaxRetryBackoff,
		batchSize:      opts.BatchSize,
	}
}

func (p *Processor) Run(ctx context.Context) {
	if p == nil || p.store == nil || p.publisher == nil {
		return
	}

	p.ProcessOnce(ctx)

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.ProcessOnce(ctx)
		}
	}
}

func (p *Processor) ProcessOnce(ctx context.Context) error {
	if p == nil || p.store == nil || p.publisher == nil {
		return nil
	}

	now := p.clock()
	records, err := p.store.ClaimBatch(ctx, p.name, p.batchSize, p.leaseDuration, now)
	if err != nil {
		return fmt.Errorf("claim outbox batch: %w", err)
	}
	for _, record := range records {
		p.processRecord(ctx, record)
	}
	return nil
}

func (p *Processor) processRecord(ctx context.Context, record Record) {
	if strings.TrimSpace(record.ID) == "" {
		return
	}

	now := p.clock()
	topic := strings.TrimSpace(record.Topic)
	if topic == "" {
		topic = coreevents.DefaultTopic
	}

	publishCtx, cancel := context.WithTimeout(ctx, p.publishTimeout)
	defer cancel()

	if err := p.publisher.Publish(publishCtx, topic, record.Envelope); err != nil {
		nextAttemptAt := now.Add(p.retryBackoff(record.Attempts + 1))
		markErr := p.store.MarkFailed(ctx, record.ID, nextAttemptAt, truncateError(err), now)
		if markErr != nil && p.logger != nil {
			p.logger.ErrorFCtx(ctx, "outbox processor %s failed to mark record %s as failed: %v", p.name, record.ID, markErr)
		}
		if p.logger != nil {
			p.logger.ErrorFCtx(ctx, "outbox processor %s failed to publish %s: %v", p.name, record.ID, err)
		}
		return
	}

	if err := p.store.MarkDelivered(ctx, record.ID, now); err != nil {
		if p.logger != nil {
			p.logger.ErrorFCtx(ctx, "outbox processor %s failed to mark record %s delivered: %v", p.name, record.ID, err)
		}
		return
	}

	if p.logger != nil {
		p.logger.DebugFCtx(ctx, "outbox processor %s delivered %s (%s)", p.name, record.ID, record.Envelope.EventType)
	}
}

func (p *Processor) retryBackoff(attempt int) time.Duration {
	if attempt <= 1 {
		return p.minRetry
	}
	backoff := p.minRetry
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff >= p.maxRetry {
			return p.maxRetry
		}
	}
	if backoff < p.minRetry {
		return p.minRetry
	}
	if backoff > p.maxRetry {
		return p.maxRetry
	}
	return backoff
}

func truncateError(err error) string {
	if err == nil {
		return ""
	}
	const maxLen = 2048
	msg := strings.TrimSpace(err.Error())
	if len(msg) <= maxLen {
		return msg
	}
	return msg[:maxLen]
}
