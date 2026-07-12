package gemini

import (
	"context"
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	if got := estimateTokens(""); got != 1 {
		t.Fatalf("empty text: got %d, want 1", got)
	}
	if got := estimateTokens("abcd"); got != 1 {
		t.Fatalf("4 chars: got %d, want 1", got)
	}
	if got := estimateTokens("abcdefgh"); got != 2 {
		t.Fatalf("8 chars: got %d, want 2", got)
	}
}

func TestSummarizeRejectsEmptyText(t *testing.T) {
	p := &Provider{model: "gemini-2.0-flash"}
	_, err := p.Summarize(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty text")
	}
	_, err = p.Summarize(context.Background(), "   ")
	if err == nil {
		t.Fatal("expected error for whitespace text")
	}
}

func TestEmbedRejectsEmptyText(t *testing.T) {
	p := &Provider{embedModel: "text-embedding-004", dimensions: 768}
	_, err := p.Embed(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}
