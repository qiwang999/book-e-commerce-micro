// Package bookevent publishes book lifecycle events to RabbitMQ for downstream
// consumers (e.g. AI embedding service).
package bookevent

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	// ExchangeBookChanged is a durable fanout exchange; consumers bind their queues with empty routing key.
	ExchangeBookChanged = "book.changed"
)

// EventCreated / EventUpdated are event field values in BookChangedPayload.Event.
const (
	EventCreated = "created"
	EventUpdated = "updated"
)

type BookChangedPayload struct {
	Event  string `json:"event"`
	BookID string `json:"book_id"`
}

// Publish declares the exchange (idempotent) and publishes a JSON payload.
func Publish(ctx context.Context, ch *amqp.Channel, event, bookID string) error {
	if ch == nil || bookID == "" {
		return nil
	}
	if err := ch.ExchangeDeclare(ExchangeBookChanged, "fanout", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange %s: %w", ExchangeBookChanged, err)
	}
	body, err := json.Marshal(BookChangedPayload{Event: event, BookID: bookID})
	if err != nil {
		return err
	}
	return ch.PublishWithContext(ctx, ExchangeBookChanged, "", false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
}
