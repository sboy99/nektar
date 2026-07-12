package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDoSucceedsFirstAttempt(t *testing.T) {
	calls := 0
	err := Do(context.Background(), Policy{MaxAttempts: 3, BaseBackoff: time.Millisecond}, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls: got %d want 1", calls)
	}
}

func TestDoRetriesThenSucceeds(t *testing.T) {
	calls := 0
	err := Do(context.Background(), Policy{MaxAttempts: 3, BaseBackoff: time.Millisecond}, func() error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Fatalf("calls: got %d want 3", calls)
	}
}

func TestDoExhaustsAttempts(t *testing.T) {
	calls := 0
	err := Do(context.Background(), Policy{MaxAttempts: 2, BaseBackoff: time.Millisecond}, func() error {
		calls++
		return errors.New("fail")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 2 {
		t.Fatalf("calls: got %d want 2", calls)
	}
}

func TestDoRespectsContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	err := Do(ctx, Policy{MaxAttempts: 5, BaseBackoff: 50 * time.Millisecond}, func() error {
		calls++
		cancel()
		return errors.New("fail")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls: got %d want 1", calls)
	}
}
