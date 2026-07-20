package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

func requireNonEmpty(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fieldError(field)
	}
	return nil
}

func fieldError(field string) error {
	switch field {
	case "id":
		return ErrInvalidID
	case "user_id":
		return ErrInvalidUserID
	case "email":
		return ErrInvalidEmail
	case "gmail_message_id":
		return ErrInvalidGmailMessageID
	case "article_id":
		return ErrInvalidArticleID
	case "cluster_id":
		return ErrInvalidClusterID
	default:
		return ErrInvalidID
	}
}

// NewUser creates a validated User.
func NewUser(id, email, name string) (*User, error) {
	u := &User{
		ID:        id,
		Email:     email,
		Name:      name,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	return u, u.Validate()
}

func (u *User) Validate() error {
	if err := requireNonEmpty("id", u.ID); err != nil {
		return err
	}
	if _, err := uuid.Parse(strings.TrimSpace(u.ID)); err != nil {
		return ErrInvalidID
	}
	return requireNonEmpty("email", u.Email)
}

func (g *GmailSync) Validate() error {
	if err := requireNonEmpty("user_id", g.UserID); err != nil {
		return err
	}
	return nil
}

// NewEmail creates a validated Email in the fetched stage.
func NewEmail(userID, gmailMessageID, subject, from string, receivedAt time.Time) (*Email, error) {
	e := &Email{
		UserID:         userID,
		GmailMessageID: gmailMessageID,
		Subject:        subject,
		From:           from,
		ReceivedAt:     receivedAt,
		Stage:          StageFetched,
		Headers:        make(map[string]string),
	}
	return e, e.Validate()
}

func (e *Email) Validate() error {
	if err := requireNonEmpty("user_id", e.UserID); err != nil {
		return err
	}
	return requireNonEmpty("gmail_message_id", e.GmailMessageID)
}

// NewArticle creates a validated Article in the extracted stage.
func NewArticle(userID, emailID, title string, position int) (*Article, error) {
	a := &Article{
		UserID:    userID,
		EmailID:   emailID,
		Title:     title,
		Position:  position,
		Stage:     StageExtracted,
		CreatedAt: time.Now().UTC(),
	}
	return a, a.Validate()
}

func (a *Article) Validate() error {
	if err := requireNonEmpty("user_id", a.UserID); err != nil {
		return err
	}
	if err := requireNonEmpty("id", a.ID); err != nil && a.ID != "" {
		return err
	}
	if strings.TrimSpace(a.EmailID) == "" {
		return ErrInvalidID
	}
	return nil
}

func (emb *Embedding) Validate() error {
	if err := requireNonEmpty("user_id", emb.UserID); err != nil {
		return err
	}
	if err := requireNonEmpty("article_id", emb.ArticleID); err != nil {
		return err
	}
	if emb.Dimensions > 0 && len(emb.Vector) != emb.Dimensions {
		return ErrInvalidVector
	}
	return nil
}

func (c *Cluster) Validate() error {
	return requireNonEmpty("user_id", c.UserID)
}

func (t *Topic) Validate() error {
	if err := requireNonEmpty("user_id", t.UserID); err != nil {
		return err
	}
	return requireNonEmpty("cluster_id", t.ClusterID)
}

func (d *Digest) Validate() error {
	return requireNonEmpty("user_id", d.UserID)
}

func (p *PipelineStatus) Validate() error {
	if err := requireNonEmpty("user_id", p.UserID); err != nil {
		return err
	}
	if err := requireNonEmpty("id", p.ResourceID); err != nil {
		return err
	}
	if p.ResourceType != ResourceTypeEmail && p.ResourceType != ResourceTypeArticle && p.ResourceType != ResourceTypeDigest {
		return ErrInvalidResourceType
	}
	return nil
}

func (r *LLMRequest) Validate() error {
	if err := requireNonEmpty("user_id", r.UserID); err != nil {
		return err
	}
	if r.ResourceType != ResourceTypeEmail && r.ResourceType != ResourceTypeArticle && r.ResourceType != ResourceTypeDigest {
		return ErrInvalidResourceType
	}
	return nil
}
