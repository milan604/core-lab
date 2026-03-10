package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/milan604/core-lab/pkg/config"
	"github.com/milan604/core-lab/pkg/logger"
	"github.com/segmentio/kafka-go"
)

const (
	defaultAuditQueueSize    = 256
	defaultAuditWriteTimeout = 2 * time.Second
)

type KafkaPublisherConfig struct {
	Enabled      bool
	Topic        string
	QueueSize    int
	WriteTimeout time.Duration
}

type KafkaPublisher struct {
	log          logger.LogManager
	writer       *kafka.Writer
	queue        chan Event
	writeTimeout time.Duration

	closeOnce sync.Once
	wg        sync.WaitGroup
}

func NewKafkaPublisher(log logger.LogManager, brokers []string, cfg KafkaPublisherConfig) Publisher {
	if !cfg.Enabled {
		return NoopPublisher{}
	}

	normalizedBrokers := normalizeBrokerList(brokers)
	if len(normalizedBrokers) == 0 {
		if log != nil {
			log.WarnF("KafkaBrokers not configured; audit events will be discarded")
		}
		return NoopPublisher{}
	}

	if strings.TrimSpace(cfg.Topic) == "" {
		cfg.Topic = DefaultTopic
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = defaultAuditQueueSize
	}
	if cfg.WriteTimeout <= 0 {
		cfg.WriteTimeout = defaultAuditWriteTimeout
	}

	publisher := &KafkaPublisher{
		log: log,
		writer: &kafka.Writer{
			Addr:                   kafka.TCP(normalizedBrokers...),
			Topic:                  cfg.Topic,
			Balancer:               &kafka.LeastBytes{},
			AllowAutoTopicCreation: true,
		},
		queue:        make(chan Event, cfg.QueueSize),
		writeTimeout: cfg.WriteTimeout,
	}

	publisher.wg.Add(1)
	go publisher.run()

	return publisher
}

func NewKafkaPublisherFromConfig(log logger.LogManager, cfg *config.Config) Publisher {
	if cfg == nil {
		return NoopPublisher{}
	}

	return NewKafkaPublisher(log, auditBrokerList(cfg), KafkaPublisherConfig{
		Enabled:      cfg.GetBoolD("AuditEnabled", true),
		Topic:        strings.TrimSpace(cfg.GetStringD("AuditTopic", DefaultTopic)),
		QueueSize:    cfg.GetIntD("AuditQueueSize", defaultAuditQueueSize),
		WriteTimeout: cfg.GetDurationD("AuditWriteTimeout", defaultAuditWriteTimeout),
	})
}

func (p *KafkaPublisher) Publish(_ context.Context, event Event) error {
	if p == nil {
		return nil
	}

	select {
	case p.queue <- event:
		return nil
	default:
		err := fmt.Errorf("audit queue is full")
		if p.log != nil {
			p.log.WarnF("dropping audit event %s (%s): %v", event.Action, event.Resource, err)
		}
		return err
	}
}

func (p *KafkaPublisher) Close() error {
	if p == nil {
		return nil
	}

	var closeErr error
	p.closeOnce.Do(func() {
		close(p.queue)
		p.wg.Wait()
		closeErr = p.writer.Close()
	})
	return closeErr
}

func (p *KafkaPublisher) run() {
	defer p.wg.Done()

	for event := range p.queue {
		payload, err := json.Marshal(event)
		if err != nil {
			if p.log != nil {
				p.log.ErrorF("failed to marshal audit event %s (%s): %v", event.Action, event.Resource, err)
			}
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), p.writeTimeout)
		err = p.writer.WriteMessages(ctx, kafka.Message{
			Key:   []byte(strings.TrimSpace(event.TenantID)),
			Value: payload,
		})
		cancel()

		if err != nil && p.log != nil {
			p.log.ErrorF("failed to publish audit event %s (%s): %v", event.Action, event.Resource, err)
		}
	}
}

func auditBrokerList(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}

	raw := cfg.Get("KafkaBrokers")
	switch v := raw.(type) {
	case []string:
		return normalizeBrokerList(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return normalizeBrokerList(out)
	case string:
		return normalizeBrokerList(strings.Split(v, ","))
	default:
		return normalizeBrokerList(cfg.GetStringSlice("KafkaBrokers"))
	}
}

func normalizeBrokerList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))

	for _, value := range values {
		broker := strings.TrimSpace(value)
		if broker == "" {
			continue
		}
		if _, ok := seen[broker]; ok {
			continue
		}
		seen[broker] = struct{}{}
		out = append(out, broker)
	}

	return out
}
