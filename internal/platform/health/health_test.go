package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadyAllOk(t *testing.T) {
	c := New(map[string]Check{
		"storage": func(context.Context) error { return nil },
		"cache":   func(context.Context) error { return nil },
	})
	status, ok := c.Ready(context.Background())
	if !ok {
		t.Fatalf("expected ready, status=%v", status)
	}
	if status["storage"] != "ok" || status["cache"] != "ok" {
		t.Fatalf("unexpected status: %v", status)
	}
}

func TestReadyReportsFailure(t *testing.T) {
	c := New(map[string]Check{
		"storage": func(context.Context) error { return errors.New("down") },
		"cache":   func(context.Context) error { return nil },
	})
	status, ok := c.Ready(context.Background())
	if ok {
		t.Fatal("expected not ready")
	}
	if status["storage"] != "down" {
		t.Fatalf("storage status: %q", status["storage"])
	}
}

func TestHandlerStatusCodes(t *testing.T) {
	okChecker := New(map[string]Check{
		"x": func(context.Context) error { return nil },
	})
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()
	okChecker.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", rr.Code)
	}

	failChecker := New(map[string]Check{
		"x": func(context.Context) error { return errors.New("boom") },
	})
	rr = httptest.NewRecorder()
	failChecker.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want 503", rr.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if body["ready"] != false {
		t.Fatalf("ready field: %v", body["ready"])
	}
}
