package redis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/sboy99/nektar/internal/platform/retry"
	porteventbus "github.com/sboy99/nektar/internal/ports/eventbus"
	"github.com/sboy99/nektar/shared/events"
)

func testBus(t *testing.T) (*Bus, *goredis.Client) {
	t.Helper()
	addr := os.Getenv("NEKTAR_REDIS_ADDR")
	if addr == "" {
		t.Skip("NEKTAR_REDIS_ADDR not set")
	}
	client := NewClient(addr, "", 0)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("redis ping: %v", err)
	}

	prefix := fmt.Sprintf("nektar-test-%d", time.Now().UnixNano())
	bus := NewBus(BusConfig{
		Client:       client,
		StreamPrefix: prefix,
		GroupName:    "nektar-test",
		ConsumerName: "consumer-1",
		Retry: retry.Policy{
			MaxAttempts: 2,
			BaseBackoff: 10 * time.Millisecond,
		},
	})
	t.Cleanup(func() {
		_ = bus.Close()
		cleanupStreams(t, client, prefix)
	})
	return bus, client
}

func cleanupStreams(t *testing.T, client *goredis.Client, prefix string) {
	t.Helper()
	ctx := context.Background()
	topics := []string{events.TopicEmailFetched, events.TopicEmailDetected}
	for _, topic := range topics {
		_ = client.Del(ctx, fmt.Sprintf("%s:%s", prefix, topic)).Err()
		_ = client.Del(ctx, fmt.Sprintf("%s:%s:dlq", prefix, topic)).Err()
	}
}

func TestBusPublishSubscribe(t *testing.T) {
	bus, _ := testBus(t)
	ctx := context.Background()

	var (
		mu       sync.Mutex
		received []string
		done     = make(chan struct{})
	)
	err := bus.Subscribe(ctx, events.TopicEmailFetched, func(_ context.Context, event porteventbus.Event) error {
		mu.Lock()
		received = append(received, event.Name())
		mu.Unlock()
		select {
		case <-done:
		default:
			close(done)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	if err := bus.Publish(ctx, events.EmailFetched{
		UserID:  "user-1",
		EmailID: "email-1",
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for handler")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 || received[0] != events.TopicEmailFetched {
		t.Fatalf("received: %+v", received)
	}
}

func TestBusFailingHandlerMovesToDLQ(t *testing.T) {
	bus, client := testBus(t)
	ctx := context.Background()

	done := make(chan struct{})
	err := bus.Subscribe(ctx, events.TopicEmailDetected, func(_ context.Context, _ porteventbus.Event) error {
		select {
		case <-done:
		default:
			close(done)
		}
		return errors.New("forced failure")
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	if err := bus.Publish(ctx, events.EmailDetected{
		UserID:  "user-1",
		EmailID: "email-1",
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for handler")
	}

	dlq := fmt.Sprintf("%s:%s:dlq", bus.streamPrefix, events.TopicEmailDetected)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		n, err := client.XLen(ctx, dlq).Result()
		if err != nil {
			t.Fatalf("xlen dlq: %v", err)
		}
		if n >= 1 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("expected message in DLQ")
}
