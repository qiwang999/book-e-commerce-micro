package handler

import (
	"go-micro.dev/v4/client"

	"github.com/qiwang/book-e-commerce-micro/common"
	"github.com/qiwang/book-e-commerce-micro/common/storage"
	aiPb "github.com/qiwang/book-e-commerce-micro/proto/ai"
	bookPb "github.com/qiwang/book-e-commerce-micro/proto/book"
	cartPb "github.com/qiwang/book-e-commerce-micro/proto/cart"
	inventoryPb "github.com/qiwang/book-e-commerce-micro/proto/inventory"
	orderPb "github.com/qiwang/book-e-commerce-micro/proto/order"
	paymentPb "github.com/qiwang/book-e-commerce-micro/proto/payment"
	storePb "github.com/qiwang/book-e-commerce-micro/proto/store"
	userPb "github.com/qiwang/book-e-commerce-micro/proto/user"
)

type Handlers struct {
	User      userPb.UserService
	Store     storePb.StoreService
	Book      bookPb.BookService
	Inventory inventoryPb.InventoryService
	Cart      cartPb.CartService
	Order     orderPb.OrderService
	Payment   paymentPb.PaymentService
	AI        aiPb.AIService
	Storage   *storage.MinIOClient
}

func NewHandlers(client client.Client, store *storage.MinIOClient) *Handlers {
	return &Handlers{
		User:      userPb.NewUserService(common.ServiceUser, client),
		Store:     storePb.NewStoreService(common.ServiceStore, client),
		Book:      bookPb.NewBookService(common.ServiceBook, client),
		Inventory: inventoryPb.NewInventoryService(common.ServiceInventory, client),
		Cart:      cartPb.NewCartService(common.ServiceCart, client),
		Order:     orderPb.NewOrderService(common.ServiceOrder, client),
		Payment:   paymentPb.NewPaymentService(common.ServicePayment, client),
		AI:        aiPb.NewAIService(common.ServiceAI, client),
		Storage:   store,
	}
}
