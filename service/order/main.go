package main

import (
	"fmt"
	"log"

	"github.com/go-micro/plugins/v4/registry/consul"
	consulapi "github.com/hashicorp/consul/api"
	amqp "github.com/rabbitmq/amqp091-go"
	"go-micro.dev/v4"

	"github.com/qiwang/book-e-commerce-micro/common"
	"github.com/qiwang/book-e-commerce-micro/common/config"
	"github.com/qiwang/book-e-commerce-micro/common/sharding"
	inventoryPb "github.com/qiwang/book-e-commerce-micro/proto/inventory"
	pb "github.com/qiwang/book-e-commerce-micro/proto/order"
	"github.com/qiwang/book-e-commerce-micro/service/order/consumer"
	"github.com/qiwang/book-e-commerce-micro/service/order/handler"
	"github.com/qiwang/book-e-commerce-micro/service/order/model"
	"github.com/qiwang/book-e-commerce-micro/service/order/repository"
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
		mysqlToShardConfigs(cfg.Sharding.Order.Shards),
		&model.Order{}, &model.OrderItem{},
	)
	if err != nil {
		log.Fatalf("failed to init order shard router: %v", err)
	}

	mqConn, err := amqp.Dial(cfg.RabbitMQ.URL)
	if err != nil {
		log.Fatalf("failed to connect rabbitmq: %v", err)
	}
	defer mqConn.Close()

	pubCh, err := mqConn.Channel()
	if err != nil {
		log.Fatalf("failed to open rabbitmq publish channel: %v", err)
	}
	defer pubCh.Close()

	consCh, err := mqConn.Channel()
	if err != nil {
		log.Fatalf("failed to open rabbitmq consume channel: %v", err)
	}
	defer consCh.Close()

	repo := repository.NewOrderRepo(router, pubCh)

	reg := consul.NewRegistry(consul.Config(&consulapi.Config{Address: cfg.Consul.Address}))

	svc := micro.NewService(
		micro.Name(common.ServiceOrder),
		micro.Version("1.0.0"),
		micro.Address(fmt.Sprintf(":%d", cfg.Services.Order.GRPCPort)),
		micro.Registry(reg),
	)
	svc.Init()

	inventoryClient := inventoryPb.NewInventoryService(common.ServiceInventory, svc.Client())
	h := handler.NewOrderHandler(repo, inventoryClient)

	paymentConsumer := consumer.NewPaymentConsumer(consCh, repo, inventoryClient)
	if err := paymentConsumer.Start(); err != nil {
		log.Fatalf("failed to start payment consumer: %v", err)
	}

	if err := pb.RegisterOrderServiceHandler(svc.Server(), h); err != nil {
		log.Fatalf("failed to register handler: %v", err)
	}

	if err := svc.Run(); err != nil {
		log.Fatalf("service exited with error: %v", err)
	}
}
