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
│   └── ai_handler.go           # gRPC 接口实现（6 个 RPC）
├── agent/                      # AI Agent 定义
│   ├── chatmodel.go            # ChatModel 工厂（创建 OpenAI ChatModel）
│   ├── librarian.go            # LibrarianAgent：AI 图书馆员
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
    └── find_similar_books.go   # FindSimilarBooksTool：语义相似搜索
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
| `GetRecommendations` | 智能推荐 | RAG Retriever + RecommenderAgent |
| `ChatWithLibrarian` | AI 图书馆员对话 | LibrarianAgent（含 4 个 Tool） |
| `GenerateBookSummary` | 生成图书摘要 | SummarizerAgent + MongoDB 缓存 |
| `SmartSearch` | 自然语言搜索 | SmartSearchAgent → Book.SearchBooks |
| `AnalyzeReadingTaste` | 阅读偏好分析 | TasteAnalyzerAgent |
| `GetSimilarBooks` | 语义相似图书 | Milvus 向量搜索 |

## 五、Agent 架构

### 5.1 Agent 体系

所有 Agent 基于 Eino 的 `ChatModelAgent` 构建，使用 `NewChatModel()` 创建 OpenAI ChatModel 实例（模型：gpt-4o）。

| Agent | System Prompt 核心 | 工具 |
|-------|-------------------|------|
| **LibrarianAgent** | 专业图书馆员，只推荐有库存的书 | search_books, get_book_detail, check_stock, find_similar_books |
| **RecommenderAgent** | 基于上下文生成推荐理由 | 无（纯 LLM 推理） |
| **SummarizerAgent** | 生成结构化书评（JSON 输出） | 无 |
| **SmartSearchAgent** | 解析自然语言为搜索参数（JSON 输出） | 无 |
| **TasteAnalyzerAgent** | 分析阅读偏好生成用户画像 | 无 |

### 5.2 Tool 定义

使用 Eino 的 `utils.InferTool` + jsonschema tag 自动生成 Schema：

| Tool | 输入 | 功能 | 调用的服务 |
|------|------|------|-----------|
| `search_books` | keyword, category, author | 搜索图书 | Book.SearchBooks |
| `get_book_detail` | book_id | 获取详情 | Book.GetBookDetail |
| `check_stock` | store_id, book_id | 查库存 | Inventory.CheckStock |
| `find_similar_books` | book_id, limit | 语义相似 | Milvus 向量搜索 |

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

### 6.4 Librarian 对话（ReAct 循环）

```
用户: "有没有《三体》，门店 1 有货吗？"
  │
  LibrarianAgent (ReAct):
  ├─ Thought: 需要搜索三体这本书
  ├─ Action: search_books(keyword="三体") → 返回书籍列表
  ├─ Thought: 找到了，需要检查门店 1 的库存
  ├─ Action: check_stock(store_id=1, book_id="xxx") → 有库存，qty=5
  ├─ Thought: 有货，可以推荐
  └─ Answer: "《三体》目前在门店 1 有 5 本库存，可以购买。这是一部..."
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

### 外部依赖

- **OpenAI API**：ChatCompletion（GPT-4o）、Embeddings（text-embedding-3-small）
- **Milvus**：向量存储与 ANN 搜索

### 被依赖

- **API Gateway**：转发 AI 相关请求
