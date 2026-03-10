# AI 服务（智能推荐与对话服务）

> 服务名：`bookhive.ai` | 端口：9008 | 数据库：MongoDB + Milvus | AI 框架：CloudWeGo Eino

## 一、服务概述

AI 服务是系统的智能化核心，基于 **CloudWeGo Eino** 框架构建多 Agent 体系，集成 OpenAI GPT-4o 大模型和 Milvus 向量数据库，实现智能图书推荐、AI 对话、图书摘要生成、自然语言搜索、阅读偏好分析和语义相似图书查找等功能。通过 **RAG（检索增强生成）** 技术，将向量语义检索与实时库存信息注入 LLM 上下文，生成既相关又可购买的推荐结果。

## 二、目录结构

```
service/ai/
├── main.go                     # 服务入口：MongoDB + Redis + Milvus + Eino 初始化
├── model/
│   └── ai_model.go             # 数据模型：ChatSession、BookSummaryCache 等
├── repository/
│   └── ai_repo.go              # 数据访问层（MongoDB）
├── handler/
│   └── ai_handler.go           # gRPC 接口实现（7 个 RPC，含 StreamChat 流式，11 个 Tool）
├── agent/                      # AI Agent 定义 + 动态加载
│   ├── chatmodel.go            # ChatModel 工厂（创建 OpenAI ChatModel）
│   ├── registry.go             # ToolRegistry — Tool 分组注册、Prompt 动态管理
│   ├── intent.go               # IntentRouter — 两阶段意图路由（关键词 + LLM fallback）
│   ├── librarian.go            # LibrarianAgent（动态创建，按需注入 Tool + Prompt）
│   ├── recommender.go          # RecommenderAgent：智能推荐
│   ├── summarizer.go           # SummarizerAgent：图书摘要
│   ├── smart_search.go         # SmartSearchAgent：自然语言搜索
│   └── taste_analyzer.go       # TasteAnalyzerAgent：阅读偏好分析
├── embedding/
│   └── embedder.go             # Embedding 管理（OpenAI text-embedding-3-small）
├── vectorstore/
│   └── milvus.go               # Milvus 向量数据库操作
├── rag/
│   └── retriever.go            # RAG 检索器（向量检索 + 库存过滤）
└── tools/                      # Agent 工具（Tool）
    ├── search_books.go         # SearchBooksTool：调用 Book 服务搜索
    ├── get_book_detail.go      # GetBookDetailTool：获取图书详情
    ├── check_stock.go          # CheckStockTool：查询库存
    ├── find_similar_books.go   # FindSimilarBooksTool：语义相似搜索
    ├── get_user_orders.go      # GetUserOrdersTool：查询用户购买历史
    ├── add_to_cart.go          # AddToCartTool：加入购物车
    ├── get_cart.go             # GetCartTool：查看购物车
    ├── create_order.go         # CreateOrderTool：创建订单
    ├── get_order_detail.go     # GetOrderDetailTool：查询订单详情
    ├── cancel_order.go         # CancelOrderTool：取消订单
    └── create_payment.go       # CreatePaymentTool：创建支付
```

## 三、数据模型

### 3.1 chat_histories 集合（MongoDB）

| 字段 | 类型 | 说明 |
|------|------|------|
| session_id | string | 会话 ID |
| user_id | uint64 | 用户 ID |
| messages | []ChatMessage | 消息列表 |
| created_at / updated_at | time.Time | 时间戳 |

**ChatMessage**：`{role, content, created_at}`

### 3.2 book_summaries 集合（MongoDB）

| 字段 | 类型 | 说明 |
|------|------|------|
| book_id | string | 图书 ID |
| title | string | 书名 |
| summary | string | AI 生成摘要 |
| key_themes | []string | 核心主题 |
| target_audience | string | 目标读者 |
| reading_difficulty | string | 阅读难度 |
| est_reading_hours | float64 | 预估阅读时长 |
| created_at | time.Time | 生成时间 |

### 3.3 book_embeddings（Milvus Collection）

| 字段 | 类型 | 说明 |
|------|------|------|
| book_id | VARCHAR(64) PK | 图书 ID |
| title | VARCHAR(500) | 书名 |
| author | VARCHAR(200) | 作者 |
| category | VARCHAR(100) | 分类 |
| embedding | FloatVector(1536) | OpenAI 向量 |

- **索引**：HNSW（M=16, efConstruction=256）
- **度量**：Cosine Similarity
- **搜索参数**：ef=64

## 四、RPC 接口

| 方法 | 功能 | 使用的 Agent/组件 |
|------|------|-------------------|
| `GetRecommendations` | 智能推荐 | RAG Retriever + RecommenderAgent + 购买历史 |
| `ChatWithLibrarian` | AI 图书馆员对话（同步） | LibrarianAgent（含 11 个 Tool）+ RAG |
| `StreamChat` | AI 图书馆员对话（流式） | LibrarianAgent + Eino Streaming + SSE |
| `GenerateBookSummary` | 生成图书摘要 | SummarizerAgent + Redis/MongoDB 两级缓存 |
| `SmartSearch` | 自然语言搜索 | SmartSearchAgent → Book.SearchBooks |
| `AnalyzeReadingTaste` | 阅读偏好分析 | TasteAnalyzerAgent + 购买历史 |
| `GetSimilarBooks` | 语义相似图书 | Milvus ANN 向量搜索 |

## 五、Agent 架构

### 5.1 Agent 体系

所有 Agent 基于 Eino 的 `ChatModelAgent` 构建，使用 `NewChatModel()` 创建 OpenAI ChatModel 实例（模型：gpt-4o）。

| Agent | System Prompt 核心 | 工具 | Temperature | ChatModel |
|-------|-------------------|------|-------------|-----------|
| **LibrarianAgent** | 专业图书馆员，Tool + Prompt 动态注入 | **动态加载**（按意图注入 3-5 个 Tool） | **1.3** | chatModelConversation |
| **RecommenderAgent** | 基于上下文 + 购买历史生成推荐理由 | 无（纯 LLM 推理） | **1.0** | chatModelAnalysis |
| **SummarizerAgent** | 生成结构化书评（JSON 输出） | get_book_detail | **1.0** | chatModelAnalysis |
| **SmartSearchAgent** | 解析自然语言为搜索参数（JSON 输出） | search_books | **1.0** | chatModelAnalysis |
| **TasteAnalyzerAgent** | 基于购买历史分析阅读偏好生成用户画像 | 无（纯 LLM 推理） | **1.0** | chatModelAnalysis |
| **IntentRouter** | 意图分类（LLM fallback） | 无 | **1.0** | chatModelAnalysis |

> Temperature 设置遵循 **DeepSeek 官方推荐**：通用对话 1.3、数据抽取/分析 1.0

### 5.2 Tool 定义

使用 Eino 的 `utils.InferTool` + jsonschema tag 自动生成 Schema：

| Tool | 输入 | 功能 | 调用的服务 |
|------|------|------|-----------|
| `search_books` | keyword, category, author | 搜索图书 | Book.SearchBooks |
| `get_book_detail` | book_id | 获取详情 | Book.GetBookDetail |
| `check_stock` | store_id, book_id | 查库存 | Inventory.CheckStock |
| `find_similar_books` | book_id, limit | 语义相似 | Milvus ANN 向量搜索 |
| `get_user_orders` | user_id, status, limit | 查询购买历史 | Order.ListOrders |
| `add_to_cart` | user_id, store_id, book_id, quantity | 加入购物车 | Cart.AddToCart |
| `get_cart` | user_id | 查看购物车 | Cart.GetCart |
| `create_order` | user_id, store_id, items, pickup_method | 创建订单（需确认） | Order.CreateOrder |
| `get_order_detail` | order_id/order_no, user_id | 查询订单详情 | Order.GetOrder |
| `cancel_order` | order_id, user_id | 取消订单（需确认） | Order.CancelOrder |
| `create_payment` | order_id, user_id, amount, method | 创建支付（需确认） | Payment.CreatePayment |

## 五-B、动态 Tool 加载

### 架构概览

Librarian Agent 不再一次性加载全部 11 个 Tool，而是通过 **两阶段意图路由** 按需注入：

```
用户消息 → IntentRouter.Classify()
           ├─ Phase 1: 关键词匹配（遍历 ToolRegistry.GroupMeta.Keywords）
           └─ Phase 2: LLM Fallback（仅关键词漏检时触发，chatModelAnalysis）
                ↓
          []ToolGroup  (如 [discovery, shopping])
                ↓
          NewLibrarianAgent(ctx, cm, registry, groups)
           ├─ registry.GetByGroups() → 3-5 个 Tool
           └─ buildInstruction() → 动态 Prompt
```

### ToolRegistry 分组

| ToolGroup | 包含的 Tool | 触发关键词示例 |
|-----------|------------|-------------|
| `discovery` | search_books, get_book_detail, check_stock, find_similar, get_user_orders | 找书、推荐、搜索、库存 |
| `shopping` | add_to_cart, get_cart | 购物车、加购、买 |
| `checkout` | create_order, create_payment | 下单、结账、支付 |
| `order` | get_order_detail, cancel_order | 订单、取消、退 |

### LLM Fallback 兜底

当关键词匹配仅命中 `discovery` 时（即用户使用了隐式表述），IntentRouter 会调用 LLM 进行精确分类。分类 Prompt 和类别描述全部从 `ToolRegistry.GroupMeta.IntentHint` 动态读取，零硬编码。

---

## 六、核心技术方案

### 6.1 RAG 检索增强生成

```
用户输入 "推荐几本科幻小说"
  │
  ├─ 1. Embedding：text-embedding-3-small 生成 1536 维查询向量
  │
  ├─ 2. Milvus ANN 搜索：HNSW + Cosine，返回 Top-K 相似图书
  │
  ├─ 3. 库存过滤：BatchCheckStock 查询各书在指定门店的库存
  │     └─ 仅保留有库存的图书
  │
  ├─ 4. 上下文构建：将图书信息（标题、作者、分类、库存）格式化为文本
  │
  ├─ 5. RecommenderAgent：注入上下文 + 用户偏好 → LLM 生成推荐理由
  │
  └─ 返回：推荐图书列表 + AI 生成的推荐理由
```

### 6.2 Embedding 管理

- **模型**：OpenAI text-embedding-3-small（1536 维）
- **初始化**：服务启动时后台 goroutine 为所有图书批量生成 embedding 并写入 Milvus
- **增量**：`find_similar_books` 时若图书无 embedding 则自动生成
- **文本模板**：`"{title} by {author}. Category: {category}. {description}. Tags: {tags}"`

### 6.3 Milvus 向量存储

- **集合**：`book_embeddings`
- **创建逻辑**：服务启动时自动创建集合 + HNSW 索引 + 加载到内存
- **Upsert**：先检查是否存在（按 book_id），存在则删除后重新插入
- **搜索**：`Search(embedding, topK)` → 返回 `[]SimilarBook{BookID, Title, Author, Category, Score}`

### 6.4 Librarian 对话（ReAct 循环，含流式输出）

```
用户: "有没有《三体》，门店 1 有货吗？帮我加到购物车"
  │
  LibrarianAgent (ReAct, 支持 StreamChat 流式输出):
  ├─ Thought: 需要搜索三体这本书
  ├─ Action: search_books(keyword="三体") → 返回书籍列表
  ├─ Thought: 找到了，需要检查门店 1 的库存
  ├─ Action: check_stock(store_id=1, book_id="xxx") → 有库存，qty=5
  ├─ Thought: 用户要求加购，先确认
  ├─ Action: add_to_cart(user_id=1, store_id=1, book_id="xxx") → 成功
  └─ Answer: "《三体》目前在门店 1 有 5 本库存，已帮您加入购物车..."
                ↑ 流式时逐 token 推送到客户端 (SSE delta 事件)
```

## 七、技术选型

| 技术 | 用途 |
|------|------|
| CloudWeGo Eino | AI Agent 框架（ChatModelAgent + Tool 抽象 + ReAct） |
| eino-ext/openai | OpenAI ChatModel 适配器 |
| OpenAI GPT-4o | 大语言模型 |
| OpenAI text-embedding-3-small | 文本向量化（1536 维） |
| Milvus v2.4.4 | 向量数据库（HNSW + Cosine） |
| MongoDB | 对话历史、摘要缓存 |
| Redis | 预留缓存 |
| go-micro v4 | 微服务框架 |
| Consul | 服务注册与发现 |

## 八、依赖关系

### 调用的服务

- **Book 服务**：`SearchBooks`、`GetBookDetail`、`GetBooksByIds`（Tool 调用 + RAG 上下文）
- **Inventory 服务**：`CheckStock`、`BatchCheckStock`（库存校验 + RAG 过滤）
- **Order 服务**：`ListOrders`、`CreateOrder`、`GetOrder`、`CancelOrder`（用户购买历史 → 推荐 + 偏好分析 + Librarian Tool）
- **Cart 服务**：`AddToCart`、`GetCart`（AI 图书馆员帮用户加购物车）
- **Payment 服务**：`CreatePayment`（AI 图书馆员帮用户创建支付）

### 外部依赖

- **OpenAI / DeepSeek API**：ChatCompletion（GPT-4o / deepseek-chat，通过 `base_url` 切换）、Embeddings（text-embedding-3-small）
- **Milvus**：向量存储与 HNSW ANN 搜索

### 被依赖

- **API Gateway**：转发 AI 相关请求（含 SSE 流式端点 `/ai/chat/stream`）
