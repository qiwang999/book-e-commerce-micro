# BookHive - 微服务架构的连锁书店在线购书系统

> 基于 go-micro v4 的毕业设计项目，参考日本 K-books 模式的连锁书店在线购书平台

## 项目简介

BookHive 是一个采用微服务架构的在线书店系统，旨在为连锁书店提供统一的线上购书、库存管理、订单处理及 AI 智能推荐能力。

- **框架**：基于 go-micro v4 微服务框架
- **核心服务**：8 个微服务（User、Store、Book、Inventory、Cart、Order、Payment、AI）
- **存储**：MySQL + MongoDB 混合存储，Redis 缓存，RabbitMQ 异步消息，Milvus 向量数据库
- **AI 能力**：基于 Eino 框架的多 Agent 体系 + RAG 检索增强生成 + 流式对话

---

## 系统架构图

```mermaid
flowchart TB
    subgraph Client["客户端"]
        Web[Web App]
        Mobile[Mobile App]
    end

    subgraph Gateway["API Gateway :8080"]
        Gin[Gin HTTP]
    end

    subgraph Services["微服务层"]
        User[bookhive.user :9001]
        Store[bookhive.store :9002]
        Book[bookhive.book :9003]
        Inventory[bookhive.inventory :9004]
        Cart[bookhive.cart :9005]
        Order[bookhive.order :9006]
        Payment[bookhive.payment :9007]
        AI[bookhive.ai :9008]
    end

    subgraph Discovery["服务发现"]
        Consul[Consul :8500]
    end

    subgraph Data["数据层"]
        MySQL[(MySQL 8.0)]
        Mongo[(MongoDB 7.0)]
        Redis[(Redis 7)]
        ES[(Elasticsearch 8)]
        Milvus[(Milvus 向量数据库)]
    end

    subgraph MQ["消息队列"]
        RabbitMQ[RabbitMQ]
    end

    Web --> Gin
    Mobile --> Gin
    Gin --> User
    Gin --> Store
    Gin --> Book
    Gin --> Inventory
    Gin --> Cart
    Gin --> Order
    Gin --> Payment
    Gin --> AI

    User --> Consul
    Store --> Consul
    Book --> Consul
    Inventory --> Consul
    Cart --> Consul
    Order --> Consul
    Payment --> Consul
    AI --> Consul

    User --> MySQL
    Store --> MySQL
    Store --> Redis
    Book --> MySQL
    Book --> ES
    Inventory --> MySQL
    Cart --> Redis
    Order --> MySQL
    Payment --> MySQL
    AI --> Mongo
    AI --> Redis
    AI --> Milvus

    Book -.-> RabbitMQ
    Order -.-> RabbitMQ
    Payment -.-> RabbitMQ
    RabbitMQ -.-> AI
```

---

## 技术栈

| 组件 | 技术选型 |
|------|----------|
| 微服务框架 | go-micro v4 |
| 服务通信 | gRPC + Protobuf |
| 服务发现 | Consul |
| API 网关 | Gin |
| 数据库 | MySQL 8.0 + MongoDB 7.0 |
| 缓存 | Redis 7 |
| 消息队列 | RabbitMQ |
| 搜索引擎 | Elasticsearch 8 |
| 对象存储 | MinIO（图片/文件统一存储） |
| 向量数据库 | Milvus 2.5（AI 语义检索，Compose 使用多架构镜像） |
| AI 框架 | CloudWeGo Eino（Agent + Tool + RAG） |
| AI 模型 | OpenAI 兼容 API（如 GPT-4o、DeepSeek 等，由 `openai.model` / `base_url` 配置） |
| 文本向量化 | OpenAI text-embedding-3-small（1536 维） |
| 容器化 | Docker + Docker Compose |
| 认证 | JWT |

---

## 项目结构

```
book-e-commerce-micro/
├── api-gateway/              # API 网关 (Gin)
│   ├── handler/              # HTTP 请求处理器
│   ├── middleware/           # 认证、鉴权中间件
│   ├── router/               # 路由配置
│   └── main.go
├── service/                  # 微服务
│   ├── user/                 # 用户服务
│   ├── store/                # 门店服务
│   ├── book/                 # 图书服务
│   ├── inventory/            # 库存服务
│   ├── cart/                 # 购物车服务
│   ├── order/                # 订单服务
│   ├── payment/              # 支付服务
│   └── ai/                   # AI 服务（Eino 框架）
│       ├── agent/            # Agent 定义 + 动态加载
│       │   ├── registry.go   # ToolRegistry 分组注册（Tool + Prompt 动态管理）
│       │   ├── intent.go     # IntentRouter 意图路由（关键词 + LLM fallback）
│       │   ├── librarian.go  # LibrarianAgent（动态创建，按需注入 Tool）
│       │   └── ...           # Recommender/Summarizer/SmartSearch/TasteAnalyzer
│       ├── tools/            # Eino Tool（search_books/check_stock/add_to_cart/...）
│       ├── rag/              # RAG 检索器（向量检索 + 库存过滤）
│       ├── vectorstore/      # Milvus 向量数据库封装
│       └── embedding/        # Embedding 管理（向量生成与存储）
├── proto/                    # Protobuf 定义
│   ├── user/
│   ├── store/
│   ├── book/
│   ├── inventory/
│   ├── cart/
│   ├── order/
│   ├── payment/
│   └── ai/
├── common/                   # 公共模块
│   ├── auth/                 # JWT 认证
│   ├── config/               # 配置加载
│   ├── email/                # SMTP 发信（验证码等）
│   ├── storage/              # MinIO 客户端封装
│   └── util/                 # 工具函数
├── deploy/                   # 部署配置
│   ├── docker-compose.yml    # 基础设施 + 应用服务
│   ├── config.docker.yaml    # 写入 Consul 的示例配置（部署前请填写密钥）
│   ├── mysql/init.sql
│   └── mongo/init.js
├── Dockerfile                # 多阶段构建（各服务共用镜像入口）
├── .dockerignore
├── docs/                     # 文档
│   ├── ai-eino-design.md    # AI 服务 Eino 框架设计文档
│   └── api.md               # HTTP API 参考（路由、鉴权、限流、请求与响应约定）
├── config.yaml               # 应用配置
├── Makefile
├── go.mod
└── go.sum
```

---

## 微服务说明

| 服务名 | 端口 | 说明 |
|--------|------|------|
| **bookhive.user** | 9001 | 用户注册、登录、个人资料、阅读偏好管理 |
| **bookhive.store** | 9002 | 门店信息、最近门店、半径内门店查询 |
| **bookhive.book** | 9003 | 图书检索、分类、详情，支持 Elasticsearch 全文搜索 |
| **bookhive.inventory** | 9004 | 库存查询、扣减、补货 |
| **bookhive.cart** | 9005 | 购物车增删改查，基于 Redis |
| **bookhive.order** | 9006 | 订单创建、查询、状态流转 |
| **bookhive.payment** | 9007 | 支付创建、处理、状态查询 |
| **bookhive.ai** | 9008 | AI 推荐、对话（含流式）、摘要、智能搜索、阅读偏好分析、语义相似图书 |
| **API Gateway** | 8080 | 统一 HTTP 入口，路由转发，JWT 鉴权 |

---

## AI 功能亮点

基于 **CloudWeGo Eino** 框架构建多 Agent 体系，集成 **RAG（检索增强生成）** 技术和 **Milvus** 向量数据库。

1. **智能推荐 (Recommend)**：RAG 检索相关图书 + 用户购买历史 → LLM 生成个性化推荐，含推荐理由与匹配度评分。仅推荐书库中实际存在且有库存的书籍。

2. **AI 图书馆员 (Chat / StreamChat)**：多轮对话式 AI 助手，一个对话入口覆盖**搜书 → 推荐 → 加购 → 下单 → 支付**全流程。采用**动态 Tool 加载**（ToolRegistry 四组 + IntentRouter 关键词 + LLM fallback），支持 **SSE 流式**。RAG 注入确保优先围绕书库与库存回答。**多 Agent（MVP）**：在偏「纯荐书」且未命中下单/购物意图时，先跑推荐子 Agent，将结果作为内部上下文再交给馆员。**Human-in-the-Loop**：下单、支付、取消订单在 Redis 可用时经 API 字段 `hitl_confirm_*` 二次确认，服务端冻结工具参数防改参（详见 `docs/api.md`）。

3. **智能搜索 (Smart Search)**：自然语言查询解析，Agent 自动提取结构化过滤条件（分类、作者、价格等），调用 search_books Tool 返回精准结果。

4. **图书摘要 (Summary)**：为指定图书生成结构化摘要，包含核心主题、目标读者、阅读难度、预估阅读时长，支持 Redis + MongoDB 两级缓存。

5. **阅读偏好分析 (Taste)**：基于用户购买历史，分析阅读偏好，输出偏好分类、作者、人格标签、阅读画像及跨类型发现推荐。

6. **语义相似图书 (Similar Books)**：基于 Milvus 向量数据库的 ANN 搜索，通过 OpenAI Embedding 计算图书间语义相似度，返回 Top-N 最相似图书。

7. **向量增量同步**：图书创建/更新后，Book 服务经 RabbitMQ 交换机 `book.changed` 发事件，AI 服务消费队列 `ai.book.embedding` 并对单本书执行 `EmbedBook`，与启动时全量补向量任务互补（需配置 `rabbitmq.url`）。

---

## 快速开始

### 1. 环境要求

- **Go** 1.22+
- **Docker** 与 **Docker Compose**
- **protoc**（Protocol Buffers 编译器）及 Go 插件

安装 protoc 插件：

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install go-micro.dev/v4/cmd/protoc-gen-micro@latest
```

### 2. 克隆与依赖

```bash
git clone <repository-url>
cd book-e-commerce-micro
go mod tidy
```

### 3. 启动基础设施

```bash
make docker-up
```

将启动 Consul、MySQL、MongoDB、Redis、RabbitMQ、Elasticsearch、Milvus、MinIO 及网关与各微服务（详见 `deploy/docker-compose.yml`）。**首次部署前**请编辑 `deploy/config.docker.yaml`（填写 `openai.api_key`、邮件 SMTP 等），勿将含真实密钥的文件推送到公开仓库。

更多排障说明见 `deploy/README.md`。

### 4. 生成 Proto 代码

```bash
make proto
```

### 5. 运行服务

每个服务在独立终端中启动（建议按顺序）：

```bash
# 终端 1 - API 网关
make run-gateway

# 终端 2 - 用户服务
make run-user

# 终端 3 - 门店服务
make run-store

# 终端 4 - 图书服务
make run-book

# 终端 5 - 库存服务
make run-inventory

# 终端 6 - 购物车服务
make run-cart

# 终端 7 - 订单服务
make run-order

# 终端 8 - 支付服务
make run-payment

# 终端 9 - AI 服务
make run-ai
```

### 6. 访问 API

- API 地址：`http://localhost:8080`
- Consul UI：`http://localhost:8500`
- RabbitMQ 管理界面：`http://localhost:15672`

---

## API 文档

完整约定（统一响应 `code: 0`、分页结构、限流档位、multipart/SSE 等）见 **[docs/api.md](docs/api.md)**。下表为速览。

### 认证 (Auth)

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/auth/send-code` | 发送邮箱验证码 |
| POST | `/api/v1/auth/register` | 用户注册（需验证码） |
| POST | `/api/v1/auth/login` | 用户登录 |

### 用户 (User) - 需 JWT

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/user/profile` | 获取个人资料 |
| PUT | `/api/v1/user/profile` | 更新个人资料 |
| GET | `/api/v1/user/preferences` | 获取阅读偏好 |
| PUT | `/api/v1/user/preferences` | 更新阅读偏好 |
| GET | `/api/v1/user/addresses` | 收货地址列表 |
| POST | `/api/v1/user/addresses` | 创建收货地址 |

### 图书 (Books)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/books/search` | 搜索图书 |
| GET | `/api/v1/books/:id` | 获取图书详情 |
| GET | `/api/v1/books/categories` | 获取分类列表 |
| POST | `/api/v1/books` | 创建图书（管理员） |
| POST | `/api/v1/books/upload-cover` | 上传图书封面（管理员，multipart） |

### 上传 - 需 JWT

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/upload` | 通用图片上传（MinIO，`category` 查询参数可选） |

### 门店 (Stores)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/stores` | 门店列表 |
| GET | `/api/v1/stores/nearest` | 最近门店 |
| GET | `/api/v1/stores/radius` | 半径内门店 |
| GET | `/api/v1/stores/:id` | 门店详情 |

### 库存 (Inventory)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/inventory/stock` | 查询库存（store_id + book_id） |
| GET | `/api/v1/inventory/store/:store_id/books` | 门店图书列表 |

### 购物车 (Cart) - 需 JWT

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/cart` | 获取购物车 |
| POST | `/api/v1/cart/items` | 添加商品 |
| PUT | `/api/v1/cart/items/:item_id` | 更新数量 |
| DELETE | `/api/v1/cart/items/:item_id` | 移除商品 |
| DELETE | `/api/v1/cart` | 清空购物车 |

### 订单 (Orders) - 需 JWT

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/orders` | 创建订单 |
| GET | `/api/v1/orders` | 订单列表 |
| GET | `/api/v1/orders/:id` | 订单详情（支持数字 ID 或订单号） |
| POST | `/api/v1/orders/:id/cancel` | 取消订单 |

### 支付 (Payments) - 需 JWT

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/payments` | 创建支付 |
| POST | `/api/v1/payments/:payment_no/process` | 处理支付 |
| GET | `/api/v1/payments/:payment_no` | 支付状态 |
| POST | `/api/v1/payments/:payment_no/refund` | 退款 |
| GET | `/api/v1/payments/order/:order_id` | 按订单查支付 |

### AI

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| POST | `/api/v1/ai/recommend` | 智能推荐 | JWT |
| POST | `/api/v1/ai/chat` | AI 图书馆员对话（同步，支持 `hitl_confirm_*` 敏感操作确认） | JWT |
| POST | `/api/v1/ai/chat/stream` | AI 图书馆员对话（SSE 流式，HITL 经 metadata 下发） | JWT |
| GET | `/api/v1/ai/taste` | 阅读偏好分析 | JWT |
| GET | `/api/v1/ai/summary/:book_id` | 图书摘要 | 公开 |
| POST | `/api/v1/ai/search` | 智能搜索 | 公开 |
| GET | `/api/v1/ai/similar/:book_id` | 语义相似图书 | 公开 |

> **关于流式传输**：目前仅对话接口（Chat）提供 SSE 流式端点，因为对话输出为自由文本，流式可实现打字机效果显著提升体验。其他 AI 接口（推荐、摘要、偏好分析、搜索）返回的是结构化 JSON，需 LLM 完整输出后解析才能提取字段，流式推送 JSON 片段对前端无意义，因此保持同步调用。

---

## 配置说明

### 配置文件位置

主配置文件为项目根目录下的 `config.yaml`，程序会依次从当前目录、`../`、`../../` 查找。

### 主要配置项

```yaml
# 应用
app:
  name: bookhive
  version: 1.0.0

# 服务发现
consul:
  address: 127.0.0.1:8500

# MySQL
mysql:
  host: 127.0.0.1
  port: 3306
  user: root
  password: <password>
  database: bookhive

# MongoDB
mongodb:
  uri: mongodb://user:pass@127.0.0.1:27017
  database: bookhive

# Redis
redis:
  address: 127.0.0.1:6379
  password: <password>

# RabbitMQ
rabbitmq:
  url: amqp://user:pass@127.0.0.1:5672/

# JWT
jwt:
  secret: <your-secret>
  expire_hours: 72

# Milvus 向量数据库（AI 向量检索）
milvus:
  address: 127.0.0.1:19530

# MinIO（网关上传、封面等）
minio:
  endpoint: 127.0.0.1:9000
  access_key: <access-key>
  secret_key: <secret-key>
  bucket: bookhive
  use_ssl: false

# 邮件（注册验证码等，可选）
email:
  host: smtp.example.com
  port: 465
  username: ""
  password: ""
  from: "BookHive <noreply@example.com>"

# OpenAI 兼容 API（AI 功能必需）
openai:
  api_key: <your-api-key>
  model: deepseek-v3.2     # 或 gpt-4o 等
  embedding_model: text-embedding-3-small
  base_url: https://api.openai.com/v1   # 兼容网关需包含 /v1 后缀
```

### 环境变量（敏感信息）

建议通过环境变量覆盖敏感配置，例如：

```bash
# OpenAI API Key（AI 功能必需）
export OPENAI_API_KEY=sk-xxx

# 数据库密码
export MYSQL_PASSWORD=your_password
export MONGODB_URI=mongodb://user:pass@host:27017
export REDIS_PASSWORD=your_redis_password
export RABBITMQ_URL=amqp://user:pass@host:5672/

# JWT 密钥
export JWT_SECRET=your_jwt_secret
```

> 注意：当前实现使用 Viper 的 `AutomaticEnv()`，具体环境变量名需与 config 结构体字段对应。生产环境建议将 `config.yaml` / `deploy/config.docker.yaml` 中的敏感字段留空，通过环境变量或私有配置注入。

---

## 单元测试

网关 HTTP 与 mock gRPC 集成测试（不依赖真实数据库）：

```bash
go test ./api-gateway/handler/... -count=1
```

全模块执行（含无测试的包）：

```bash
go test ./... -count=1
```

---

## License

Apache License 2.0
