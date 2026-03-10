# Order 服务（订单服务）

> 服务名：`bookhive.order` | 端口：9006 | 数据库：MySQL | 消息队列：RabbitMQ

## 一、服务概述

订单服务是系统的核心交易模块，负责订单创建、查询、取消和状态流转。通过**库存锁机制**保证下单不超卖，通过 **RabbitMQ** 消费支付成功事件实现订单与支付的异步解耦。订单号使用时间戳+随机数生成，确保唯一性。

## 二、目录结构

```
service/order/
├── main.go                     # 服务入口：MySQL + RabbitMQ + gRPC 客户端
├── model/
│   └── order.go                # 数据模型：Order、OrderItem
├── repository/
│   └── order_repo.go           # 数据访问层（GORM 事务）
├── handler/
│   └── order_handler.go        # gRPC 接口实现（5 个 RPC）
└── consumer/
    └── payment_consumer.go     # RabbitMQ 消费者：监听支付成功事件
```

## 三、数据模型

### 3.1 orders 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT PK | 主键自增 |
| order_no | VARCHAR(64) UNIQUE | 订单号（时间戳+随机数） |
| user_id | BIGINT INDEX | 用户 ID |
| store_id | BIGINT | 门店 ID |
| total_amount | DECIMAL(10,2) | 订单总金额 |
| status | VARCHAR(32) INDEX | 订单状态 |
| pickup_method | VARCHAR(32) | 取货方式 |
| address_id | BIGINT | 收货地址 ID |
| remark | TEXT | 备注 |
| paid_at | TIMESTAMP | 支付时间 |
| completed_at | TIMESTAMP | 完成时间 |
| cancelled_at | TIMESTAMP | 取消时间 |

### 3.2 order_items 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT PK | 主键自增 |
| order_id | BIGINT INDEX | 关联订单 |
| book_id | VARCHAR(64) | 图书 ID |
| book_title | string | 书名（冗余快照） |
| book_author | string | 作者（冗余快照） |
| book_cover | string | 封面（冗余快照） |
| price | DECIMAL(10,2) | 下单时价格（快照） |
| quantity | INT | 数量 |
| lock_id | VARCHAR(64) | 库存锁 ID |

### 3.3 订单状态流转

```
pending_payment → paid → completed
       │                    
       └──→ cancelled       
```

## 四、RPC 接口

| 方法 | 功能 | 关键逻辑 |
|------|------|----------|
| `CreateOrder` | 创建订单 | 逐项锁库存 → 计算总价 → 创建订单+订单项 → 发布 order.created |
| `GetOrder` | 查询订单 | 支持按 order_id 或 order_no 查询，含订单项 |
| `ListOrders` | 订单列表 | 按 user_id 分页，支持 status 过滤 |
| `CancelOrder` | 取消订单 | 仅 pending_payment 可取消，逐项释放库存锁 |
| `UpdateOrderStatus` | 更新状态 | 通用状态更新接口 |

## 五、技术选型

| 技术 | 用途 |
|------|------|
| go-micro v4 | 微服务框架，gRPC |
| MySQL | 订单持久化，事务保证一致性 |
| GORM | ORM + 事务 |
| RabbitMQ | 消费 payment.success 事件 |
| Consul | 服务注册与发现 |

## 六、核心业务流程

### 创建订单流程

```
客户端 → CreateOrder(user_id, store_id, items[], pickup_method, address_id, remark)
  │
  ├─ 遍历每个商品：
  │   └─ gRPC → Inventory.LockStock(store_id, book_id, quantity)
  │       ├─ 成功 → 记录 lock_id
  │       └─ 失败 → 回滚：释放已成功锁定的所有库存 → 返回错误
  │
  ├─ 计算 total_amount = Σ(price × quantity)
  ├─ 生成 order_no = 时间戳 + 6 位随机数
  │
  ├─ 事务写入：
  │   ├─ INSERT orders (status = "pending_payment")
  │   └─ INSERT order_items[] (含 lock_id)
  │
  ├─ 发布 RabbitMQ 事件：order.created
  └─ 返回 Order 响应
```

### 支付成功消费流程

```
RabbitMQ ← payment.success {order_id, payment_no, amount, paid_at}
  │
  PaymentConsumer:
  ├─ 查询订单（含 order_items）
  ├─ 更新订单状态 → "paid"，写入 paid_at
  └─ 遍历 order_items：
      └─ gRPC → Inventory.DeductStock(store_id, book_id, lock_id, quantity)
          → 真正扣减库存
```

### 取消订单流程

```
客户端 → CancelOrder(order_id)
  ├─ 查询订单，状态必须为 "pending_payment"
  ├─ 遍历 order_items：
  │   └─ gRPC → Inventory.ReleaseStock(store_id, book_id, lock_id)
  ├─ 更新订单状态 → "cancelled"，写入 cancelled_at
  └─ 返回成功
```

### RabbitMQ 配置

| 配置项 | 值 |
|--------|-----|
| Exchange | `order.events`（fanout） |
| Queue | `order.payment.success` |
| Routing Key | `payment.success` |
| 消费模式 | 手动 ACK |

## 七、依赖关系

### 调用的服务

- **Inventory 服务**：`LockStock`（下单锁库存）、`ReleaseStock`（取消释放）、`DeductStock`（支付后扣减）

### 消息队列交互

- **消费**：`payment.success`（来自 Payment 服务）
- **发布**：`order.created`（通知下游，如前端轮询）

### 被依赖

- **API Gateway**：转发订单操作请求
- **Payment 服务**：通过 order_id 关联支付单
