package tools

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	inventoryPb "github.com/qiwang/book-e-commerce-micro/proto/inventory"
)

type CheckStockInput struct {
	StoreID string `json:"store_id" jsonschema:"description=The store ID to check,required"`
	BookID  string `json:"book_id" jsonschema:"description=The book ID to check,required"`
}

type CheckStockOutput struct {
	InStock  bool    `json:"in_stock"`
	Quantity int32   `json:"quantity"`
	Price    float64 `json:"price"`
}

func NewCheckStockTool(inventorySvc inventoryPb.InventoryService) (tool.InvokableTool, error) {
	return utils.InferTool(
		"check_stock",
		"Check if a book is in stock at a specific store and get its price",
		func(ctx context.Context, input *CheckStockInput) (*CheckStockOutput, error) {
			storeID, _ := strconv.ParseUint(input.StoreID, 10, 64)
			resp, err := inventorySvc.CheckStock(ctx, &inventoryPb.CheckStockRequest{
				StoreId: storeID,
				BookId:  input.BookID,
			})
			if err != nil {
				return nil, fmt.Errorf("check stock: %w", err)
			}
			return &CheckStockOutput{
				InStock:  resp.Available > 0,
				Quantity: resp.Available,
				Price:    resp.Price,
			}, nil
		},
	)
}
