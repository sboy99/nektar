package inmemory_test

import (
	"context"
	"testing"

	"github.com/sboy99/nektar/internal/adapters/inmemory"
	porteventbus "github.com/sboy99/nektar/internal/ports/eventbus"
	"github.com/sboy99/nektar/shared/events"
)

func TestBusPublishSubscribe(t *testing.T) {
	bus := inmemory.NewBus()
	ctx := context.Background()

	var received bool
	err := bus.Subscribe(ctx, events.TopicEmailFetched, func(_ context.Context, event porteventbus.Event) error {
		received = true
		if event.Name() != events.TopicEmailFetched {
			t.Fatalf("expected topic %s, got %s", events.TopicEmailFetched, event.Name())
		}
		return nil
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	if err := bus.Publish(ctx, events.EmailFetched{EmailID: "email-1"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if !received {
		t.Fatal("handler was not called")
	}
}
