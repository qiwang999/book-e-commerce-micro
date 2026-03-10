# Cart 服务（购物车服务）

> 服务名：`bookhive.cart` | 端口：9005 | 存储：Redis

## 一、服务概述

购物车服务基于 Redis 实现轻量级购物车管理。核心设计约束为**单门店购物车**——同一用户同一时间只能持有一家门店的购物车，切换门店时自动清空。加购时实时调用 Book 和 Inventory 服务获取最新商品信息与库存价格。

## 二、目录结构

```
service/cart/
├── main.go                 # 服务入口：Redis 连接、gRPC 客户端初始化
├── model/
│   └── cart.go             # 数据模型：CartData、CartItem
├── repository/
│   └── cart_repo.go        # 数据访问层（Redis JSON 存储）
└── handler/
    └── cart_handler.go     # gRPC 接口实现（5 个 RPC）
```

## 三、数据模型

### 3.1 CartData（Redis JSON）

| 字段 | 类型 | 说明 |
|------|------|------|
| user_id | uint64 | 用户 ID |
| store_id | uint64 | 门店 ID |
| items | []CartItem | 商品列表 |

### 3.2 CartItem

| 字段 | 类型 | 说明 |
|------|------|------|
| item_id | string | 唯一标识（UUID） |
| book_id | string | 图书 ID |
| book_title | string | 书名（冗余） |
| book_author | string | 作者（冗余） |
| book_cover | string | 封面 URL（冗余） |
| price | float64 | 价格（优先取门店价） |
| quantity | int32 | 数量 |
| store_id | uint64 | 门店 ID |

### 3.3 Redis 存储

- **Key**：`cart:{userID}`
- **Value**：JSON 序列化的 CartData
- **TTL**：7 天

## 四、RPC 接口

| 方法 | 功能 | 关键逻辑 |
|------|------|----------|
| `AddToCart` | 添加商品 | 门店不同则清空 → 已存在则累加数量 → 新品则调用 Book/Inventory |
| `RemoveFromCart` | 删除商品 | 按 item_id 从列表移除 |
| `UpdateCartItem` | 修改数量 | 数量为 0 则自动删除，否则更新 quantity |
| `GetCart` | 获取购物车 | 返回完整购物车数据 |
| `ClearCart` | 清空购物车 | 删除 Redis Key |

## 五、技术选型

| 技术 | 用途 |
|------|------|
| go-micro v4 | 微服务框架，gRPC |
| Redis 7 | 购物车存储（JSON 序列化） |
| Consul | 服务注册与发现 |

## 六、核心业务流程

### 加购流程

```
客户端 → AddToCart(user_id, store_id, book_id, quantity)
  │
  ├─ 读取现有购物车（Redis）
  │
  ├─ 门店检查：
  │   └─ 购物车已有商品且 store_id 不同？→ 清空购物车
  │
  ├─ 书籍检查：
  │   ├─ 已在购物车？→ quantity += new_quantity → 保存
  │   └─ 不在购物车？
  │       ├─ gRPC → Book.GetBookDetail(book_id) → 获取书名、作者、封面、定价
  │       ├─ gRPC → Inventory.CheckStock(store_id, book_id) → 获取库存和门店价
  │       ├─ 价格策略：门店价 > 0 ? 门店价 : 图书定价
  │       └─ 创建 CartItem → 写入 Redis
  │
  └─ 返回更新后的购物车
```

### 单门店约束

```
用户 A 的购物车：[门店 1 的书 X, 门店 1 的书 Y]
  │
  ├─ AddToCart(store_id=1, book_Z) → 正常加入
  └─ AddToCart(store_id=2, book_W) → 清空购物车 → 仅保留门店 2 的书 W
```

## 七、依赖关系

### 调用的服务

- **Book 服务**：`GetBookDetail` — 获取书籍详情（书名、封面、定价等）
- **Inventory 服务**：`CheckStock` — 获取门店库存和售价

### 被依赖

- **Order 服务**：订单创建后通常由前端调用 `ClearCart` 清空购物车
- **API Gateway**：转发购物车操作请求
