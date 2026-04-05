package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/qiwang/book-e-commerce-micro/common/bookevent"
	"github.com/qiwang/book-e-commerce-micro/service/ai/embedding"
)

const bookEmbeddingQueue = "ai.book.embedding"

// RunBookEmbeddingConsumer starts a background loop that consumes book.changed events
// and re-embeds the affected book. It reconnects on failure until ctx is cancelled.
func RunBookEmbeddingConsumer(ctx context.Context, rabbitURL string, emb *embedding.Service) {
	if rabbitURL == "" || emb == nil {
		return
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if err := consumeLoop(ctx, rabbitURL, emb); err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("[AI] book embedding consumer: %v, reconnect in 5s", err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
				}
			}
		}
	}()
	log.Println("[AI] book embedding consumer started")
}

func consumeLoop(ctx context.Context, rabbitURL string, emb *embedding.Service) error {
	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		return fmt.Errorf("dial rabbitmq: %w", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open channel: %w", err)
	}
	defer ch.Close()

	if err := ch.ExchangeDeclare(bookevent.ExchangeBookChanged, "fanout", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange: %w", err)
	}

	q, err := ch.QueueDeclare(bookEmbeddingQueue, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("declare queue: %w", err)
	}
	if err := ch.QueueBind(q.Name, "", bookevent.ExchangeBookChanged, false, nil); err != nil {
		return fmt.Errorf("bind queue: %w", err)
	}

	msgs, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-msgs:
			if !ok {
				return fmt.Errorf("delivery channel closed")
			}
			var p bookevent.BookChangedPayload
			if err := json.Unmarshal(d.Body, &p); err != nil {
				log.Printf("[AI] book embedding consumer: bad json: %v", err)
				_ = d.Ack(false)
				continue
			}
			if p.BookID != "" && (p.Event == bookevent.EventCreated || p.Event == bookevent.EventUpdated) {
				if _, err := emb.EmbedBook(ctx, p.BookID); err != nil {
					log.Printf("[AI] EmbedBook %s: %v", p.BookID, err)
				}
			}
			_ = d.Ack(false)
		}
	}
}
