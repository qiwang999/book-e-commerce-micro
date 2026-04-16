# BookHive HTTP API 参考（含请求 / 响应示例）

面向 **HTTP 客户端对接**：下列 **JSON 字段名**与网关背后 gRPC 的 Protobuf 序列化一致（`proto` 生成的 `json` 标签，**多为 snake_case**，如 `user_id`、`book_id`）。

- **Base URL**：`http://<host>:8080`
- **业务前缀**：`/api/v1`
- **成功时外层**：HTTP `200`，`code: 0`，`message: "success"`（见下文「信封格式」）

> **最后更新**：2026-04-16  
> 主要变更：地址 PUT/DELETE 接口补充、AI 向量搜索切换为本地 `intfloat/multilingual-e5-large`、错误码细化（404/400 语义化）

---

## 目录

1. [信封格式与错误](#1-信封格式与错误)  
2. [认证方式](#2-认证方式)  
3. [限流](#3-限流)  
4. [CORS](#4-cors)  
5. [接口详述（含请求体与返回值）](#5-接口详述含请求体与返回值)  
6. [SSE 流式对话](#6-sse-流式对话)  
7. [接口速查表](#7-接口速查表)

---

## 1. 信封格式与错误

### 1.1 成功（普通 JSON）

```json
{
  "code": 0,
  "message": "success",
  "data": { }
}
```

### 1.2 分页列表（`data` 内部）

图书搜索、门店列表、订单列表等：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "list": [],
    "total": 100,
    "page": 1,
    "page_size": 20
  }
}
```

### 1.3 错误

```json
{
  "code": 400,
  "message": "错误说明"
}
```

| HTTP | code | 常见原因 |
|------|------|----------|
| 400 | 400 | 参数非法、JSON 无法绑定、库存不足、业务规则校验失败 |
| 401 | 401 | 未登录或 Token 无效 |
| 403 | 403 | 非管理员访问管理接口、无权操作他人资源 |
| 404 | 404 | 订单/支付/地址不存在 |
| 413 | 413 | 上传超过 5MB |
| 429 | 429 | 触发限流 |
| 500 | 500 | 下游服务异常；AI 接口不可用时返回降级响应（`code: 0`）而非 500 |

---

## 2. 认证方式

需登录接口在请求头携带：

```http
Authorization: Bearer <登录或注册返回的 token>
```

注册/登录成功后，从 `data.token` 取 Token；`data.user` 中含 `id`、`email`、`name`、`role` 等。

---

## 3. 限流

按客户端 IP 令牌桶，超限返回 HTTP **429**，`code: 429`，`message: rate limit exceeded`。

| 范围 | 约 req/s | 突发 |
|------|----------|------|
| 全局默认 | 100 | 200 |
| `/api/v1/auth/*` | 10 | 20 |
| `/api/v1/ai/*`（含公开与需登录） | 20 | 40 |

---

## 4. CORS

支持携带 Cookie 的跨域配置；**`OPTIONS` 预检**返回 **204**，无 body。

---

## 5. 接口详述（含请求体与返回值）

以下示例中，**省略号或 `...` 表示可有多项或字段可选**；未出现的可选字段服务端可能不返回。

---

### 5.1 `GET /health`

无请求体。

**响应示例：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "ok"
  }
}
```

---

### 5.2 认证 `/api/v1/auth`

#### `POST /api/v1/auth/send-code`

**Content-Type:** `application/json`

**请求体：**

```json
{
  "email": "user@example.com"
}
```

**响应示例（`data` 为用户服务 `CommonResponse`）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "code": 0,
    "message": "verification code sent"
  }
}
```

> `data.code` / `data.message` 为 **业务微服务**返回，与外层网关 `code` 含义不同。

---

#### `POST /api/v1/auth/register`

**请求体：**

```json
{
  "email": "user@example.com",
  "password": "your-password",
  "name": "张三",
  "code": "123456"
}
```

**响应示例（`data` 为 `AuthResponse`）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "code": 0,
    "message": "",
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "user": {
      "id": 1,
      "email": "user@example.com",
      "name": "张三",
      "avatar_url": "",
      "role": "user"
    }
  }
}
```

---

#### `POST /api/v1/auth/login`

**请求体：**

```json
{
  "email": "user@example.com",
  "password": "your-password"
}
```

**响应示例：** 与注册相同结构（`data` 含 `token`、`user`）。

---

### 5.3 图书 `/api/v1/books`（公开）

#### `GET /api/v1/books/search`

**Query（均为可选，除非注明）：**

| 参数 | 说明 |
|------|------|
| keyword, category, author, language, sort_by | 过滤/排序 |
| min_price, max_price | 数字字符串 |
| page | 默认 1 |
| page_size | 默认 20，最大 100 |

**响应示例（分页 + `list` 内为 `Book`）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "list": [
      {
        "id": "book_001",
        "title": "三体",
        "author": "刘慈欣",
        "isbn": "978-7-5366-9293-0",
        "publisher": "重庆出版社",
        "publish_date": "2008-01-01",
        "price": 23.0,
        "category": "科幻",
        "subcategory": "",
        "description": "...",
        "cover_url": "https://...",
        "pages": 302,
        "language": "zh",
        "tags": ["科幻", "雨果奖"],
        "rating": 4.8,
        "rating_count": 1200
      }
    ],
    "total": 50,
    "page": 1,
    "page_size": 20
  }
}
```

---

#### `GET /api/v1/books/categories`

**响应示例（`data` 为字符串数组）：**

```json
{
  "code": 0,
  "message": "success",
  "data": ["科幻", "文学", "计算机"]
}
```

---

#### `GET /api/v1/books/:id`

路径参数 `id` 为图书 ID。

**响应示例（`data` 为单本 `Book`，字段同搜索列表中的单条）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "book_001",
    "title": "三体",
    "author": "刘慈欣",
    "isbn": "978-7-5366-9293-0",
    "publisher": "重庆出版社",
    "publish_date": "2008-01-01",
    "price": 23.0,
    "category": "科幻",
    "subcategory": "",
    "description": "...",
    "cover_url": "https://...",
    "pages": 302,
    "language": "zh",
    "tags": ["科幻"],
    "rating": 4.8,
    "rating_count": 1200
  }
}
```

---

#### `POST /api/v1/books`（需 JWT + **admin**）

**请求体（`CreateBookRequest`，未列字段可选）：**

```json
{
  "title": "新书标题",
  "author": "作者",
  "isbn": "978-0-00-000000-0",
  "publisher": "出版社",
  "publish_date": "2024-01-01",
  "price": 59.9,
  "category": "计算机",
  "subcategory": "Go",
  "description": "简介",
  "cover_url": "https://cdn.example.com/cover.jpg",
  "pages": 400,
  "language": "zh",
  "tags": ["编程", "Go"]
}
```

**响应示例（`data` 为创建后的 `Book`）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "new-book-id",
    "title": "新书标题",
    "author": "作者",
    "isbn": "978-0-00-000000-0",
    "price": 59.9,
    "category": "计算机",
    "cover_url": "https://cdn.example.com/cover.jpg"
  }
}
```

**侧链（向量索引）**：创建或更新图书成功后，若 Book 服务配置了 `rabbitmq.url` 且 RabbitMQ 可用，会向 fanout 交换机 `book.changed` 发布 JSON 事件（`event` 为 `created` / `updated`，`book_id` 为 MongoDB 图书 ID）。AI 服务订阅队列 `ai.book.embedding` 后对该书调用 `EmbedBook`，用于 Milvus 语义检索与 RAG 的增量更新（与启动时全量 `EmbedAllBooks` 互补）。

---

#### `POST /api/v1/books/upload-cover`（需 JWT + **admin**）

**Content-Type:** `multipart/form-data`  
**表单字段：** `file`（图片文件）  
**说明：** 等价于 `category=covers` 的上传逻辑。

**响应示例：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "url": "http://minio-host/bucket/covers/uuid.jpg"
  }
}
```

---

### 5.4 门店 `/api/v1/stores`（公开）

#### `GET /api/v1/stores`

**Query：** `city`（可选），`page`（默认 1），`page_size`（默认 20，最大 100）

**响应示例（分页，`list` 内为 `Store`）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "list": [
      {
        "id": 1,
        "name": "BookHive 科技园店",
        "description": "",
        "address": "科技园路1号",
        "city": "深圳市",
        "district": "南山区",
        "phone": "0755-00000000",
        "latitude": 22.5,
        "longitude": 113.9,
        "business_hours": "10:00-22:00",
        "status": 1,
        "image_url": "",
        "distance_km": 0
      }
    ],
    "total": 10,
    "page": 1,
    "page_size": 20
  }
}
```

---

#### `GET /api/v1/stores/nearest`

**Query（必填）：** `lat`，`lng`

**响应示例（`data` 为单个 `Store`，`distance_km` 可能有值）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "name": "BookHive 科技园店",
    "address": "科技园路1号",
    "city": "深圳市",
    "latitude": 22.5,
    "longitude": 113.9,
    "distance_km": 1.2
  }
}
```

---

#### `GET /api/v1/stores/radius`

**Query（必填）：** `lat`，`lng`，`radius`（公里，0–500）  
**可选：** `limit`（默认 20，最大 100）

**响应示例（`data` 为 `StoreListResponse` 结构：门店列表 + 总数）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "stores": [
      {
        "id": 1,
        "name": "门店A",
        "latitude": 22.5,
        "longitude": 113.9,
        "distance_km": 2.5
      }
    ],
    "total": 3
  }
}
```

---

#### `GET /api/v1/stores/:id`

路径 `id` 须为 **数字**。

**响应示例（`data` 为 `Store`）：** 同单店字段，与「列表中单条」一致。

---

### 5.5 库存 `/api/v1/inventory`（公开）

#### `GET /api/v1/inventory/stock`

**Query（必填）：** `store_id`，`book_id`

**响应示例（`data` 为 `StockInfo`）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "store_id": 1,
    "book_id": "book_001",
    "quantity": 100,
    "locked_quantity": 5,
    "available": 95,
    "price": 23.0
  }
}
```

---

#### `GET /api/v1/inventory/store/:store_id/books`

**Query：** `page`，`page_size`

**响应示例（`data` 为 `StoreBookListResponse`）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [
      {
        "book_id": "book_001",
        "quantity": 100,
        "available": 95,
        "price": 23.0
      }
    ],
    "total": 200
  }
}
```

---

### 5.6 用户 `/api/v1/user`（需 JWT）

#### `GET /api/v1/user/profile`

**响应示例（`data` 为 `UserProfile`）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "user_id": 1,
    "email": "user@example.com",
    "name": "张三",
    "avatar_url": "",
    "phone": "13800000000",
    "gender": 0,
    "birthday": "1990-01-01",
    "favorite_categories": ["科幻"],
    "favorite_authors": ["刘慈欣"],
    "reading_preferences": []
  }
}
```

---

#### `PUT /api/v1/user/profile`

**请求体（字段均可选；勿传 `user_id`，由网关注入）：**

```json
{
  "name": "新昵称",
  "avatar_url": "https://cdn.example.com/a.jpg",
  "phone": "13800000000",
  "gender": 1,
  "birthday": "1990-01-01"
}
```

**响应示例（`data` 为 `CommonResponse`）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "code": 0,
    "message": "success"
  }
}
```

---

#### `GET /api/v1/user/preferences`

**响应示例（`data` 为 `UserPreferences`）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "user_id": 1,
    "favorite_categories": ["科幻", "计算机"],
    "favorite_authors": ["刘慈欣"],
    "reading_preferences": []
  }
}
```

---

#### `PUT /api/v1/user/preferences`

**请求体（字段可选）：**

```json
{
  "favorite_categories": ["科幻"],
  "favorite_authors": ["刘慈欣"],
  "reading_preferences": ["硬科幻"]
}
```

**响应示例：** 同 `PUT /profile`（`data` 为 `CommonResponse`）。

---

#### `GET /api/v1/user/addresses`

**响应示例（`data` 为地址数组，非分页对象）：**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 1,
      "user_id": 1,
      "name": "张三",
      "phone": "13800138000",
      "province": "广东省",
      "city": "深圳市",
      "district": "南山区",
      "detail": "科技园路1号",
      "is_default": true
    }
  ]
}
```

---

#### `POST /api/v1/user/addresses`

**请求体：**

```json
{
  "name": "张三",
  "phone": "13800138000",
  "province": "广东省",
  "city": "深圳市",
  "district": "南山区",
  "detail": "科技园路1号",
  "is_default": true
}
```

**响应示例（`data` 为新建的 `Address`）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 2,
    "user_id": 1,
    "name": "张三",
    "phone": "13800138000",
    "province": "广东省",
    "city": "深圳市",
    "district": "南山区",
    "detail": "科技园路1号",
    "is_default": true
  }
}
```

---

#### `PUT /api/v1/user/addresses/:id`

路径 `id` 为地址数字 ID。

**请求体（字段均可选，只传需要修改的部分）：**

```json
{
  "name": "李四",
  "phone": "13900000000",
  "province": "广东省",
  "city": "广州市",
  "district": "天河区",
  "detail": "天河路1号",
  "is_default": false
}
```

**响应示例（`data` 为更新后的 `Address`）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 2,
    "user_id": 1,
    "name": "李四",
    "phone": "13900000000",
    "province": "广东省",
    "city": "广州市",
    "district": "天河区",
    "detail": "天河路1号",
    "is_default": false
  }
}
```

**地址不存在时返回 HTTP 404：**

```json
{
  "code": 404,
  "message": "address not found"
}
```

---

#### `DELETE /api/v1/user/addresses/:id`

路径 `id` 为地址数字 ID，无请求体。

**响应示例：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "code": 0,
    "message": "success"
  }
}
```

**地址不存在时返回 HTTP 404。**

---

### 5.7 上传 `POST /api/v1/upload`（需 JWT）

**Content-Type:** `multipart/form-data`  
**表单字段：** `file`  
**Query（可选）：** `category`，默认 `general`（如 `covers`、`avatars`）

**响应示例：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "url": "http://minio-host/bucket/general/uuid.png"
  }
}
```

---

### 5.8 购物车 `/api/v1/cart`（需 JWT）

#### `GET /api/v1/cart`

**响应示例（`data` 为 `Cart`）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "user_id": 1,
    "store_id": 1,
    "items": [
      {
        "item_id": "cart-item-uuid",
        "book_id": "book_001",
        "book_title": "三体",
        "book_author": "刘慈欣",
        "book_cover": "https://...",
        "price": 23.0,
        "quantity": 2,
        "store_id": 1
      }
    ],
    "total_amount": 46.0,
    "total_count": 2
  }
}
```

---

#### `POST /api/v1/cart/items`

**请求体（不要传 `user_id`）：**

```json
{
  "store_id": 1,
  "book_id": "book_001",
  "quantity": 1
}
```

**响应示例：** 同 `GET /cart`（返回最新整辆购物车 `Cart`）。

---

#### `DELETE /api/v1/cart/items/:item_id`

无请求体。

**响应示例：** 同 `GET /cart`。

---

#### `PUT /api/v1/cart/items/:item_id`

**请求体：**

```json
{
  "quantity": 5
}
```

**响应示例：** 同 `GET /cart`。

---

#### `DELETE /api/v1/cart`

无请求体。

**响应示例（`data` 为购物车服务 `CommonResponse`）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "code": 0,
    "message": "success"
  }
}
```

---

### 5.9 订单 `/api/v1/orders`（需 JWT）

#### `POST /api/v1/orders`

**请求体（不要传 `user_id`）：**

```json
{
  "store_id": 1,
  "items": [
    {
      "book_id": "book_001",
      "book_title": "三体",
      "book_author": "刘慈欣",
      "book_cover": "https://...",
      "price": 23.0,
      "quantity": 1
    }
  ],
  "pickup_method": "self_pickup",
  "address_id": 0,
  "remark": ""
}
```

**响应示例（`data` 为 `Order`）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1001,
    "order_no": "ORD-20240401-001",
    "user_id": 1,
    "store_id": 1,
    "total_amount": 23.0,
    "status": "pending_payment",
    "pickup_method": "self_pickup",
    "address_id": 0,
    "remark": "",
    "items": [
      {
        "id": 1,
        "book_id": "book_001",
        "book_title": "三体",
        "book_author": "刘慈欣",
        "book_cover": "https://...",
        "price": 23.0,
        "quantity": 1,
        "lock_id": "lock-xxx"
      }
    ],
    "paid_at": "",
    "completed_at": "",
    "cancelled_at": "",
    "created_at": "2024-04-01T12:00:00Z"
  }
}
```

> `status`、时间字段以实际服务为准。

---

#### `GET /api/v1/orders`

**Query：** `status`（可选），`page`，`page_size`

**响应示例（分页，`list` 内为 `Order` 摘要或完整对象，取决于下游）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "list": [
      {
        "id": 1001,
        "order_no": "ORD-20240401-001",
        "total_amount": 23.0,
        "status": "paid",
        "created_at": "2024-04-01T12:00:00Z"
      }
    ],
    "total": 1,
    "page": 1,
    "page_size": 20
  }
}
```

---

#### `GET /api/v1/orders/:id`

路径 `id` 可为 **数字主键** 或 **订单号字符串**。

**响应示例：** 同创建订单返回的完整 `Order` 结构。

---

#### `POST /api/v1/orders/:id/cancel`

路径 `id` **仅支持数字**订单 ID。

**成功响应（`data` 常省略）：**

```json
{
  "code": 0,
  "message": "success"
}
```

**常见错误：**

| HTTP | 原因 |
|------|------|
| 400 | 参数非法（`id` 非数字） |
| 403 | 无权取消他人订单 |
| 404 | 订单不存在 |
| 400 | 订单状态不允许取消（已支付、已完成等） |

---

### 5.10 支付 `/api/v1/payments`（需 JWT）

> **参数校验**：`order_id` 必须 > 0，`amount` 必须 > 0，`method` 为 `wechat` / `alipay` / `cash` 之一，否则返回 HTTP 400。

#### `POST /api/v1/payments`

**请求体（不要传 `user_id`）：**

```json
{
  "order_id": 1001,
  "amount": 23.0,
  "method": "wechat"
}
```

**响应示例（`data` 为 `Payment`）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "payment_no": "PAY-20240401-0001",
    "order_id": 1001,
    "user_id": 1,
    "amount": 23.0,
    "method": "wechat",
    "status": "pending",
    "paid_at": "",
    "refunded_at": "",
    "created_at": "2024-04-01T12:05:00Z"
  }
}
```

---

#### `POST /api/v1/payments/:payment_no/process`

无请求体。

**响应示例（`data` 为 `PaymentResult`）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "code": 0,
    "message": "success",
    "payment": {
      "payment_no": "PAY-20240401-0001",
      "order_id": 1001,
      "status": "paid",
      "paid_at": "2024-04-01T12:06:00Z"
    }
  }
}
```

---

#### `GET /api/v1/payments/:payment_no`

**响应示例（`data` 为 `Payment`）：** 同创建支付返回结构。

---

#### `POST /api/v1/payments/:payment_no/refund`

**请求体（可选）：**

```json
{
  "reason": "用户申请退款"
}
```

**响应示例（`data` 为 `PaymentResult`）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "code": 0,
    "message": "refunded",
    "payment": {
      "payment_no": "PAY-20240401-0001",
      "status": "refunded",
      "refunded_at": "2024-04-01T13:00:00Z"
    }
  }
}
```

**常见错误：**

| HTTP | 原因 |
|------|------|
| 404 | 支付单不存在 |
| 400 | 支付单状态不允许退款（未支付、已退款等） |
| 403 | 无权操作他人支付单 |

---

#### `GET /api/v1/payments/order/:order_id`

路径 `order_id` 为 **数字**。

**响应示例（`data` 为 `Payment`）：** 同上。  
**支付单不存在时返回 HTTP 404。**

---

### 5.11 AI `/api/v1/ai`

> **向量搜索说明**：`/ai/similar` 与 `/ai/search` 底层使用本地部署的 `intfloat/multilingual-e5-large`（1024 维，ONNX Runtime），支持中英文多语言语义检索，**无需 OpenAI API Key**。  
> LLM 生成类接口（`/ai/summary`、`/ai/recommend`、`/ai/chat`、`/ai/taste`）依赖 OpenAI 兼容网关；若 LLM 不可用，接口返回 HTTP 200 + 降级占位内容，而非 500 错误。

#### `GET /api/v1/ai/summary/:book_id`（公开）

无请求体。

**响应示例（`data` 为 `SummaryResponse`）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "book_id": "book_001",
    "title": "三体",
    "summary": "本书讲述了……",
    "key_themes": ["宇宙社会学", "黑暗森林"],
    "target_audience": "科幻爱好者",
    "reading_difficulty": "中等",
    "estimated_reading_hours": 8
  }
}
```

---

#### `POST /api/v1/ai/search`（公开）

**请求体：**

```json
{
  "query": "想找一本讲 Go 语言实战的书，价格别太贵",
  "user_id": 0,
  "limit": 10
}
```

> 公开调用时 `user_id` 可省略或传 `0`。

**响应示例（`data` 为 `SmartSearchResponse`）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "results": [
      {
        "book_id": "book_go_001",
        "title": "Go 程序设计语言",
        "author": "Alan Donovan",
        "cover_url": "https://...",
        "price": 89.0,
        "category": "计算机",
        "score": 0.95,
        "reason": "与查询高度相关"
      }
    ],
    "interpreted_query": "分类:计算机, 关键词:Go",
    "extracted_filters": {
      "category": "计算机"
    }
  }
}
```

---

#### `GET /api/v1/ai/similar/:book_id`（公开）

**Query：** `limit`（默认 5，最大 50）

**响应示例（`data` 为 `SimilarBooksResponse`）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "book_id": "book_001",
    "similar_books": [
      {
        "book_id": "book_002",
        "title": "球状闪电",
        "author": "刘慈欣",
        "cover_url": "https://...",
        "price": 28.0,
        "category": "科幻",
        "score": 0.88,
        "reason": "同一作者且题材相近"
      }
    ]
  }
}
```

---

#### `POST /api/v1/ai/recommend`（需 JWT）

**请求体：**

```json
{
  "context": "我喜欢硬科幻和宇宙题材",
  "limit": 5
}
```

> `user_id` 由网关注入，无需传递。

**响应示例（`data` 为 `RecommendResponse`）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "recommendations": [
      {
        "book_id": "book_001",
        "title": "三体",
        "author": "刘慈欣",
        "cover_url": "https://...",
        "price": 23.0,
        "category": "科幻",
        "score": 0.92,
        "reason": "符合你对宇宙题材的偏好"
      }
    ]
  }
}
```

---

#### `POST /api/v1/ai/chat`（需 JWT）

**请求体：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `message` | string | 用户消息（必填） |
| `session_id` | string | 会话 ID；省略则由服务端生成并在响应中返回 |
| `hitl_confirm_action_id` | string | Human-in-the-Loop：用户在前一轮响应中确认敏感操作后，将服务端返回的 `hitl_action_id` 原样带回 |
| `hitl_confirm_secret` | string | 与 `hitl_action_id` 配对的密钥，来自上一轮 `hitl_secret` |

`user_id` 由网关注入，客户端无需传递。

```json
{
  "message": "帮我推荐两本科幻小说",
  "session_id": "sess-uuid-可选",
  "hitl_confirm_action_id": "",
  "hitl_confirm_secret": ""
}
```

**Human-in-the-Loop（敏感工具）**

对 **创建订单**、**创建支付**、**取消订单** 三类工具，服务端在 Redis 可用时会强制执行二次确认：

1. 首次触发工具：不调用下游订单/支付 gRPC，响应里 `hitl_pending=true`，并返回 `hitl_action_id`、`hitl_secret`、`hitl_summary`（摘要供 UI 展示）。
2. 用户在客户端确认后，**保持同一 `session_id`**，再次调用本接口：可先只发确认字段（`message` 可与上一轮相同或简要说明「已确认」），请求体携带 `hitl_confirm_action_id` 与 `hitl_confirm_secret`；服务端将已冻结的工具参数写入「待执行」状态。
3. 下一轮对话中模型再次调用同一工具时，使用**已批准参数**执行真实 RPC，避免模型改参。

若 Redis 未配置或不可用，上述工具退化为仅依赖模型提示词约束（与旧行为一致）。

**响应示例（`data` 为 `ChatResponse`）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "reply": "根据您的偏好，推荐以下图书……",
    "session_id": "sess-uuid",
    "suggested_books": [
      {
        "book_id": "book_001",
        "title": "三体",
        "author": "刘慈欣",
        "cover_url": "https://...",
        "price": 23.0,
        "category": "科幻",
        "score": 0.9,
        "reason": "经典硬科幻"
      }
    ],
    "actions": [
      {
        "type": "add_to_cart",
        "label": "加入购物车",
        "payload": "{\"book_id\":\"book_001\"}"
      }
    ],
    "hitl_pending": false,
    "hitl_action_id": "",
    "hitl_secret": "",
    "hitl_summary": ""
  }
}
```

当需要用户确认敏感操作时，`hitl_pending` 为 `true`，且 `hitl_action_id`、`hitl_secret`、`hitl_summary` 有值（`hitl_secret` 仅用于回传确认，勿记录日志或泄露给第三方）。

---

#### `GET /api/v1/ai/taste`（需 JWT）

无请求体。

**响应示例（`data` 为 `TasteResponse`）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "user_id": 1,
    "top_categories": ["科幻", "计算机"],
    "top_authors": ["刘慈欣"],
    "personality_tags": ["理性", "好奇"],
    "taste_summary": "您偏好科幻与科技类读物……",
    "discovery_suggestions": [
      {
        "book_id": "book_010",
        "title": "时间简史",
        "author": "霍金",
        "cover_url": "",
        "price": 45.0,
        "category": "科普",
        "score": 0.75,
        "reason": "扩展阅读边界"
      }
    ]
  }
}
```

---

## 6. SSE 流式对话

**路径：** `POST /api/v1/ai/chat/stream`  
**认证：** 需 JWT  
**请求体：** 与 `POST /api/v1/ai/chat` 相同（JSON），含 `hitl_confirm_*` 时行为与同步接口一致。

**响应：** `Content-Type: text/event-stream`，非 JSON 信封。

首条事件一般为仅含 `session_id` 的 `metadata`。流式结束前可能额外推送 `metadata`：`suggested_books` / `actions`；若本轮触发 HITL 拦截，还会推送一条带 `hitl_pending`、`hitl_action_id`、`hitl_secret`、`hitl_summary` 的 `metadata`（字段名与 `ChatStreamChunk` / JSON 序列化一致，如 `hitl_pending`）。

示例片段：

```text
event: delta
data: {"delta":"你"}

event: delta
data: {"delta":"好"}

event: metadata
data: {"session_id":"sess-1","hitl_pending":true,"hitl_action_id":"abc...","hitl_secret":"...","hitl_summary":"create_order user=1 store=2 ..."}

event: metadata
data: {"session_id":"sess-1","suggested_books":[],"actions":[]}

event: done
data: [DONE]
```

错误事件示例：

```text
event: error
data: {"error":"stream interrupted"}
```

---

## 7. 接口速查表

| 模块 | 方法 | 路径 | 认证 |
|------|------|------|------|
| 健康 | GET | `/health` | 否 |
| 认证 | POST | `/api/v1/auth/send-code` | 否 |
| 认证 | POST | `/api/v1/auth/register` | 否 |
| 认证 | POST | `/api/v1/auth/login` | 否 |
| 图书 | GET | `/api/v1/books/search` | 否 |
| 图书 | GET | `/api/v1/books/categories` | 否 |
| 图书 | GET | `/api/v1/books/:id` | 否 |
| 图书 | POST | `/api/v1/books` | 管理员 |
| 图书 | POST | `/api/v1/books/upload-cover` | 管理员 |
| 上传 | POST | `/api/v1/upload` | 是 |
| 门店 | GET | `/api/v1/stores` | 否 |
| 门店 | GET | `/api/v1/stores/nearest` | 否 |
| 门店 | GET | `/api/v1/stores/radius` | 否 |
| 门店 | GET | `/api/v1/stores/:id` | 否 |
| 库存 | GET | `/api/v1/inventory/stock` | 否 |
| 库存 | GET | `/api/v1/inventory/store/:store_id/books` | 否 |
| 用户 | GET/PUT | `/api/v1/user/profile` | 是 |
| 用户 | GET/PUT | `/api/v1/user/preferences` | 是 |
| 用户 | GET/POST | `/api/v1/user/addresses` | 是 |
| 用户 | PUT | `/api/v1/user/addresses/:id` | 是 |
| 用户 | DELETE | `/api/v1/user/addresses/:id` | 是 |
| 购物车 | GET | `/api/v1/cart` | 是 |
| 购物车 | POST | `/api/v1/cart/items` | 是 |
| 购物车 | PUT | `/api/v1/cart/items/:item_id` | 是 |
| 购物车 | DELETE | `/api/v1/cart/items/:item_id` | 是 |
| 购物车 | DELETE | `/api/v1/cart` | 是 |
| 订单 | POST | `/api/v1/orders` | 是 |
| 订单 | GET | `/api/v1/orders` | 是 |
| 订单 | GET | `/api/v1/orders/:id` | 是 |
| 订单 | POST | `/api/v1/orders/:id/cancel` | 是 |
| 支付 | POST | `/api/v1/payments` | 是 |
| 支付 | POST | `/api/v1/payments/:payment_no/process` | 是 |
| 支付 | GET | `/api/v1/payments/:payment_no` | 是 |
| 支付 | POST | `/api/v1/payments/:payment_no/refund` | 是 |
| 支付 | GET | `/api/v1/payments/order/:order_id` | 是 |
| AI | GET | `/api/v1/ai/summary/:book_id` | 否 |
| AI | POST | `/api/v1/ai/search` | 否 |
| AI | GET | `/api/v1/ai/similar/:book_id` | 否 |
| AI | POST | `/api/v1/ai/recommend` | 是 |
| AI | POST | `/api/v1/ai/chat` | 是 |
| AI | POST | `/api/v1/ai/chat/stream` | 是 |
| AI | GET | `/api/v1/ai/taste` | 是 |

---

**说明：**
- 若某字段在实际环境中为空，JSON 中可能省略该字段（`omitempty`）。更严格的 schema 请以仓库内 `proto/**/*.proto` 为准。
- AI 向量检索（`/ai/similar`、`/ai/search` 的向量部分）使用本地 `intfloat/multilingual-e5-large` 模型，对应服务 `embed-server`（端口 8001）；LLM 文本生成部分走 OpenAI 兼容网关。
- 订单、支付、地址接口已细化 HTTP 状态码：资源不存在返回 **404**，参数/业务错误返回 **400**，权限不足返回 **403**。
