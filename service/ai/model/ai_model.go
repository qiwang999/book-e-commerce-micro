package model

import "time"

type ChatMessage struct {
	Role      string    `bson:"role" json:"role"`
	Content   string    `bson:"content" json:"content"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
}

type ChatSession struct {
	SessionID string        `bson:"session_id"`
	UserID    uint64        `bson:"user_id"`
	Messages  []ChatMessage `bson:"messages"`
	CreatedAt time.Time     `bson:"created_at"`
	UpdatedAt time.Time     `bson:"updated_at"`
}

type BookSummaryCache struct {
	BookID            string    `bson:"book_id"`
	Title             string    `bson:"title"`
	Summary           string    `bson:"summary"`
	KeyThemes         []string  `bson:"key_themes"`
	TargetAudience    string    `bson:"target_audience"`
	ReadingDifficulty string    `bson:"reading_difficulty"`
	EstReadingHours   int32     `bson:"est_reading_hours"`
	CreatedAt         time.Time `bson:"created_at"`
}

