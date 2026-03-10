package model

import "time"

type Payment struct {
	ID         uint64     `gorm:"primaryKey;autoIncrement"`
	PaymentNo  string     `gorm:"uniqueIndex;size:32"`
	OrderID    uint64     `gorm:"index"`
	UserID     uint64     `gorm:"index"`
	Amount     float64    `gorm:"type:decimal(10,2)"`
	Method     string     `gorm:"size:20;default:simulated"`
	Status     string     `gorm:"size:20;default:pending"`
	PaidAt     *time.Time
	RefundedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (Payment) TableName() string {
	return "payments"
}
