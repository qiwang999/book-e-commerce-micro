# Inventory 服务（库存服务）

> 服务名：`bookhive.inventory` | 端口：9004 | 数据库：MySQL + Redis

## 一、服务概述

库存服务管理各门店的图书库存，提供库存查询、设置、锁定、释放、扣减等完整的库存生命周期管理。通过**库存锁机制**实现下单期间的库存预留，防止超卖。使用 Redis 缓存加速高频库存查询。

## 二、目录结构

```
service/inventory/
├── main.go                     # 服务入口：MySQL + Redis 连接、服务注册
├── model/
│   └── inventory.go            # 数据模型：StoreInventory、InventoryLock
├── repository/
│   └── inventory_repo.go       # 数据访问层（GORM + Redis 缓存）
└── handler/
    └── inventory_handler.go    # gRPC 接口实现（7 个 RPC）
```

## 三、数据模型

### 3.1 store_inventory 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT PK | 主键自增 |
| store_id | BIGINT | 门店 ID |
| book_id | VARCHAR(64) | 图书 ID（MongoDB ObjectID 字符串） |
| quantity | INT | 可用库存数量 |
| locked_quantity | INT | 已锁定数量（默认 0） |
| price | DECIMAL(10,2) | 门店售价 |
| created_at / updated_at | TIMESTAMP | 时间戳 |

> `(store_id, book_id)` 联合唯一索引

### 3.2 inventory_locks 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT PK | 主键自增 |
| lock_id | VARCHAR(64) UNIQUE | 锁唯一标识（UUID） |
| store_id | BIGINT | 门店 ID |
| book_id | VARCHAR(64) | 图书 ID |
| quantity | INT | 锁定数量 |
| status | TINYINT | 状态：1=锁定, 2=已释放, 3=已扣减 |
| expire_at | TIMESTAMP | 过期时间（创建后 15 分钟） |
| created_at | TIMESTAMP | 创建时间 |

## 四、RPC 接口

| 方法 | 功能 | 关键逻辑 |
|------|------|----------|
| `CheckStock` | 查询单条库存 | 优先读 Redis 缓存，miss 回源 MySQL |
| `BatchCheckStock` | 批量查询库存 | 按 store_id + book_ids 批量查询 |
| `SetStock` | 设置库存与价格 | 存在则更新，不存在则创建（Upsert），刷新缓存 |
| `LockStock` | 锁定库存 | 事务：增加 locked_quantity + 创建 InventoryLock |
| `ReleaseStock` | 释放锁定 | 事务：减少 locked_quantity + 更新锁状态为 Released |
| `DeductStock` | 扣减库存 | 事务：减少 quantity 和 locked_quantity + 更新锁状态为 Deducted |
| `GetStoreBooks` | 门店库存列表 | 按 store_id 分页查询，quantity > 0 |

## 五、技术选型

| 技术 | 用途 |
|------|------|
| go-micro v4 | 微服务框架，gRPC |
| MySQL | 库存持久化，事务保证一致性 |
| GORM | ORM + 事务管理 |
| Redis | 库存缓存（Key: `inventory:{storeID}:{bookID}`，TTL: 5 分钟） |
| Consul | 服务注册与发现 |

## 六、核心业务流程

### 库存锁机制（防超卖）

```
                      ┌──────────┐
                      │ 可用库存  │
                      │ qty=10   │
                      └────┬─────┘
                           │
              LockStock(qty=2)
                           │
                      ┌────▼─────┐
                      │ qty=10   │   InventoryLock
                      │ locked=2 │ → {lock_id, qty=2, status=Locked, expire=15min}
                      └────┬─────┘
                           │
           ┌───────────────┼───────────────┐
      支付成功              │              支付超时/取消
      DeductStock       (等待中)          ReleaseStock
           │                                  │
      ┌────▼─────┐                       ┌────▼─────┐
      │ qty=8    │                       │ qty=10   │
      │ locked=0 │                       │ locked=0 │
      │ Deducted │                       │ Released │
      └──────────┘                       └──────────┘
```

### 缓存策略

```
读取：CheckStock
  ├─ Redis GET inventory:{storeID}:{bookID}
  ├─ 命中 → 返回缓存
  └─ 未命中 → MySQL 查询 → 写入 Redis (TTL 5min) → 返回

写入/锁定/扣减：
  └─ 操作完成后 → DEL inventory:{storeID}:{bookID}（失效策略）
```

## 七、被依赖关系

- **Cart 服务**：调用 `CheckStock` 获取库存和门店价格
- **Order 服务**：调用 `LockStock`（下单锁库存）、`ReleaseStock`（取消释放）、`DeductStock`（支付后扣减）
- **AI 服务**：调用 `CheckStock`、`BatchCheckStock` 做推荐时的库存校验
