package tools

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	cartPb "github.com/qiwang/book-e-commerce-micro/proto/cart"
)

type GetCartInput struct {
	UserID uint64 `json:"user_id" jsonschema:"description=The user ID,required"`
}

type CartItemResult struct {
	ItemID   string  `json:"item_id"`
	BookID   string  `json:"book_id"`
	Title    string  `json:"title"`
	Author   string  `json:"author"`
	Price    float64 `json:"price"`
	Quantity int32   `json:"quantity"`
	StoreID  uint64  `json:"store_id"`
}

type GetCartOutput struct {
	Items       []CartItemResult `json:"items"`
	TotalCount  int32            `json:"total_count"`
	TotalAmount float64          `json:"total_amount"`
	StoreID     uint64           `json:"store_id"`
}

func NewGetCartTool(cartSvc cartPb.CartService) (tool.InvokableTool, error) {
	return utils.InferTool(
		"get_cart",
		"Get the contents of the user's shopping cart, including all items, quantities, prices, and totals",
		func(ctx context.Context, input *GetCartInput) (*GetCartOutput, error) {
			resp, err := cartSvc.GetCart(ctx, &cartPb.GetCartRequest{
				UserId: input.UserID,
			})
			if err != nil {
				return nil, fmt.Errorf("get cart: %w", err)
			}

			out := &GetCartOutput{
				TotalCount:  resp.TotalCount,
				TotalAmount: resp.TotalAmount,
				StoreID:     resp.StoreId,
			}
			for _, item := range resp.Items {
				out.Items = append(out.Items, CartItemResult{
					ItemID:   item.ItemId,
					BookID:   item.BookId,
					Title:    item.BookTitle,
					Author:   item.BookAuthor,
					Price:    item.Price,
					Quantity: item.Quantity,
					StoreID:  item.StoreId,
				})
			}
			return out, nil
		},
	)
}
