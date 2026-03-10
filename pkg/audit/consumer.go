package audit

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/milan604/core-lab/pkg/config"
	"github.com/milan604/core-lab/pkg/logger"
	"github.com/segmentio/kafka-go"
)

const (
	defaultAuditConsumerGroupID = "platform-audit-sink"
	defaultAuditRetryBackoff    = 2 * time.Second
)

type Store interface {
	Append(ctx context.Context, event Event) error
}

type Consumer interface {
	Close() error
}

type NoopConsumer struct{}

func (NoopConsumer) Close() error { return nil }

type KafkaConsumerConfig struct {
	Enabled      bool
	Topic        string
	GroupID      string
	RetryBackoff time.Duration
}

type KafkaConsumer struct {
	log          logger.LogManager
	reader       *kafka.Reader
	store        Store
	retryBackoff time.Duration

	closeOnce sync.Once
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func NewKafkaConsumer(log logger.LogManager, brokers []string, store Store, cfg KafkaConsumerConfig) Consumer {
	if !cfg.Enabled || store == nil {
		return NoopConsumer{}
	}

	normalizedBrokers := normalizeBrokerList(brokers)
	if len(normalizedBrokers) == 0 {
		if log != nil {
			log.WarnF("KafkaBrokers not configured; audit consumer will not start")
		}
		return NoopConsumer{}
	}

	if strings.TrimSpace(cfg.Topic) == "" {
		cfg.Topic = DefaultTopic
	}
	if strings.TrimSpace(cfg.GroupID) == "" {
		cfg.GroupID = defaultAuditConsumerGroupID
	}
	if cfg.RetryBackoff <= 0 {
		cfg.RetryBackoff = defaultAuditRetryBackoff
	}

	ctx, cancel := context.WithCancel(context.Background())
	consumer := &KafkaConsumer{
		log: log,
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: normalizedBrokers,
			Topic:   cfg.Topic,
			GroupID: cfg.GroupID,
		}),
		store:        store,
		retryBackoff: cfg.RetryBackoff,
		cancel:       cancel,
	}

	consumer.wg.Add(1)
	go consumer.run(ctx)

	return consumer
}

func NewKafkaConsumerFromConfig(log logger.LogManager, cfg *config.Config, store Store) Consumer {
	if cfg == nil {
		return NoopConsumer{}
	}

	return NewKafkaConsumer(log, auditBrokerList(cfg), store, KafkaConsumerConfig{
		Enabled:      cfg.GetBoolD("AuditConsumerEnabled", true),
		Topic:        strings.TrimSpace(cfg.GetStringD("AuditConsumerTopic", cfg.GetStringD("AuditTopic", DefaultTopic))),
		GroupID:      strings.TrimSpace(cfg.GetStringD("AuditConsumerGroupID", defaultAuditConsumerGroupID)),
		RetryBackoff: cfg.GetDurationD("AuditConsumerRetryBackoff", defaultAuditRetryBackoff),
	})
}

func (c *KafkaConsumer) Close() error {
	if c == nil {
		return nil
	}

	var closeErr error
	c.closeOnce.Do(func() {
		c.cancel()
		c.wg.Wait()
		closeErr = c.reader.Close()
	})
	return closeErr
}

func (c *KafkaConsumer) run(ctx context.Context) {
	defer c.wg.Done()

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if c.log != nil {
				c.log.ErrorFCtx(ctx, "failed to fetch audit message: %v", err)
			}
			time.Sleep(c.retryBackoff)
			continue
		}

		var event Event
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			if c.log != nil {
				c.log.ErrorFCtx(ctx, "failed to decode audit message at offset %d: %v", msg.Offset, err)
			}
			_ = c.reader.CommitMessages(ctx, msg)
			continue
		}

		for {
			if err := c.store.Append(ctx, event); err != nil {
				if ctx.Err() != nil {
					return
				}
				if c.log != nil {
					c.log.ErrorFCtx(ctx, "failed to append audit event %s: %v", event.EventID, err)
				}
				time.Sleep(c.retryBackoff)
				continue
			}
			break
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil && c.log != nil {
			c.log.ErrorFCtx(ctx, "failed to commit audit message at offset %d: %v", msg.Offset, err)
		}
	}
}
