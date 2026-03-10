package tools

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	orderPb "github.com/qiwang/book-e-commerce-micro/proto/order"
)

type GetOrderDetailInput struct {
	OrderID uint64 `json:"order_id,omitempty" jsonschema:"description=The numeric order ID"`
	OrderNo string `json:"order_no,omitempty" jsonschema:"description=The order number (e.g. ORD-xxx)"`
	UserID  uint64 `json:"user_id" jsonschema:"description=The user ID,required"`
}

type OrderDetailItemResult struct {
	BookID    string  `json:"book_id"`
	BookTitle string  `json:"book_title"`
	Author    string  `json:"book_author"`
	Price     float64 `json:"price"`
	Quantity  int32   `json:"quantity"`
}

type GetOrderDetailOutput struct {
	OrderNo      string                  `json:"order_no"`
	OrderID      uint64                  `json:"order_id"`
	Status       string                  `json:"status"`
	TotalAmount  float64                 `json:"total_amount"`
	StoreID      uint64                  `json:"store_id"`
	PickupMethod string                  `json:"pickup_method"`
	Remark       string                  `json:"remark,omitempty"`
	Items        []OrderDetailItemResult `json:"items"`
	CreatedAt    string                  `json:"created_at"`
	PaidAt       string                  `json:"paid_at,omitempty"`
}

func NewGetOrderDetailTool(orderSvc orderPb.OrderService) (tool.InvokableTool, error) {
	return utils.InferTool(
		"get_order_detail",
		"Get detailed information about a specific order by order ID or order number",
		func(ctx context.Context, input *GetOrderDetailInput) (*GetOrderDetailOutput, error) {
			resp, err := orderSvc.GetOrder(ctx, &orderPb.GetOrderRequest{
				OrderId: input.OrderID,
				OrderNo: input.OrderNo,
				UserId:  input.UserID,
			})
			if err != nil {
				return nil, fmt.Errorf("get order detail: %w", err)
			}

			out := &GetOrderDetailOutput{
				OrderNo:      resp.OrderNo,
				OrderID:      resp.Id,
				Status:       resp.Status,
				TotalAmount:  resp.TotalAmount,
				StoreID:      resp.StoreId,
				PickupMethod: resp.PickupMethod,
				Remark:       resp.Remark,
				CreatedAt:    resp.CreatedAt,
				PaidAt:       resp.PaidAt,
			}
			for _, item := range resp.Items {
				out.Items = append(out.Items, OrderDetailItemResult{
					BookID:    item.BookId,
					BookTitle: item.BookTitle,
					Author:    item.BookAuthor,
					Price:     item.Price,
					Quantity:  item.Quantity,
				})
			}
			return out, nil
		},
	)
}
