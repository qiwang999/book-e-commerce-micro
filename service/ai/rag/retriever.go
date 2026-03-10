package rag

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/cloudwego/eino/schema"

	inventoryPb "github.com/qiwang/book-e-commerce-micro/proto/inventory"
	"github.com/qiwang/book-e-commerce-micro/service/ai/embedding"
	"github.com/qiwang/book-e-commerce-micro/service/ai/vectorstore"
)

const defaultTopK = 8

// BookRetriever implements the Eino Retriever interface, combining Milvus vector
// search with real-time inventory checks. Each retrieved document contains the
// book's metadata plus stock availability so the Agent can give accurate answers.
type BookRetriever struct {
	embSvc       *embedding.Service
	milvus       *vectorstore.MilvusStore
	inventorySvc inventoryPb.InventoryService
}

func NewBookRetriever(
	embSvc *embedding.Service,
	milvus *vectorstore.MilvusStore,
	inventorySvc inventoryPb.InventoryService,
) *BookRetriever {
	return &BookRetriever{
		embSvc:       embSvc,
		milvus:       milvus,
		inventorySvc: inventorySvc,
	}
}

// Retrieve searches Milvus for books semantically similar to the query, then
// enriches each result with real-time stock information. Returns Eino Documents
// ready for injection into the Agent's context.
func (r *BookRetriever) Retrieve(ctx context.Context, query string, topK int) ([]*schema.Document, error) {
	if topK <= 0 {
		topK = defaultTopK
	}

	vec, err := r.embSvc.EmbedText(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query for retrieval: %w", err)
	}

	similar, err := r.milvus.SearchByVector(ctx, vec, topK)
	if err != nil {
		return nil, fmt.Errorf("milvus search: %w", err)
	}

	if len(similar) == 0 {
		return nil, nil
	}

	bookIDs := make([]string, len(similar))
	for i, s := range similar {
		bookIDs[i] = s.BookID
	}

	stockMap := r.batchCheckStock(ctx, bookIDs)

	docs := make([]*schema.Document, 0, len(similar))
	for _, book := range similar {
		stock, hasStock := stockMap[book.BookID]

		var stockLine string
		if !hasStock {
			stockLine = "库存状态: 未知（未录入库存系统）"
		} else if stock.Available <= 0 {
			stockLine = fmt.Sprintf("库存状态: ⚠️ 库存不足（当前可用: %d）", stock.Available)
		} else {
			stockLine = fmt.Sprintf("库存状态: 有货（可用: %d，价格: ¥%.2f）", stock.Available, stock.Price)
		}

		content := fmt.Sprintf(
			"书名: %s\n作者: %s\n分类: %s\n相似度: %.2f\n%s",
			book.Title, book.Author, book.Category, book.Score, stockLine,
		)

		docs = append(docs, &schema.Document{
			ID:      book.BookID,
			Content: content,
			MetaData: map[string]any{
				"book_id":  book.BookID,
				"title":    book.Title,
				"author":   book.Author,
				"category": book.Category,
				"score":    book.Score,
				"in_stock": hasStock && stock.Available > 0,
			},
		})
	}

	return docs, nil
}

type stockResult struct {
	Available int32
	Price     float64
}

// batchCheckStock queries inventory across all stores for the given book IDs.
// It tries store_id=0 first (all stores aggregate), falling back to store_id=1.
func (r *BookRetriever) batchCheckStock(ctx context.Context, bookIDs []string) map[string]stockResult {
	result := make(map[string]stockResult)

	resp, err := r.inventorySvc.BatchCheckStock(ctx, &inventoryPb.BatchCheckStockRequest{
		StoreId: 0,
		BookIds: bookIDs,
	})
	if err != nil {
		log.Printf("[rag] batch check stock (store=0) error: %v, trying store=1", err)
		resp, err = r.inventorySvc.BatchCheckStock(ctx, &inventoryPb.BatchCheckStockRequest{
			StoreId: 1,
			BookIds: bookIDs,
		})
		if err != nil {
			log.Printf("[rag] batch check stock (store=1) error: %v", err)
			return result
		}
	}

	for _, item := range resp.Items {
		result[item.BookId] = stockResult{
			Available: item.Available,
			Price:     item.Price,
		}
	}
	return result
}

// FormatDocsAsContext formats retrieved documents into a text block suitable for
// injection into an Agent's system prompt or user message.
func FormatDocsAsContext(docs []*schema.Document) string {
	if len(docs) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("=== 以下是从我们书库中检索到的相关图书（基于语义相似度） ===\n\n")
	for i, doc := range docs {
		b.WriteString(fmt.Sprintf("[%d] %s\n\n", i+1, doc.Content))
	}
	b.WriteString("=== 检索结果结束 ===\n")
	b.WriteString("重要提示：请仅基于以上检索到的书库数据进行推荐和回答。如果某本书库存不足，请明确告知用户。不要推荐检索结果中没有的书。")
	return b.String()
}
