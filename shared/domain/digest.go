package domain

import "time"

// PublishStatus tracks digest publication state.
type PublishStatus string

const (
	PublishStatusDraft     PublishStatus = "draft"
	PublishStatusReady     PublishStatus = "ready"
	PublishStatusPublished PublishStatus = "published"
	PublishStatusFailed    PublishStatus = "failed"
)

// Digest is a curated summary of clustered articles ready to publish.
type Digest struct {
	ID                 string
	UserID             string
	Title              string
	Markdown           string
	Summary            string
	ArticleIDs         []string
	ClusterIDs         []string
	ReadingTimeMinutes int
	PublishStatus      PublishStatus
	PublishedAt        *time.Time
	CreatedAt          time.Time
}

// MarkReady transitions the digest to ready for publishing.
func (d *Digest) MarkReady() error {
	if d.PublishStatus != PublishStatusDraft {
		return ErrInvalidTransition
	}
	d.PublishStatus = PublishStatusReady
	return nil
}

// MarkPublished transitions the digest to published.
func (d *Digest) MarkPublished(at time.Time) error {
	if d.PublishStatus != PublishStatusReady {
		return ErrInvalidTransition
	}
	d.PublishStatus = PublishStatusPublished
	d.PublishedAt = &at
	return nil
}

// MarkFailed transitions the digest to failed.
func (d *Digest) MarkFailed() error {
	if d.PublishStatus != PublishStatusReady {
		return ErrInvalidTransition
	}
	d.PublishStatus = PublishStatusFailed
	return nil
}
