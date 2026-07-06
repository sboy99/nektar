package publisher

import (
	"context"

	"github.com/sboy99/nektar/shared/domain"
)

// Publisher delivers digests to external channels (Discord, Slack, etc.).
type Publisher interface {
	Publish(ctx context.Context, digest *domain.Digest) error
}
