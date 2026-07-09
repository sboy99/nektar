// Package domain defines the core business model for Nektar.
//
// Aggregate boundaries:
//
//   - User (root): owns Gmail sync state and account identity
//   - Email (root): fetched message scoped to a user
//   - Article (root): extracted content from an email
//   - Cluster (root): semantic grouping of articles
//   - Topic (root): human-facing label for a cluster
//   - Digest (root): curated summary ready to publish
//
// Entities (not aggregate roots): Embedding, LLMRequest, PipelineStatus
package domain
