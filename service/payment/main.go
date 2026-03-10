package main

import (
	"fmt"
	"log"

	"github.com/go-micro/plugins/v4/registry/consul"
	consulapi "github.com/hashicorp/consul/api"
	amqp "github.com/rabbitmq/amqp091-go"
	"go-micro.dev/v4"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/qiwang/book-e-commerce-micro/common"
	"github.com/qiwang/book-e-commerce-micro/common/config"
	pb "github.com/qiwang/book-e-commerce-micro/proto/payment"
	"github.com/qiwang/book-e-commerce-micro/service/payment/handler"
	"github.com/qiwang/book-e-commerce-micro/service/payment/model"
	"github.com/qiwang/book-e-commerce-micro/service/payment/repository"
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

	if err := db.AutoMigrate(&model.Payment{}); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}

	mqConn, err := amqp.Dial(cfg.RabbitMQ.URL)
	if err != nil {
		log.Fatalf("failed to connect rabbitmq: %v", err)
	}
	defer mqConn.Close()

	mqCh, err := mqConn.Channel()
	if err != nil {
		log.Fatalf("failed to open rabbitmq channel: %v", err)
	}
	defer mqCh.Close()

	repo := repository.NewPaymentRepo(db, mqCh)
	h := handler.NewPaymentHandler(repo)

	reg := consul.NewRegistry(consul.Config(&consulapi.Config{Address: cfg.Consul.Address}))

	svc := micro.NewService(
		micro.Name(common.ServicePayment),
		micro.Version("1.0.0"),
		micro.Address(fmt.Sprintf(":%d", cfg.Services.Payment.GRPCPort)),
		micro.Registry(reg),
	)
	svc.Init()

	if err := pb.RegisterPaymentServiceHandler(svc.Server(), h); err != nil {
		log.Fatalf("failed to register handler: %v", err)
	}

	if err := svc.Run(); err != nil {
		log.Fatalf("service exited with error: %v", err)
	}
}
