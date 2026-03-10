package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/consul/api"
	consulplugin "github.com/go-micro/plugins/v4/registry/consul"
	"go-micro.dev/v4"

	"github.com/qiwang/book-e-commerce-micro/api-gateway/handler"
	"github.com/qiwang/book-e-commerce-micro/api-gateway/router"
	"github.com/qiwang/book-e-commerce-micro/common"
	"github.com/qiwang/book-e-commerce-micro/common/auth"
	"github.com/qiwang/book-e-commerce-micro/common/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	reg := consulplugin.NewRegistry(consulplugin.Config(&api.Config{Address: cfg.Consul.Address}))
	svc := micro.NewService(
		micro.Name(common.ServiceGateway),
		micro.Registry(reg),
	)
	svc.Init()

	jwtMgr := auth.NewJWTManager(cfg.JWT.Secret, cfg.JWT.ExpireHours)
	h := handler.NewHandlers(svc.Client())
	r := router.SetupRouter(h, jwtMgr)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Services.Gateway.HTTPPort),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	gatewayPort := cfg.Services.Gateway.HTTPPort
	serviceID := common.ServiceGateway + "-" + uuid.New().String()[:8]
	var consulClient *api.Client

	go func() {
		log.Printf("API Gateway starting on :%d", gatewayPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("gateway stopped: %v", err)
		}
	}()

	// Register gateway to Consul after server is up
	time.Sleep(200 * time.Millisecond)
	gatewayHost := cfg.Services.Gateway.Host
	if gatewayHost == "" {
		gatewayHost = "127.0.0.1"
	}
	consulCfg := api.DefaultConfig()
	consulCfg.Address = cfg.Consul.Address
	consulClient, err = api.NewClient(consulCfg)
	if err != nil {
		log.Printf("failed to create consul client for registration: %v", err)
	} else {
		healthURL := fmt.Sprintf("http://%s:%d/health", gatewayHost, gatewayPort)
		err = consulClient.Agent().ServiceRegister(&api.AgentServiceRegistration{
			ID:      serviceID,
			Name:    common.ServiceGateway,
			Address: gatewayHost,
			Port:    gatewayPort,
			Check: &api.AgentServiceCheck{
				HTTP:                           healthURL,
				Interval:                       "10s",
				Timeout:                        "2s",
				DeregisterCriticalServiceAfter: "30s",
			},
		})
		if err != nil {
			log.Printf("failed to register gateway to Consul: %v", err)
		} else {
			log.Printf("API Gateway registered to Consul as %s (id=%s)", common.ServiceGateway, serviceID)
		}
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("API Gateway shutting down...")
	if consulClient != nil {
		if err := consulClient.Agent().ServiceDeregister(serviceID); err != nil {
			log.Printf("failed to deregister gateway from Consul: %v", err)
		} else {
			log.Println("API Gateway deregistered from Consul")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("gateway forced shutdown: %v", err)
	}
	log.Println("API Gateway exited")
}
