package vectorstore

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

const (
	CollectionName = "book_embeddings"
	VectorDim      = 1536 // text-embedding-3-small
	FieldBookID    = "book_id"
	FieldTitle     = "title"
	FieldAuthor    = "author"
	FieldCategory  = "category"
	FieldVector    = "embedding"
)

type SimilarBook struct {
	BookID   string
	Title    string
	Author   string
	Category string
	Score    float64
}

type MilvusStore struct {
	client client.Client
}

func NewMilvusStore(ctx context.Context, address string) (*MilvusStore, error) {
	c, err := client.NewClient(ctx, client.Config{
		Address: address,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to milvus at %s: %w", address, err)
	}

	store := &MilvusStore{client: c}
	if err := store.ensureCollection(ctx); err != nil {
		c.Close()
		return nil, err
	}

	return store, nil
}

func (m *MilvusStore) Close() error {
	return m.client.Close()
}

func (m *MilvusStore) ensureCollection(ctx context.Context) error {
	exists, err := m.client.HasCollection(ctx, CollectionName)
	if err != nil {
		return fmt.Errorf("check collection existence: %w", err)
	}

	if !exists {
		schema := entity.NewSchema().
			WithName(CollectionName).
			WithDescription("Book embedding vectors for semantic similarity search").
			WithField(entity.NewField().WithName(FieldBookID).WithDataType(entity.FieldTypeVarChar).WithMaxLength(64).WithIsPrimaryKey(true)).
			WithField(entity.NewField().WithName(FieldTitle).WithDataType(entity.FieldTypeVarChar).WithMaxLength(512)).
			WithField(entity.NewField().WithName(FieldAuthor).WithDataType(entity.FieldTypeVarChar).WithMaxLength(256)).
			WithField(entity.NewField().WithName(FieldCategory).WithDataType(entity.FieldTypeVarChar).WithMaxLength(128)).
			WithField(entity.NewField().WithName(FieldVector).WithDataType(entity.FieldTypeFloatVector).WithDim(VectorDim))

		if err := m.client.CreateCollection(ctx, schema, entity.DefaultShardNumber); err != nil {
			return fmt.Errorf("create collection: %w", err)
		}
		log.Printf("[milvus] created collection %s", CollectionName)

		idx, err := entity.NewIndexHNSW(entity.COSINE, 16, 256)
		if err != nil {
			return fmt.Errorf("create HNSW index params: %w", err)
		}
		if err := m.client.CreateIndex(ctx, CollectionName, FieldVector, idx, false); err != nil {
			return fmt.Errorf("create vector index: %w", err)
		}
		log.Printf("[milvus] created HNSW index on %s.%s", CollectionName, FieldVector)
	}

	if err := m.client.LoadCollection(ctx, CollectionName, false); err != nil {
		return fmt.Errorf("load collection: %w", err)
	}
	log.Printf("[milvus] collection %s loaded into memory", CollectionName)

	return nil
}

// UpsertBookEmbedding inserts or updates a book's embedding vector. Milvus treats
// insert of an existing primary key as an upsert in v2.4+.
func (m *MilvusStore) UpsertBookEmbedding(ctx context.Context, bookID, title, author, category string, vector []float32) error {
	bookIDs := entity.NewColumnVarChar(FieldBookID, []string{bookID})
	titles := entity.NewColumnVarChar(FieldTitle, []string{title})
	authors := entity.NewColumnVarChar(FieldAuthor, []string{author})
	categories := entity.NewColumnVarChar(FieldCategory, []string{category})
	vectors := entity.NewColumnFloatVector(FieldVector, VectorDim, [][]float32{vector})

	if _, err := m.client.Upsert(ctx, CollectionName, "", bookIDs, titles, authors, categories, vectors); err != nil {
		return fmt.Errorf("upsert embedding for book %s: %w", bookID, err)
	}

	return nil
}

// BulkUpsertBookEmbeddings inserts/updates multiple book embeddings in a single Milvus call.
func (m *MilvusStore) BulkUpsertBookEmbeddings(ctx context.Context, items []BookEmbedding) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, len(items))
	titles := make([]string, len(items))
	authors := make([]string, len(items))
	categories := make([]string, len(items))
	vectors := make([][]float32, len(items))
	for i, item := range items {
		ids[i] = item.BookID
		titles[i] = truncStr(item.Title, 510)
		authors[i] = truncStr(item.Author, 254)
		categories[i] = truncStr(item.Category, 126)
		vectors[i] = item.Vector
	}

	idCol := entity.NewColumnVarChar(FieldBookID, ids)
	titleCol := entity.NewColumnVarChar(FieldTitle, titles)
	authorCol := entity.NewColumnVarChar(FieldAuthor, authors)
	categoryCol := entity.NewColumnVarChar(FieldCategory, categories)
	vectorCol := entity.NewColumnFloatVector(FieldVector, VectorDim, vectors)

	if _, err := m.client.Upsert(ctx, CollectionName, "", idCol, titleCol, authorCol, categoryCol, vectorCol); err != nil {
		return fmt.Errorf("bulk upsert %d embeddings: %w", len(items), err)
	}
	return nil
}

// HasEmbeddings checks multiple book IDs for existing embeddings in one query.
func (m *MilvusStore) HasEmbeddings(ctx context.Context, bookIDs []string) (map[string]bool, error) {
	result := make(map[string]bool, len(bookIDs))
	if len(bookIDs) == 0 {
		return result, nil
	}

	// Build IN expression: book_id in ["id1","id2",...]
	escaped := make([]string, len(bookIDs))
	for i, id := range bookIDs {
		escaped[i] = fmt.Sprintf(`"%s"`, escapeID(id))
	}
	expr := fmt.Sprintf(`%s in [%s]`, FieldBookID, strings.Join(escaped, ","))

	rows, err := m.client.Query(ctx, CollectionName, nil, expr, []string{FieldBookID})
	if err != nil {
		return nil, fmt.Errorf("batch query embedding existence: %w", err)
	}

	for _, col := range rows {
		if col.Name() != FieldBookID {
			continue
		}
		vc, ok := col.(*entity.ColumnVarChar)
		if !ok {
			continue
		}
		for i := 0; i < vc.Len(); i++ {
			v, _ := vc.ValueByIdx(i)
			result[v] = true
		}
	}
	return result, nil
}

func truncStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// escapeID sanitizes a string for safe use in Milvus filter expressions.
func escapeID(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// HasEmbedding checks if a book already has an embedding stored.
func (m *MilvusStore) HasEmbedding(ctx context.Context, bookID string) (bool, error) {
	expr := fmt.Sprintf(`%s == "%s"`, FieldBookID, escapeID(bookID))
	result, err := m.client.Query(ctx, CollectionName, nil, expr, []string{FieldBookID})
	if err != nil {
		return false, fmt.Errorf("query embedding existence: %w", err)
	}
	if len(result) == 0 {
		return false, nil
	}
	col := result[0]
	return col.Len() > 0, nil
}

// GetEmbedding retrieves the embedding vector for a given book.
func (m *MilvusStore) GetEmbedding(ctx context.Context, bookID string) ([]float32, error) {
	expr := fmt.Sprintf(`%s == "%s"`, FieldBookID, escapeID(bookID))
	result, err := m.client.Query(ctx, CollectionName, nil, expr, []string{FieldVector})
	if err != nil {
		return nil, fmt.Errorf("query embedding: %w", err)
	}
	for _, col := range result {
		if col.Name() == FieldVector {
			fvCol, ok := col.(*entity.ColumnFloatVector)
			if !ok || fvCol.Len() == 0 {
				return nil, nil
			}
			vec := fvCol.Data()[0]
			return vec, nil
		}
	}
	return nil, nil
}

// FindSimilarBooks performs ANN search via Milvus HNSW index with cosine metric.
func (m *MilvusStore) FindSimilarBooks(ctx context.Context, sourceBookID string, queryVector []float32, topN int) ([]SimilarBook, error) {
	sp, err := entity.NewIndexHNSWSearchParam(128)
	if err != nil {
		return nil, fmt.Errorf("create search params: %w", err)
	}

	// Request topN+1 to filter out the source book
	limit := topN + 1

	results, err := m.client.Search(
		ctx,
		CollectionName,
		nil, // partitions
		fmt.Sprintf(`%s != "%s"`, FieldBookID, escapeID(sourceBookID)),
		[]string{FieldBookID, FieldTitle, FieldAuthor, FieldCategory},
		[]entity.Vector{entity.FloatVector(queryVector)},
		FieldVector,
		entity.COSINE,
		limit,
		sp,
	)
	if err != nil {
		return nil, fmt.Errorf("milvus search: %w", err)
	}

	var similar []SimilarBook
	for _, result := range results {
		var bookIDCol, titleCol, authorCol, categoryCol *entity.ColumnVarChar
		for _, field := range result.Fields {
			switch field.Name() {
			case FieldBookID:
				bookIDCol, _ = field.(*entity.ColumnVarChar)
			case FieldTitle:
				titleCol, _ = field.(*entity.ColumnVarChar)
			case FieldAuthor:
				authorCol, _ = field.(*entity.ColumnVarChar)
			case FieldCategory:
				categoryCol, _ = field.(*entity.ColumnVarChar)
			}
		}

		for i := 0; i < result.ResultCount && i < topN; i++ {
			sb := SimilarBook{
				Score: float64(result.Scores[i]),
			}
			if bookIDCol != nil {
				sb.BookID, _ = bookIDCol.ValueByIdx(i)
			}
			if titleCol != nil {
				sb.Title, _ = titleCol.ValueByIdx(i)
			}
			if authorCol != nil {
				sb.Author, _ = authorCol.ValueByIdx(i)
			}
			if categoryCol != nil {
				sb.Category, _ = categoryCol.ValueByIdx(i)
			}
			similar = append(similar, sb)
		}
	}

	return similar, nil
}

// SearchByVector performs ANN search without excluding any book (for RAG queries).
func (m *MilvusStore) SearchByVector(ctx context.Context, queryVector []float32, topN int) ([]SimilarBook, error) {
	sp, err := entity.NewIndexHNSWSearchParam(128)
	if err != nil {
		return nil, fmt.Errorf("create search params: %w", err)
	}

	results, err := m.client.Search(
		ctx,
		CollectionName,
		nil,
		"",
		[]string{FieldBookID, FieldTitle, FieldAuthor, FieldCategory},
		[]entity.Vector{entity.FloatVector(queryVector)},
		FieldVector,
		entity.COSINE,
		topN,
		sp,
	)
	if err != nil {
		return nil, fmt.Errorf("milvus search: %w", err)
	}

	var similar []SimilarBook
	for _, result := range results {
		var bookIDCol, titleCol, authorCol, categoryCol *entity.ColumnVarChar
		for _, field := range result.Fields {
			switch field.Name() {
			case FieldBookID:
				bookIDCol, _ = field.(*entity.ColumnVarChar)
			case FieldTitle:
				titleCol, _ = field.(*entity.ColumnVarChar)
			case FieldAuthor:
				authorCol, _ = field.(*entity.ColumnVarChar)
			case FieldCategory:
				categoryCol, _ = field.(*entity.ColumnVarChar)
			}
		}
		for i := 0; i < result.ResultCount && i < topN; i++ {
			sb := SimilarBook{Score: float64(result.Scores[i])}
			if bookIDCol != nil {
				sb.BookID, _ = bookIDCol.ValueByIdx(i)
			}
			if titleCol != nil {
				sb.Title, _ = titleCol.ValueByIdx(i)
			}
			if authorCol != nil {
				sb.Author, _ = authorCol.ValueByIdx(i)
			}
			if categoryCol != nil {
				sb.Category, _ = categoryCol.ValueByIdx(i)
			}
			similar = append(similar, sb)
		}
	}
	return similar, nil
}

// FlushCollection forces a flush to persist data to storage.
func (m *MilvusStore) FlushCollection(ctx context.Context) error {
	flushCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return m.client.Flush(flushCtx, CollectionName, false)
}

var _ Store = (*MilvusStore)(nil)
