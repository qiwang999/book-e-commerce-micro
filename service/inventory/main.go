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
	"github.com/qiwang/book-e-commerce-micro/common/sharding"
	pb "github.com/qiwang/book-e-commerce-micro/proto/inventory"
	"github.com/qiwang/book-e-commerce-micro/service/inventory/handler"
	"github.com/qiwang/book-e-commerce-micro/service/inventory/model"
	"github.com/qiwang/book-e-commerce-micro/service/inventory/repository"
)

func mysqlToShardConfigs(cfgs []config.MySQLConfig) []sharding.ShardConfig {
	out := make([]sharding.ShardConfig, len(cfgs))
	for i, c := range cfgs {
		out[i] = sharding.ShardConfig{
			Host:         c.Host,
			Port:         c.Port,
			User:         c.User,
			Password:     c.Password,
			Database:     c.Database,
			MaxOpenConns: c.MaxOpenConns,
			MaxIdleConns: c.MaxIdleConns,
		}
	}
	return out
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	router, err := sharding.NewShardRouter(
		mysqlToShardConfigs(cfg.Sharding.Inventory.Shards),
		&model.StoreInventory{}, &model.InventoryLock{},
	)
	if err != nil {
		log.Fatalf("failed to init inventory shard router: %v", err)
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
		micro.Name(common.ServiceInventory),
		micro.Version("1.0.0"),
		micro.Address(fmt.Sprintf(":%d", cfg.Services.Inventory.GRPCPort)),
		micro.Registry(reg),
	)
	svc.Init()

	repo := repository.NewInventoryRepo(router, rdb)
	h := handler.NewInventoryHandler(repo)

	if err := pb.RegisterInventoryServiceHandler(svc.Server(), h); err != nil {
		log.Fatalf("failed to register handler: %v", err)
	}

	if err := svc.Run(); err != nil {
		log.Fatalf("service stopped: %v", err)
	}
}
