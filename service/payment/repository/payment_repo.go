package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/qiwang/book-e-commerce-micro/service/payment/model"
	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PaymentRepo struct {
	db      *gorm.DB
	mqCh    *amqp.Channel
}

func NewPaymentRepo(db *gorm.DB, mqCh *amqp.Channel) *PaymentRepo {
	return &PaymentRepo{db: db, mqCh: mqCh}
}

func (r *PaymentRepo) CreatePayment(ctx context.Context, p *model.Payment) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *PaymentRepo) GetByPaymentNo(ctx context.Context, paymentNo string) (*model.Payment, error) {
	var p model.Payment
	if err := r.db.WithContext(ctx).Where("payment_no = ?", paymentNo).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PaymentRepo) GetByOrderID(ctx context.Context, orderID uint64) (*model.Payment, error) {
	var p model.Payment
	if err := r.db.WithContext(ctx).Where("order_id = ?", orderID).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PaymentRepo) ProcessPayment(ctx context.Context, paymentNo string) (*model.Payment, error) {
	var result model.Payment

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var p model.Payment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("payment_no = ?", paymentNo).First(&p).Error; err != nil {
			return fmt.Errorf("payment not found: %w", err)
		}
		if p.Status != "pending" {
			return fmt.Errorf("payment already processed, current status: %s", p.Status)
		}

		now := time.Now()
		if err := tx.Model(&p).Updates(map[string]interface{}{
			"status":  "success",
			"paid_at": now,
		}).Error; err != nil {
			return fmt.Errorf("failed to update payment: %w", err)
		}
		p.Status = "success"
		p.PaidAt = &now
		result = p
		return nil
	})
	if err != nil {
		return nil, err
	}

	r.publishPaymentEvent("payment.success", &result)
	return &result, nil
}

func (r *PaymentRepo) RefundPayment(ctx context.Context, paymentNo string) (*model.Payment, error) {
	var result model.Payment

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var p model.Payment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("payment_no = ?", paymentNo).First(&p).Error; err != nil {
			return fmt.Errorf("payment not found: %w", err)
		}
		if p.Status != "success" {
			return fmt.Errorf("only successful payments can be refunded, current status: %s", p.Status)
		}

		now := time.Now()
		if err := tx.Model(&p).Updates(map[string]interface{}{
			"status":      "refunded",
			"refunded_at": now,
		}).Error; err != nil {
			return fmt.Errorf("failed to refund payment: %w", err)
		}
		p.Status = "refunded"
		p.RefundedAt = &now
		result = p
		return nil
	})
	if err != nil {
		return nil, err
	}

	r.publishPaymentEvent("payment.refunded", &result)
	return &result, nil
}

type paymentEvent struct {
	PaymentNo string  `json:"payment_no"`
	OrderID   uint64  `json:"order_id"`
	UserID    uint64  `json:"user_id"`
	Amount    float64 `json:"amount"`
	Status    string  `json:"status"`
}

func (r *PaymentRepo) publishPaymentEvent(routingKey string, p *model.Payment) {
	if r.mqCh == nil {
		return
	}

	evt := paymentEvent{
		PaymentNo: p.PaymentNo,
		OrderID:   p.OrderID,
		UserID:    p.UserID,
		Amount:    p.Amount,
		Status:    p.Status,
	}

	body, err := json.Marshal(evt)
	if err != nil {
		log.Printf("failed to marshal payment event: %v", err)
		return
	}

	err = r.mqCh.PublishWithContext(context.Background(),
		"",         // default exchange
		routingKey, // routing key / queue name
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
	if err != nil {
		log.Printf("failed to publish %s event: %v", routingKey, err)
	}
}
