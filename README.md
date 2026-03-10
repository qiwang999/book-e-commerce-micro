# BookHive - 微服务架构的连锁书店在线购书系统

> 基于 go-micro v4 的毕业设计项目，参考日本 K-books 模式的连锁书店在线购书平台

## 项目简介

BookHive 是一个采用微服务架构的在线书店系统，旨在为连锁书店提供统一的线上购书、库存管理、订单处理及 AI 智能推荐能力。

- **框架**：基于 go-micro v4 微服务框架
- **核心服务**：8 个微服务（User、Store、Book、Inventory、Cart、Order、Payment、AI）
- **存储**：MySQL + MongoDB 混合存储，Redis 缓存，RabbitMQ 异步消息
- **AI 能力**：智能推荐、AI 图书管理员、智能搜索、图书摘要、阅读偏好分析

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

    Order -.-> RabbitMQ
    Payment -.-> RabbitMQ
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
| AI 模型 | OpenAI GPT-4o |
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
│   └── ai/                   # AI 服务
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
│   └── util/                 # 工具函数
├── deploy/                   # 部署配置
│   ├── docker-compose.yml    # 基础设施
│   ├── mysql/init.sql
│   └── mongo/init.js
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
| **bookhive.ai** | 9008 | AI 推荐、对话、摘要、智能搜索、阅读偏好分析 |
| **API Gateway** | 8080 | 统一 HTTP 入口，路由转发，JWT 鉴权 |

---

## AI 功能亮点

1. **智能推荐 (Recommend)**：基于用户上下文，使用 GPT-4o 生成个性化图书推荐，含推荐理由与匹配度评分。

2. **AI 图书管理员 (Chat)**：多轮对话式 AI 助手，支持 Function Calling 调用搜索、查书、查库存等工具，提供书籍建议与操作引导。

3. **智能搜索 (Smart Search)**：自然语言查询解析，提取结构化过滤条件（分类、作者、价格等），提升搜索体验。

4. **图书摘要 (Summary)**：为指定图书生成结构化摘要，包含核心主题、目标读者、阅读难度、预估阅读时长，支持 MongoDB 缓存。

5. **阅读偏好分析 (Taste)**：分析用户阅读行为，输出偏好分类、作者、人格标签、阅读画像及跨类型发现推荐。

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

将启动 Consul、MySQL、MongoDB、Redis、RabbitMQ、Elasticsearch。

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

### 认证 (Auth)

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/auth/register` | 用户注册 |
| POST | `/api/v1/auth/login` | 用户登录 |

### 用户 (User) - 需 JWT

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/user/profile` | 获取个人资料 |
| PUT | `/api/v1/user/profile` | 更新个人资料 |
| GET | `/api/v1/user/preferences` | 获取阅读偏好 |
| PUT | `/api/v1/user/preferences` | 更新阅读偏好 |

### 图书 (Books)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/books/search` | 搜索图书 |
| GET | `/api/v1/books/:id` | 获取图书详情 |
| GET | `/api/v1/books/categories` | 获取分类列表 |
| POST | `/api/v1/books` | 创建图书（管理员） |

### 门店 (Stores)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/stores` | 门店列表 |
| GET | `/api/v1/stores/nearest` | 最近门店 |
| GET | `/api/v1/stores/radius` | 半径内门店 |
| GET | `/api/v1/stores/:id` | 门店详情 |

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
| GET | `/api/v1/orders/:id` | 订单详情 |

### 支付 (Payments) - 需 JWT

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/payments` | 创建支付 |
| POST | `/api/v1/payments/:payment_no/process` | 处理支付 |
| GET | `/api/v1/payments/:payment_no` | 支付状态 |

### AI

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| POST | `/api/v1/ai/recommend` | 智能推荐 | JWT |
| POST | `/api/v1/ai/chat` | AI 图书管理员对话 | JWT |
| GET | `/api/v1/ai/summary/:book_id` | 图书摘要 | 公开 |
| POST | `/api/v1/ai/search` | 智能搜索 | 公开 |
| GET | `/api/v1/ai/taste` | 阅读偏好分析 | JWT |

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

# OpenAI（AI 功能必需）
openai:
  api_key: <your-openai-api-key>
  model: gpt-4o
  embedding_model: text-embedding-3-small
  base_url: https://api.openai.com/v1
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

> 注意：当前实现使用 Viper 的 `AutomaticEnv()`，具体环境变量名需与 config 结构体字段对应。生产环境建议将 `config.yaml` 中的敏感字段留空，通过环境变量注入。

---

## License

Apache License 2.0
