package embedding

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	einoEmbedding "github.com/cloudwego/eino-ext/components/embedding/openai"
	"github.com/cloudwego/eino/components/embedding"

	"github.com/qiwang/book-e-commerce-micro/common/config"
	bookPb "github.com/qiwang/book-e-commerce-micro/proto/book"
	"github.com/qiwang/book-e-commerce-micro/service/ai/vectorstore"
)

const (
	embedBatchSize         = 20  // batch size per embedding API call
	embedPageSize          = int32(200)
	embedWorkers           = 2   // concurrent embedding workers
	embedMilvusBatch       = 100 // Milvus bulk upsert size
	defaultMaxErrors       = 500
	embedFirstPageAttempts = 12
	embedFirstPageWait     = 3 * time.Second
	embedLogInterval       = 200
)

// EmbedAllBooksOptions configures scanning BookService.SearchBooks and writing vectors.
type EmbedAllBooksOptions struct {
	Force bool
	// MaxEmbeddings: stop after this many successful upserts (0 = no limit).
	MaxEmbeddings int
	// MaxErrors: abort after this many failures (0 = defaultMaxErrors).
	MaxErrors int
}

// EmbedAllBooksStats is returned after a bulk embed run (including partial runs).
type EmbedAllBooksStats struct {
	Embedded int
	Skipped  int
	Errors   int
}

// DefaultEmbedAllBooksOptions is used on AI service startup for background catch-up.
func DefaultEmbedAllBooksOptions() EmbedAllBooksOptions {
	return EmbedAllBooksOptions{}
}

type Service struct {
	embedder embedding.Embedder
	model    string
	store    vectorstore.Store
	bookSvc  bookPb.BookService
	// e5Prefix: 为 E5 系列模型自动添加 "passage:"/"query:" 前缀
	e5Prefix bool
}

// NewService 根据配置创建 Embedding 服务。
//   - provider == "local"：连接本地 embed-server（OpenAI 兼容接口，如 intfloat/multilingual-e5-large）
//   - 其他/空：使用 openaiCfg 中的 API Key 和 BaseURL（默认 OpenAI 或兼容网关）
func NewService(ctx context.Context, embCfg *config.EmbeddingConfig, openaiCfg *config.OpenAIConfig, store vectorstore.Store, bookSvc bookPb.BookService) (*Service, error) {
	var embedder embedding.Embedder
	var modelName string
	var useE5Prefix bool

	if embCfg != nil && embCfg.Provider == "local" && embCfg.URL != "" {
		// 本地 embedding 服务器（OpenAI 兼容接口，无需真实 API Key）
		modelName = embCfg.Model
		if modelName == "" {
			modelName = "intfloat/multilingual-e5-large"
		}
		cfg := &einoEmbedding.EmbeddingConfig{
			APIKey:  "local", // 本地服务器不校验 key，但 Eino 客户端要求非空
			BaseURL: embCfg.URL + "/v1",
			Model:   modelName,
		}
		var err error
		embedder, err = einoEmbedding.NewEmbedder(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("create local eino embedder: %w", err)
		}
		useE5Prefix = embCfg.E5Prefix
		log.Printf("[embedding] using local server %s, model=%s e5_prefix=%v", embCfg.URL, modelName, useE5Prefix)
	} else {
		// OpenAI / 兼容网关
		modelName = openaiCfg.EmbeddingModel
		if modelName == "" {
			modelName = "text-embedding-3-small"
		}
		cfg := &einoEmbedding.EmbeddingConfig{
			APIKey: openaiCfg.APIKey,
			Model:  modelName,
		}
		if openaiCfg.BaseURL != "" {
			cfg.BaseURL = openaiCfg.BaseURL
		}
		var err error
		embedder, err = einoEmbedding.NewEmbedder(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("create eino embedder: %w", err)
		}
		log.Printf("[embedding] using OpenAI-compatible API, model=%s", modelName)
	}

	return &Service{
		embedder: embedder,
		model:    modelName,
		store:    store,
		bookSvc:  bookSvc,
		e5Prefix: useE5Prefix,
	}, nil
}

// EmbedText 对单段文本（书籍内容 / passage）生成 float32 向量。
// 若启用 E5Prefix，自动添加 "passage: " 前缀。
func (s *Service) EmbedText(ctx context.Context, text string) ([]float32, error) {
	vecs, err := s.EmbedTexts(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return vecs[0], nil
}

// EmbedQuery 对检索 Query 生成向量。
// 若启用 E5Prefix，自动添加 "query: " 前缀（E5 系列模型必须区分 query/passage）。
func (s *Service) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	input := query
	if s.e5Prefix {
		input = "query: " + query
	}
	vecs, err := s.embedRaw(ctx, []string{input})
	if err != nil {
		return nil, err
	}
	return vecs[0], nil
}

// EmbedTexts 批量对 passage 文本生成向量（供书籍批量 embedding 使用）。
func (s *Service) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	inputs := texts
	if s.e5Prefix {
		inputs = make([]string, len(texts))
		for i, t := range texts {
			inputs[i] = "passage: " + t
		}
	}
	return s.embedRaw(ctx, inputs)
}

// embedRaw 调用底层 Embedder，不做任何前缀处理。
func (s *Service) embedRaw(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	vectors, err := s.embedder.EmbedStrings(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("embed texts (batch=%d): %w", len(texts), err)
	}
	if len(vectors) != len(texts) {
		return nil, fmt.Errorf("expected %d vectors, got %d", len(texts), len(vectors))
	}
	result := make([][]float32, len(vectors))
	for i, f64 := range vectors {
		f32 := make([]float32, len(f64))
		for j, v := range f64 {
			f32[j] = float32(v)
		}
		result[i] = f32
	}
	return result, nil
}

// EmbedBook fetches book details, generates an embedding, and stores it in the vector store.
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

	if err := s.store.UpsertBookEmbedding(ctx, bookID, resp.Title, resp.Author, resp.Category, vec); err != nil {
		return nil, fmt.Errorf("save book embedding to store: %w", err)
	}

	return vec, nil
}

// FindSimilarBooks uses ANN search (Cosine) to find semantically
// similar books. If the source book has no embedding yet, it is generated first.
func (s *Service) FindSimilarBooks(ctx context.Context, bookID string, topN int) ([]vectorstore.SimilarBook, error) {
	vec, err := s.store.GetEmbedding(ctx, bookID)
	if err != nil {
		return nil, err
	}
	if vec == nil {
		vec, err = s.EmbedBook(ctx, bookID)
		if err != nil {
			return nil, fmt.Errorf("auto-generate embedding: %w", err)
		}
	}

	return s.store.FindSimilarBooks(ctx, bookID, vec, topN)
}

// bookItem is passed through the pipeline.
type bookItem struct {
	ID          string
	Title       string
	Author      string
	Category    string
	Description string
	Price       float64
	Rating      float64
}

// EmbedAllBooks pages through BookService and upserts vectors using concurrent workers.
// Pipeline: page reader → filter (batch HasEmbedding) → N workers (batch EmbedTexts) → batch Milvus upsert.
func (s *Service) EmbedAllBooks(ctx context.Context, opts EmbedAllBooksOptions) EmbedAllBooksStats {
	maxErr := opts.MaxErrors
	if maxErr <= 0 {
		maxErr = defaultMaxErrors
	}

	var stats EmbedAllBooksStats
	var mu sync.Mutex
	startTime := time.Now()
	log.Printf("[embedding] starting bulk embed (force=%v max_embeddings=%d max_errors=%d workers=%d batch=%d)...",
		opts.Force, opts.MaxEmbeddings, maxErr, embedWorkers, embedBatchSize)

	// Channel of batches ready for embedding
	batchCh := make(chan []bookItem, embedWorkers*2)
	// Channel of results ready for Milvus upsert
	resultCh := make(chan []vectorstore.BookEmbedding, embedWorkers*2)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Stage 1: Page reader → filter → batch → send to batchCh
	go func() {
		defer close(batchCh)
		s.produceBookBatches(ctx, opts, maxErr, &stats, &mu, cancel, batchCh)
	}()

	// Stage 2: N workers consume batchCh, call OpenAI, send results to resultCh
	var wg sync.WaitGroup
	for i := 0; i < embedWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for batch := range batchCh {
				select {
				case <-ctx.Done():
					return
				default:
				}
				embs := s.embedBatch(ctx, batch, workerID, &stats, &mu, maxErr, cancel)
				if len(embs) > 0 {
					select {
					case resultCh <- embs:
					case <-ctx.Done():
						return
					}
				}
				// Throttle to leave API headroom for real-time chat/search requests
				time.Sleep(500 * time.Millisecond)
			}
		}(i)
	}
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Stage 3: Milvus bulk upsert consumer
	milvusBuf := make([]vectorstore.BookEmbedding, 0, embedMilvusBatch)
	for embs := range resultCh {
		milvusBuf = append(milvusBuf, embs...)
		if len(milvusBuf) >= embedMilvusBatch {
			s.flushMilvusBatch(ctx, milvusBuf, &stats, &mu, maxErr, cancel)
			milvusBuf = milvusBuf[:0]
		}
	}
	if len(milvusBuf) > 0 {
		s.flushMilvusBatch(ctx, milvusBuf, &stats, &mu, maxErr, cancel)
	}
	s.flushIfSupported(ctx, "final")

	mu.Lock()
	elapsed := time.Since(startTime)
	rate := float64(stats.Embedded) / elapsed.Seconds()
	log.Printf("[embedding] bulk embed complete: embedded=%d skipped=%d errors=%d elapsed=%s rate=%.1f/s",
		stats.Embedded, stats.Skipped, stats.Errors, elapsed.Round(time.Second), rate)
	mu.Unlock()
	return stats
}

func (s *Service) produceBookBatches(ctx context.Context, opts EmbedAllBooksOptions, maxErr int, stats *EmbedAllBooksStats, mu *sync.Mutex, cancel context.CancelFunc, batchCh chan<- []bookItem) {
	page := int32(1)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		maxAttempts := 1
		if page == 1 {
			maxAttempts = embedFirstPageAttempts
		}
		var resp *bookPb.BookListResponse
		var err error
		for attempt := 0; attempt < maxAttempts; attempt++ {
			if attempt > 0 {
				log.Printf("[embedding] BookService.SearchBooks page=%d retry %d/%d", page, attempt+1, maxAttempts)
				select {
				case <-ctx.Done():
					return
				case <-time.After(embedFirstPageWait):
				}
			}
			resp, err = s.bookSvc.SearchBooks(ctx, &bookPb.SearchBooksRequest{
				Page:     page,
				PageSize: embedPageSize,
			})
			if err != nil {
				log.Printf("[embedding] search books page %d error (attempt %d): %v", page, attempt+1, err)
				if page == 1 && attempt < maxAttempts-1 {
					continue
				}
				mu.Lock()
				stats.Errors++
				mu.Unlock()
				break
			}
			if len(resp.Books) > 0 || page != 1 {
				break
			}
			if page == 1 && attempt < maxAttempts-1 {
				log.Printf("[embedding] SearchBooks page 1 returned 0 books, will retry")
				continue
			}
			break
		}
		if err != nil {
			return
		}
		if len(resp.Books) == 0 {
			if page == 1 {
				log.Printf("[embedding] still 0 books after retries — run make mongo-seed-books or seed-books-10k")
			}
			return
		}

		// Batch HasEmbeddings check
		var candidates []bookItem
		if !opts.Force {
			ids := make([]string, len(resp.Books))
			for i, b := range resp.Books {
				ids[i] = b.Id
			}
			existing, err := s.store.HasEmbeddings(ctx, ids)
			if err != nil {
				log.Printf("[embedding] batch HasEmbeddings error: %v", err)
				mu.Lock()
				stats.Errors++
				mu.Unlock()
			}
			for _, b := range resp.Books {
				if existing[b.Id] {
					mu.Lock()
					stats.Skipped++
					mu.Unlock()
					continue
				}
				candidates = append(candidates, bookItem{
					ID: b.Id, Title: b.Title, Author: b.Author,
					Category: b.Category, Description: b.Description,
					Price: b.Price, Rating: b.Rating,
				})
			}
		} else {
			for _, b := range resp.Books {
				candidates = append(candidates, bookItem{
					ID: b.Id, Title: b.Title, Author: b.Author,
					Category: b.Category, Description: b.Description,
					Price: b.Price, Rating: b.Rating,
				})
			}
		}

		// Check max embeddings limit
		mu.Lock()
		if opts.MaxEmbeddings > 0 && stats.Embedded+len(candidates) > opts.MaxEmbeddings {
			remain := opts.MaxEmbeddings - stats.Embedded
			if remain <= 0 {
				mu.Unlock()
				return
			}
			candidates = candidates[:remain]
		}
		mu.Unlock()

		// Split into embedBatchSize chunks and send
		for i := 0; i < len(candidates); i += embedBatchSize {
			end := i + embedBatchSize
			if end > len(candidates) {
				end = len(candidates)
			}
			select {
			case batchCh <- candidates[i:end]:
			case <-ctx.Done():
				return
			}
		}

		if int32(len(resp.Books)) < embedPageSize {
			return
		}
		page++
	}
}

func (s *Service) embedBatch(ctx context.Context, batch []bookItem, workerID int, stats *EmbedAllBooksStats, mu *sync.Mutex, maxErr int, cancel context.CancelFunc) []vectorstore.BookEmbedding {
	texts := make([]string, len(batch))
	for i, b := range batch {
		texts[i] = buildBookText(b.Title, b.Author, b.Category, b.Description, b.Price, b.Rating)
	}

	vecs, err := s.EmbedTexts(ctx, texts)
	if err != nil {
		log.Printf("[embedding] worker %d batch embed error (%d texts): %v", workerID, len(texts), err)
		mu.Lock()
		stats.Errors += len(batch)
		if stats.Errors >= maxErr {
			cancel()
		}
		mu.Unlock()
		return nil
	}

	items := make([]vectorstore.BookEmbedding, len(batch))
	for i, b := range batch {
		items[i] = vectorstore.BookEmbedding{
			BookID:   b.ID,
			Title:    b.Title,
			Author:   b.Author,
			Category: b.Category,
			Vector:   vecs[i],
		}
	}
	return items
}

func (s *Service) flushMilvusBatch(ctx context.Context, items []vectorstore.BookEmbedding, stats *EmbedAllBooksStats, mu *sync.Mutex, maxErr int, cancel context.CancelFunc) {
	if err := s.store.BulkUpsertBookEmbeddings(ctx, items); err != nil {
		log.Printf("[embedding] Milvus bulk upsert error (%d items): %v", len(items), err)
		mu.Lock()
		stats.Errors += len(items)
		if stats.Errors >= maxErr {
			cancel()
		}
		mu.Unlock()
		return
	}

	mu.Lock()
	stats.Embedded += len(items)
	if stats.Embedded%embedLogInterval < len(items) {
		elapsed := time.Since(time.Time{})
		_ = elapsed
		log.Printf("[embedding] progress: embedded=%d skipped=%d errors=%d",
			stats.Embedded, stats.Skipped, stats.Errors)
	}
	mu.Unlock()

	s.flushIfSupported(ctx, "intermediate")
}

// EmbedSingleBook generates or updates the embedding for a single book.
// Useful for event-driven incremental updates when a book is created or modified.
func (s *Service) EmbedSingleBook(ctx context.Context, bookID string) error {
	_, err := s.EmbedBook(ctx, bookID)
	if err != nil {
		return err
	}
	s.flushIfSupported(ctx, "single")
	return nil
}

func (s *Service) flushIfSupported(ctx context.Context, stage string) {
	type flusher interface {
		FlushCollection(ctx context.Context) error
	}
	if f, ok := s.store.(flusher); ok {
		if err := f.FlushCollection(ctx); err != nil {
			log.Printf("[embedding] %s flush error: %v", stage, err)
		}
	}
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
