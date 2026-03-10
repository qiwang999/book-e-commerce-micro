package tools

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	orderPb "github.com/qiwang/book-e-commerce-micro/proto/order"
)

type CancelOrderInput struct {
	OrderID uint64 `json:"order_id" jsonschema:"description=The numeric order ID to cancel,required"`
	UserID  uint64 `json:"user_id" jsonschema:"description=The user ID,required"`
}

type CancelOrderOutput struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func NewCancelOrderTool(orderSvc orderPb.OrderService) (tool.InvokableTool, error) {
	return utils.InferTool(
		"cancel_order",
		"Cancel an existing order. IMPORTANT: You MUST confirm with the user before calling this tool — inform them that cancellation is irreversible and wait for explicit confirmation.",
		func(ctx context.Context, input *CancelOrderInput) (*CancelOrderOutput, error) {
			resp, err := orderSvc.CancelOrder(ctx, &orderPb.CancelOrderRequest{
				OrderId: input.OrderID,
				UserId:  input.UserID,
			})
			if err != nil {
				return nil, fmt.Errorf("cancel order: %w", err)
			}

			return &CancelOrderOutput{
				Success: resp.Code == 0,
				Message: resp.Message,
			}, nil
		},
	)
}
