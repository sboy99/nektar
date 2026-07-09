package domain

import "time"

// Email is the aggregate root for a fetched message.
type Email struct {
	ID               string
	UserID           string
	GmailMessageID   string
	ThreadID         string
	Subject          string
	From             string
	RawBody          string
	Headers          map[string]string
	ReceivedAt       time.Time
	IsNewsletter     bool
	NewsletterScore  float64
	ListID           string
	ListUnsubscribe  string
	Stage            PipelineStage
}
