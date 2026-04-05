package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-micro/plugins/v4/registry/consul"
	consulapi "github.com/hashicorp/consul/api"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/qiwang/book-e-commerce-micro/common"
	"github.com/qiwang/book-e-commerce-micro/common/config"
	pb "github.com/qiwang/book-e-commerce-micro/proto/book"
	"github.com/qiwang/book-e-commerce-micro/service/book/handler"
	"github.com/qiwang/book-e-commerce-micro/service/book/repository"
	"go-micro.dev/v4"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoDB.URI))
	if err != nil {
		log.Fatalf("failed to connect to MongoDB: %v", err)
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			log.Printf("failed to disconnect MongoDB: %v", err)
		}
	}()

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("failed to ping MongoDB: %v", err)
	}
	log.Println("connected to MongoDB")

	db := client.Database(cfg.MongoDB.Database)
	repo := repository.NewBookRepository(db)

	var esRepo *repository.ESRepo
	if len(cfg.Elasticsearch.Addresses) > 0 {
		es, err := repository.NewESRepo(cfg.Elasticsearch.Addresses)
		if err != nil {
			log.Printf("failed to create ES client, full-text search disabled: %v", err)
		} else {
			if err := es.EnsureIndex(ctx); err != nil {
				log.Printf("failed to ensure ES index, full-text search disabled: %v", err)
			} else {
				esRepo = es
				log.Println("connected to Elasticsearch, full-text search enabled")
			}
		}
	}

	var mqConn *amqp.Connection
	var mqCh *amqp.Channel
	if cfg.RabbitMQ.URL != "" {
		c, err := amqp.Dial(cfg.RabbitMQ.URL)
		if err != nil {
			log.Printf("RabbitMQ unavailable, book.changed events disabled: %v", err)
		} else {
			ch, err := c.Channel()
			if err != nil {
				log.Printf("RabbitMQ channel failed: %v", err)
				_ = c.Close()
			} else {
				mqConn, mqCh = c, ch
				log.Println("connected to RabbitMQ (book.changed publisher)")
			}
		}
	}
	if mqConn != nil {
		defer mqConn.Close()
	}
	if mqCh != nil {
		defer mqCh.Close()
	}

	h := handler.NewBookHandler(repo, esRepo, mqCh)

	reg := consul.NewRegistry(consul.Config(&consulapi.Config{Address: cfg.Consul.Address}))
	svc := micro.NewService(
		micro.Name(common.ServiceBook),
		micro.Version("1.0.0"),
		micro.Address(fmt.Sprintf(":%d", cfg.Services.Book.GRPCPort)),
		micro.Registry(reg),
	)
	svc.Init()

	if err := pb.RegisterBookServiceHandler(svc.Server(), h); err != nil {
		log.Fatalf("failed to register handler: %v", err)
	}

	if err := svc.Run(); err != nil {
		log.Fatalf("service exited with error: %v", err)
	}
}
