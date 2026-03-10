package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/qiwang/book-e-commerce-micro/common/sharding"
	"github.com/qiwang/book-e-commerce-micro/common/util"
	"github.com/qiwang/book-e-commerce-micro/service/order/model"
	"gorm.io/gorm"
)

type OrderRepo struct {
	router *sharding.ShardRouter
	mqCh   *amqp.Channel
}

func NewOrderRepo(router *sharding.ShardRouter, mqCh *amqp.Channel) *OrderRepo {
	return &OrderRepo{router: router, mqCh: mqCh}
}

func (r *OrderRepo) db(userID uint64) *gorm.DB {
	return r.router.GetDB(userID)
}

func (r *OrderRepo) CreateOrder(ctx context.Context, order *model.Order, items []model.OrderItem) error {
	db := r.db(order.UserID)
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(order).Error; err != nil {
			return fmt.Errorf("create order: %w", err)
		}
		for i := range items {
			items[i].OrderID = order.ID
		}
		if err := tx.Create(&items).Error; err != nil {
			return fmt.Errorf("create order items: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	r.publishEvent("order.created", map[string]interface{}{
		"order_id":   order.ID,
		"order_no":   order.OrderNo,
		"user_id":    order.UserID,
		"store_id":   order.StoreID,
		"total":      order.TotalAmount,
		"status":     order.Status,
		"items_count": len(items),
		"created_at": order.CreatedAt.Format(time.RFC3339),
	})

	return nil
}

// GetOrderByID requires userID for shard routing.
func (r *OrderRepo) GetOrderByID(ctx context.Context, orderID uint64, userID uint64) (*model.Order, error) {
	var order model.Order
	if err := r.db(userID).WithContext(ctx).First(&order, orderID).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

// GetOrderByNo extracts the shard key embedded in the order number.
func (r *OrderRepo) GetOrderByNo(ctx context.Context, orderNo string) (*model.Order, error) {
	shardKey, err := util.ExtractShardKeyFromOrderNo(orderNo)
	if err != nil {
		return nil, fmt.Errorf("route order_no: %w", err)
	}
	var order model.Order
	if err := r.router.GetDB(shardKey).WithContext(ctx).Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *OrderRepo) GetOrderItems(ctx context.Context, orderID uint64, userID uint64) ([]model.OrderItem, error) {
	var items []model.OrderItem
	if err := r.db(userID).WithContext(ctx).Where("order_id = ?", orderID).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *OrderRepo) GetOrderItemsByIDs(ctx context.Context, orderIDs []uint64, userID uint64) (map[uint64][]model.OrderItem, error) {
	if len(orderIDs) == 0 {
		return make(map[uint64][]model.OrderItem), nil
	}
	var items []model.OrderItem
	if err := r.db(userID).WithContext(ctx).Where("order_id IN ?", orderIDs).Find(&items).Error; err != nil {
		return nil, err
	}
	result := make(map[uint64][]model.OrderItem, len(orderIDs))
	for _, item := range items {
		result[item.OrderID] = append(result[item.OrderID], item)
	}
	return result, nil
}

func (r *OrderRepo) ListOrders(ctx context.Context, userID uint64, status string, page, pageSize int) ([]model.Order, int64, error) {
	var total int64
	q := r.db(userID).WithContext(ctx).Model(&model.Order{}).Where("user_id = ?", userID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var orders []model.Order
	offset := (page - 1) * pageSize
	if err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// ListAllOrders fans out to all shards for admin queries. Results are merged
// by created_at descending. This is inherently slower than single-shard queries.
func (r *OrderRepo) ListAllOrders(ctx context.Context, status string, page, pageSize int) ([]model.Order, int64, error) {
	allDBs := r.router.AllDBs()
	type shardResult struct {
		orders []model.Order
		total  int64
		err    error
	}
	results := make([]shardResult, len(allDBs))
	var wg sync.WaitGroup

	for i, db := range allDBs {
		wg.Add(1)
		go func(idx int, sdb *gorm.DB) {
			defer wg.Done()
			q := sdb.WithContext(ctx).Model(&model.Order{})
			if status != "" {
				q = q.Where("status = ?", status)
			}
			var total int64
			if err := q.Count(&total).Error; err != nil {
				results[idx] = shardResult{err: err}
				return
			}
			var orders []model.Order
			offset := (page - 1) * pageSize
			if err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&orders).Error; err != nil {
				results[idx] = shardResult{err: err}
				return
			}
			results[idx] = shardResult{orders: orders, total: total}
		}(i, db)
	}
	wg.Wait()

	var merged []model.Order
	var totalCount int64
	for _, sr := range results {
		if sr.err != nil {
			return nil, 0, sr.err
		}
		merged = append(merged, sr.orders...)
		totalCount += sr.total
	}

	return merged, totalCount, nil
}

var validTransitions = map[string][]string{
	"pending_payment": {"paid", "cancelled"},
	"paid":            {"completed", "refunding"},
	"refunding":       {"refunded"},
	"completed":       {},
	"cancelled":       {},
	"refunded":        {},
}

func isValidTransition(from, to string) bool {
	for _, allowed := range validTransitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

func (r *OrderRepo) UpdateStatus(ctx context.Context, orderID uint64, userID uint64, status string) error {
	var order model.Order
	if err := r.db(userID).WithContext(ctx).Select("status").First(&order, orderID).Error; err != nil {
		return fmt.Errorf("order not found: %w", err)
	}
	if !isValidTransition(order.Status, status) {
		return fmt.Errorf("invalid status transition: %s -> %s", order.Status, status)
	}

	updates := map[string]interface{}{"status": status}
	now := time.Now()
	switch status {
	case "paid":
		updates["paid_at"] = now
	case "completed":
		updates["completed_at"] = now
	case "cancelled":
		updates["cancelled_at"] = now
	}
	return r.db(userID).WithContext(ctx).Model(&model.Order{}).Where("id = ?", orderID).Updates(updates).Error
}

func (r *OrderRepo) CancelOrder(ctx context.Context, orderID uint64, userID uint64) error {
	db := r.db(userID)
	var order model.Order
	if err := db.WithContext(ctx).First(&order, orderID).Error; err != nil {
		return err
	}
	if order.Status != "pending_payment" {
		return fmt.Errorf("order cannot be cancelled in status: %s", order.Status)
	}
	now := time.Now()
	return db.WithContext(ctx).Model(&order).Updates(map[string]interface{}{
		"status":       "cancelled",
		"cancelled_at": now,
	}).Error
}

func (r *OrderRepo) publishEvent(exchange string, body map[string]interface{}) {
	if r.mqCh == nil {
		return
	}
	data, err := json.Marshal(body)
	if err != nil {
		log.Printf("marshal mq message: %v", err)
		return
	}

	if err := r.mqCh.ExchangeDeclare(exchange, "fanout", true, false, false, false, nil); err != nil {
		log.Printf("declare exchange %s: %v", exchange, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := r.mqCh.PublishWithContext(ctx, exchange, "", false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        data,
	}); err != nil {
		log.Printf("publish to %s: %v", exchange, err)
	}
}
