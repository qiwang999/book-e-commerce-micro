package handler

import (
	"context"
	"errors"
	"log"

	pb "github.com/qiwang/book-e-commerce-micro/proto/inventory"
	"github.com/qiwang/book-e-commerce-micro/service/inventory/repository"
	"gorm.io/gorm"
)

type InventoryHandler struct {
	repo *repository.InventoryRepo
}

func NewInventoryHandler(repo *repository.InventoryRepo) *InventoryHandler {
	return &InventoryHandler{repo: repo}
}

func (h *InventoryHandler) CheckStock(ctx context.Context, req *pb.CheckStockRequest, rsp *pb.StockInfo) error {
	inv, err := h.repo.GetStock(ctx, req.StoreId, req.BookId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			rsp.StoreId = req.StoreId
			rsp.BookId = req.BookId
			return nil
		}
		log.Printf("CheckStock error: %v", err)
		return err
	}

	rsp.StoreId = inv.StoreID
	rsp.BookId = inv.BookID
	rsp.Quantity = inv.Quantity
	rsp.LockedQuantity = inv.LockedQuantity
	rsp.Available = inv.Quantity - inv.LockedQuantity
	rsp.Price = inv.Price
	return nil
}

func (h *InventoryHandler) BatchCheckStock(ctx context.Context, req *pb.BatchCheckStockRequest, rsp *pb.BatchStockResponse) error {
	inventories, err := h.repo.BatchGetStock(ctx, req.StoreId, req.BookIds)
	if err != nil {
		log.Printf("BatchCheckStock error: %v", err)
		return err
	}

	for _, inv := range inventories {
		rsp.Items = append(rsp.Items, &pb.StockInfo{
			StoreId:        inv.StoreID,
			BookId:         inv.BookID,
			Quantity:       inv.Quantity,
			LockedQuantity: inv.LockedQuantity,
			Available:      inv.Quantity - inv.LockedQuantity,
			Price:          inv.Price,
		})
	}
	return nil
}

func (h *InventoryHandler) SetStock(ctx context.Context, req *pb.SetStockRequest, rsp *pb.CommonResponse) error {
	if req.StoreId == 0 || req.BookId == "" {
		rsp.Code = 400
		rsp.Message = "store_id and book_id are required"
		return nil
	}
	if req.Quantity < 0 {
		rsp.Code = 400
		rsp.Message = "quantity must be non-negative"
		return nil
	}
	if req.Price < 0 {
		rsp.Code = 400
		rsp.Message = "price must be non-negative"
		return nil
	}

	if err := h.repo.SetStock(ctx, req.StoreId, req.BookId, req.Quantity, req.Price); err != nil {
		log.Printf("SetStock error: %v", err)
		rsp.Code = 500
		rsp.Message = "failed to set stock"
		return nil
	}

	rsp.Code = 0
	rsp.Message = "success"
	return nil
}

func (h *InventoryHandler) LockStock(ctx context.Context, req *pb.LockStockRequest, rsp *pb.LockStockResponse) error {
	if req.StoreId == 0 || req.BookId == "" || req.Quantity <= 0 {
		rsp.Code = 400
		rsp.Message = "store_id, book_id and positive quantity are required"
		return nil
	}

	lockID, err := h.repo.LockStock(ctx, req.StoreId, req.BookId, req.Quantity)
	if err != nil {
		log.Printf("LockStock error: %v", err)
		rsp.Code = 500
		rsp.Message = "failed to lock stock"
		return nil
	}

	rsp.Code = 0
	rsp.Message = "success"
	rsp.LockId = lockID
	return nil
}

func (h *InventoryHandler) ReleaseStock(ctx context.Context, req *pb.ReleaseStockRequest, rsp *pb.CommonResponse) error {
	if err := h.repo.ReleaseStock(ctx, req.LockId, req.StoreId); err != nil {
		log.Printf("ReleaseStock error: %v", err)
		rsp.Code = 500
		rsp.Message = "failed to release stock"
		return nil
	}

	rsp.Code = 0
	rsp.Message = "success"
	return nil
}

func (h *InventoryHandler) DeductStock(ctx context.Context, req *pb.DeductStockRequest, rsp *pb.CommonResponse) error {
	if err := h.repo.DeductStock(ctx, req.LockId, req.StoreId); err != nil {
		log.Printf("DeductStock error: %v", err)
		rsp.Code = 500
		rsp.Message = "failed to deduct stock"
		return nil
	}

	rsp.Code = 0
	rsp.Message = "success"
	return nil
}

func (h *InventoryHandler) GetStoreBooks(ctx context.Context, req *pb.GetStoreBooksRequest, rsp *pb.StoreBookListResponse) error {
	page := int(req.Page)
	pageSize := int(req.PageSize)
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	inventories, total, err := h.repo.GetStoreBooks(ctx, req.StoreId, page, pageSize)
	if err != nil {
		log.Printf("GetStoreBooks error: %v", err)
		return err
	}

	rsp.Total = total
	for _, inv := range inventories {
		rsp.Items = append(rsp.Items, &pb.StoreBookItem{
			BookId:    inv.BookID,
			Quantity:  inv.Quantity,
			Available: inv.Quantity - inv.LockedQuantity,
			Price:     inv.Price,
		})
	}
	return nil
}
