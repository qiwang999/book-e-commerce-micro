package repository

import (
	"context"
	"time"

	"github.com/qiwang/book-e-commerce-micro/service/book/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type BookRepository struct {
	collection *mongo.Collection
}

func NewBookRepository(db *mongo.Database) *BookRepository {
	return &BookRepository{
		collection: db.Collection("books"),
	}
}

func (r *BookRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*model.Book, error) {
	var book model.Book
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&book)
	if err != nil {
		return nil, err
	}
	return &book, nil
}

func (r *BookRepository) FindByIDs(ctx context.Context, ids []primitive.ObjectID) ([]*model.Book, error) {
	filter := bson.M{"_id": bson.M{"$in": ids}}
	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var books []*model.Book
	if err := cursor.All(ctx, &books); err != nil {
		return nil, err
	}
	return books, nil
}

func (r *BookRepository) Search(ctx context.Context, keyword, category, author, language string, minPrice, maxPrice float64, sortBy string, page, pageSize int32) ([]*model.Book, int64, error) {
	filter := bson.M{}

	if keyword != "" {
		filter["$or"] = bson.A{
			bson.M{"title": bson.M{"$regex": keyword, "$options": "i"}},
			bson.M{"description": bson.M{"$regex": keyword, "$options": "i"}},
			bson.M{"author": bson.M{"$regex": keyword, "$options": "i"}},
		}
	}
	if category != "" {
		filter["category"] = category
	}
	if author != "" {
		filter["author"] = bson.M{"$regex": author, "$options": "i"}
	}
	if language != "" {
		filter["language"] = language
	}
	if minPrice > 0 || maxPrice > 0 {
		priceFilter := bson.M{}
		if minPrice > 0 {
			priceFilter["$gte"] = minPrice
		}
		if maxPrice > 0 {
			priceFilter["$lte"] = maxPrice
		}
		filter["price"] = priceFilter
	}

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	opts := options.Find().
		SetSkip(int64((page - 1) * pageSize)).
		SetLimit(int64(pageSize))

	switch sortBy {
	case "price_asc":
		opts.SetSort(bson.D{{Key: "price", Value: 1}})
	case "price_desc":
		opts.SetSort(bson.D{{Key: "price", Value: -1}})
	case "rating":
		opts.SetSort(bson.D{{Key: "rating", Value: -1}})
	case "newest":
		opts.SetSort(bson.D{{Key: "created_at", Value: -1}})
	default:
		opts.SetSort(bson.D{{Key: "created_at", Value: -1}})
	}

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var books []*model.Book
	if err := cursor.All(ctx, &books); err != nil {
		return nil, 0, err
	}
	return books, total, nil
}

func (r *BookRepository) ListByCategory(ctx context.Context, category, subcategory string, page, pageSize int32) ([]*model.Book, int64, error) {
	filter := bson.M{"category": category}
	if subcategory != "" {
		filter["subcategory"] = subcategory
	}

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	opts := options.Find().
		SetSkip(int64((page - 1) * pageSize)).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var books []*model.Book
	if err := cursor.All(ctx, &books); err != nil {
		return nil, 0, err
	}
	return books, total, nil
}

func (r *BookRepository) Create(ctx context.Context, book *model.Book) (*model.Book, error) {
	now := time.Now()
	book.CreatedAt = now
	book.UpdatedAt = now

	result, err := r.collection.InsertOne(ctx, book)
	if err != nil {
		return nil, err
	}
	book.ID = result.InsertedID.(primitive.ObjectID)
	return book, nil
}

func (r *BookRepository) Update(ctx context.Context, id primitive.ObjectID, update bson.M) (*model.Book, error) {
	update["updated_at"] = time.Now()

	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *BookRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

type CategoryResult struct {
	Name         string   `bson:"_id"`
	Subcategories []string `bson:"subcategories"`
	Count        int64    `bson:"count"`
}

func (r *BookRepository) ListCategories(ctx context.Context) ([]*CategoryResult, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id":           "$category",
			"subcategories": bson.M{"$addToSet": "$subcategory"},
			"count":         bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.M{"count": -1}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []*CategoryResult
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}
