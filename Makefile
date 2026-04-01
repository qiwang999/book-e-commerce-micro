.PHONY: proto build build-linux config-init \
       run-gateway run-user run-store run-book run-inventory run-cart run-order run-payment run-ai run-all \
       docker-up docker-down docker-build docker-logs docker-ps clean tidy

PROTO_DIR=proto

# Generate protobuf code for all services
proto:
	@echo "Generating protobuf code..."
	@for dir in $(PROTO_DIR)/*; do \
		if [ -d "$$dir" ]; then \
			protoc --proto_path=$(PROTO_DIR) \
				--go_out=$(PROTO_DIR) --go_opt=paths=source_relative \
				--micro_out=$(PROTO_DIR) --micro_opt=paths=source_relative \
				$$dir/*.proto; \
		fi; \
	done
	@echo "Done."

# Build all services (native)
build:
	@echo "Building all services..."
	go build -o bin/api-gateway ./api-gateway
	go build -o bin/user-service ./service/user
	go build -o bin/store-service ./service/store
	go build -o bin/book-service ./service/book
	go build -o bin/inventory-service ./service/inventory
	go build -o bin/cart-service ./service/cart
	go build -o bin/order-service ./service/order
	go build -o bin/payment-service ./service/payment
	go build -o bin/ai-service ./service/ai
	@echo "Done."

# Cross-compile static Linux binaries (for container image)
build-linux:
	@echo "Cross-compiling for Linux..."
	@mkdir -p bin/linux
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/linux/api-gateway    ./api-gateway
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/linux/user-service    ./service/user
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/linux/store-service   ./service/store
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/linux/book-service    ./service/book
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/linux/inventory-service ./service/inventory
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/linux/cart-service    ./service/cart
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/linux/order-service   ./service/order
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/linux/payment-service ./service/payment
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/linux/ai-service      ./service/ai
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/linux/config-init     ./cmd/config-init
	@echo "Done."

# Push local config.yaml to Consul KV config center
config-init:
	@echo "Pushing config.yaml to Consul KV..."
	go run ./cmd/config-init -consul=127.0.0.1:8500 -file=config.yaml
	@echo "Done. Config is now managed by Consul KV."

# Run individual services
run-gateway:
	go run ./api-gateway

run-user:
	go run ./service/user

run-store:
	go run ./service/store

run-book:
	go run ./service/book

run-inventory:
	go run ./service/inventory

run-cart:
	go run ./service/cart

run-order:
	go run ./service/order

run-payment:
	go run ./service/payment

run-ai:
	go run ./service/ai

# Podman: build binaries locally, then build image & start everything
docker-up: build-linux
	podman-compose -f deploy/docker-compose.yml up -d --build

docker-down:
	podman-compose -f deploy/docker-compose.yml down

docker-build: build-linux
	podman-compose -f deploy/docker-compose.yml build

docker-logs:
	podman-compose -f deploy/docker-compose.yml logs -f

docker-ps:
	podman-compose -f deploy/docker-compose.yml ps

# Clean build artifacts
clean:
	rm -rf bin/
	@echo "Cleaned."

# Run all services (for development)
run-all: run-gateway run-user run-store run-book run-inventory run-cart run-order run-payment run-ai

# Tidy dependencies
tidy:
	go mod tidy
