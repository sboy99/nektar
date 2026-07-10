package postgres

import (
	"context"
	"time"

	"github.com/sboy99/nektar/shared/domain"
)

type llmRequestRepo struct{ s *Storage }

func (r *llmRequestRepo) Save(ctx context.Context, req *domain.LLMRequest) error {
	_, err := r.s.pool.Exec(ctx, `
		INSERT INTO llm_requests (
			id, user_id, resource_type, resource_id, provider, model,
			tokens, latency_ms, estimated_cost_usd, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10
		)
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			resource_type = EXCLUDED.resource_type,
			resource_id = EXCLUDED.resource_id,
			provider = EXCLUDED.provider,
			model = EXCLUDED.model,
			tokens = EXCLUDED.tokens,
			latency_ms = EXCLUDED.latency_ms,
			estimated_cost_usd = EXCLUDED.estimated_cost_usd,
			created_at = EXCLUDED.created_at
	`,
		req.ID, req.UserID, string(req.ResourceType), req.ResourceID, req.Provider, req.Model,
		req.Tokens, req.LatencyMs, req.EstimatedCostUSD, req.CreatedAt,
	)
	return err
}

func (r *llmRequestRepo) ListByUser(ctx context.Context, userID string, since time.Time) ([]*domain.LLMRequest, error) {
	rows, err := r.s.pool.Query(ctx, `
		SELECT id, user_id, resource_type, resource_id, provider, model,
			tokens, latency_ms, estimated_cost_usd, created_at
		FROM llm_requests
		WHERE user_id = $1 AND created_at >= $2
		ORDER BY created_at DESC
	`, userID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*domain.LLMRequest
	for rows.Next() {
		var (
			req          domain.LLMRequest
			resourceType string
		)
		err := rows.Scan(
			&req.ID, &req.UserID, &resourceType, &req.ResourceID, &req.Provider, &req.Model,
			&req.Tokens, &req.LatencyMs, &req.EstimatedCostUSD, &req.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		req.ResourceType = domain.ResourceType(resourceType)
		result = append(result, &req)
	}
	return result, rows.Err()
}
