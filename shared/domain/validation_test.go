package domain_test

import (
	"testing"
	"time"

	"github.com/sboy99/nektar/shared/domain"
)

func TestNewUser(t *testing.T) {
	u, err := domain.NewUser("11111111-1111-1111-1111-111111111111", "test@example.com", "Test User")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Email != "test@example.com" {
		t.Fatalf("expected email test@example.com, got %s", u.Email)
	}
}

func TestNewUserInvalidID(t *testing.T) {
	_, err := domain.NewUser("user-1", "test@example.com", "Test User")
	if err != domain.ErrInvalidID {
		t.Fatalf("expected ErrInvalidID, got %v", err)
	}
}

func TestNewUserMissingEmail(t *testing.T) {
	_, err := domain.NewUser("11111111-1111-1111-1111-111111111111", "", "Test User")
	if err != domain.ErrInvalidEmail {
		t.Fatalf("expected ErrInvalidEmail, got %v", err)
	}
}

func TestNewEmail(t *testing.T) {
	e, err := domain.NewEmail("user-1", "msg-123", "Subject", "sender@example.com", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Stage != domain.StageFetched {
		t.Fatalf("expected stage fetched, got %s", e.Stage)
	}
}

func TestNewEmailMissingGmailMessageID(t *testing.T) {
	_, err := domain.NewEmail("user-1", "", "Subject", "sender@example.com", time.Now())
	if err != domain.ErrInvalidGmailMessageID {
		t.Fatalf("expected ErrInvalidGmailMessageID, got %v", err)
	}
}

func TestNewArticle(t *testing.T) {
	a, err := domain.NewArticle("user-1", "email-1", "Title", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Stage != domain.StageExtracted {
		t.Fatalf("expected stage extracted, got %s", a.Stage)
	}
}

func TestEmbeddingVectorDimensions(t *testing.T) {
	emb := &domain.Embedding{
		UserID:     "user-1",
		ArticleID:  "article-1",
		Vector:     []float32{0.1, 0.2},
		Dimensions: 3,
	}
	if err := emb.Validate(); err != domain.ErrInvalidVector {
		t.Fatalf("expected ErrInvalidVector, got %v", err)
	}
}

func TestDigestStatusTransitions(t *testing.T) {
	d := &domain.Digest{UserID: "user-1", PublishStatus: domain.PublishStatusDraft}

	if err := d.MarkReady(); err != nil {
		t.Fatalf("MarkReady: %v", err)
	}
	if d.PublishStatus != domain.PublishStatusReady {
		t.Fatalf("expected ready, got %s", d.PublishStatus)
	}

	now := time.Now()
	if err := d.MarkPublished(now); err != nil {
		t.Fatalf("MarkPublished: %v", err)
	}
	if d.PublishStatus != domain.PublishStatusPublished {
		t.Fatalf("expected published, got %s", d.PublishStatus)
	}
}

func TestDigestInvalidTransition(t *testing.T) {
	d := &domain.Digest{UserID: "user-1", PublishStatus: domain.PublishStatusDraft}
	if err := d.MarkPublished(time.Now()); err != domain.ErrInvalidTransition {
		t.Fatalf("expected ErrInvalidTransition, got %v", err)
	}
}

func TestPipelineStatusInvalidResourceType(t *testing.T) {
	p := &domain.PipelineStatus{
		ResourceType: "invalid",
		ResourceID:   "res-1",
		UserID:       "user-1",
	}
	if err := p.Validate(); err != domain.ErrInvalidResourceType {
		t.Fatalf("expected ErrInvalidResourceType, got %v", err)
	}
}
