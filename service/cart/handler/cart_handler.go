package handler

import (
	"context"
	"fmt"
	"math"

	"github.com/google/uuid"

	bookPb "github.com/qiwang/book-e-commerce-micro/proto/book"
	pb "github.com/qiwang/book-e-commerce-micro/proto/cart"
	inventoryPb "github.com/qiwang/book-e-commerce-micro/proto/inventory"
	"github.com/qiwang/book-e-commerce-micro/service/cart/model"
	"github.com/qiwang/book-e-commerce-micro/service/cart/repository"
)

func roundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}

type CartHandler struct {
	repo         *repository.CartRepo
	bookSvc      bookPb.BookService
	inventorySvc inventoryPb.InventoryService
}

func NewCartHandler(repo *repository.CartRepo, bookSvc bookPb.BookService, inventorySvc inventoryPb.InventoryService) *CartHandler {
	return &CartHandler{repo: repo, bookSvc: bookSvc, inventorySvc: inventorySvc}
}

const maxCartItems = 50

func (h *CartHandler) AddToCart(ctx context.Context, req *pb.AddToCartRequest, rsp *pb.Cart) error {
	if req.UserId == 0 || req.BookId == "" || req.StoreId == 0 {
		return fmt.Errorf("user_id, book_id, and store_id are required")
	}
	if req.Quantity <= 0 || req.Quantity > 99 {
		return fmt.Errorf("quantity must be between 1 and 99")
	}

	cart, err := h.repo.GetCart(ctx, req.UserId)
	if err != nil {
		return err
	}

	// Different store → replace cart
	if len(cart.Items) > 0 && cart.StoreID != req.StoreId {
		cart = &model.CartData{
			UserID:  req.UserId,
			StoreID: req.StoreId,
		}
	}
	cart.StoreID = req.StoreId

	// If the same book already exists, increment quantity
	found := false
	for i, item := range cart.Items {
		if item.BookID == req.BookId {
			cart.Items[i].Quantity += req.Quantity
			found = true
			break
		}
	}

	if !found {
		if len(cart.Items) >= maxCartItems {
			return fmt.Errorf("cart is full (max %d items)", maxCartItems)
		}
		bookResp, err := h.bookSvc.GetBookDetail(ctx, &bookPb.GetBookDetailRequest{BookId: req.BookId})
		if err != nil {
			return fmt.Errorf("failed to get book detail: %w", err)
		}

		stockResp, err := h.inventorySvc.CheckStock(ctx, &inventoryPb.CheckStockRequest{
			StoreId: req.StoreId,
			BookId:  req.BookId,
		})
		if err != nil {
			return fmt.Errorf("failed to check stock: %w", err)
		}

		price := stockResp.Price
		if price == 0 {
			price = bookResp.Price
		}

		cart.Items = append(cart.Items, model.CartItem{
			ItemID:     uuid.New().String(),
			BookID:     req.BookId,
			BookTitle:  bookResp.Title,
			BookAuthor: bookResp.Author,
			BookCover:  bookResp.CoverUrl,
			Price:      price,
			Quantity:   req.Quantity,
			StoreID:    req.StoreId,
		})
	}

	if err := h.repo.SaveCart(ctx, cart); err != nil {
		return err
	}

	fillCartResponse(cart, rsp)
	return nil
}

func (h *CartHandler) RemoveFromCart(ctx context.Context, req *pb.RemoveFromCartRequest, rsp *pb.Cart) error {
	cart, err := h.repo.GetCart(ctx, req.UserId)
	if err != nil {
		return err
	}

	items := cart.Items[:0]
	for _, item := range cart.Items {
		if item.ItemID != req.ItemId {
			items = append(items, item)
		}
	}
	cart.Items = items

	if err := h.repo.SaveCart(ctx, cart); err != nil {
		return err
	}

	fillCartResponse(cart, rsp)
	return nil
}

func (h *CartHandler) UpdateCartItem(ctx context.Context, req *pb.UpdateCartItemRequest, rsp *pb.Cart) error {
	cart, err := h.repo.GetCart(ctx, req.UserId)
	if err != nil {
		return err
	}

	for i, item := range cart.Items {
		if item.ItemID == req.ItemId {
			if req.Quantity <= 0 {
				cart.Items = append(cart.Items[:i], cart.Items[i+1:]...)
			} else {
				cart.Items[i].Quantity = req.Quantity
			}
			break
		}
	}

	if err := h.repo.SaveCart(ctx, cart); err != nil {
		return err
	}

	fillCartResponse(cart, rsp)
	return nil
}

func (h *CartHandler) GetCart(ctx context.Context, req *pb.GetCartRequest, rsp *pb.Cart) error {
	cart, err := h.repo.GetCart(ctx, req.UserId)
	if err != nil {
		return err
	}

	fillCartResponse(cart, rsp)
	return nil
}

func (h *CartHandler) ClearCart(ctx context.Context, req *pb.ClearCartRequest, rsp *pb.CommonResponse) error {
	if err := h.repo.DeleteCart(ctx, req.UserId); err != nil {
		return err
	}

	rsp.Code = 0
	rsp.Message = "cart cleared"
	return nil
}

func fillCartResponse(cart *model.CartData, rsp *pb.Cart) {
	rsp.UserId = cart.UserID
	rsp.StoreId = cart.StoreID
	rsp.Items = make([]*pb.CartItem, 0, len(cart.Items))

	var totalAmount float64
	var totalCount int32

	for _, item := range cart.Items {
		rsp.Items = append(rsp.Items, &pb.CartItem{
			ItemId:     item.ItemID,
			BookId:     item.BookID,
			BookTitle:  item.BookTitle,
			BookAuthor: item.BookAuthor,
			BookCover:  item.BookCover,
			Price:      item.Price,
			Quantity:   item.Quantity,
			StoreId:    item.StoreID,
		})
		totalAmount += roundMoney(item.Price * float64(item.Quantity))
		totalCount += item.Quantity
	}

	rsp.TotalAmount = totalAmount
	rsp.TotalCount = totalCount
}
