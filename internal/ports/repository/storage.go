package repository

// Storage groups all repository ports for convenience.
type Storage interface {
	Users() UserRepository
	Emails() EmailRepository
	Articles() ArticleRepository
	Clusters() ClusterRepository
	Digests() DigestRepository
	Embeddings() EmbeddingRepository
	LLMRequests() LLMRequestRepository
	Close() error
}
