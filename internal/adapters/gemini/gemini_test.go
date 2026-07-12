package gemini

import "testing"

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
