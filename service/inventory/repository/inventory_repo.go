package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/qiwang/book-e-commerce-micro/common/sharding"
	"github.com/qiwang/book-e-commerce-micro/service/inventory/model"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	stockCachePrefix = "inventory:stock:"
	stockCacheTTL    = 5 * time.Minute
	lockExpiry       = 15 * time.Minute
)

type InventoryRepo struct {
	router *sharding.ShardRouter
	rdb    *redis.Client
}

func NewInventoryRepo(router *sharding.ShardRouter, rdb *redis.Client) *InventoryRepo {
	return &InventoryRepo{router: router, rdb: rdb}
}

func (r *InventoryRepo) db(storeID uint64) *gorm.DB {
	return r.router.GetDB(storeID)
}

func stockCacheKey(storeID uint64, bookID string) string {
	return fmt.Sprintf("%s%d:%s", stockCachePrefix, storeID, bookID)
}

func (r *InventoryRepo) GetStock(ctx context.Context, storeID uint64, bookID string) (*model.StoreInventory, error) {
	key := stockCacheKey(storeID, bookID)
	cached, err := r.rdb.Get(ctx, key).Bytes()
	if err == nil {
		var inv model.StoreInventory
		if json.Unmarshal(cached, &inv) == nil {
			return &inv, nil
		}
	}

	var inv model.StoreInventory
	result := r.db(storeID).WithContext(ctx).
		Where("store_id = ? AND book_id = ?", storeID, bookID).
		First(&inv)
	if result.Error != nil {
		return nil, result.Error
	}

	r.cacheStock(ctx, key, &inv)
	return &inv, nil
}

func (r *InventoryRepo) BatchGetStock(ctx context.Context, storeID uint64, bookIDs []string) ([]model.StoreInventory, error) {
	var inventories []model.StoreInventory
	result := r.db(storeID).WithContext(ctx).
		Where("store_id = ? AND book_id IN ?", storeID, bookIDs).
		Find(&inventories)
	if result.Error != nil {
		return nil, result.Error
	}
	return inventories, nil
}

func (r *InventoryRepo) SetStock(ctx context.Context, storeID uint64, bookID string, quantity int32, price float64) error {
	db := r.db(storeID)
	inv := model.StoreInventory{
		StoreID:  storeID,
		BookID:   bookID,
		Quantity: quantity,
		Price:    price,
	}

	result := db.WithContext(ctx).
		Where("store_id = ? AND book_id = ?", storeID, bookID).
		First(&model.StoreInventory{})

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		if err := db.WithContext(ctx).Create(&inv).Error; err != nil {
			return err
		}
	} else if result.Error != nil {
		return result.Error
	} else {
		if err := db.WithContext(ctx).
			Model(&model.StoreInventory{}).
			Where("store_id = ? AND book_id = ?", storeID, bookID).
			Updates(map[string]interface{}{
				"quantity": quantity,
				"price":    price,
			}).Error; err != nil {
			return err
		}
	}

	r.invalidateCache(ctx, storeID, bookID)
	return nil
}

func (r *InventoryRepo) LockStock(ctx context.Context, storeID uint64, bookID string, quantity int32) (string, error) {
	lockID := uuid.New().String()
	db := r.db(storeID)

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var inv model.StoreInventory
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("store_id = ? AND book_id = ?", storeID, bookID).
			First(&inv).Error; err != nil {
			return fmt.Errorf("inventory not found: %w", err)
		}

		available := inv.Quantity - inv.LockedQuantity
		if available < quantity {
			return fmt.Errorf("insufficient stock: available %d, requested %d", available, quantity)
		}

		if err := tx.Model(&model.StoreInventory{}).
			Where("store_id = ? AND book_id = ?", storeID, bookID).
			Update("locked_quantity", gorm.Expr("locked_quantity + ?", quantity)).Error; err != nil {
			return err
		}

		lock := model.InventoryLock{
			LockID:   lockID,
			StoreID:  storeID,
			BookID:   bookID,
			Quantity: quantity,
			Status:   model.LockStatusLocked,
			ExpireAt: time.Now().Add(lockExpiry),
		}
		return tx.Create(&lock).Error
	})

	if err != nil {
		return "", err
	}

	r.invalidateCache(ctx, storeID, bookID)
	return lockID, nil
}

// ReleaseStock requires storeID for shard routing. The lock record and inventory
// live in the same shard (keyed by store_id), so the transaction is local.
func (r *InventoryRepo) ReleaseStock(ctx context.Context, lockID string, storeID uint64) error {
	db := r.db(storeID)
	var bookID string

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var lock model.InventoryLock
		if err := tx.Where("lock_id = ? AND status = ?", lockID, model.LockStatusLocked).
			First(&lock).Error; err != nil {
			return fmt.Errorf("lock not found or already processed: %w", err)
		}
		bookID = lock.BookID

		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("store_id = ? AND book_id = ?", lock.StoreID, lock.BookID).
			First(&model.StoreInventory{}).Error; err != nil {
			return err
		}

		if err := tx.Model(&model.StoreInventory{}).
			Where("store_id = ? AND book_id = ? AND locked_quantity >= ?", lock.StoreID, lock.BookID, lock.Quantity).
			Update("locked_quantity", gorm.Expr("locked_quantity - ?", lock.Quantity)).Error; err != nil {
			return err
		}

		return tx.Model(&lock).Update("status", model.LockStatusReleased).Error
	})
	if err != nil {
		return err
	}
	r.invalidateCache(ctx, storeID, bookID)
	return nil
}

// DeductStock requires storeID for shard routing.
func (r *InventoryRepo) DeductStock(ctx context.Context, lockID string, storeID uint64) error {
	db := r.db(storeID)
	var bookID string

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var lock model.InventoryLock
		if err := tx.Where("lock_id = ? AND status = ?", lockID, model.LockStatusLocked).
			First(&lock).Error; err != nil {
			return fmt.Errorf("lock not found or already processed: %w", err)
		}
		bookID = lock.BookID

		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("store_id = ? AND book_id = ?", lock.StoreID, lock.BookID).
			First(&model.StoreInventory{}).Error; err != nil {
			return err
		}

		if err := tx.Model(&model.StoreInventory{}).
			Where("store_id = ? AND book_id = ? AND quantity >= ? AND locked_quantity >= ?",
				lock.StoreID, lock.BookID, lock.Quantity, lock.Quantity).
			Updates(map[string]interface{}{
				"quantity":        gorm.Expr("quantity - ?", lock.Quantity),
				"locked_quantity": gorm.Expr("locked_quantity - ?", lock.Quantity),
			}).Error; err != nil {
			return err
		}

		return tx.Model(&lock).Update("status", model.LockStatusDeducted).Error
	})
	if err != nil {
		return err
	}
	r.invalidateCache(ctx, storeID, bookID)
	return nil
}

func (r *InventoryRepo) GetStoreBooks(ctx context.Context, storeID uint64, page, pageSize int) ([]model.StoreInventory, int64, error) {
	db := r.db(storeID)
	var total int64
	if err := db.WithContext(ctx).
		Model(&model.StoreInventory{}).
		Where("store_id = ?", storeID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var inventories []model.StoreInventory
	offset := (page - 1) * pageSize
	result := db.WithContext(ctx).
		Where("store_id = ?", storeID).
		Offset(offset).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&inventories)
	if result.Error != nil {
		return nil, 0, result.Error
	}
	return inventories, total, nil
}

func (r *InventoryRepo) cacheStock(ctx context.Context, key string, inv *model.StoreInventory) {
	data, err := json.Marshal(inv)
	if err != nil {
		return
	}
	r.rdb.Set(ctx, key, data, stockCacheTTL)
}

func (r *InventoryRepo) invalidateCache(ctx context.Context, storeID uint64, bookID string) {
	r.rdb.Del(ctx, stockCacheKey(storeID, bookID))
}
