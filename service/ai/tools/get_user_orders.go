package tools

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	orderPb "github.com/qiwang/book-e-commerce-micro/proto/order"
)

type GetUserOrdersInput struct {
	UserID uint64 `json:"user_id" jsonschema:"description=The user ID to look up orders for,required"`
	Status string `json:"status,omitempty" jsonschema:"description=Filter by order status (pending/paid/completed/cancelled). Leave empty for all."`
	Limit  int    `json:"limit,omitempty" jsonschema:"description=Maximum number of orders to return (default 10)"`
}

type OrderResult struct {
	OrderNo     string            `json:"order_no"`
	Status      string            `json:"status"`
	TotalAmount float64           `json:"total_amount"`
	CreatedAt   string            `json:"created_at"`
	Items       []OrderItemResult `json:"items"`
}

type OrderItemResult struct {
	BookID    string  `json:"book_id"`
	BookTitle string  `json:"book_title"`
	Author    string  `json:"book_author"`
	Price     float64 `json:"price"`
	Quantity  int32   `json:"quantity"`
}

type GetUserOrdersOutput struct {
	Orders []OrderResult `json:"orders"`
	Total  int64         `json:"total"`
}

func NewGetUserOrdersTool(orderSvc orderPb.OrderService) (tool.InvokableTool, error) {
	return utils.InferTool(
		"get_user_orders",
		"Get a user's order history to understand their purchase patterns and reading preferences",
		func(ctx context.Context, input *GetUserOrdersInput) (*GetUserOrdersOutput, error) {
			limit := int32(input.Limit)
			if limit <= 0 {
				limit = 10
			}
			if limit > 20 {
				limit = 20
			}

			resp, err := orderSvc.ListOrders(ctx, &orderPb.ListOrdersRequest{
				UserId:   input.UserID,
				Status:   input.Status,
				Page:     1,
				PageSize: limit,
			})
			if err != nil {
				return nil, fmt.Errorf("list user orders: %w", err)
			}

			out := &GetUserOrdersOutput{Total: resp.Total}
			for _, o := range resp.Orders {
				or := OrderResult{
					OrderNo:     o.OrderNo,
					Status:      o.Status,
					TotalAmount: o.TotalAmount,
					CreatedAt:   o.CreatedAt,
				}
				for _, item := range o.Items {
					or.Items = append(or.Items, OrderItemResult{
						BookID:    item.BookId,
						BookTitle: item.BookTitle,
						Author:    item.BookAuthor,
						Price:     item.Price,
						Quantity:  item.Quantity,
					})
				}
				out.Orders = append(out.Orders, or)
			}
			return out, nil
		},
	)
}
