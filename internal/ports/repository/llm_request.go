package repository

import (
	"context"
	"time"

	"github.com/sboy99/nektar/shared/domain"
)

// LLMRequestRepository persists LLM API call records for cost tracking.
type LLMRequestRepository interface {
	Save(ctx context.Context, req *domain.LLMRequest) error
	ListByUser(ctx context.Context, userID string, since time.Time) ([]*domain.LLMRequest, error)
}
