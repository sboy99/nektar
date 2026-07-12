package events

// Topic constants for the event bus.
const (
	TopicEmailFetched     = "email.fetched"
	TopicEmailDetected    = "email.detected"
	TopicArticleCreated   = "article.created"
	TopicEmbeddingCreated = "embedding.created"
	TopicClusterUpdated   = "cluster.updated"
	TopicDigestReady      = "digest.ready"
)

// EmailFetched is emitted when an email has been fetched and stored.
type EmailFetched struct {
	UserID        string
	EmailID       string
	CorrelationID string
}

func (e EmailFetched) Name() string { return TopicEmailFetched }
func (e EmailFetched) Payload() any { return e }

// EmailDetected is emitted when an email has been classified as a newsletter.
type EmailDetected struct {
	UserID        string
	EmailID       string
	CorrelationID string
}

func (e EmailDetected) Name() string { return TopicEmailDetected }
func (e EmailDetected) Payload() any { return e }

// ArticleCreated is emitted when an article has been extracted from a newsletter.
type ArticleCreated struct {
	UserID        string
	ArticleID     string
	EmailID       string
	CorrelationID string
}

func (e ArticleCreated) Name() string { return TopicArticleCreated }
func (e ArticleCreated) Payload() any { return e }

// EmbeddingCreated is emitted when an article embedding has been generated.
type EmbeddingCreated struct {
	UserID        string
	EmbeddingID   string
	ArticleID     string
	CorrelationID string
}

func (e EmbeddingCreated) Name() string { return TopicEmbeddingCreated }
func (e EmbeddingCreated) Payload() any { return e }

// ClusterUpdated is emitted when article clustering has been updated.
type ClusterUpdated struct {
	UserID        string
	ClusterID     string
	CorrelationID string
}

func (e ClusterUpdated) Name() string { return TopicClusterUpdated }
func (e ClusterUpdated) Payload() any { return e }

// DigestReady is emitted when a digest is ready for publishing.
type DigestReady struct {
	UserID        string
	DigestID      string
	CorrelationID string
}

func (e DigestReady) Name() string { return TopicDigestReady }
func (e DigestReady) Payload() any { return e }

// CorrelationIDFromMap extracts a correlation ID from a JSON-decoded payload map.
func CorrelationIDFromMap(m map[string]any) string {
	if m == nil {
		return ""
	}
	id, _ := m["CorrelationID"].(string)
	return id
}

// CorrelationIDFromPayload extracts a correlation ID from a typed or map payload.
func CorrelationIDFromPayload(payload any) string {
	switch p := payload.(type) {
	case EmailFetched:
		return p.CorrelationID
	case EmailDetected:
		return p.CorrelationID
	case ArticleCreated:
		return p.CorrelationID
	case EmbeddingCreated:
		return p.CorrelationID
	case ClusterUpdated:
		return p.CorrelationID
	case DigestReady:
		return p.CorrelationID
	case map[string]any:
		return CorrelationIDFromMap(p)
	default:
		return ""
	}
}
