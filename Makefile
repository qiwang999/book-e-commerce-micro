.PHONY: proto build build-linux config-init \
       run-gateway run-user run-store run-book run-inventory run-cart run-order run-payment run-ai run-all \
       docker-up docker-up-nobuild docker-down docker-restart docker-restart-check docker-fix-ai docker-diagnose docker-build docker-logs docker-ps docker-attu clean tidy \
       seed-big seed-big-smoke backfill-embeddings es-delete-books-index mongo-seed-books seed-books-openlibrary seed-books-10k seed-books-100k

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
	@find $(PROTO_DIR) -name '*.pb.micro.go' -print0 | xargs -0 perl -pi -e 's|go-micro.dev/v5/|go-micro.dev/v4/|g' 2>/dev/null || true
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
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/linux/openlibrary-seed ./cmd/openlibrary-seed
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

# Podman: 交叉编译 Linux 二进制 → 构建镜像 → 启动全栈。冷启动时 Milvus/ES/多套 MySQL 首次初始化可占数分钟，
# 终端可能长时间只有容器名、无新行，另开终端可看: make docker-ps 或 podman logs -f bookhive-milvus
docker-up: build-linux
	podman-compose -f deploy/docker-compose.yml up -d --build

# 不跑 build-linux（加快反复 up）；镜像或 bin/linux 已就绪时用。需重建应用镜像时先 make docker-build。
docker-up-nobuild:
	podman-compose -f deploy/docker-compose.yml up -d

docker-down:
	podman-compose -f deploy/docker-compose.yml down

# podman-compose 无 `rm` 子命令；用 down（默认不删卷）卸容器与网络，再 up -d。不 --build。
docker-restart:
	podman-compose -f deploy/docker-compose.yml down
	podman-compose -f deploy/docker-compose.yml up -d

# down → up -d，轮询关键容器 Running + 网关 /health（最长约 5 分钟）
docker-restart-check:
	podman-compose -f deploy/docker-compose.yml down
	podman-compose -f deploy/docker-compose.yml up -d
	@echo "Waiting for critical containers + gateway /health (max ~300s)..."
	@deadline=$$(( $$(date +%s) + 300 )); \
	while [ $$(date +%s) -lt $$deadline ]; do \
	  ok=true; \
	  for c in bookhive-consul bookhive-mongo bookhive-book bookhive-ai bookhive-gateway; do \
	    st=$$(podman inspect -f '{{.State.Running}}' $$c 2>/dev/null || echo false); \
	    if [ "$$st" != "true" ]; then ok=false; break; fi; \
	  done; \
	  if $$ok && curl -sf http://127.0.0.1:8080/health >/dev/null; then \
	    echo "Restart OK: critical containers running + gateway /health"; \
	    exit 0; \
	  fi; \
	  sleep 5; \
	done; \
	echo "Restart check FAILED"; \
	podman ps -a --filter name=bookhive- 2>/dev/null | head -50 || true; \
	exit 1

# 仅删掉并重建 ai-service（依赖需已运行）
docker-fix-ai:
	podman rm -f bookhive-ai 2>/dev/null || true
	podman-compose -f deploy/docker-compose.yml up -d ai-service

# bookhive-ai 报 internal libpod error 时：看 19008/8080 占用、AI 最近日志、Podman 版本
docker-diagnose:
	@echo '--- lsof 19008 8080 (可能端口占用) ---'
	-lsof -nP -iTCP:19008 -sTCP:LISTEN 2>/dev/null || true
	-lsof -nP -iTCP:8080 -sTCP:LISTEN 2>/dev/null || true
	@echo '--- podman version ---'
	-podman version 2>/dev/null || true
	@echo '--- bookhive-ai last logs ---'
	-podman logs --tail 30 bookhive-ai 2>/dev/null || true

docker-build: build-linux
	podman-compose -f deploy/docker-compose.yml build

docker-logs:
	podman-compose -f deploy/docker-compose.yml logs -f

docker-ps:
	podman-compose -f deploy/docker-compose.yml ps

# Milvus 可视化（需先有 milvus 容器；已全量 up 过时可用 --no-deps 避免重建依赖）
docker-attu:
	podman-compose -f deploy/docker-compose.yml up -d --no-deps attu

# Clean build artifacts
clean:
	rm -rf bin/
	@echo "Cleaned."

# Run all services (for development)
run-all: run-gateway run-user run-store run-book run-inventory run-cart run-order run-payment run-ai

# Large demo dataset (MongoDB books + MySQL main + order/inventory shards). Destructive with -truncate.
# MySQL defaults: Docker compose ports. Mongo default: no-auth localhost:27017; compose Mongo needs:
#   -mongo-uri='mongodb://bookhive:bookhive123@127.0.0.1:27017/?authSource=admin&authMechanism=SCRAM-SHA-256'
seed-big-smoke:
	go run ./cmd/seed-big -truncate -mongo-wipe \
		-users=20000 -orders=5000 -inventory-rows=30000 -payments=2000 -books=2000 -stores=100

# 在 compose 默认网络内调用 AI gRPC（避免宿主机映射 19008 时 Podman internal libpod error）
COMPOSE_NETWORK ?= deploy_default
backfill-embeddings:
	podman run --rm -v $$(pwd):/src:Z -w /src --network $(COMPOSE_NETWORK) \
	  docker.io/library/golang:bookworm \
	  go run ./cmd/backfill-embeddings -consul-addr=consul:8500 -ai-addr=ai-service:9008

# 删除 ES books 索引（compose 默认 http://127.0.0.1:9200）。若曾用旧 mapping 建过错索引，删后重启 book-service 会按新 mapping 重建。
es-delete-books-index:
	@curl -sS -X DELETE "http://127.0.0.1:9200/books" -w "\nHTTP %{http_code}\n" || true

# 本机直连 Mongo：按 ISBN 从 Open Library 拉元数据并 upsert（需 27017 可连，如 compose 已 up）
seed-books-openlibrary:
	go run ./cmd/openlibrary-seed \
		-mongo-uri='mongodb://bookhive:bookhive123@127.0.0.1:27017/?authSource=admin&authMechanism=SCRAM-SHA-256' \
		-database=bookhive \
		-isbns-file=deploy/mongo/seed-isbns.txt \
		-upsert=true

# 从 OL Search API 按主题批量拉取 ~10000 本书写入 Mongo（compose 网络内运行）
seed-books-10k: build-linux
	podman-compose -f deploy/docker-compose.yml build mongo-seed-books
	podman run --rm --network $(COMPOSE_NETWORK) \
	  localhost/deploy_mongo-seed-books:latest \
	  /app/openlibrary-seed \
	  -mode=search \
	  "-mongo-uri=mongodb://bookhive:bookhive123@bookhive-mongo:27017/?authSource=admin&authMechanism=SCRAM-SHA-256" \
	  -database=bookhive \
	  -total=10000 \
	  -fill-desc=false

# 从 OL Search API 按主题批量拉取 ~100000 本书写入 Mongo（compose 网络内运行，约 30-60 分钟）
seed-books-100k: build-linux
	podman-compose -f deploy/docker-compose.yml build mongo-seed-books
	podman run --rm --network $(COMPOSE_NETWORK) \
	  localhost/deploy_mongo-seed-books:latest \
	  /app/openlibrary-seed \
	  -mode=search \
	  "-mongo-uri=mongodb://bookhive:bookhive123@bookhive-mongo:27017/?authSource=admin&authMechanism=SCRAM-SHA-256" \
	  -database=bookhive \
	  -per-subject=2000 \
	  -total=100000 \
	  -delay=500ms \
	  -fill-desc=false

# 再执行 compose 中的灌库任务（镜像内 -if-empty，不覆盖已有 books；需已 make build-linux）
mongo-seed-books:
	podman-compose -f deploy/docker-compose.yml run --rm mongo-seed-books

seed-big:
	go run ./cmd/seed-big -truncate -mongo-wipe \
		-users=1000000 -orders=800000 -inventory-rows=1200000 -payments=400000 -books=50000 -stores=400

# Tidy dependencies
tidy:
	go mod tidy
