package logger

import (
	"log/slog"
	"testing"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"":        slog.LevelInfo,
		"info":    slog.LevelInfo,
		"DEBUG":   slog.LevelDebug,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
		"nope":    slog.LevelInfo,
	}
	for in, want := range cases {
		if got := parseLevel(in); got != want {
			t.Fatalf("parseLevel(%q)=%v want %v", in, got, want)
		}
	}
}

func TestWithCorrelation(t *testing.T) {
	base := New("info")
	if WithCorrelation(base, "") != base {
		t.Fatal("empty id should return same logger")
	}
	enriched := WithCorrelation(base, "corr-1")
	if enriched == nil || enriched == base {
		t.Fatal("expected enriched logger")
	}
}
