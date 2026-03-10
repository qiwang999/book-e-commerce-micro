package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Book struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	Title       string             `bson:"title"`
	Author      string             `bson:"author"`
	ISBN        string             `bson:"isbn"`
	Publisher   string             `bson:"publisher"`
	PublishDate string             `bson:"publish_date"`
	Price       float64            `bson:"price"`
	Category    string             `bson:"category"`
	Subcategory string             `bson:"subcategory"`
	Description string             `bson:"description"`
	CoverURL    string             `bson:"cover_url"`
	Pages       int32              `bson:"pages"`
	Language    string             `bson:"language"`
	Tags        []string           `bson:"tags"`
	Rating      float64            `bson:"rating"`
	RatingCount int64              `bson:"rating_count"`
	CreatedAt   time.Time          `bson:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at"`
}
