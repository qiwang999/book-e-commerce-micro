package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	paymentPb "github.com/qiwang/book-e-commerce-micro/proto/payment"
	"github.com/qiwang/book-e-commerce-micro/service/ai/hitl"
)

type CreatePaymentInput struct {
	OrderID uint64  `json:"order_id" jsonschema:"description=The order ID to pay for,required"`
	UserID  uint64  `json:"user_id" jsonschema:"description=The user ID,required"`
	Amount  float64 `json:"amount" jsonschema:"description=Payment amount,required"`
	Method  string  `json:"method" jsonschema:"description=Payment method: wechat or alipay,required"`
}

type CreatePaymentOutput struct {
	PaymentNo string  `json:"payment_no"`
	Status    string  `json:"status"`
	Amount    float64 `json:"amount"`
	Method    string  `json:"method"`
	Message   string  `json:"message"`
}

func summarizeCreatePayment(in *CreatePaymentInput) string {
	return fmt.Sprintf("create_payment order=%d user=%d amount=%.2f method=%s", in.OrderID, in.UserID, in.Amount, in.Method)
}

func NewCreatePaymentTool(paymentSvc paymentPb.PaymentService) (tool.InvokableTool, error) {
	return utils.InferTool(
		"create_payment",
		"Create a payment for an order. IMPORTANT: You MUST confirm the payment amount and method with the user before calling this tool.",
		func(ctx context.Context, input *CreatePaymentInput) (*CreatePaymentOutput, error) {
			meta, metaOK := hitl.MetaFromContext(ctx)
			gate := hitl.GateFromContext(ctx)
			if gate != nil && metaOK && gate.Enabled() {
				if raw, ok := gate.TryConsumeApprovedArgs(ctx, meta, "create_payment"); ok {
					var stored CreatePaymentInput
					if err := json.Unmarshal(raw, &stored); err != nil {
						return nil, fmt.Errorf("hitl: bad stored create_payment args: %w", err)
					}
					input = &stored
				} else {
					b, err := json.Marshal(input)
					if err != nil {
						return nil, err
					}
					if err := gate.RegisterPending(ctx, meta, "create_payment", b, summarizeCreatePayment(input)); err != nil {
						return nil, err
					}
					return &CreatePaymentOutput{
						Message: "Payment creation is blocked until the user confirms in the client (see API hitl_* fields).",
					}, nil
				}
			}

			resp, err := paymentSvc.CreatePayment(ctx, &paymentPb.CreatePaymentRequest{
				OrderId: input.OrderID,
				UserId:  input.UserID,
				Amount:  input.Amount,
				Method:  input.Method,
			})
			if err != nil {
				return nil, fmt.Errorf("create payment: %w", err)
			}

			return &CreatePaymentOutput{
				PaymentNo: resp.PaymentNo,
				Status:    resp.Status,
				Amount:    resp.Amount,
				Method:    resp.Method,
				Message:   fmt.Sprintf("Payment %s created, please complete the payment via %s", resp.PaymentNo, input.Method),
			}, nil
		},
	)
}
