package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/qiwang/book-e-commerce-micro/service/cart/model"
)

const cartTTL = 7 * 24 * time.Hour

type CartRepo struct {
	rdb *redis.Client
}

func NewCartRepo(rdb *redis.Client) *CartRepo {
	return &CartRepo{rdb: rdb}
}

func cartKey(userID uint64) string {
	return fmt.Sprintf("cart:%d", userID)
}

func (r *CartRepo) GetCart(ctx context.Context, userID uint64) (*model.CartData, error) {
	data, err := r.rdb.Get(ctx, cartKey(userID)).Bytes()
	if err == redis.Nil {
		return &model.CartData{UserID: userID}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}

	var cart model.CartData
	if err := json.Unmarshal(data, &cart); err != nil {
		return nil, fmt.Errorf("unmarshal cart: %w", err)
	}
	return &cart, nil
}

func (r *CartRepo) SaveCart(ctx context.Context, cart *model.CartData) error {
	data, err := json.Marshal(cart)
	if err != nil {
		return fmt.Errorf("marshal cart: %w", err)
	}
	return r.rdb.Set(ctx, cartKey(cart.UserID), data, cartTTL).Err()
}

func (r *CartRepo) DeleteCart(ctx context.Context, userID uint64) error {
	return r.rdb.Del(ctx, cartKey(userID)).Err()
}
