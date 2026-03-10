package handler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"

	"github.com/qiwang/book-e-commerce-micro/common/util"
	inventoryPb "github.com/qiwang/book-e-commerce-micro/proto/inventory"
	pb "github.com/qiwang/book-e-commerce-micro/proto/order"
	"github.com/qiwang/book-e-commerce-micro/service/order/model"
	"github.com/qiwang/book-e-commerce-micro/service/order/repository"
	"gorm.io/gorm"
)

func roundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}

type OrderHandler struct {
	repo         *repository.OrderRepo
	inventorySvc inventoryPb.InventoryService
}

func NewOrderHandler(repo *repository.OrderRepo, inventorySvc inventoryPb.InventoryService) *OrderHandler {
	return &OrderHandler{repo: repo, inventorySvc: inventorySvc}
}

func (h *OrderHandler) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest, rsp *pb.Order) error {
	if req.UserId == 0 {
		return errors.New("user_id is required")
	}
	if req.StoreId == 0 {
		return errors.New("store_id is required")
	}
	if len(req.Items) == 0 {
		return errors.New("order must have at least one item")
	}
	for _, item := range req.Items {
		if item.BookId == "" || item.Quantity <= 0 || item.Price <= 0 {
			return errors.New("each item requires book_id, positive quantity and price")
		}
	}

	var totalAmount float64
	items := make([]model.OrderItem, 0, len(req.Items))
	for _, item := range req.Items {
		totalAmount += roundMoney(item.Price * float64(item.Quantity))
		items = append(items, model.OrderItem{
			BookID:     item.BookId,
			BookTitle:  item.BookTitle,
			BookAuthor: item.BookAuthor,
			BookCover:  item.BookCover,
			Price:      item.Price,
			Quantity:   item.Quantity,
		})
	}

	var lockedIDs []string
	for i, item := range items {
		lockResp, err := h.inventorySvc.LockStock(ctx, &inventoryPb.LockStockRequest{
			StoreId:  req.StoreId,
			BookId:   item.BookID,
			Quantity: item.Quantity,
		})
		if err != nil {
			h.releaseLockedStock(ctx, lockedIDs, req.StoreId)
			return fmt.Errorf("failed to lock stock for book %s: %w", item.BookID, err)
		}
		if lockResp.Code != 0 {
			h.releaseLockedStock(ctx, lockedIDs, req.StoreId)
			return fmt.Errorf("insufficient stock for book %s: %s", item.BookID, lockResp.Message)
		}
		items[i].LockID = lockResp.LockId
		lockedIDs = append(lockedIDs, lockResp.LockId)
	}

	order := &model.Order{
		OrderNo:      util.GenerateOrderNo(req.UserId),
		UserID:       req.UserId,
		StoreID:      req.StoreId,
		TotalAmount:  totalAmount,
		Status:       "pending_payment",
		PickupMethod: req.PickupMethod,
		AddressID:    req.AddressId,
		Remark:       req.Remark,
	}

	if err := h.repo.CreateOrder(ctx, order, items); err != nil {
		log.Printf("CreateOrder error: %v", err)
		h.releaseLockedStock(ctx, lockedIDs, req.StoreId)
		return err
	}

	h.fillOrderResponse(rsp, order, items)
	return nil
}

func (h *OrderHandler) releaseLockedStock(ctx context.Context, lockIDs []string, storeID uint64) {
	for _, lockID := range lockIDs {
		if _, err := h.inventorySvc.ReleaseStock(ctx, &inventoryPb.ReleaseStockRequest{LockId: lockID, StoreId: storeID}); err != nil {
			log.Printf("failed to release stock lock %s: %v", lockID, err)
		}
	}
}

func (h *OrderHandler) GetOrder(ctx context.Context, req *pb.GetOrderRequest, rsp *pb.Order) error {
	var (
		order *model.Order
		err   error
	)

	if req.OrderNo != "" {
		order, err = h.repo.GetOrderByNo(ctx, req.OrderNo)
	} else if req.UserId != 0 {
		order, err = h.repo.GetOrderByID(ctx, req.OrderId, req.UserId)
	} else {
		return errors.New("user_id or order_no is required for shard routing")
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("order not found")
		}
		log.Printf("GetOrder error: %v", err)
		return err
	}

	items, err := h.repo.GetOrderItems(ctx, order.ID, order.UserID)
	if err != nil {
		log.Printf("GetOrderItems error: %v", err)
		return err
	}

	h.fillOrderResponse(rsp, order, items)
	return nil
}

func (h *OrderHandler) ListOrders(ctx context.Context, req *pb.ListOrdersRequest, rsp *pb.OrderListResponse) error {
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

	orders, total, err := h.repo.ListOrders(ctx, req.UserId, req.Status, page, pageSize)
	if err != nil {
		log.Printf("ListOrders error: %v", err)
		return err
	}

	rsp.Total = total

	orderIDs := make([]uint64, len(orders))
	for i, o := range orders {
		orderIDs[i] = o.ID
	}
	itemsMap, err := h.repo.GetOrderItemsByIDs(ctx, orderIDs, req.UserId)
	if err != nil {
		log.Printf("GetOrderItemsByIDs error: %v", err)
		itemsMap = make(map[uint64][]model.OrderItem)
	}

	for _, o := range orders {
		pbOrder := &pb.Order{}
		h.fillOrderResponse(pbOrder, &o, itemsMap[o.ID])
		rsp.Orders = append(rsp.Orders, pbOrder)
	}
	return nil
}

func (h *OrderHandler) CancelOrder(ctx context.Context, req *pb.CancelOrderRequest, rsp *pb.CommonResponse) error {
	order, err := h.repo.GetOrderByID(ctx, req.OrderId, req.UserId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			rsp.Code = 404
			rsp.Message = "order not found"
			return nil
		}
		return err
	}

	if order.UserID != req.UserId {
		rsp.Code = 403
		rsp.Message = "not authorized to cancel this order"
		return nil
	}

	if err := h.repo.CancelOrder(ctx, req.OrderId, req.UserId); err != nil {
		log.Printf("CancelOrder error: %v", err)
		rsp.Code = 400
		rsp.Message = err.Error()
		return nil
	}

	items, err := h.repo.GetOrderItems(ctx, req.OrderId, req.UserId)
	if err != nil {
		log.Printf("CancelOrder: failed to get order items for stock release: %v", err)
	} else {
		for _, item := range items {
			if item.LockID != "" {
				if _, err := h.inventorySvc.ReleaseStock(ctx, &inventoryPb.ReleaseStockRequest{LockId: item.LockID, StoreId: order.StoreID}); err != nil {
					log.Printf("CancelOrder: failed to release stock lock %s: %v", item.LockID, err)
				}
			}
		}
	}

	rsp.Code = 0
	rsp.Message = "success"
	return nil
}

func (h *OrderHandler) UpdateOrderStatus(ctx context.Context, req *pb.UpdateOrderStatusRequest, rsp *pb.CommonResponse) error {
	if err := h.repo.UpdateStatus(ctx, req.OrderId, req.UserId, req.Status); err != nil {
		log.Printf("UpdateOrderStatus error: %v", err)
		rsp.Code = 500
		rsp.Message = err.Error()
		return nil
	}

	rsp.Code = 0
	rsp.Message = "success"
	return nil
}

func (h *OrderHandler) fillOrderResponse(rsp *pb.Order, order *model.Order, items []model.OrderItem) {
	rsp.Id = order.ID
	rsp.OrderNo = order.OrderNo
	rsp.UserId = order.UserID
	rsp.StoreId = order.StoreID
	rsp.TotalAmount = order.TotalAmount
	rsp.Status = order.Status
	rsp.PickupMethod = order.PickupMethod
	rsp.AddressId = order.AddressID
	rsp.Remark = order.Remark
	rsp.CreatedAt = order.CreatedAt.Format("2006-01-02 15:04:05")

	if order.PaidAt != nil {
		rsp.PaidAt = order.PaidAt.Format("2006-01-02 15:04:05")
	}
	if order.CompletedAt != nil {
		rsp.CompletedAt = order.CompletedAt.Format("2006-01-02 15:04:05")
	}
	if order.CancelledAt != nil {
		rsp.CancelledAt = order.CancelledAt.Format("2006-01-02 15:04:05")
	}

	rsp.Items = make([]*pb.OrderItem, 0, len(items))
	for _, item := range items {
		rsp.Items = append(rsp.Items, &pb.OrderItem{
			Id:         item.ID,
			BookId:     item.BookID,
			BookTitle:  item.BookTitle,
			BookAuthor: item.BookAuthor,
			BookCover:  item.BookCover,
			Price:      item.Price,
			Quantity:   item.Quantity,
			LockId:     item.LockID,
		})
	}
}
