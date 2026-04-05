package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	orderPb "github.com/qiwang/book-e-commerce-micro/proto/order"
	"github.com/qiwang/book-e-commerce-micro/service/ai/hitl"
)

type CancelOrderInput struct {
	OrderID uint64 `json:"order_id" jsonschema:"description=The numeric order ID to cancel,required"`
	UserID  uint64 `json:"user_id" jsonschema:"description=The user ID,required"`
}

type CancelOrderOutput struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func summarizeCancelOrder(in *CancelOrderInput) string {
	return fmt.Sprintf("cancel_order order=%d user=%d", in.OrderID, in.UserID)
}

func NewCancelOrderTool(orderSvc orderPb.OrderService) (tool.InvokableTool, error) {
	return utils.InferTool(
		"cancel_order",
		"Cancel an existing order. IMPORTANT: You MUST confirm with the user before calling this tool — inform them that cancellation is irreversible and wait for explicit confirmation.",
		func(ctx context.Context, input *CancelOrderInput) (*CancelOrderOutput, error) {
			meta, metaOK := hitl.MetaFromContext(ctx)
			gate := hitl.GateFromContext(ctx)
			if gate != nil && metaOK && gate.Enabled() {
				if raw, ok := gate.TryConsumeApprovedArgs(ctx, meta, "cancel_order"); ok {
					var stored CancelOrderInput
					if err := json.Unmarshal(raw, &stored); err != nil {
						return nil, fmt.Errorf("hitl: bad stored cancel_order args: %w", err)
					}
					input = &stored
				} else {
					b, err := json.Marshal(input)
					if err != nil {
						return nil, err
					}
					if err := gate.RegisterPending(ctx, meta, "cancel_order", b, summarizeCancelOrder(input)); err != nil {
						return nil, err
					}
					return &CancelOrderOutput{
						Success: false,
						Message: "Cancellation is blocked until the user confirms in the client (see API hitl_* fields).",
					}, nil
				}
			}

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
