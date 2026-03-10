package model

import "time"

type Store struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name          string    `gorm:"type:varchar(100);not null" json:"name"`
	Description   string    `gorm:"type:text" json:"description"`
	Address       string    `gorm:"type:varchar(255);not null" json:"address"`
	City          string    `gorm:"type:varchar(50);not null;index" json:"city"`
	District      string    `gorm:"type:varchar(50)" json:"district"`
	Phone         string    `gorm:"type:varchar(20)" json:"phone"`
	Latitude      float64   `gorm:"type:double;not null" json:"latitude"`
	Longitude     float64   `gorm:"type:double;not null" json:"longitude"`
	BusinessHours string    `gorm:"type:varchar(100)" json:"business_hours"`
	Status        int32     `gorm:"type:tinyint;default:1;index" json:"status"`
	ImageURL      string    `gorm:"type:varchar(255)" json:"image_url"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Store) TableName() string {
	return "stores"
}

// StoreWithDistance is used for spatial queries that return a computed distance.
type StoreWithDistance struct {
	Store
	Distance float64 `gorm:"column:distance" json:"distance"`
}
