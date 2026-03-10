package model

import "time"

type LockStatus int

const (
	LockStatusLocked   LockStatus = 1
	LockStatusReleased LockStatus = 2
	LockStatusDeducted LockStatus = 3
)

type StoreInventory struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement"`
	StoreID        uint64    `gorm:"index:idx_store_book,unique;not null"`
	BookID         string    `gorm:"type:varchar(64);index:idx_store_book,unique;not null"`
	Quantity       int32     `gorm:"not null;default:0"`
	LockedQuantity int32     `gorm:"not null;default:0"`
	Price          float64   `gorm:"type:decimal(10,2);not null;default:0"`
	CreatedAt      time.Time `gorm:"autoCreateTime"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime"`
}

func (StoreInventory) TableName() string {
	return "store_inventory"
}

type InventoryLock struct {
	ID        uint64     `gorm:"primaryKey;autoIncrement"`
	LockID    string     `gorm:"type:varchar(64);uniqueIndex;not null"`
	StoreID   uint64     `gorm:"index;not null"`
	BookID    string     `gorm:"type:varchar(64);not null"`
	Quantity  int32      `gorm:"not null"`
	Status    LockStatus `gorm:"not null;default:1"`
	ExpireAt  time.Time  `gorm:"index;not null"`
	CreatedAt time.Time  `gorm:"autoCreateTime"`
}

func (InventoryLock) TableName() string {
	return "inventory_locks"
}
