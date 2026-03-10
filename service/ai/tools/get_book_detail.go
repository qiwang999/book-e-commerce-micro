package tools

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	bookPb "github.com/qiwang/book-e-commerce-micro/proto/book"
)

type GetBookDetailInput struct {
	BookID string `json:"book_id" jsonschema:"description=The unique book ID,required"`
}

type GetBookDetailOutput struct {
	BookID      string  `json:"book_id"`
	Title       string  `json:"title"`
	Author      string  `json:"author"`
	Category    string  `json:"category"`
	Price       float64 `json:"price"`
	Rating      float64 `json:"rating"`
	Description string  `json:"description"`
	ISBN        string  `json:"isbn"`
}

func NewGetBookDetailTool(bookSvc bookPb.BookService) (tool.InvokableTool, error) {
	return utils.InferTool(
		"get_book_detail",
		"Get detailed information about a specific book by its ID",
		func(ctx context.Context, input *GetBookDetailInput) (*GetBookDetailOutput, error) {
			resp, err := bookSvc.GetBookDetail(ctx, &bookPb.GetBookDetailRequest{BookId: input.BookID})
			if err != nil {
				return nil, fmt.Errorf("get book detail: %w", err)
			}
			return &GetBookDetailOutput{
				BookID:      resp.Id,
				Title:       resp.Title,
				Author:      resp.Author,
				Category:    resp.Category,
				Price:       resp.Price,
				Rating:      resp.Rating,
				Description: resp.Description,
				ISBN:        resp.Isbn,
			}, nil
		},
	)
}
