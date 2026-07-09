package domain

import "errors"

var (
	ErrInvalidID            = errors.New("domain: id is required")
	ErrInvalidUserID        = errors.New("domain: user_id is required")
	ErrInvalidEmail         = errors.New("domain: email is required")
	ErrInvalidGmailMessageID = errors.New("domain: gmail_message_id is required")
	ErrInvalidArticleID     = errors.New("domain: article_id is required")
	ErrInvalidClusterID     = errors.New("domain: cluster_id is required")
	ErrInvalidVector        = errors.New("domain: vector dimensions mismatch")
	ErrInvalidStage         = errors.New("domain: invalid pipeline stage")
	ErrInvalidTransition    = errors.New("domain: invalid status transition")
	ErrInvalidResourceType  = errors.New("domain: invalid resource type")
)
