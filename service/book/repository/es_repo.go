package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/elastic/go-elasticsearch/v8"
)

const bookIndex = "books"

type ESRepo struct {
	client *elasticsearch.Client
}

func NewESRepo(addresses []string) (*ESRepo, error) {
	cfg := elasticsearch.Config{
		Addresses: addresses,
	}
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create ES client: %w", err)
	}
	return &ESRepo{client: client}, nil
}

// EnsureIndex creates the books index with proper mappings if it doesn't exist
func (r *ESRepo) EnsureIndex(ctx context.Context) error {
	// Check if index exists
	res, err := r.client.Indices.Exists([]string{bookIndex})
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == 200 {
		return nil
	}

	// Create index with mappings
	mapping := map[string]interface{}{
		"settings": map[string]interface{}{
			"analysis": map[string]interface{}{
				"analyzer": map[string]interface{}{
					"book_analyzer": map[string]interface{}{
						"type":      "custom",
						"tokenizer": "standard",
						"filter":    []string{"lowercase", "stop"},
					},
				},
			},
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				// ES 8+ 不允许在 mapping 里写 boost；检索权重用 multi_match 的 fields（如 title^3）即可
				"title":       map[string]interface{}{"type": "text", "analyzer": "book_analyzer"},
				"author":      map[string]interface{}{"type": "text"},
				"description":  map[string]interface{}{"type": "text", "analyzer": "book_analyzer"},
				"category":     map[string]interface{}{"type": "keyword"},
				"subcategory":  map[string]interface{}{"type": "keyword"},
				"tags":         map[string]interface{}{"type": "keyword"},
				"isbn":         map[string]interface{}{"type": "keyword"},
				"publisher":    map[string]interface{}{"type": "text"},
				"language":     map[string]interface{}{"type": "keyword"},
				"price":        map[string]interface{}{"type": "float"},
				"rating":       map[string]interface{}{"type": "float"},
				"rating_count": map[string]interface{}{"type": "long"},
				"book_id":      map[string]interface{}{"type": "keyword"},
			},
		},
	}

	body, _ := json.Marshal(mapping)
	res, err = r.client.Indices.Create(bookIndex, r.client.Indices.Create.WithBody(bytes.NewReader(body)))
	if err != nil {
		return err
	}
	if res.IsError() {
		firstErr := res.String()
		res.Body.Close()
		// 旧 mapping / 半创建状态：删索引再建一次（常见于升级 ES 或改 mapping 后）
		delRes, delErr := r.client.Indices.Delete([]string{bookIndex})
		if delErr == nil && delRes != nil && delRes.Body != nil {
			_ = delRes.Body.Close()
		}
		res, err = r.client.Indices.Create(bookIndex, r.client.Indices.Create.WithBody(bytes.NewReader(body)))
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.IsError() {
			return fmt.Errorf("failed to create index: %s (after delete, first attempt was: %s)", res.String(), firstErr)
		}
		return nil
	}
	defer res.Body.Close()
	return nil
}

// IndexBook indexes a book document
func (r *ESRepo) IndexBook(ctx context.Context, bookID string, doc map[string]interface{}) error {
	doc["book_id"] = bookID
	body, _ := json.Marshal(doc)
	res, err := r.client.Index(bookIndex, bytes.NewReader(body),
		r.client.Index.WithDocumentID(bookID),
		r.client.Index.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}

// SearchBooks performs full-text search with optional filters
func (r *ESRepo) SearchBooks(ctx context.Context, keyword string, category string, author string, minPrice, maxPrice float64, language string, page, pageSize int) ([]string, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	from := (page - 1) * pageSize

	must := []map[string]interface{}{}
	filter := []map[string]interface{}{}

	if keyword != "" {
		must = append(must, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":    keyword,
				"fields":   []string{"title^3", "author^2", "description", "tags"},
				"type":     "best_fields",
				"fuzziness": "AUTO",
			},
		})
	}

	if category != "" {
		filter = append(filter, map[string]interface{}{
			"term": map[string]interface{}{"category": category},
		})
	}
	if author != "" {
		must = append(must, map[string]interface{}{
			"match": map[string]interface{}{"author": author},
		})
	}
	if language != "" {
		filter = append(filter, map[string]interface{}{
			"term": map[string]interface{}{"language": language},
		})
	}
	if minPrice > 0 || maxPrice > 0 {
		priceRange := map[string]interface{}{}
		if minPrice > 0 {
			priceRange["gte"] = minPrice
		}
		if maxPrice > 0 {
			priceRange["lte"] = maxPrice
		}
		filter = append(filter, map[string]interface{}{
			"range": map[string]interface{}{"price": priceRange},
		})
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must":   must,
				"filter": filter,
			},
		},
		"from":    from,
		"size":    pageSize,
		"_source": []string{"book_id"},
	}

	if len(must) == 0 && len(filter) == 0 {
		query["query"] = map[string]interface{}{"match_all": map[string]interface{}{}}
	}

	body, _ := json.Marshal(query)
	res, err := r.client.Search(
		r.client.Search.WithContext(ctx),
		r.client.Search.WithIndex(bookIndex),
		r.client.Search.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, 0, fmt.Errorf("search error: %s", res.String())
	}

	var result struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source struct {
					BookID string `json:"book_id"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, 0, err
	}

	bookIDs := make([]string, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		bookIDs = append(bookIDs, hit.Source.BookID)
	}

	return bookIDs, result.Hits.Total.Value, nil
}

// DeleteBook removes a book from the index
func (r *ESRepo) DeleteBook(ctx context.Context, bookID string) error {
	res, err := r.client.Delete(bookIndex, bookID, r.client.Delete.WithContext(ctx))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}
