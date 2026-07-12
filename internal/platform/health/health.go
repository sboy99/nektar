package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Check is a named readiness probe.
type Check func(ctx context.Context) error

// Checker runs readiness checks for dependent infrastructure.
type Checker struct {
	mu     sync.RWMutex
	checks map[string]Check
}

// New creates a checker from named checks.
func New(checks map[string]Check) *Checker {
	copied := make(map[string]Check, len(checks))
	for name, check := range checks {
		copied[name] = check
	}
	return &Checker{checks: copied}
}

// Ready runs all checks and returns per-check status plus overall readiness.
func (c *Checker) Ready(ctx context.Context) (map[string]string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := make(map[string]string, len(c.checks))
	ok := true
	for name, check := range c.checks {
		if err := check(ctx); err != nil {
			status[name] = err.Error()
			ok = false
			continue
		}
		status[name] = "ok"
	}
	return status, ok
}

// Handler returns an HTTP handler for the readiness probe.
func (c *Checker) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		status, ok := c.Ready(ctx)
		w.Header().Set("Content-Type", "application/json")
		if !ok {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ready":  ok,
			"checks": status,
		})
	}
}
