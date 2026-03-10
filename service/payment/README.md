# Payment 服务（支付服务）

> 服务名：`bookhive.payment` | 端口：9007 | 数据库：MySQL | 消息队列：RabbitMQ

## 一、服务概述

支付服务处理订单的支付与退款流程。当前采用**模拟支付**模式（100~500ms 随机延迟），支付成功后通过 RabbitMQ 发布 `payment.success` 事件，通知 Order 服务完成后续库存扣减和状态更新，实现订单与支付的完全异步解耦。

## 二、目录结构

```
service/payment/
├── main.go                     # 服务入口：MySQL + RabbitMQ 连接
├── model/
│   └── payment.go              # 数据模型：Payment
├── repository/
│   └── payment_repo.go         # 数据访问层（GORM）
└── handler/
    └── payment_handler.go      # gRPC 接口实现（5 个 RPC）
```

## 三、数据模型

### 3.1 payments 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT PK | 主键自增 |
| payment_no | VARCHAR(64) UNIQUE | 支付单号（时间戳+随机数） |
| order_id | BIGINT INDEX | 关联订单 ID |
| user_id | BIGINT INDEX | 用户 ID |
| amount | DECIMAL(10,2) | 支付金额 |
| method | VARCHAR(32) | 支付方式（wechat/alipay/card） |
| status | VARCHAR(32) | 状态：pending → success → refunded |
| paid_at | TIMESTAMP | 支付成功时间 |
| refunded_at | TIMESTAMP | 退款时间 |
| created_at / updated_at | TIMESTAMP | 时间戳 |

### 3.2 支付状态流转

```
pending → success → refunded
```

## 四、RPC 接口

| 方法 | 功能 | 关键逻辑 |
|------|------|----------|
| `CreatePayment` | 创建支付单 | 生成 payment_no，状态 pending |
| `ProcessPayment` | 处理支付 | 模拟延迟 → 更新为 success → 发布 RabbitMQ 事件 |
| `GetPaymentStatus` | 查询支付状态 | 按 payment_no 查询 |
| `RefundPayment` | 退款 | 仅 success 可退 → 更新为 refunded → 发布退款事件 |
| `GetPaymentByOrderId` | 按订单查询 | 按 order_id 查询支付记录 |

## 五、技术选型

| 技术 | 用途 |
|------|------|
| go-micro v4 | 微服务框架，gRPC |
| MySQL | 支付记录持久化 |
| GORM | ORM |
| RabbitMQ | 发布支付事件 |
| Consul | 服务注册与发现 |

## 六、核心业务流程

### 支付处理流程

```
客户端 → CreatePayment(order_id, user_id, amount, method)
  └─ 生成 payment_no → INSERT payments (status="pending") → 返回

客户端 → ProcessPayment(payment_no)
  ├─ 查询支付单（必须为 pending）
  ├─ 模拟支付延迟（100~500ms 随机 sleep）
  ├─ 更新 status → "success"，写入 paid_at
  ├─ 发布 RabbitMQ 消息：
  │   Exchange: "payment.events"
  │   Routing Key: "payment.success"
  │   Body: {order_id, payment_no, amount, paid_at}
  └─ 返回支付成功响应
```

### 退款流程

```
客户端 → RefundPayment(payment_no)
  ├─ 查询支付单（必须为 success）
  ├─ 更新 status → "refunded"，写入 refunded_at
  ├─ 发布 RabbitMQ 消息：
  │   Routing Key: "payment.refunded"
  │   Body: {order_id, payment_no, amount, refunded_at}
  └─ 返回退款成功响应
```

### RabbitMQ 事件

| 事件 | Exchange | Routing Key | 消费者 | 消息体 |
|------|----------|-------------|--------|--------|
| 支付成功 | payment.events | payment.success | Order 服务 | {order_id, payment_no, amount, paid_at} |
| 退款完成 | payment.events | payment.refunded | （预留） | {order_id, payment_no, amount, refunded_at} |

## 七、依赖关系

### 上游

- **API Gateway**：转发支付请求

### 下游（通过 MQ 解耦）

- **Order 服务**：消费 `payment.success` 事件，更新订单为 paid 并扣减库存

### 设计说明

Payment 服务不直接调用任何其他微服务的 gRPC 接口，完全通过消息队列与下游解耦。这是典型的**事件驱动架构**，保证了支付模块的独立性和可替换性（未来对接真实支付渠道时只需修改 `ProcessPayment` 内部逻辑）。
