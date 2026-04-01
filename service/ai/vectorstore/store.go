package vectorstore

import "context"

// Store is the minimal vector store interface used by the AI service.
// It allows swapping Milvus with another backend (e.g. Elasticsearch) on ARM Macs.
type Store interface {
	Close() error

	UpsertBookEmbedding(ctx context.Context, bookID, title, author, category string, vector []float32) error
	HasEmbedding(ctx context.Context, bookID string) (bool, error)
	GetEmbedding(ctx context.Context, bookID string) ([]float32, error)

	// FindSimilarBooks searches similar books excluding the source book.
	FindSimilarBooks(ctx context.Context, sourceBookID string, queryVector []float32, topN int) ([]SimilarBook, error)
	// SearchByVector searches similar books for a query (RAG).
	SearchByVector(ctx context.Context, queryVector []float32, topN int) ([]SimilarBook, error)
}

