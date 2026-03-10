package main

import (
	"fmt"
	"log"

	"github.com/go-micro/plugins/v4/registry/consul"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/qiwang/book-e-commerce-micro/common"
	"github.com/qiwang/book-e-commerce-micro/common/config"
	"github.com/qiwang/book-e-commerce-micro/service/store/handler"
	"github.com/qiwang/book-e-commerce-micro/service/store/repository"
	pb "github.com/qiwang/book-e-commerce-micro/proto/store"
	"github.com/redis/go-redis/v9"
	"go-micro.dev/v4"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := gorm.Open(mysql.Open(cfg.MySQL.DSN()), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect mysql: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("failed to get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MySQL.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MySQL.MaxIdleConns)

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	repo := repository.NewStoreRepository(db, rdb)
	h := handler.NewStoreHandler(repo)

	reg := consul.NewRegistry(consul.Config(&consulapi.Config{Address: cfg.Consul.Address}))

	svc := micro.NewService(
		micro.Name(common.ServiceStore),
		micro.Version("1.0.0"),
		micro.Address(fmt.Sprintf(":%d", cfg.Services.Store.GRPCPort)),
		micro.Registry(reg),
	)
	svc.Init()

	if err := pb.RegisterStoreServiceHandler(svc.Server(), h); err != nil {
		log.Fatalf("failed to register handler: %v", err)
	}

	if err := svc.Run(); err != nil {
		log.Fatalf("service exited with error: %v", err)
	}
}
