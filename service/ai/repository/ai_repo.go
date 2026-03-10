package repository

import (
	"context"
	"time"

	"github.com/qiwang/book-e-commerce-micro/service/ai/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AIRepository struct {
	chatSessions  *mongo.Collection
	bookSummaries *mongo.Collection
}

func NewAIRepository(db *mongo.Database) *AIRepository {
	return &AIRepository{
		chatSessions:  db.Collection("chat_sessions"),
		bookSummaries: db.Collection("book_summaries"),
	}
}

// ---------------------------------------------------------------------------
// Chat Sessions
// ---------------------------------------------------------------------------

func (r *AIRepository) SaveChatSession(ctx context.Context, session *model.ChatSession) error {
	filter := bson.M{"session_id": session.SessionID}
	update := bson.M{
		"$set": bson.M{
			"user_id":    session.UserID,
			"messages":   session.Messages,
			"updated_at": time.Now(),
		},
		"$setOnInsert": bson.M{
			"created_at": session.CreatedAt,
		},
	}
	opts := options.Update().SetUpsert(true)
	_, err := r.chatSessions.UpdateOne(ctx, filter, update, opts)
	return err
}

func (r *AIRepository) LoadChatSession(ctx context.Context, sessionID string) (*model.ChatSession, error) {
	var session model.ChatSession
	err := r.chatSessions.FindOne(ctx, bson.M{"session_id": sessionID}).Decode(&session)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}

// ---------------------------------------------------------------------------
// Book Summaries
// ---------------------------------------------------------------------------

func (r *AIRepository) SaveBookSummary(ctx context.Context, summary *model.BookSummaryCache) error {
	filter := bson.M{"book_id": summary.BookID}
	update := bson.M{"$set": summary}
	opts := options.Update().SetUpsert(true)
	_, err := r.bookSummaries.UpdateOne(ctx, filter, update, opts)
	return err
}

func (r *AIRepository) GetBookSummary(ctx context.Context, bookID string) (*model.BookSummaryCache, error) {
	var summary model.BookSummaryCache
	err := r.bookSummaries.FindOne(ctx, bson.M{"book_id": bookID}).Decode(&summary)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &summary, nil
}

