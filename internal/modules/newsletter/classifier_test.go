package newsletter

import (
	"testing"
	"time"

	"github.com/sboy99/nektar/shared/domain"
)

func baseEmail(t *testing.T) *domain.Email {
	t.Helper()
	e, err := domain.NewEmail("user-1", "msg-1", "Subject", "Sender <news@example.com>", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	e.ID = "email-1"
	e.Headers = map[string]string{}
	return e
}

func TestClassifyListIDOnly(t *testing.T) {
	e := baseEmail(t)
	e.ListID = "TLDR Tech <tldr.tech>"
	cfg := ClassifierConfig{ScoreThreshold: 0.5}

	r := Classify(e, cfg)
	if r.IsNewsletter {
		t.Fatalf("list-id alone should be below 0.5, score=%v", r.Score)
	}
	if r.Score != weightListID {
		t.Fatalf("score: got %v want %v", r.Score, weightListID)
	}
	if r.ListID != "tldr.tech" {
		t.Fatalf("list id: got %q", r.ListID)
	}
	if r.Reason != "below_threshold" {
		t.Fatalf("reason: %s", r.Reason)
	}
}

func TestClassifyListIDPassesLowerThreshold(t *testing.T) {
	e := baseEmail(t)
	e.ListID = "TLDR Tech <tldr.tech>"
	cfg := ClassifierConfig{ScoreThreshold: 0.4}

	r := Classify(e, cfg)
	if !r.IsNewsletter {
		t.Fatalf("expected newsletter, score=%v reason=%s", r.Score, r.Reason)
	}
}

func TestClassifyUnsubscribeOnlyBelowThreshold(t *testing.T) {
	e := baseEmail(t)
	e.ListUnsubscribe = "<mailto:unsub@example.com>"
	cfg := ClassifierConfig{ScoreThreshold: 0.5}

	r := Classify(e, cfg)
	if r.IsNewsletter {
		t.Fatalf("expected reject, score=%v", r.Score)
	}
	if r.Score != weightListUnsubscribe {
		t.Fatalf("score: got %v want %v", r.Score, weightListUnsubscribe)
	}
	if r.Reason != "below_threshold" {
		t.Fatalf("reason: %s", r.Reason)
	}
}

func TestClassifyListIDAndUnsubscribe(t *testing.T) {
	e := baseEmail(t)
	e.ListID = "<news.example.com>"
	e.ListUnsubscribe = "<https://example.com/unsub>"
	cfg := ClassifierConfig{ScoreThreshold: 0.5}

	r := Classify(e, cfg)
	if !r.IsNewsletter {
		t.Fatal("expected newsletter")
	}
	want := weightListID + weightListUnsubscribe
	if r.Score != want {
		t.Fatalf("score: got %v want %v", r.Score, want)
	}
}

func TestClassifyPrecedenceBulk(t *testing.T) {
	e := baseEmail(t)
	e.Headers["Precedence"] = "bulk"
	e.ListUnsubscribe = "<mailto:u@example.com>"
	cfg := ClassifierConfig{ScoreThreshold: 0.5}

	r := Classify(e, cfg)
	if !r.IsNewsletter {
		t.Fatalf("expected newsletter, score=%v", r.Score)
	}
	want := weightListUnsubscribe + weightPrecedence
	if r.Score != want {
		t.Fatalf("score: got %v want %v", r.Score, want)
	}
}

func TestClassifyBulkCampaignHeaders(t *testing.T) {
	e := baseEmail(t)
	e.Headers["X-Campaign"] = "weekly-digest"
	e.ListUnsubscribe = "<mailto:u@example.com>"
	cfg := ClassifierConfig{ScoreThreshold: 0.5}

	r := Classify(e, cfg)
	if !r.IsNewsletter {
		t.Fatalf("expected newsletter, score=%v", r.Score)
	}
}

func TestClassifyAllowlistOverride(t *testing.T) {
	e := baseEmail(t)
	e.From = "Personal <alice@friend.com>"
	cfg := ClassifierConfig{
		ScoreThreshold: 0.5,
		Allowlist:      []string{"alice@friend.com"},
	}

	r := Classify(e, cfg)
	if !r.IsNewsletter || r.Score != 1 || r.Reason != "allowlist" {
		t.Fatalf("got %+v", r)
	}
}

func TestClassifyAllowlistDomain(t *testing.T) {
	e := baseEmail(t)
	e.From = "TLDR <digest@substack.com>"
	cfg := ClassifierConfig{
		ScoreThreshold: 0.5,
		Allowlist:      []string{"@substack.com"},
	}

	r := Classify(e, cfg)
	if !r.IsNewsletter || r.Reason != "allowlist" {
		t.Fatalf("got %+v", r)
	}
}

func TestClassifyDenylistOverride(t *testing.T) {
	e := baseEmail(t)
	e.ListID = "<news.example.com>"
	e.ListUnsubscribe = "<mailto:u@example.com>"
	e.From = "Spam <spam@bad.com>"
	cfg := ClassifierConfig{
		ScoreThreshold: 0.5,
		Denylist:       []string{"spam@bad.com"},
	}

	r := Classify(e, cfg)
	if r.IsNewsletter || r.Reason != "denylist" {
		t.Fatalf("got %+v", r)
	}
}

func TestClassifySpamReject(t *testing.T) {
	e := baseEmail(t)
	e.ListID = "<news.example.com>"
	e.Headers["X-Spam-Flag"] = "YES"
	cfg := ClassifierConfig{ScoreThreshold: 0.5}

	r := Classify(e, cfg)
	if r.IsNewsletter || r.Reason != "spam" {
		t.Fatalf("got %+v", r)
	}
}

func TestClassifySpamStatusYes(t *testing.T) {
	e := baseEmail(t)
	e.Headers["X-Spam-Status"] = "Yes, score=8.0"
	cfg := ClassifierConfig{ScoreThreshold: 0.5}

	r := Classify(e, cfg)
	if r.IsNewsletter || r.Reason != "spam" {
		t.Fatalf("got %+v", r)
	}
}

func TestClassifyHeadersFallback(t *testing.T) {
	e := baseEmail(t)
	e.Headers["List-Id"] = "Weekly <weekly.news>"
	e.Headers["List-Unsubscribe"] = "<mailto:u@weekly.news>"
	cfg := ClassifierConfig{ScoreThreshold: 0.5}

	r := Classify(e, cfg)
	if !r.IsNewsletter {
		t.Fatalf("expected newsletter, got %+v", r)
	}
	if r.ListID != "weekly.news" {
		t.Fatalf("list id: %q", r.ListID)
	}
}

func TestParseListID(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"<tldr.tech>", "tldr.tech"},
		{"TLDR <tldr.tech>", "tldr.tech"},
		{"plain-id", "plain-id"},
	}
	for _, tc := range cases {
		if got := parseListID(tc.in); got != tc.want {
			t.Errorf("parseListID(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestExtractAddress(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Alice <alice@Example.COM>", "alice@example.com"},
		{"bob@test.com", "bob@test.com"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := extractAddress(tc.in); got != tc.want {
			t.Errorf("extractAddress(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}
