package embedding

import (
	"context"
	"fmt"
	"log"
	"time"

	einoEmbedding "github.com/cloudwego/eino-ext/components/embedding/openai"
	"github.com/cloudwego/eino/components/embedding"

	"github.com/qiwang/book-e-commerce-micro/common/config"
	bookPb "github.com/qiwang/book-e-commerce-micro/proto/book"
	"github.com/qiwang/book-e-commerce-micro/service/ai/vectorstore"
)

const (
	embedRateLimit    = 200 * time.Millisecond // ~5 requests/sec to stay within API rate limits
	embedBatchSize    = 10
	embedPageSize     = int32(50)
)

type Service struct {
	embedder embedding.Embedder
	model    string
	milvus   *vectorstore.MilvusStore
	bookSvc  bookPb.BookService
}

func NewService(ctx context.Context, cfg *config.OpenAIConfig, milvus *vectorstore.MilvusStore, bookSvc bookPb.BookService) (*Service, error) {
	embModel := cfg.EmbeddingModel
	if embModel == "" {
		embModel = "text-embedding-3-small"
	}

	embCfg := &einoEmbedding.EmbeddingConfig{
		APIKey: cfg.APIKey,
		Model:  embModel,
	}
	if cfg.BaseURL != "" {
		embCfg.BaseURL = cfg.BaseURL
	}

	embedder, err := einoEmbedding.NewEmbedder(ctx, embCfg)
	if err != nil {
		return nil, fmt.Errorf("create eino embedder: %w", err)
	}

	return &Service{
		embedder: embedder,
		model:    embModel,
		milvus:   milvus,
		bookSvc:  bookSvc,
	}, nil
}

// EmbedText generates an embedding vector for the given text, returned as float32
// for direct use with Milvus.
func (s *Service) EmbedText(ctx context.Context, text string) ([]float32, error) {
	vectors, err := s.embedder.EmbedStrings(ctx, []string{text})
	if err != nil {
		return nil, fmt.Errorf("embed text: %w", err)
	}
	if len(vectors) == 0 || len(vectors[0]) == 0 {
		return nil, fmt.Errorf("empty embedding returned")
	}

	f64 := vectors[0]
	f32 := make([]float32, len(f64))
	for i, v := range f64 {
		f32[i] = float32(v)
	}
	return f32, nil
}

// EmbedBook fetches book details, generates an embedding, and stores it in Milvus.
func (s *Service) EmbedBook(ctx context.Context, bookID string) ([]float32, error) {
	resp, err := s.bookSvc.GetBookDetail(ctx, &bookPb.GetBookDetailRequest{BookId: bookID})
	if err != nil {
		return nil, fmt.Errorf("get book detail for embedding: %w", err)
	}

	text := buildBookText(resp.Title, resp.Author, resp.Category, resp.Description, resp.Price, resp.Rating)
	vec, err := s.EmbedText(ctx, text)
	if err != nil {
		return nil, err
	}

	if err := s.milvus.UpsertBookEmbedding(ctx, bookID, resp.Title, resp.Author, resp.Category, vec); err != nil {
		return nil, fmt.Errorf("save book embedding to milvus: %w", err)
	}

	return vec, nil
}

// FindSimilarBooks uses Milvus ANN search (HNSW + Cosine) to find semantically
// similar books. If the source book has no embedding yet, it is generated first.
func (s *Service) FindSimilarBooks(ctx context.Context, bookID string, topN int) ([]vectorstore.SimilarBook, error) {
	vec, err := s.milvus.GetEmbedding(ctx, bookID)
	if err != nil {
		return nil, err
	}
	if vec == nil {
		vec, err = s.EmbedBook(ctx, bookID)
		if err != nil {
			return nil, fmt.Errorf("auto-generate embedding: %w", err)
		}
	}

	return s.milvus.FindSimilarBooks(ctx, bookID, vec, topN)
}

// EmbedAllBooks generates embeddings for all books that don't have one yet.
// It uses rate limiting to avoid hitting OpenAI API rate limits and processes
// books in batches with periodic flushes.
func (s *Service) EmbedAllBooks(ctx context.Context) {
	log.Println("[embedding] starting background embedding of all books...")

	page := int32(1)
	total := 0
	errors := 0
	batchCount := 0
	ticker := time.NewTicker(embedRateLimit)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[embedding] background embedding cancelled after %d embeddings", total)
			return
		default:
		}

		resp, err := s.bookSvc.SearchBooks(ctx, &bookPb.SearchBooksRequest{
			Page:     page,
			PageSize: embedPageSize,
		})
		if err != nil {
			log.Printf("[embedding] search books page %d error: %v", page, err)
			break
		}
		if len(resp.Books) == 0 {
			break
		}

		for _, b := range resp.Books {
			exists, _ := s.milvus.HasEmbedding(ctx, b.Id)
			if exists {
				continue
			}

			<-ticker.C

			text := buildBookText(b.Title, b.Author, b.Category, b.Description, b.Price, b.Rating)
			vec, err := s.EmbedText(ctx, text)
			if err != nil {
				log.Printf("[embedding] embed book %s error: %v", b.Id, err)
				errors++
				if errors > 10 {
					log.Printf("[embedding] too many errors (%d), stopping", errors)
					return
				}
				continue
			}

			if err := s.milvus.UpsertBookEmbedding(ctx, b.Id, b.Title, b.Author, b.Category, vec); err != nil {
				log.Printf("[embedding] save embedding for book %s error: %v", b.Id, err)
				continue
			}
			total++
			batchCount++

			if batchCount >= embedBatchSize {
				if err := s.milvus.FlushCollection(ctx); err != nil {
					log.Printf("[embedding] intermediate flush error: %v", err)
				}
				batchCount = 0
				log.Printf("[embedding] progress: %d embeddings generated so far", total)
			}
		}

		if int32(len(resp.Books)) < embedPageSize {
			break
		}
		page++
	}

	if batchCount > 0 {
		if err := s.milvus.FlushCollection(ctx); err != nil {
			log.Printf("[embedding] final flush error: %v", err)
		}
	}

	log.Printf("[embedding] background embedding complete: %d new embeddings, %d errors", total, errors)
}

// EmbedSingleBook generates or updates the embedding for a single book.
// Useful for event-driven incremental updates when a book is created or modified.
func (s *Service) EmbedSingleBook(ctx context.Context, bookID string) error {
	_, err := s.EmbedBook(ctx, bookID)
	if err != nil {
		return err
	}
	return s.milvus.FlushCollection(ctx)
}

func buildBookText(title, author, category, description string, price float64, rating float64) string {
	text := fmt.Sprintf("Title: %s\nAuthor: %s\nCategory: %s", title, author, category)
	if description != "" {
		desc := description
		if len(desc) > 500 {
			desc = desc[:500] + "..."
		}
		text += fmt.Sprintf("\nDescription: %s", desc)
	}
	if price > 0 {
		text += fmt.Sprintf("\nPrice: ¥%.2f", price)
	}
	if rating > 0 {
		text += fmt.Sprintf("\nRating: %.1f/5.0", rating)
	}
	return text
}
