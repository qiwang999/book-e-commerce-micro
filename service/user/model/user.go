package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

type StringSlice []string

func (s StringSlice) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal StringSlice: %w", err)
	}
	return string(b), nil
}

func (s *StringSlice) Scan(value interface{}) error {
	if value == nil {
		*s = StringSlice{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		return fmt.Errorf("unsupported type for StringSlice: %T", value)
	}
	return json.Unmarshal(bytes, s)
}

type User struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement"`
	Email        string    `gorm:"uniqueIndex;size:128;not null"`
	PasswordHash string    `gorm:"size:256;not null"`
	Name         string    `gorm:"size:64;not null"`
	AvatarURL    string    `gorm:"size:512"`
	Role         string    `gorm:"size:32;default:user"`
	Status       int       `gorm:"default:1"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}

func (User) TableName() string {
	return "users"
}

type UserProfile struct {
	ID                 uint64      `gorm:"primaryKey;autoIncrement"`
	UserID             uint64      `gorm:"uniqueIndex:user_id;not null"`
	User               User        `gorm:"constraint:OnDelete:CASCADE;foreignKey:UserID;references:ID"`
	Phone              string      `gorm:"size:20;default:''"`
	Gender             int32       `gorm:"default:0"`
	Birthday           *time.Time  `gorm:"type:date"`
	FavoriteCategories StringSlice `gorm:"type:json"`
	FavoriteAuthors    StringSlice `gorm:"type:json"`
	ReadingPreferences StringSlice `gorm:"type:json"`
	CreatedAt          time.Time   `gorm:"autoCreateTime"`
	UpdatedAt          time.Time   `gorm:"autoUpdateTime"`
}

func (UserProfile) TableName() string {
	return "user_profiles"
}

type UserAddress struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	UserID    uint64    `gorm:"index;not null"`
	Name      string    `gorm:"size:64;not null"`
	Phone     string    `gorm:"size:32;not null"`
	Province  string    `gorm:"size:64;not null"`
	City      string    `gorm:"size:64;not null"`
	District  string    `gorm:"size:64;not null"`
	Detail    string    `gorm:"size:256;not null"`
	IsDefault bool      `gorm:"default:false"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (UserAddress) TableName() string {
	return "user_addresses"
}
