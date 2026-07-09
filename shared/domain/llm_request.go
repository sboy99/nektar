package domain

import "time"

// LLMRequest records cost and latency for an LLM API call.
type LLMRequest struct {
	ID               string
	UserID           string
	ResourceType     ResourceType
	ResourceID       string
	Provider         string
	Model            string
	Tokens           int
	LatencyMs        int64
	EstimatedCostUSD float64
	CreatedAt        time.Time
}
