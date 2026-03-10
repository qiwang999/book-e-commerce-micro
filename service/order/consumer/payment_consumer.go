package consumer

import (
	"context"
	"encoding/json"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	inventoryPb "github.com/qiwang/book-e-commerce-micro/proto/inventory"
	"github.com/qiwang/book-e-commerce-micro/service/order/repository"
)

type PaymentEvent struct {
	PaymentNo string  `json:"payment_no"`
	OrderID   uint64  `json:"order_id"`
	UserID    uint64  `json:"user_id"`
	Amount    float64 `json:"amount"`
	Status    string  `json:"status"`
}

type PaymentConsumer struct {
	ch           *amqp.Channel
	repo         *repository.OrderRepo
	inventorySvc inventoryPb.InventoryService
}

func NewPaymentConsumer(ch *amqp.Channel, repo *repository.OrderRepo, inventorySvc inventoryPb.InventoryService) *PaymentConsumer {
	return &PaymentConsumer{ch: ch, repo: repo, inventorySvc: inventorySvc}
}

func (c *PaymentConsumer) Start() error {
	q, err := c.ch.QueueDeclare("order.payment.success", true, false, false, false, nil)
	if err != nil {
		return err
	}

	if err := c.ch.QueueBind(q.Name, "payment.success", "payment.success", false, nil); err != nil {
		log.Printf("[consumer] queue bind warning: %v, will try direct consume", err)
	}

	msgs, err := c.ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		for msg := range msgs {
			c.handlePaymentSuccess(msg)
		}
		log.Println("[consumer] payment.success channel closed, consumer stopped")
	}()

	log.Println("[consumer] payment.success consumer started")
	return nil
}

func (c *PaymentConsumer) handlePaymentSuccess(msg amqp.Delivery) {
	var event PaymentEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		log.Printf("[consumer] failed to parse payment event: %v", err)
		msg.Nack(false, false)
		return
	}

	log.Printf("[consumer] processing payment.success for order %d (user %d)", event.OrderID, event.UserID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	order, err := c.repo.GetOrderByID(ctx, event.OrderID, event.UserID)
	if err != nil {
		log.Printf("[consumer] failed to get order %d: %v", event.OrderID, err)
		msg.Nack(false, true)
		return
	}

	if err := c.repo.UpdateStatus(ctx, event.OrderID, event.UserID, "paid"); err != nil {
		log.Printf("[consumer] failed to update order status: %v", err)
		msg.Nack(false, true)
		return
	}

	items, err := c.repo.GetOrderItems(ctx, event.OrderID, event.UserID)
	if err != nil {
		log.Printf("[consumer] failed to get order items: %v", err)
		msg.Nack(false, true)
		return
	}

	for _, item := range items {
		if item.LockID != "" {
			_, err := c.inventorySvc.DeductStock(ctx, &inventoryPb.DeductStockRequest{LockId: item.LockID, StoreId: order.StoreID})
			if err != nil {
				log.Printf("[consumer] failed to deduct stock for lock %s: %v", item.LockID, err)
			}
		}
	}

	msg.Ack(false)
	log.Printf("[consumer] order %d processed: status=paid, inventory deducted", event.OrderID)
}
