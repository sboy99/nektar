package retry

import (
	"context"
	"time"
)

// Policy describes exponential backoff retries.
type Policy struct {
	MaxAttempts int
	BaseBackoff time.Duration
}

// Do runs fn up to MaxAttempts times with exponential backoff between failures.
func Do(ctx context.Context, p Policy, fn func() error) error {
	p = p.normalized()
	var lastErr error
	for attempt := 1; attempt <= p.MaxAttempts; attempt++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt == p.MaxAttempts {
			break
		}
		backoff := p.BaseBackoff * time.Duration(1<<(attempt-1))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
	return lastErr
}

func (p Policy) normalized() Policy {
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = 3
	}
	if p.BaseBackoff <= 0 {
		p.BaseBackoff = 100 * time.Millisecond
	}
	return p
}
