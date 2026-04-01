package tools

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	bookPb "github.com/qiwang/book-e-commerce-micro/proto/book"
)

type SearchBooksInput struct {
	Keyword  string `json:"keyword,omitempty" jsonschema:"description=Search keyword for title or description"`
	Category string `json:"category,omitempty" jsonschema:"description=Book category to filter by"`
	Author   string `json:"author,omitempty" jsonschema:"description=Author name to filter by"`
}

type BookResult struct {
	BookID      string  `json:"book_id"`
	Title       string  `json:"title"`
	Author      string  `json:"author"`
	Category    string  `json:"category"`
	Price       float64 `json:"price"`
	Rating      float64 `json:"rating"`
	CoverURL    string  `json:"cover_url,omitempty"`
	Description string  `json:"description,omitempty"`
}

type SearchBooksOutput struct {
	Results []BookResult `json:"results"`
	Total   int64        `json:"total"`
}

func NewSearchBooksTool(bookSvc bookPb.BookService) (tool.InvokableTool, error) {
	return utils.InferTool(
		"search_books",
		"Search for books in the BookHive catalog by keyword, category, or author",
		func(ctx context.Context, input *SearchBooksInput) (*SearchBooksOutput, error) {
			resp, err := bookSvc.SearchBooks(ctx, &bookPb.SearchBooksRequest{
				Keyword:  input.Keyword,
				Category: input.Category,
				Author:   input.Author,
				Page:     1,
				PageSize: 5,
			})
			if err != nil {
				return nil, fmt.Errorf("search books: %w", err)
			}

			out := &SearchBooksOutput{Total: resp.Total}
			for _, b := range resp.Books {
			out.Results = append(out.Results, BookResult{
				BookID:      b.Id,
				Title:       b.Title,
				Author:      b.Author,
				Category:    b.Category,
				Price:       b.Price,
				Rating:      b.Rating,
				CoverURL:    b.CoverUrl,
				Description: b.Description,
			})
			}
			return out, nil
		},
	)
}
