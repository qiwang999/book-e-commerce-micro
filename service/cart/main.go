package main

import (
	"context"
	"fmt"
	"log"

	"github.com/go-micro/plugins/v4/registry/consul"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/redis/go-redis/v9"
	"go-micro.dev/v4"

	"github.com/qiwang/book-e-commerce-micro/common"
	"github.com/qiwang/book-e-commerce-micro/common/config"
	bookPb "github.com/qiwang/book-e-commerce-micro/proto/book"
	pb "github.com/qiwang/book-e-commerce-micro/proto/cart"
	inventoryPb "github.com/qiwang/book-e-commerce-micro/proto/inventory"
	"github.com/qiwang/book-e-commerce-micro/service/cart/handler"
	"github.com/qiwang/book-e-commerce-micro/service/cart/repository"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer rdb.Close()
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("failed to connect redis: %v", err)
	}

	reg := consul.NewRegistry(consul.Config(&consulapi.Config{Address: cfg.Consul.Address}))

	svc := micro.NewService(
		micro.Name(common.ServiceCart),
		micro.Version("1.0.0"),
		micro.Address(fmt.Sprintf(":%d", cfg.Services.Cart.GRPCPort)),
		micro.Registry(reg),
	)
	svc.Init()

	repo := repository.NewCartRepo(rdb)
	bookClient := bookPb.NewBookService(common.ServiceBook, svc.Client())
	inventoryClient := inventoryPb.NewInventoryService(common.ServiceInventory, svc.Client())
	h := handler.NewCartHandler(repo, bookClient, inventoryClient)

	if err := pb.RegisterCartServiceHandler(svc.Server(), h); err != nil {
		log.Fatalf("failed to register handler: %v", err)
	}

	if err := svc.Run(); err != nil {
		log.Fatalf("service stopped: %v", err)
	}
}
