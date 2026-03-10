# BookHive API 接口文档

> 基础路径: `/api/v1`  
> 认证方式: JWT Bearer Token（`Authorization: Bearer <token>`）  
> 限流: 全局 100 req/s，认证/AI 接口 20 req/s

---

## 一、通用格式

### 成功响应

```json
{
  "code": 200,
  "message": "success",
  "data": { ... }
}
```

### 分页响应

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "items": [ ... ],
    "total": 100,
    "page": 1,
    "page_size": 20
  }
}
```

### 错误响应

```json
{
  "code": 400,
  "message": "error description",
  "data": null
}
```

| HTTP 状态码 | 含义 |
|------------|------|
| 200 | 成功 |
| 400 | 请求参数错误 |
| 401 | 未认证 / Token 无效 |
| 403 | 无权限（非管理员） |
| 404 | 资源不存在 |
| 429 | 请求频率超限 |
| 500 | 服务器内部错误 |

---

## 二、健康检查

| 方法 | 路径 | 认证 |
|------|------|------|
| GET | `/health` | 否 |

**响应**:
```json
{ "code": 200, "data": { "status": "ok" } }
```

---

## 三、认证 (`/api/v1/auth`)

限流：10 req/s

### 3.1 注册

| 方法 | 路径 | 认证 |
|------|------|------|
| POST | `/api/v1/auth/register` | 否 |

**请求体**:
```json
{
  "username": "zhangsan",
  "email": "zhangsan@example.com",
  "password": "password123"
}
```

**响应**:
```json
{
  "code": 200,
  "data": {
    "user_id": 1,
    "username": "zhangsan",
    "email": "zhangsan@example.com",
    "token": "eyJhbGciOiJI..."
  }
}
```

### 3.2 登录

| 方法 | 路径 | 认证 |
|------|------|------|
| POST | `/api/v1/auth/login` | 否 |

**请求体**:
```json
{
  "email": "zhangsan@example.com",
  "password": "password123"
}
```

**响应**: 同注册响应格式。

---

## 四、图书 (`/api/v1/books`)

### 4.1 搜索图书（公开）

| 方法 | 路径 | 认证 |
|------|------|------|
| GET | `/api/v1/books/search` | 否 |

**Query 参数**:

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| keyword | string | 否 | 搜索关键词（标题/描述） |
| category | string | 否 | 分类过滤 |
| author | string | 否 | 作者过滤 |
| min_price | float | 否 | 最低价格 |
| max_price | float | 否 | 最高价格 |
| page | int | 否 | 页码（默认 1） |
| page_size | int | 否 | 每页数量（默认 20） |

**响应**:
```json
{
  "code": 200,
  "data": {
    "books": [
      {
        "book_id": "book_001",
        "title": "三体",
        "author": "刘慈欣",
        "category": "科幻",
        "price": 59.90,
        "rating": 4.8,
        "cover_url": "https://..."
      }
    ],
    "total": 50
  }
}
```

### 4.2 获取图书详情（公开）

| 方法 | 路径 | 认证 |
|------|------|------|
| GET | `/api/v1/books/:id` | 否 |

**响应**:
```json
{
  "code": 200,
  "data": {
    "book_id": "book_001",
    "title": "三体",
    "author": "刘慈欣",
    "category": "科幻",
    "price": 59.90,
    "rating": 4.8,
    "description": "...",
    "isbn": "978-7-5366-9293-0",
    "publisher": "重庆出版社",
    "tags": ["科幻", "硬科幻", "中国科幻"]
  }
}
```

### 4.3 获取分类列表（公开）

| 方法 | 路径 | 认证 |
|------|------|------|
| GET | `/api/v1/books/categories` | 否 |

**响应**:
```json
{
  "code": 200,
  "data": {
    "categories": ["科幻", "文学", "历史", "计算机", "经济"]
  }
}
```

### 4.4 创建图书（管理员）

| 方法 | 路径 | 认证 |
|------|------|------|
| POST | `/api/v1/books` | 是（管理员） |

**请求体**:
```json
{
  "title": "三体",
  "author": "刘慈欣",
  "category": "科幻",
  "price": 59.90,
  "description": "...",
  "isbn": "978-7-5366-9293-0"
}
```

---

## 五、门店 (`/api/v1/stores`)

### 5.1 门店列表（公开）

| 方法 | 路径 | 认证 |
|------|------|------|
| GET | `/api/v1/stores` | 否 |

### 5.2 最近门店（公开）

| 方法 | 路径 | 认证 |
|------|------|------|
| GET | `/api/v1/stores/nearest` | 否 |

**Query 参数**:

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| latitude | float | 是 | 纬度 |
| longitude | float | 是 | 经度 |

### 5.3 半径内门店（公开）

| 方法 | 路径 | 认证 |
|------|------|------|
| GET | `/api/v1/stores/radius` | 否 |

**Query 参数**:

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| latitude | float | 是 | 纬度 |
| longitude | float | 是 | 经度 |
| radius | float | 否 | 半径（km，默认 5） |

### 5.4 门店详情（公开）

| 方法 | 路径 | 认证 |
|------|------|------|
| GET | `/api/v1/stores/:id` | 否 |

---

## 六、库存 (`/api/v1/inventory`)

### 6.1 查询库存（公开）

| 方法 | 路径 | 认证 |
|------|------|------|
| GET | `/api/v1/inventory/stock` | 否 |

**Query 参数**:

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| store_id | uint64 | 是 | 门店 ID |
| book_id | string | 是 | 图书 ID |

**响应**:
```json
{
  "code": 200,
  "data": {
    "in_stock": true,
    "quantity": 15,
    "price": 59.90
  }
}
```

### 6.2 门店图书列表（公开）

| 方法 | 路径 | 认证 |
|------|------|------|
| GET | `/api/v1/inventory/store/:store_id/books` | 否 |

---

## 七、用户 (`/api/v1/user`)

> 以下接口均需认证

### 7.1 获取个人信息

| 方法 | 路径 | 认证 |
|------|------|------|
| GET | `/api/v1/user/profile` | 是 |

### 7.2 更新个人信息

| 方法 | 路径 | 认证 |
|------|------|------|
| PUT | `/api/v1/user/profile` | 是 |

### 7.3 获取偏好设置

| 方法 | 路径 | 认证 |
|------|------|------|
| GET | `/api/v1/user/preferences` | 是 |

### 7.4 更新偏好设置

| 方法 | 路径 | 认证 |
|------|------|------|
| PUT | `/api/v1/user/preferences` | 是 |

### 7.5 地址列表

| 方法 | 路径 | 认证 |
|------|------|------|
| GET | `/api/v1/user/addresses` | 是 |

### 7.6 新增地址

| 方法 | 路径 | 认证 |
|------|------|------|
| POST | `/api/v1/user/addresses` | 是 |

**请求体**:
```json
{
  "name": "张三",
  "phone": "13800138000",
  "province": "北京市",
  "city": "北京市",
  "district": "海淀区",
  "detail": "中关村大街1号",
  "is_default": true
}
```

---

## 八、购物车 (`/api/v1/cart`)

> 以下接口均需认证

### 8.1 查看购物车

| 方法 | 路径 | 认证 |
|------|------|------|
| GET | `/api/v1/cart` | 是 |

**响应**:
```json
{
  "code": 200,
  "data": {
    "items": [
      {
        "item_id": 1,
        "book_id": "book_001",
        "title": "三体",
        "quantity": 2,
        "price": 59.90
      }
    ],
    "total_count": 2,
    "total_amount": 119.80
  }
}
```

### 8.2 添加商品

| 方法 | 路径 | 认证 |
|------|------|------|
| POST | `/api/v1/cart/items` | 是 |

**请求体**:
```json
{
  "store_id": 1,
  "book_id": "book_001",
  "quantity": 1
}
```

### 8.3 删除商品

| 方法 | 路径 | 认证 |
|------|------|------|
| DELETE | `/api/v1/cart/items/:item_id` | 是 |

### 8.4 更新数量

| 方法 | 路径 | 认证 |
|------|------|------|
| PUT | `/api/v1/cart/items/:item_id` | 是 |

**请求体**:
```json
{ "quantity": 3 }
```

### 8.5 清空购物车

| 方法 | 路径 | 认证 |
|------|------|------|
| DELETE | `/api/v1/cart` | 是 |

---

## 九、订单 (`/api/v1/orders`)

> 以下接口均需认证

### 9.1 创建订单

| 方法 | 路径 | 认证 |
|------|------|------|
| POST | `/api/v1/orders` | 是 |

**请求体**:
```json
{
  "store_id": 1,
  "items": [
    { "book_id": "book_001", "quantity": 2 }
  ],
  "pickup_method": "self_pickup",
  "address_id": 0,
  "remark": "请包装好"
}
```

**响应**:
```json
{
  "code": 200,
  "data": {
    "order_id": 1001,
    "order_no": "ORD20260310001",
    "status": "pending",
    "total_amount": 119.80
  }
}
```

### 9.2 订单列表

| 方法 | 路径 | 认证 |
|------|------|------|
| GET | `/api/v1/orders` | 是 |

**Query 参数**:

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| status | string | 否 | 过滤状态：pending/paid/completed/cancelled |
| page | int | 否 | 页码 |
| page_size | int | 否 | 每页数量 |

### 9.3 订单详情

| 方法 | 路径 | 认证 |
|------|------|------|
| GET | `/api/v1/orders/:id` | 是 |

### 9.4 取消订单

| 方法 | 路径 | 认证 |
|------|------|------|
| POST | `/api/v1/orders/:id/cancel` | 是 |

---

## 十、支付 (`/api/v1/payments`)

> 以下接口均需认证

### 10.1 创建支付

| 方法 | 路径 | 认证 |
|------|------|------|
| POST | `/api/v1/payments` | 是 |

**请求体**:
```json
{
  "order_id": 1001,
  "amount": 119.80,
  "method": "wechat"
}
```

**响应**:
```json
{
  "code": 200,
  "data": {
    "payment_no": "PAY20260310001",
    "status": "pending",
    "amount": 119.80,
    "method": "wechat"
  }
}
```

### 10.2 处理支付（模拟回调）

| 方法 | 路径 | 认证 |
|------|------|------|
| POST | `/api/v1/payments/:payment_no/process` | 是 |

### 10.3 查询支付状态

| 方法 | 路径 | 认证 |
|------|------|------|
| GET | `/api/v1/payments/:payment_no` | 是 |

### 10.4 申请退款

| 方法 | 路径 | 认证 |
|------|------|------|
| POST | `/api/v1/payments/:payment_no/refund` | 是 |

### 10.5 按订单查支付

| 方法 | 路径 | 认证 |
|------|------|------|
| GET | `/api/v1/payments/order/:order_id` | 是 |

---

## 十一、AI 服务 (`/api/v1/ai`)

### 公开接口（限流 20 req/s）

#### 11.1 生成图书摘要

| 方法 | 路径 | 认证 |
|------|------|------|
| GET | `/api/v1/ai/summary/:book_id` | 否 |

**响应**:
```json
{
  "code": 200,
  "data": {
    "book_id": "book_001",
    "title": "三体",
    "summary": "...",
    "key_themes": ["宇宙社会学", "黑暗森林", "技术爆炸"],
    "target_audience": "科幻爱好者、物理学爱好者",
    "reading_difficulty": "中等",
    "estimated_reading_hours": 12.5
  }
}
```

#### 11.2 自然语言搜索

| 方法 | 路径 | 认证 |
|------|------|------|
| POST | `/api/v1/ai/search` | 否 |

**请求体**:
```json
{
  "query": "适合10岁孩子的冒险故事"
}
```

**响应**:
```json
{
  "code": 200,
  "data": {
    "results": [ ... ],
    "interpreted_query": "儿童冒险类图书",
    "extracted_filters": {
      "category": "Children",
      "keyword": "adventure"
    }
  }
}
```

#### 11.3 语义相似图书

| 方法 | 路径 | 认证 |
|------|------|------|
| GET | `/api/v1/ai/similar/:book_id` | 否 |

**Query 参数**:

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| limit | int | 否 | 返回数量（默认 5） |

**响应**:
```json
{
  "code": 200,
  "data": {
    "book_id": "book_001",
    "similar_books": [
      {
        "book_id": "book_042",
        "title": "流浪地球",
        "author": "刘慈欣",
        "category": "科幻",
        "score": 0.92
      }
    ]
  }
}
```

### 认证接口（限流 20 req/s）

#### 11.4 智能推荐

| 方法 | 路径 | 认证 |
|------|------|------|
| POST | `/api/v1/ai/recommend` | 是 |

**请求体**:
```json
{
  "category": "科幻",
  "limit": 5
}
```

**响应**:
```json
{
  "code": 200,
  "data": {
    "recommendations": [
      {
        "book_id": "book_001",
        "title": "三体",
        "author": "刘慈欣",
        "category": "科幻",
        "reason": "基于您对硬科幻的偏好...",
        "match_score": 0.95
      }
    ],
    "ai_summary": "根据您的阅读历史，为您推荐以下科幻作品..."
  }
}
```

#### 11.5 AI 图书馆员对话（同步）

| 方法 | 路径 | 认证 |
|------|------|------|
| POST | `/api/v1/ai/chat` | 是 |

**请求体**:
```json
{
  "message": "有没有三体？帮我查一下门店1的库存",
  "session_id": ""
}
```

> `session_id` 为空则创建新会话，非空则继续已有对话。

**响应**:
```json
{
  "code": 200,
  "data": {
    "reply": "《三体》目前在门店 1 有 5 本库存，售价 59.90 元...",
    "session_id": "sess_abc123",
    "suggested_books": [
      { "title": "三体", "author": "刘慈欣", "category": "科幻", "reason": "您查询的图书" }
    ],
    "actions": [
      { "type": "add_to_cart", "label": "加入购物车", "payload": { "book_id": "book_001", "store_id": 1 } }
    ]
  }
}
```

#### 11.6 AI 图书馆员对话（流式 SSE）

| 方法 | 路径 | 认证 |
|------|------|------|
| POST | `/api/v1/ai/chat/stream` | 是 |

**请求体**: 同 11.5

**响应**: `Content-Type: text/event-stream`

SSE 事件格式：

| 事件类型 | 数据格式 | 说明 |
|---------|---------|------|
| `delta` | `{"delta":"..."}` | 增量文本 token（打字机效果） |
| `metadata` | `{"session_id":"...", "suggested_books":[...], "actions":[...]}` | 最终元数据 |
| `error` | `{"error":"..."}` | 错误信息 |
| `done` | `[DONE]` | 流结束标志 |

**前端调用示例**:
```javascript
const response = await fetch('/api/v1/ai/chat/stream', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': 'Bearer <token>'
  },
  body: JSON.stringify({ message: '推荐一本科幻小说', session_id: '' })
});

const reader = response.body.getReader();
const decoder = new TextDecoder();
let buffer = '';

while (true) {
  const { done, value } = await reader.read();
  if (done) break;
  buffer += decoder.decode(value, { stream: true });
  const lines = buffer.split('\n');
  buffer = lines.pop();
  for (const line of lines) {
    if (line.startsWith('data: ')) {
      const data = line.slice(6);
      if (data === '[DONE]') return;
      const chunk = JSON.parse(data);
      if (chunk.delta) appendToUI(chunk.delta);
    }
  }
}
```

#### 11.7 阅读偏好分析

| 方法 | 路径 | 认证 |
|------|------|------|
| GET | `/api/v1/ai/taste` | 是 |

**响应**:
```json
{
  "code": 200,
  "data": {
    "favorite_categories": ["科幻", "计算机"],
    "favorite_authors": ["刘慈欣", "阿西莫夫"],
    "personality_tags": ["理性主义者", "技术乐观派"],
    "reading_profile": "您是一位偏好硬科幻和技术类书籍的读者...",
    "discovery_recommendations": [
      { "category": "哲学", "reason": "基于您对宇宙终极问题的关注..." }
    ]
  }
}
```

---

## 十二、接口速查表

| 模块 | 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|------|
| 健康 | GET | `/health` | 否 | 健康检查 |
| 认证 | POST | `/api/v1/auth/register` | 否 | 注册 |
| 认证 | POST | `/api/v1/auth/login` | 否 | 登录 |
| 图书 | GET | `/api/v1/books/search` | 否 | 搜索图书 |
| 图书 | GET | `/api/v1/books/categories` | 否 | 分类列表 |
| 图书 | GET | `/api/v1/books/:id` | 否 | 图书详情 |
| 图书 | POST | `/api/v1/books` | 管理员 | 创建图书 |
| 门店 | GET | `/api/v1/stores` | 否 | 门店列表 |
| 门店 | GET | `/api/v1/stores/nearest` | 否 | 最近门店 |
| 门店 | GET | `/api/v1/stores/radius` | 否 | 半径内门店 |
| 门店 | GET | `/api/v1/stores/:id` | 否 | 门店详情 |
| 库存 | GET | `/api/v1/inventory/stock` | 否 | 查询库存 |
| 库存 | GET | `/api/v1/inventory/store/:store_id/books` | 否 | 门店图书 |
| 用户 | GET | `/api/v1/user/profile` | 是 | 获取资料 |
| 用户 | PUT | `/api/v1/user/profile` | 是 | 更新资料 |
| 用户 | GET | `/api/v1/user/preferences` | 是 | 获取偏好 |
| 用户 | PUT | `/api/v1/user/preferences` | 是 | 更新偏好 |
| 用户 | GET | `/api/v1/user/addresses` | 是 | 地址列表 |
| 用户 | POST | `/api/v1/user/addresses` | 是 | 新增地址 |
| 购物车 | GET | `/api/v1/cart` | 是 | 查看购物车 |
| 购物车 | POST | `/api/v1/cart/items` | 是 | 添加商品 |
| 购物车 | DELETE | `/api/v1/cart/items/:item_id` | 是 | 删除商品 |
| 购物车 | PUT | `/api/v1/cart/items/:item_id` | 是 | 更新数量 |
| 购物车 | DELETE | `/api/v1/cart` | 是 | 清空购物车 |
| 订单 | POST | `/api/v1/orders` | 是 | 创建订单 |
| 订单 | GET | `/api/v1/orders` | 是 | 订单列表 |
| 订单 | GET | `/api/v1/orders/:id` | 是 | 订单详情 |
| 订单 | POST | `/api/v1/orders/:id/cancel` | 是 | 取消订单 |
| 支付 | POST | `/api/v1/payments` | 是 | 创建支付 |
| 支付 | POST | `/api/v1/payments/:payment_no/process` | 是 | 处理支付 |
| 支付 | GET | `/api/v1/payments/:payment_no` | 是 | 支付状态 |
| 支付 | POST | `/api/v1/payments/:payment_no/refund` | 是 | 申请退款 |
| 支付 | GET | `/api/v1/payments/order/:order_id` | 是 | 按订单查支付 |
| AI | GET | `/api/v1/ai/summary/:book_id` | 否 | 图书摘要 |
| AI | POST | `/api/v1/ai/search` | 否 | 自然语言搜索 |
| AI | GET | `/api/v1/ai/similar/:book_id` | 否 | 相似图书 |
| AI | POST | `/api/v1/ai/recommend` | 是 | 智能推荐 |
| AI | POST | `/api/v1/ai/chat` | 是 | AI 对话（同步） |
| AI | POST | `/api/v1/ai/chat/stream` | 是 | AI 对话（流式 SSE） |
| AI | GET | `/api/v1/ai/taste` | 是 | 阅读偏好分析 |
