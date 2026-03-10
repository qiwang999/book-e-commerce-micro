package tools

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	orderPb "github.com/qiwang/book-e-commerce-micro/proto/order"
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

func NewCreateOrderTool(orderSvc orderPb.OrderService) (tool.InvokableTool, error) {
	return utils.InferTool(
		"create_order",
		"Create an order for the user. IMPORTANT: You MUST confirm with the user before calling this tool — show them the order summary (items, quantities, prices, total, store, pickup method) and wait for explicit confirmation.",
		func(ctx context.Context, input *CreateOrderInput) (*CreateOrderOutput, error) {
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
