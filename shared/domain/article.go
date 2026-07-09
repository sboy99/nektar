package domain

import "time"

// ContentFormat identifies the primary content representation of an article.
type ContentFormat string

const (
	ContentFormatHTML       ContentFormat = "html"
	ContentFormatMarkdown   ContentFormat = "markdown"
	ContentFormatPlainText  ContentFormat = "plain_text"
)

// Article is the aggregate root for extracted newsletter content.
type Article struct {
	ID                 string
	UserID             string
	EmailID            string
	Title              string
	RawHTML            string
	Markdown           string
	PlainText          string
	URL                string
	ReadingTimeMinutes int
	Position           int
	Stage              PipelineStage
	CreatedAt          time.Time
}
