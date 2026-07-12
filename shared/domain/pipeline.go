package domain

import "time"

// ResourceType identifies the kind of resource tracked in the pipeline.
type ResourceType string

const (
	ResourceTypeEmail   ResourceType = "email"
	ResourceTypeArticle ResourceType = "article"
	ResourceTypeDigest  ResourceType = "digest"
)

// PipelineStage represents where a resource is in the processing pipeline.
type PipelineStage string

const (
	StageFetched     PipelineStage = "fetched"
	StageDetected    PipelineStage = "detected"
	StageRejected    PipelineStage = "rejected"
	StageExtracting  PipelineStage = "extracting"
	StageExtracted   PipelineStage = "extracted"
	StageEmbedding   PipelineStage = "embedding"
	StageEmbedded    PipelineStage = "embedded"
	StageClustered   PipelineStage = "clustered"
	StageDigesting   PipelineStage = "digesting"
	StageDigestReady PipelineStage = "digest_ready"
	StagePublished   PipelineStage = "published"
	StageFailed      PipelineStage = "failed"
)

// PipelineStatus tracks processing state for an email or article.
type PipelineStatus struct {
	ResourceType ResourceType
	ResourceID   string
	UserID       string
	Stage        PipelineStage
	Error        string
	UpdatedAt    time.Time
}
