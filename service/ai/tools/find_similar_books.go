package tools

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"github.com/qiwang/book-e-commerce-micro/service/ai/embedding"
)

type FindSimilarBooksInput struct {
	BookID string `json:"book_id" jsonschema:"description=The book ID to find similar books for,required"`
	Limit  int    `json:"limit,omitempty" jsonschema:"description=Maximum number of similar books to return (default 5)"`
}

type SimilarBookResult struct {
	BookID   string  `json:"book_id"`
	Title    string  `json:"title"`
	Author   string  `json:"author"`
	Category string  `json:"category"`
	Score    float64 `json:"score"`
}

type FindSimilarBooksOutput struct {
	SimilarBooks []SimilarBookResult `json:"similar_books"`
}

func NewFindSimilarBooksTool(embSvc *embedding.Service) (tool.InvokableTool, error) {
	return utils.InferTool(
		"find_similar_books",
		"Find books similar to a given book using semantic embedding similarity",
		func(ctx context.Context, input *FindSimilarBooksInput) (*FindSimilarBooksOutput, error) {
			limit := input.Limit
			if limit <= 0 {
				limit = 5
			}

			similar, err := embSvc.FindSimilarBooks(ctx, input.BookID, limit)
			if err != nil {
				return nil, fmt.Errorf("find similar books: %w", err)
			}

			out := &FindSimilarBooksOutput{}
			for _, s := range similar {
				out.SimilarBooks = append(out.SimilarBooks, SimilarBookResult{
					BookID:   s.BookID,
					Title:    s.Title,
					Author:   s.Author,
					Category: s.Category,
					Score:    s.Score,
				})
			}
			return out, nil
		},
	)
}
