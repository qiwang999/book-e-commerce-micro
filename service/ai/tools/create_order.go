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

type CreateOrderItemInput struct {
	BookID    string  `json:"book_id" jsonschema:"description=The book ID,required"`
	BookTitle string  `json:"book_title" jsonschema:"description=The book title,required"`
	BookAuthor string `json:"book_author,omitempty" jsonschema:"description=The book author"`
	Price     float64 `json:"price" jsonschema:"description=Unit price of the book,required"`
	Quantity  int32   `json:"quantity" jsonschema:"description=Number of copies,required"`
}

type CreateOrderInput struct {
	UserID       uint64                 `json:"user_id" jsonschema:"description=The user ID,required"`
	StoreID      uint64                 `json:"store_id" jsonschema:"description=The store ID,required"`
	Items        []CreateOrderItemInput `json:"items" jsonschema:"description=List of books to order,required"`
	PickupMethod string                 `json:"pickup_method" jsonschema:"description=Pickup method: self_pickup or delivery,required"`
	AddressID    uint64                 `json:"address_id,omitempty" jsonschema:"description=Delivery address ID (required for delivery)"`
	Remark       string                 `json:"remark,omitempty" jsonschema:"description=Optional order remark"`
}

type CreateOrderOutput struct {
	OrderNo     string  `json:"order_no"`
	OrderID     uint64  `json:"order_id"`
	Status      string  `json:"status"`
	TotalAmount float64 `json:"total_amount"`
	Message     string  `json:"message"`
}

func summarizeCreateOrder(in *CreateOrderInput) string {
	var sum float64
	for _, it := range in.Items {
		sum += it.Price * float64(it.Quantity)
	}
	return fmt.Sprintf("create_order user=%d store=%d pickup=%s lines=%d est_total=%.2f", in.UserID, in.StoreID, in.PickupMethod, len(in.Items), sum)
}

func NewCreateOrderTool(orderSvc orderPb.OrderService) (tool.InvokableTool, error) {
	return utils.InferTool(
		"create_order",
		"Create an order for the user. IMPORTANT: You MUST confirm with the user before calling this tool — show them the order summary (items, quantities, prices, total, store, pickup method) and wait for explicit confirmation.",
		func(ctx context.Context, input *CreateOrderInput) (*CreateOrderOutput, error) {
			meta, metaOK := hitl.MetaFromContext(ctx)
			gate := hitl.GateFromContext(ctx)
			if gate != nil && metaOK && gate.Enabled() {
				if raw, ok := gate.TryConsumeApprovedArgs(ctx, meta, "create_order"); ok {
					var stored CreateOrderInput
					if err := json.Unmarshal(raw, &stored); err != nil {
						return nil, fmt.Errorf("hitl: bad stored create_order args: %w", err)
					}
					input = &stored
				} else {
					b, err := json.Marshal(input)
					if err != nil {
						return nil, err
					}
					if err := gate.RegisterPending(ctx, meta, "create_order", b, summarizeCreateOrder(input)); err != nil {
						return nil, err
					}
					return &CreateOrderOutput{
						Message: "This order is blocked until the user confirms in the client (see API hitl_* fields). Ask them to confirm in the app, then retry.",
					}, nil
				}
			}

			var items []*orderPb.OrderItemInput
			for _, item := range input.Items {
				items = append(items, &orderPb.OrderItemInput{
					BookId:     item.BookID,
					BookTitle:  item.BookTitle,
					BookAuthor: item.BookAuthor,
					Price:      item.Price,
					Quantity:   item.Quantity,
				})
			}

			resp, err := orderSvc.CreateOrder(ctx, &orderPb.CreateOrderRequest{
				UserId:       input.UserID,
				StoreId:      input.StoreID,
				Items:        items,
				PickupMethod: input.PickupMethod,
				AddressId:    input.AddressID,
				Remark:       input.Remark,
			})
			if err != nil {
				return nil, fmt.Errorf("create order: %w", err)
			}

			return &CreateOrderOutput{
				OrderNo:     resp.OrderNo,
				OrderID:     resp.Id,
				Status:      resp.Status,
				TotalAmount: resp.TotalAmount,
				Message:     fmt.Sprintf("Order %s created successfully", resp.OrderNo),
			}, nil
		},
	)
}
