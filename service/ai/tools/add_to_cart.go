package tools

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	cartPb "github.com/qiwang/book-e-commerce-micro/proto/cart"
)

type AddToCartInput struct {
	UserID   uint64 `json:"user_id" jsonschema:"description=The user ID,required"`
	StoreID  uint64 `json:"store_id" jsonschema:"description=The store ID,required"`
	BookID   string `json:"book_id" jsonschema:"description=The book ID to add to cart,required"`
	Quantity int32  `json:"quantity,omitempty" jsonschema:"description=Number of copies to add (default 1)"`
}

type AddToCartOutput struct {
	Success    bool    `json:"success"`
	TotalItems int32   `json:"total_items"`
	TotalAmount float64 `json:"total_amount"`
	Message    string  `json:"message"`
}

func NewAddToCartTool(cartSvc cartPb.CartService) (tool.InvokableTool, error) {
	return utils.InferTool(
		"add_to_cart",
		"Add a book to the user's shopping cart",
		func(ctx context.Context, input *AddToCartInput) (*AddToCartOutput, error) {
			qty := input.Quantity
			if qty <= 0 {
				qty = 1
			}

			resp, err := cartSvc.AddToCart(ctx, &cartPb.AddToCartRequest{
				UserId:   input.UserID,
				StoreId:  input.StoreID,
				BookId:   input.BookID,
				Quantity: qty,
			})
			if err != nil {
				return nil, fmt.Errorf("add to cart: %w", err)
			}

			return &AddToCartOutput{
				Success:     true,
				TotalItems:  resp.TotalCount,
				TotalAmount: resp.TotalAmount,
				Message:     fmt.Sprintf("Successfully added %d copy(s) to cart", qty),
			}, nil
		},
	)
}
