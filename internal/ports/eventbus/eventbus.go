package eventbus

import "context"

// Event is a domain event published on the bus.
type Event interface {
	Name() string
	Payload() any
}

// Handler processes a single event.
type Handler func(ctx context.Context, event Event) error

// EventBus is the messaging port. Implementations may use Redis Streams,
// Kafka, NATS, or in-memory dispatch.
type EventBus interface {
	Publish(ctx context.Context, event Event) error
	Subscribe(ctx context.Context, topic string, handler Handler) error
	Close() error
}
