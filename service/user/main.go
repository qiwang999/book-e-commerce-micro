package main

import (
	"fmt"
	"log"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/go-micro/plugins/v4/registry/consul"
	"github.com/redis/go-redis/v9"
	"go-micro.dev/v4"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/qiwang/book-e-commerce-micro/common"
	"github.com/qiwang/book-e-commerce-micro/common/auth"
	"github.com/qiwang/book-e-commerce-micro/common/config"
	"github.com/qiwang/book-e-commerce-micro/common/email"
	pb "github.com/qiwang/book-e-commerce-micro/proto/user"
	"github.com/qiwang/book-e-commerce-micro/service/user/handler"
	"github.com/qiwang/book-e-commerce-micro/service/user/model"
	"github.com/qiwang/book-e-commerce-micro/service/user/repository"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := gorm.Open(mysql.Open(cfg.MySQL.DSN()), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to mysql: %v", err)
	}

	if err := db.AutoMigrate(&model.User{}, &model.UserProfile{}, &model.UserAddress{}); err != nil {
		log.Fatalf("failed to auto migrate: %v", err)
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

	jwtManager := auth.NewJWTManager(cfg.JWT.Secret, cfg.JWT.ExpireHours)
	emailSender := email.NewSender(cfg.Email)

	reg := consul.NewRegistry(consul.Config(&consulapi.Config{Address: cfg.Consul.Address}))

	svc := micro.NewService(
		micro.Name(common.ServiceUser),
		micro.Version("1.0.0"),
		micro.Address(fmt.Sprintf(":%d", cfg.Services.User.GRPCPort)),
		micro.Registry(reg),
	)
	svc.Init()

	userHandler := handler.NewUserHandler(
		repository.NewUserRepository(db),
		jwtManager,
		rdb,
		emailSender,
	)

	if err := pb.RegisterUserServiceHandler(svc.Server(), userHandler); err != nil {
		log.Fatalf("failed to register handler: %v", err)
	}

	if err := svc.Run(); err != nil {
		log.Fatalf("failed to run service: %v", err)
	}
}
