package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/sboy99/nektar/internal/platform/logger"
	"github.com/sboy99/nektar/internal/platform/metrics"
	"github.com/sboy99/nektar/internal/platform/retry"
	"github.com/sboy99/nektar/internal/ports/eventbus"
	"github.com/sboy99/nektar/shared/events"
)

const (
	fieldEventName = "event_name"
	fieldPayload   = "payload"
)

// Bus implements EventBus using Redis Streams with consumer groups.
type Bus struct {
	client       *goredis.Client
	streamPrefix string
	groupName    string
	consumerName string
	logger       *slog.Logger
	metrics      *metrics.Registry
	retry        retry.Policy

	mu       sync.Mutex
	handlers map[string][]eventbus.Handler
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// BusConfig configures the Redis event bus.
type BusConfig struct {
	Client       *goredis.Client
	StreamPrefix string
	GroupName    string
	ConsumerName string
	Logger       *slog.Logger
	Metrics      *metrics.Registry
	Retry        retry.Policy
}

// NewBus creates a Redis Streams event bus.
func NewBus(cfg BusConfig) *Bus {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Bus{
		client:       cfg.Client,
		streamPrefix: cfg.StreamPrefix,
		groupName:    cfg.GroupName,
		consumerName: cfg.ConsumerName,
		logger:       cfg.Logger,
		metrics:      cfg.Metrics,
		retry:        cfg.Retry,
		handlers:     make(map[string][]eventbus.Handler),
	}
}

func (b *Bus) streamKey(topic string) string {
	return fmt.Sprintf("%s:%s", b.streamPrefix, topic)
}

func (b *Bus) dlqKey(topic string) string {
	return fmt.Sprintf("%s:%s:dlq", b.streamPrefix, topic)
}

// Publish adds an event to the topic stream.
func (b *Bus) Publish(ctx context.Context, event eventbus.Event) error {
	payload, err := json.Marshal(event.Payload())
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}

	_, err = b.client.XAdd(ctx, &goredis.XAddArgs{
		Stream: b.streamKey(event.Name()),
		Values: map[string]any{
			fieldEventName: event.Name(),
			fieldPayload:   string(payload),
		},
	}).Result()
	if err != nil {
		return fmt.Errorf("xadd: %w", err)
	}
	if b.metrics != nil {
		b.metrics.EventsPublished.WithLabelValues(event.Name()).Inc()
	}
	return nil
}

// Subscribe registers a handler and starts a background consumer for the topic.
func (b *Bus) Subscribe(ctx context.Context, topic string, handler eventbus.Handler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	stream := b.streamKey(topic)
	if err := b.ensureGroup(ctx, stream); err != nil {
		return err
	}

	b.handlers[topic] = append(b.handlers[topic], handler)

	if b.cancel == nil {
		consumerCtx, cancel := context.WithCancel(context.Background())
		b.cancel = cancel
		b.wg.Add(1)
		go b.consumeLoop(consumerCtx)
	}

	return nil
}

func (b *Bus) ensureGroup(ctx context.Context, stream string) error {
	err := b.client.XGroupCreateMkStream(ctx, stream, b.groupName, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("create consumer group: %w", err)
	}
	return nil
}

func (b *Bus) consumeLoop(ctx context.Context) {
	defer b.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		b.mu.Lock()
		topics := make([]string, 0, len(b.handlers))
		for topic := range b.handlers {
			topics = append(topics, topic)
		}
		b.mu.Unlock()

		if len(topics) == 0 {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// XREADGROUP STREAMS requires all keys first, then all IDs:
		// STREAMS key1 key2 ... id1 id2 ...
		streams := make([]string, 0, len(topics)*2)
		for _, topic := range topics {
			streams = append(streams, b.streamKey(topic))
		}
		for range topics {
			streams = append(streams, ">")
		}

		results, err := b.client.XReadGroup(ctx, &goredis.XReadGroupArgs{
			Group:    b.groupName,
			Consumer: b.consumerName,
			Streams:  streams,
			Count:    10,
			Block:    2 * time.Second,
		}).Result()
		if err != nil {
			if err == goredis.Nil || ctx.Err() != nil {
				continue
			}
			b.logger.Error("xreadgroup failed", "error", err)
			time.Sleep(time.Second)
			continue
		}

		for _, stream := range results {
			topic := b.topicFromStream(stream.Stream)
			for _, msg := range stream.Messages {
				b.processMessage(ctx, topic, msg)
			}
		}
	}
}

func (b *Bus) topicFromStream(stream string) string {
	prefix := b.streamPrefix + ":"
	if len(stream) > len(prefix) {
		return stream[len(prefix):]
	}
	return stream
}

func (b *Bus) processMessage(ctx context.Context, topic string, msg goredis.XMessage) {
	b.mu.Lock()
	handlers := append([]eventbus.Handler(nil), b.handlers[topic]...)
	b.mu.Unlock()

	eventName, _ := msg.Values[fieldEventName].(string)
	payloadStr, _ := msg.Values[fieldPayload].(string)

	var payload any
	_ = json.Unmarshal([]byte(payloadStr), &payload)

	event := &redisEvent{name: eventName, payload: payload}
	corrID := events.CorrelationIDFromPayload(payload)
	log := logger.WithCorrelation(b.logger, corrID)

	var lastErr error
	err := retry.Do(ctx, b.retry, func() error {
		lastErr = nil
		for _, h := range handlers {
			if err := h(ctx, event); err != nil {
				lastErr = err
				log.Warn("handler failed",
					"topic", topic,
					"event", eventName,
					"error", err,
				)
				return err
			}
		}
		return nil
	})
	if err != nil {
		lastErr = err
	}

	stream := b.streamKey(topic)
	if lastErr != nil {
		if b.metrics != nil {
			b.metrics.HandlerErrors.WithLabelValues(topic).Inc()
		}
		b.moveToDLQ(ctx, topic, msg, lastErr, log)
	} else if b.metrics != nil {
		b.metrics.EventsHandled.WithLabelValues(topic).Inc()
	}

	if err := b.client.XAck(ctx, stream, b.groupName, msg.ID).Err(); err != nil {
		log.Error("xack failed", "stream", stream, "id", msg.ID, "error", err)
	}
}

func (b *Bus) moveToDLQ(ctx context.Context, topic string, msg goredis.XMessage, handlerErr error, log *slog.Logger) {
	_, err := b.client.XAdd(ctx, &goredis.XAddArgs{
		Stream: b.dlqKey(topic),
		Values: map[string]any{
			"original_id": msg.ID,
			"event_name":  msg.Values[fieldEventName],
			"payload":     msg.Values[fieldPayload],
			"error":       handlerErr.Error(),
		},
	}).Result()
	if err != nil {
		log.Error("failed to move message to dlq", "topic", topic, "error", err)
	}
}

// ReplayDLQ re-publishes up to limitPerTopic messages from each topic's DLQ
// back onto the main stream and removes them from the DLQ.
func (b *Bus) ReplayDLQ(ctx context.Context, topics []string, limitPerTopic int) (int, error) {
	if limitPerTopic <= 0 {
		limitPerTopic = 100
	}
	total := 0
	for _, topic := range topics {
		dlq := b.dlqKey(topic)
		msgs, err := b.client.XRangeN(ctx, dlq, "-", "+", int64(limitPerTopic)).Result()
		if err != nil {
			return total, fmt.Errorf("xrange dlq %s: %w", topic, err)
		}
		for _, msg := range msgs {
			eventName, _ := msg.Values["event_name"].(string)
			payload, _ := msg.Values["payload"].(string)
			if eventName == "" {
				eventName, _ = msg.Values[fieldEventName].(string)
			}
			if payload == "" {
				payload, _ = msg.Values[fieldPayload].(string)
			}
			if _, err := b.client.XAdd(ctx, &goredis.XAddArgs{
				Stream: b.streamKey(topic),
				Values: map[string]any{
					fieldEventName: eventName,
					fieldPayload:   payload,
				},
			}).Result(); err != nil {
				return total, fmt.Errorf("xadd replay %s: %w", topic, err)
			}
			if err := b.client.XDel(ctx, dlq, msg.ID).Err(); err != nil {
				return total, fmt.Errorf("xdel dlq %s: %w", topic, err)
			}
			total++
		}
	}
	return total, nil
}

// Trim removes stream entries older than olderThan using XTRIM MINID.
func (b *Bus) Trim(ctx context.Context, topics []string, olderThan time.Duration) (int, error) {
	if olderThan <= 0 {
		return 0, nil
	}
	minID := fmt.Sprintf("%d-0", time.Now().Add(-olderThan).UnixMilli())
	total := 0
	for _, topic := range topics {
		n, err := b.client.XTrimMinIDApprox(ctx, b.streamKey(topic), minID, 0).Result()
		if err != nil {
			return total, fmt.Errorf("xtrim %s: %w", topic, err)
		}
		total += int(n)
		dlqN, err := b.client.XTrimMinIDApprox(ctx, b.dlqKey(topic), minID, 0).Result()
		if err != nil {
			return total, fmt.Errorf("xtrim dlq %s: %w", topic, err)
		}
		total += int(dlqN)
	}
	return total, nil
}

// Close stops consumers and closes the Redis client.
func (b *Bus) Close() error {
	if b.cancel != nil {
		b.cancel()
	}
	b.wg.Wait()
	return b.client.Close()
}

type redisEvent struct {
	name    string
	payload any
}

func (e *redisEvent) Name() string { return e.name }
func (e *redisEvent) Payload() any { return e.payload }
