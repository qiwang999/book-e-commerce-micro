.PHONY: proto build config-init run-gateway run-user run-store run-book run-inventory run-cart run-order run-payment run-ai docker-up docker-down clean

PROTO_DIR=proto
GO_OUT=.

# Generate protobuf code for all services
proto:
	@echo "Generating protobuf code..."
	@for dir in $(PROTO_DIR)/*; do \
		if [ -d "$$dir" ]; then \
			protoc --proto_path=$(PROTO_DIR) \
				--go_out=$(GO_OUT) --go_opt=paths=source_relative \
				--micro_out=$(GO_OUT) --micro_opt=paths=source_relative \
				$$dir/*.proto; \
		fi; \
	done
	@echo "Done."

# Build all services
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

# Docker infrastructure
docker-up:
	docker-compose -f deploy/docker-compose.yml up -d

docker-down:
	docker-compose -f deploy/docker-compose.yml down

docker-logs:
	docker-compose -f deploy/docker-compose.yml logs -f

# Clean build artifacts
clean:
	rm -rf bin/
	@echo "Cleaned."

# Run all services (for development)
run-all: run-gateway run-user run-store run-book run-inventory run-cart run-order run-payment run-ai

# Tidy dependencies
tidy:
	go mod tidy
