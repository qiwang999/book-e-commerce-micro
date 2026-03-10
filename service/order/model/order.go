package model

import "time"

type Order struct {
	ID           uint64     `gorm:"primaryKey;autoIncrement"`
	OrderNo      string     `gorm:"uniqueIndex;size:32"`
	UserID       uint64     `gorm:"index"`
	StoreID      uint64     `gorm:"index"`
	TotalAmount  float64    `gorm:"type:decimal(10,2)"`
	Status       string     `gorm:"size:20;index;default:pending_payment"`
	PickupMethod string     `gorm:"size:20;default:in_store"`
	AddressID    uint64
	Remark       string     `gorm:"type:text"`
	PaidAt       *time.Time
	CompletedAt  *time.Time
	CancelledAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (Order) TableName() string { return "orders" }

type OrderItem struct {
	ID         uint64  `gorm:"primaryKey;autoIncrement"`
	OrderID    uint64  `gorm:"index"`
	BookID     string  `gorm:"size:24"`
	BookTitle  string  `gorm:"size:500"`
	BookAuthor string  `gorm:"size:200"`
	BookCover  string  `gorm:"size:500"`
	Price      float64 `gorm:"type:decimal(10,2)"`
	Quantity   int32
	LockID     string `gorm:"size:36"`
	CreatedAt  time.Time
}

func (OrderItem) TableName() string { return "order_items" }
