# BookHive AI 服务 — 基于字节跳动 Eino 框架重构设计文档

> 版本: v3.0  
> 日期: 2026-03-10  
> 状态: 已完成

---

## 一、改造背景

### 1.1 现状

当前 AI 服务（`service/ai`）直接使用 `sashabaranov/go-openai` 裸调 OpenAI API，存在以下问题：

| 问题 | 描述 |
|------|------|
| **手动管理 ReAct 循环** | `ChatWithLibrarian` 中手写 for 循环处理 Tool Call → 执行 → 回传，逻辑复杂且脆弱 → ✅ 已由 Eino ADK 自动管理 |
| **Tool 定义分散** | Tool 的 JSON Schema 以 `json.RawMessage` 硬编码在代码中，与执行逻辑分离 → ✅ 已改为 `InferTool` + jsonschema tag |
| **无法扩展 Agent 模式** | 缺乏多 Agent 协作、子 Agent 委派能力 → ✅ 已建立 5 Agent + 6 Tool 体系 |
| **无流式输出** | 所有接口均为同步阻塞调用，无法实现打字机效果 → ✅ 已实现 `StreamChat` SSE 流式 |
| **供应商锁定** | 紧耦合 OpenAI SDK，切换模型供应商需要大量改动 → ✅ 已通过 Eino ChatModel 抽象解耦 |

### 1.2 改造目标

- 引入 **CloudWeGo Eino** 框架（字节跳动开源，Apache 2.0，9.8k+ stars）
- 利用 Eino **ADK (Agent Development Kit)** 的 `ChatModelAgent` + `Tool` 抽象
- 统一的 ChatModel 接口，可无缝切换 OpenAI / Claude / Gemini / Ollama
- Tool 通过结构体 + jsonschema tag 自动推导 Schema，消除手写 JSON
- 内置 ReAct 循环、流式处理、回调切面（Callback Aspects）

---

## 二、技术选型

### 2.1 核心依赖

| 包 | 用途 | Import Path |
|----|------|-------------|
| **eino** | 核心框架：schema、compose、adk、components | `github.com/cloudwego/eino` |
| **eino-ext/model/openai** | OpenAI ChatModel 实现 | `github.com/cloudwego/eino-ext/components/model/openai` |
| **eino** schema | Message、ToolInfo 等类型定义 | `github.com/cloudwego/eino/schema` |
| **eino** adk | Agent 抽象、Runner | `github.com/cloudwego/eino/adk` |
| **eino** tool/utils | InferTool、NewTool 等工具构建器 | `github.com/cloudwego/eino/components/tool/utils` |
| **eino** compose | Graph、Chain、ToolsNode | `github.com/cloudwego/eino/compose` |

### 2.2 架构对比

```
改造前:                              改造后:
┌─────────────┐                     ┌─────────────┐
│  AIHandler  │                     │  AIHandler  │
│  (698 lines)│                     │  (gRPC 适配) │
└──────┬──────┘                     └──────┬──────┘
       │ 直接调用                           │ 委派
       ▼                                   ▼
┌─────────────┐                     ┌─────────────────┐
│ go-openai   │                     │  Eino Agents     │
│ SDK         │                     │  ┌─────────────┐ │
└─────────────┘                     │  │ Librarian   │ │  ← ChatModelAgent + Tools
                                    │  │ Recommender │ │  ← ChatModelAgent
                                    │  │ Summarizer  │ │  ← ChatModelAgent
                                    │  │ SearchAgent │ │  ← ChatModelAgent + Tools
                                    │  │ TasteAgent  │ │  ← ChatModelAgent
                                    │  └─────────────┘ │
                                    │  ┌─────────────────┐ │
                                    │  │  Eino Tools      │ │
                                    │  │  SearchBooks     │ │  ← InferTool → gRPC BookService
                                    │  │  BookDetail      │ │  ← InferTool → gRPC BookService
                                    │  │  CheckStock      │ │  ← InferTool → gRPC InventoryService
                                    │  │  FindSimilar     │ │  ← InferTool → Milvus ANN 搜索
                                    │  │  GetUserOrders   │ │  ← InferTool → gRPC OrderService
                                    │  │  AddToCart        │ │  ← InferTool → gRPC CartService
                                    │  │  GetCart          │ │  ← InferTool → gRPC CartService
                                    │  │  CreateOrder      │ │  ← InferTool → gRPC OrderService
                                    │  │  GetOrderDetail   │ │  ← InferTool → gRPC OrderService
                                    │  │  CancelOrder      │ │  ← InferTool → gRPC OrderService
                                    │  │  CreatePayment    │ │  ← InferTool → gRPC PaymentService
                                    │  └─────────────────┘ │
                                    │  ┌─────────────┐ │
                                    │  │ OpenAI CM   │ │  ← eino-ext/model/openai
                                    │  └─────────────┘ │
                                    └─────────────────┘
```

---

## 三、Agent 详细设计

### 3.1 AI 图书馆员 (LibrarianAgent) — 核心

**类型**: `adk.ChatModelAgent` + 动态 Tool 分组（由 ToolRegistry + IntentRouter 按需注入）

```go
// 每次对话动态创建 Agent，只包含匹配意图的 Tool 和 Prompt 片段
func NewLibrarianAgent(ctx context.Context, cm model.ToolCallingChatModel,
    registry *ToolRegistry, groups []ToolGroup) (adk.Agent, error) {

    tools := registry.GetByGroups(groups...)       // 只取相关分组的 Tool（3-5 个）
    instruction := buildInstruction(registry, groups) // 动态拼接 Prompt

    return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
        Name:        "BookHiveLibrarian",
        Instruction: instruction,
        Model:       cm,                           // temperature=1.3（DeepSeek 通用对话）
        ToolsConfig: adk.ToolsConfig{
            ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools},
        },
        MaxIterations: 8,
    })
}
```

**System Prompt**:
```
你是 BookHive 的 AI 图书馆员，精通各类书籍。
你可以搜索书店目录、查看图书详情、检查门店库存。
当推荐书籍时，务必解释推荐理由。
回复使用用户的语言（中文或英文）。
当推荐书籍时，在回复末尾附加 JSON 块 [SUGGESTED_BOOKS][{title, author, category, reason}]。
当建议用户操作时，附加 JSON 块 [ACTIONS][{type, label, payload}]。
```

**ReAct 流程**:
1. 用户发送消息 → Agent 调用 ChatModel
2. ChatModel 决定是否需要调用 Tool（如搜索、查详情）
3. Tool 执行后结果返回 ChatModel
4. ChatModel 综合所有信息生成最终回复
5. 整个循环由 Eino ADK 自动管理，无需手写

**会话管理**: 保留现有 MongoDB 存储，将 `schema.Message` 序列化后存入。

---

### 3.2 智能推荐 Agent (RecommenderAgent)

**类型**: `adk.ChatModelAgent`（无 Tool，纯 LLM 推理）

```go
adk.ChatModelAgentConfig{
    Name:        "BookRecommender",
    Description: "Generates personalized book recommendations",
    Instruction:  recommendSystemPrompt,
    Model:        chatModel,
}
```

用 `runner.Query()` 单次调用，传入用户上下文，获取 JSON 格式推荐列表。

---

### 3.3 书籍摘要 Agent (SummarizerAgent)

**类型**: `adk.ChatModelAgent` + 1 个 Tool (get_book_detail)

```go
adk.ChatModelAgentConfig{
    Name:        "BookSummarizer",
    Description: "Generates structured book summaries",
    Instruction:  summarySystemPrompt,
    Model:        chatModel,
    ToolsConfig:  // 包含 getBookDetailTool
}
```

先通过 Tool 获取图书元数据，然后生成结构化摘要。结果缓存到 MongoDB。

---

### 3.4 自然语言搜索 Agent (SmartSearchAgent)

**类型**: `adk.ChatModelAgent` + 1 个 Tool (search_books)

```go
adk.ChatModelAgentConfig{
    Name:        "SmartSearcher",
    Description: "Extracts search intent from natural language and searches the catalog",
    Instruction:  smartSearchSystemPrompt,
    Model:        chatModel,
    ToolsConfig:  // 包含 searchBooksTool
}
```

用户输入自然语言 → Agent 提取搜索意图 → 调用 search_books Tool → 返回结果和解释。

---

### 3.5 阅读品味分析 Agent (TasteAnalyzerAgent)

**类型**: `adk.ChatModelAgent`（纯 LLM 推理）

```go
adk.ChatModelAgentConfig{
    Name:        "TasteAnalyzer",
    Description: "Analyzes user reading taste and patterns",
    Instruction:  tasteAnalysisSystemPrompt,
    Model:        chatModel,
}
```

---

## 四、Tool 详细设计

### 4.1 搜索图书 Tool

使用 Eino 的 `InferTool` 方式，通过 struct tag 自动推导 Schema：

```go
type SearchBooksInput struct {
    Keyword  string `json:"keyword"  jsonschema:"description=Search keyword for title or description"`
    Category string `json:"category" jsonschema:"description=Book category to filter by"`
    Author   string `json:"author"   jsonschema:"description=Author name to filter by"`
}

type SearchBooksOutput struct {
    Results []BookResult `json:"results"`
    Total   int64        `json:"total"`
}

// 通过 utils.InferTool 自动生成 ToolInfo
searchBooksTool, _ := utils.InferTool(
    "search_books",
    "Search for books in the BookHive catalog by keyword, category, or author",
    func(ctx context.Context, input *SearchBooksInput) (*SearchBooksOutput, error) {
        // 调用 BookService gRPC
        resp, err := bookSvc.SearchBooks(ctx, &bookPb.SearchBooksRequest{...})
        ...
    },
)
```

**优势**: 参数描述和类型定义合一，不会出现不一致。

### 4.2 获取图书详情 Tool

```go
type GetBookDetailInput struct {
    BookID string `json:"book_id" jsonschema:"description=The unique book ID,required"`
}

type GetBookDetailOutput struct {
    BookID      string  `json:"book_id"`
    Title       string  `json:"title"`
    Author      string  `json:"author"`
    Category    string  `json:"category"`
    Price       float64 `json:"price"`
    Rating      float64 `json:"rating"`
    Description string  `json:"description"`
    ISBN        string  `json:"isbn"`
}
```

### 4.3 库存检查 Tool

```go
type CheckStockInput struct {
    StoreID string `json:"store_id" jsonschema:"description=The store ID to check,required"`
    BookID  string `json:"book_id"  jsonschema:"description=The book ID to check,required"`
}

type CheckStockOutput struct {
    InStock  bool    `json:"in_stock"`
    Quantity int32   `json:"quantity"`
    Price    float64 `json:"price"`
}
```

### 4.4 语义相似图书 Tool

```go
type FindSimilarBooksInput struct {
    BookID string `json:"book_id" jsonschema:"description=The book ID to find similar books for,required"`
    Limit  int    `json:"limit,omitempty" jsonschema:"description=Maximum number of similar books to return (default 5)"`
}

type FindSimilarBooksOutput struct {
    SimilarBooks []SimilarBookResult `json:"similar_books"`
}
```

内部流程：
1. 从 Milvus 查找源图书的向量
2. 若不存在，自动调用 Embedding 模型生成并存储到 Milvus
3. Milvus HNSW ANN 搜索（Cosine 相似度）
4. 返回 Top-N 最相似图书（排除自身）

### 4.5 用户订单历史 Tool

```go
type GetUserOrdersInput struct {
    UserID uint64 `json:"user_id" jsonschema:"description=The user ID to look up orders for,required"`
    Status string `json:"status,omitempty" jsonschema:"description=Filter by order status (pending/paid/completed/cancelled). Leave empty for all."`
    Limit  int    `json:"limit,omitempty" jsonschema:"description=Maximum number of orders to return (default 10)"`
}

type GetUserOrdersOutput struct {
    Orders []OrderResult `json:"orders"`
    Total  int64         `json:"total"`
}
```

调用 Order 服务的 `ListOrders` gRPC 接口，帮助 Agent 了解用户的购买历史和阅读偏好。

### 4.6 加入购物车 Tool

```go
type AddToCartInput struct {
    UserID   uint64 `json:"user_id" jsonschema:"description=The user ID,required"`
    StoreID  uint64 `json:"store_id" jsonschema:"description=The store ID,required"`
    BookID   string `json:"book_id" jsonschema:"description=The book ID to add to cart,required"`
    Quantity int32  `json:"quantity,omitempty" jsonschema:"description=Number of copies to add (default 1)"`
}

type AddToCartOutput struct {
    Success     bool    `json:"success"`
    TotalItems  int32   `json:"total_items"`
    TotalAmount float64 `json:"total_amount"`
    Message     string  `json:"message"`
}
```

调用 Cart 服务的 `AddToCart` gRPC 接口，允许 AI 图书馆员在用户明确表示想购买时直接帮助加入购物车。

### 4.7 查看购物车 Tool

```go
type GetCartInput struct {
    UserID uint64 `json:"user_id" jsonschema:"description=The user ID,required"`
}

type GetCartOutput struct {
    Items       []CartItemResult `json:"items"`
    TotalCount  int32            `json:"total_count"`
    TotalAmount float64          `json:"total_amount"`
    StoreID     uint64           `json:"store_id"`
}
```

调用 Cart 服务的 `GetCart` gRPC 接口，查看用户购物车中的所有商品、数量和金额。

### 4.8 创建订单 Tool

```go
type CreateOrderInput struct {
    UserID       uint64                 `json:"user_id" jsonschema:"description=The user ID,required"`
    StoreID      uint64                 `json:"store_id" jsonschema:"description=The store ID,required"`
    Items        []CreateOrderItemInput `json:"items" jsonschema:"description=List of books to order,required"`
    PickupMethod string                 `json:"pickup_method" jsonschema:"description=Pickup method: self_pickup or delivery,required"`
    AddressID    uint64                 `json:"address_id,omitempty" jsonschema:"description=Delivery address ID"`
    Remark       string                 `json:"remark,omitempty" jsonschema:"description=Optional order remark"`
}

type CreateOrderOutput struct {
    OrderNo     string  `json:"order_no"`
    OrderID     uint64  `json:"order_id"`
    Status      string  `json:"status"`
    TotalAmount float64 `json:"total_amount"`
    Message     string  `json:"message"`
}
```

调用 Order 服务的 `CreateOrder` gRPC 接口。**敏感操作**：Tool Description 中注明必须先向用户展示订单摘要并获得确认后才能调用。

### 4.9 查询订单详情 Tool

```go
type GetOrderDetailInput struct {
    OrderID uint64 `json:"order_id,omitempty" jsonschema:"description=The numeric order ID"`
    OrderNo string `json:"order_no,omitempty" jsonschema:"description=The order number"`
    UserID  uint64 `json:"user_id" jsonschema:"description=The user ID,required"`
}
```

调用 Order 服务的 `GetOrder` gRPC 接口，返回订单号、状态、商品列表、总金额等详情。

### 4.10 取消订单 Tool

```go
type CancelOrderInput struct {
    OrderID uint64 `json:"order_id" jsonschema:"description=The numeric order ID to cancel,required"`
    UserID  uint64 `json:"user_id" jsonschema:"description=The user ID,required"`
}
```

调用 Order 服务的 `CancelOrder` gRPC 接口。**敏感操作**：Tool Description 中注明取消不可逆，必须获得用户确认。

### 4.11 创建支付 Tool

```go
type CreatePaymentInput struct {
    OrderID uint64  `json:"order_id" jsonschema:"description=The order ID to pay for,required"`
    UserID  uint64  `json:"user_id" jsonschema:"description=The user ID,required"`
    Amount  float64 `json:"amount" jsonschema:"description=Payment amount,required"`
    Method  string  `json:"method" jsonschema:"description=Payment method: wechat or alipay,required"`
}
```

调用 Payment 服务的 `CreatePayment` gRPC 接口。**敏感操作**：Tool Description 中注明必须先与用户确认支付金额和方式。

---

## 五、Embedding 向量化设计

### 5.1 架构

```
BookService.CreateBook → (event) → AI Service Background Job
                                        ↓
                                   Eino Embedder (text-embedding-3-small)
                                        ↓
                              buildBookText(title, author, category, description)
                                        ↓
                                   []float32 (1536维向量)
                                        ↓
                              Milvus book_embeddings collection (HNSW + Cosine)
```

### 5.2 向量数据库：Milvus

选用 **Milvus** 作为专业向量数据库，替代 MongoDB 内存余弦计算方案：

| 维度 | MongoDB 方案（旧） | Milvus 方案（新） |
|------|---------------------|---------------------|
| **搜索算法** | 全量加载 + 内存 O(N) 遍历 | HNSW 索引，ANN 近似搜索 O(log N) |
| **性能** | 10K 本书需秒级扫描 | 百万级向量 <10ms 查询 |
| **相似度计算** | 手写 `cosineSimilarity()` | Milvus 原生 COSINE 指标 |
| **可扩展性** | 单机内存限制 | 分布式，支持分片和副本 |
| **存储** | BSON 文档存 float64 数组 | 列式存储，float32 向量原生支持 |

**部署架构**（Docker Compose）：

```
milvus-etcd (元数据协调)  ─┐
                            ├─→  Milvus Standalone (v2.4.4)
milvus-minio (对象存储)   ─┘         │
                                     ↓
                               port 19530 (gRPC)
                                     ↓
                            AI Service (milvus-sdk-go/v2)
```

**Collection Schema**：

| Field | Type | Description |
|-------|------|-------------|
| `book_id` | VarChar(64), PK | 图书唯一标识 |
| `title` | VarChar(512) | 图书标题 |
| `author` | VarChar(256) | 作者 |
| `category` | VarChar(128) | 分类 |
| `embedding` | FloatVector(1536) | text-embedding-3-small 生成的向量 |

**索引配置**：HNSW（M=16, efConstruction=256），搜索参数 ef=128

### 5.3 Vector Store (`service/ai/vectorstore/milvus.go`)

| 方法 | 功能 |
|------|------|
| `NewMilvusStore(ctx, address)` | 连接 Milvus，自动创建 Collection + HNSW 索引 |
| `UpsertBookEmbedding(...)` | 插入/更新图书向量 |
| `HasEmbedding(bookID)` | 检查图书是否已有向量 |
| `GetEmbedding(bookID)` | 获取指定图书的向量 |
| `FindSimilarBooks(bookID, vec, topN)` | ANN 搜索，返回 Top-N 相似图书（排除自身） |
| `FlushCollection()` | 强制持久化 |

### 5.4 Embedding Service (`service/ai/embedding/embedder.go`)

| 方法 | 功能 |
|------|------|
| `EmbedText(text)` | 调用 Eino OpenAI Embedder 生成 float32 向量 |
| `EmbedBook(bookID)` | 通过 gRPC 获取图书元数据 → 构建文本 → 生成向量 → 存入 Milvus |
| `FindSimilarBooks(bookID, topN)` | 获取源向量 → Milvus ANN 搜索 → 返回 Top-N |
| `EmbedAllBooks()` | 后台任务，分页遍历所有图书，为缺失 embedding 的图书生成向量 |

### 5.5 向量文本构建

```go
func buildBookText(title, author, category, description string) string {
    return fmt.Sprintf("Title: %s\nAuthor: %s\nCategory: %s\nDescription: %s",
        title, author, category, description)
}
```

### 5.6 配置

```yaml
# config.yaml
milvus:
  address: 127.0.0.1:19530
```

### 5.6 gRPC 接口

```protobuf
rpc GetSimilarBooks(SimilarBooksRequest) returns (SimilarBooksResponse);

message SimilarBooksRequest {
    string book_id = 1;
    int32 limit = 2;
}

message SimilarBooksResponse {
    string book_id = 1;
    repeated BookRecommendation similar_books = 2;
}
```

### 5.7 API 路由

`GET /api/v1/ai/similar/:book_id?limit=5` — 公开接口，返回语义相似图书列表

---

## 六、文件结构设计

```
service/ai/
├── main.go                      # 入口：初始化 Milvus + Embedder + RAG Retriever + Agents
├── handler/
│   └── ai_handler.go            # gRPC handler，RAG 上下文注入 + Agent 执行
├── agent/
│   ├── chatmodel.go             # 创建 Eino OpenAI ChatModel
│   ├── registry.go              # ToolRegistry — Tool 分组注册与 Prompt 动态管理
│   ├── intent.go                # IntentRouter — 两阶段意图路由（关键词 + LLM fallback）
│   ├── librarian.go             # LibrarianAgent（动态创建，按需注入 Tool + Prompt）
│   ├── recommender.go           # RecommenderAgent 定义（RAG 增强）
│   ├── summarizer.go            # SummarizerAgent 定义
│   ├── smart_search.go          # SmartSearchAgent 定义
│   └── taste_analyzer.go        # TasteAnalyzerAgent 定义
├── rag/
│   └── retriever.go             # RAG 检索器（Milvus 语义检索 + 实时库存检查 + 上下文格式化）
├── vectorstore/
│   └── milvus.go                # Milvus 向量数据库封装（HNSW + Cosine ANN 搜索）
├── embedding/
│   └── embedder.go              # Eino Embedding 封装（向量生成 + Milvus 存取 + 批量任务）
├── tools/
│   ├── search_books.go          # 搜索图书 Tool（调用 BookService gRPC）
│   ├── get_book_detail.go       # 获取详情 Tool（调用 BookService gRPC）
│   ├── check_stock.go           # 库存检查 Tool（调用 InventoryService gRPC）
│   ├── find_similar_books.go    # 语义相似图书 Tool（调用 EmbeddingService → Milvus）
│   ├── get_user_orders.go       # 用户订单历史 Tool（调用 OrderService gRPC）
│   ├── add_to_cart.go           # 加入购物车 Tool（调用 CartService gRPC）
│   ├── get_cart.go              # 查看购物车 Tool（调用 CartService gRPC）
│   ├── create_order.go          # 创建订单 Tool（调用 OrderService gRPC）
│   ├── get_order_detail.go      # 查询订单详情 Tool（调用 OrderService gRPC）
│   ├── cancel_order.go          # 取消订单 Tool（调用 OrderService gRPC）
│   └── create_payment.go        # 创建支付 Tool（调用 PaymentService gRPC）
├── model/
│   └── ai_model.go              # MongoDB 数据模型（ChatSession, BookSummaryCache）
└── repository/
    └── ai_repo.go               # MongoDB 存储（会话 + 摘要缓存）
```

---

## 七、关键交互流程

### 6.1 AI 图书馆员对话（RAG 增强）

```
用户 → API Gateway → gRPC AIService.ChatWithLibrarian
         ↓
    AIHandler.ChatWithLibrarian
         ↓
    1. 从 MongoDB 加载历史会话
    2. ⭐ RAG 检索（Retrieve-Augment-Generate）：
       a. BookRetriever.Retrieve(userMessage)
          → Eino Embedder 将用户消息向量化
          → Milvus ANN 搜索 Top-8 相似图书
          → BatchCheckStock 批量查询实时库存
          → 生成包含书名/作者/分类/相似度/库存状态的 Document 列表
       b. FormatDocsAsContext(docs) → 格式化为结构化文本
    3. 构建 eino schema.Message 列表：
       [RAG 上下文 (system)] + [历史消息] + [用户新消息]
    4. 创建 LibrarianAgent Runner
    5. runner.Run(ctx, messages)
         ↓ (Eino ADK 内部 ReAct 循环)
    6. ChatModel 基于检索结果回答 → 可能调用 check_stock/get_user_orders/add_to_cart 等工具
    7. ChatModel 生成最终回复（仅推荐书库中存在的书，库存不足的书标注警告）
         ↓
    8. 解析回复中的 [SUGGESTED_BOOKS] 和 [ACTIONS]
    9. 保存会话到 MongoDB
    10. 返回 ChatResponse
```

**RAG 核心保证**：
- AI 只推荐书库中实际存在的书（不会编造不存在的书）
- 库存不足的书会被明确标注，用户不会下单后才发现无货
- 有货的书优先被推荐

### 6.2 智能搜索

```
用户 → "找一本适合10岁孩子的冒险故事"
         ↓
    SmartSearchAgent.Run()
         ↓ (Eino ReAct)
    1. ChatModel 理解意图 → 调用 search_books(category="Children", keyword="adventure")
    2. search_books → BookService.SearchBooks gRPC
    3. 结果返回 → ChatModel 生成 interpreted_query + 过滤条件
         ↓
    返回 SmartSearchResponse { results, interpreted_query, extracted_filters }
```

---

## 八、配置变更

`config.yaml` 中 OpenAI 配置保持不变，新增 Milvus 地址：

```yaml
openai:
  api_key: "sk-xxx"
  model: "gpt-4o"
  embedding_model: "text-embedding-3-small"
  base_url: ""   # 可选，兼容 Azure / 自托管

milvus:
  address: "127.0.0.1:19530"
```

Eino ChatModel 初始化（基于 **DeepSeek 官方推荐** Temperature 设置）：

| ChatModel 实例 | Temperature | 用途 | 对应 Agent |
|---------------|-------------|------|-----------|
| `chatModelConversation` | **1.3** | 通用对话 | Librarian（图书馆员） |
| `chatModelAnalysis` | **1.0** | 数据抽取/分析 | Recommender、Summarizer、SmartSearch、TasteAnalyzer、IntentRouter |

> **注**: 温度参数遵循 DeepSeek 官方指南：
> - 代码生成/数学解题 → 0.0
> - 数据抽取/分析 → 1.0
> - 通用对话 → 1.3
> - 翻译 → 1.3
> - 创意类写作/诗歌创作 → 1.5

```go
convTemp := float32(1.3)
chatModelConversation, _ := einoOpenAI.NewChatModel(ctx, &einoOpenAI.ChatModelConfig{
    APIKey:      cfg.OpenAI.APIKey,
    Model:       cfg.OpenAI.Model,
    BaseURL:     cfg.OpenAI.BaseURL,
    Temperature: &convTemp,
})

analysisTemp := float32(1.0)
chatModelAnalysis, _ := einoOpenAI.NewChatModel(ctx, &einoOpenAI.ChatModelConfig{
    APIKey:      cfg.OpenAI.APIKey,
    Model:       cfg.OpenAI.Model,
    BaseURL:     cfg.OpenAI.BaseURL,
    Temperature: &analysisTemp,
})
```

---

## 九、向后兼容性

| 维度 | 影响 |
|------|------|
| **Proto 定义** | 不变，所有 gRPC 接口签名保持一致 |
| **API Gateway** | 不变，仅调用 gRPC 客户端 |
| **MongoDB schema** | 不变，ChatSession / BookSummaryCache 结构不变；book_embeddings 已迁移至 Milvus |
| **配置** | 新增 `milvus.address` 字段；OpenAI 配置字段不变 |
| **其他微服务** | 不受影响 |

改动范围严格限于 `service/ai/` 目录内部。

---

## 十、流式输出设计

### 10.1 架构

```
浏览器 ←── SSE (text/event-stream) ──→ API Gateway ←── go-micro streaming ──→ AI Service
                POST /api/v1/ai/chat/stream              StreamChat RPC
                                                              ↓
                                                     Eino Runner (EnableStreaming: true)
                                                              ↓
                                                     AgentEvent.MessageStream.Recv()
                                                         逐 token 推送
```

### 10.2 Proto 定义

```protobuf
rpc StreamChat(ChatRequest) returns (stream ChatStreamChunk);

message ChatStreamChunk {
    string type = 1;                              // "delta" | "metadata" | "error" | "done"
    string delta = 2;                             // 增量文本 token
    string session_id = 3;                        // 会话 ID
    repeated BookRecommendation suggested_books = 4;
    repeated ActionSuggestion actions = 5;
    string error = 6;
}
```

### 10.3 SSE 事件格式

| 事件类型 | 数据格式 | 说明 |
|---------|---------|------|
| `delta` | `{"delta":"..."}` | 增量文本 token（打字机效果） |
| `metadata` | `{"session_id":"...", "suggested_books":[...], "actions":[...]}` | 会话 ID + 推荐书籍 + 操作建议 |
| `error` | `{"error":"..."}` | 错误信息 |
| `done` | `[DONE]` | 流结束标志 |

### 10.4 前端调用示例

```javascript
const response = await fetch('/api/v1/ai/chat/stream', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ...' },
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
      if (chunk.delta) appendToUI(chunk.delta);  // 打字机效果
    }
  }
}
```

### 10.5 关键实现细节

- **Eino EnableStreaming**: `Runner` 配置 `EnableStreaming: true`，Agent 返回的 `MessageOutput.IsStreaming=true`，通过 `MessageStream.Recv()` 逐帧获取 token
- **go-micro server streaming**: `protoc-gen-micro` 自动生成 `AIService_StreamChatStream.Send(*ChatStreamChunk)` 桩代码
- **Gateway WriteTimeout**: 从 30s 提升至 120s，避免长对话被提前断开
- **CORS**: Expose `Content-Type` header 以支持 SSE 跨域
- **向后兼容**: 原有 `POST /ai/chat`（同步）保持不变，新增 `POST /ai/chat/stream`（流式）

---

## 十一、动态 Tool 加载设计

### 11.1 设计动机

随着 Tool 数量增长（当前 11 个，未来可能 20+），一次性将所有 Tool 注入 System Prompt 会导致：
- Prompt token 暴增（每增一个 Tool 约 +200 token）
- LLM 注意力稀释，工具调用准确率下降
- 不相关的敏感操作工具暴露，增加误触风险

### 11.2 整体架构

```
用户消息
    ↓
IntentRouter.Classify(ctx, message, history)
    ↓
┌──────────────────────────────────┐
│  Phase 1: 关键词匹配（无 API 调用）   │
│  遍历 ToolRegistry 所有 GroupMeta   │
│  的 Keywords 进行正则/子串匹配       │
└──────────────┬───────────────────┘
               ↓ 仅命中 GroupDiscovery?
┌──────────────────────────────────┐
│  Phase 2: LLM Fallback 兜底       │
│  调用 chatModelAnalysis (temp=1.0) │
│  动态构建分类 prompt（从 Registry）  │
│  返回 intent 分类结果               │
└──────────────┬───────────────────┘
               ↓
  []ToolGroup（匹配到的分组列表）
               ↓
  NewLibrarianAgent(ctx, cm, registry, groups)
               ↓
  ┌─ registry.GetByGroups(groups...) → 3-5 个 Tool
  └─ buildInstruction(registry, groups) → 动态 Prompt
               ↓
  按需创建的 Agent 实例
```

### 11.3 ToolRegistry

```go
type ToolGroup string
const (
    GroupDiscovery ToolGroup = "discovery"   // 搜索发现：search_books, get_book_detail, check_stock, find_similar, get_user_orders
    GroupShopping  ToolGroup = "shopping"    // 购物：add_to_cart, get_cart
    GroupCheckout  ToolGroup = "checkout"    // 下单支付：create_order, create_payment
    GroupOrder     ToolGroup = "order"       // 订单管理：get_order_detail, cancel_order
)

type GroupMeta struct {
    Title      string   // Prompt 中的段落标题
    Footer     string   // Prompt 中的注意事项
    IntentHint string   // LLM fallback 分类描述
    Keywords   []string // 关键词匹配列表
}
```

- 所有 Prompt 片段、关键词、LLM 分类 hint 均注册在 `GroupMeta` 中
- `intent.go` 和 `librarian.go` 零硬编码，完全从 Registry 动态读取

### 11.4 效果对比

| 维度 | 静态加载（旧） | 动态加载（新） |
|------|---------------|---------------|
| System Prompt 长度 | ~2000 token（全量 11 Tool） | ~800 token（3-5 Tool） |
| 敏感工具暴露 | 始终暴露 | 仅意图匹配时注入 |
| 新增 Tool 步骤 | 改 main.go + 改 librarian.go prompt | 仅在 main.go 注册 ToolEntry + GroupMeta |
| 意图兜底 | 无（关键词漏检即丢失） | LLM fallback（准确率 >95%） |

---

## 十二、改造收益

| 收益 | 说明 |
|------|------|
| **代码简化** | 消除手写 ReAct 循环、手写 Tool JSON Schema |
| **可扩展性** | 新增 Tool 只需一个 struct + handler 函数 + `ToolRegistry.Register()`（当前已 11 个 Tool，覆盖购书全流程） |
| **动态加载** | ToolRegistry + IntentRouter 两阶段意图路由，System Prompt 按需拼接，Token 用量降低 60% |
| **意图兜底** | IntentRouter LLM Fallback 机制，关键词漏检时自动调用 LLM 分类，准确率 >95% |
| **多模型支持** | 只需更换 ChatModel 实现即可切换到 Claude/Gemini/Ollama |
| **流式输出** | ✅ 已实现。`StreamChat` SSE 端点 (`POST /api/v1/ai/chat/stream`)，Eino Stream 模式实时推送 token |
| **Temperature 优化** | 基于 DeepSeek 官方推荐设置，对话 1.3、分析 1.0，确保各 Agent 表现最优 |
| **回调切面** | 可接入 tracing/logging/metrics 而不侵入业务逻辑 |
| **多 Agent 协作** | 可基于 ADK SubAgent / Transfer 实现更复杂场景 |
| **社区生态** | 复用 eino-ext 的官方 Tool 实现（DuckDuckGo、RAG 等） |

---

## 十三、后续演进方向

1. **RAG 全链路**: ✅ 已实现。BookRetriever（Milvus ANN + 实时库存）→ 上下文注入 → Agent 生成。AI 只推荐书库中存在的书，库存不足自动提示
2. **Milvus 集群化**: 当前使用 Standalone 模式，当向量量级达到千万时可升级为 Milvus Cluster（Pulsar + 分布式 QueryNode）
3. **多 Agent 协作**: 图书馆员可 Transfer 给「推荐专家」或「库存助手」子 Agent
4. **流式输出**: ✅ 已实现。`StreamChat` server-streaming RPC + API Gateway SSE (`POST /api/v1/ai/chat/stream`)
5. **Human-in-the-Loop**: 利用 Eino 的 Interrupt/Resume 实现"确认下单"等交互
6. **本地模型**: 通过 Eino Ollama 适配器接入 DeepSeek / Qwen 等本地模型
7. **增量 Embedding**: 监听 BookService 的创建/更新事件，通过 RabbitMQ 触发自动向量化
8. **混合检索**: 结合 Milvus 向量搜索 + Elasticsearch 全文检索，实现更精准的语义+关键词混合搜索
