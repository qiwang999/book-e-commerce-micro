# Book 服务（图书服务）

> 服务名：`bookhive.book` | 端口：9003 | 数据库：MongoDB + Elasticsearch

## 一、服务概述

图书服务是系统的核心数据服务，管理全部图书主数据。使用 MongoDB 存储非结构化的图书信息（标签、多语言描述等），集成 Elasticsearch 实现全文检索。当有搜索关键词时优先走 ES（multi_match + fuzziness），ES 不可用时自动降级为 MongoDB 正则匹配。图书的增删改操作会自动同步 ES 索引。

## 二、目录结构

```
service/book/
├── main.go                 # 服务入口：MongoDB + ES 连接、服务注册
├── model/
│   └── book.go             # 数据模型：Book
├── handler/
│   └── book_handler.go     # gRPC 接口实现（8 个 RPC），含 ES 索引同步
└── repository/
    ├── book_repo.go        # MongoDB 数据访问层
    └── es_repo.go          # Elasticsearch 数据访问层（全文检索 + 索引管理）
```

## 三、数据模型

### 3.1 books 集合

| 字段 | 类型 | 说明 |
|------|------|------|
| _id | ObjectID | MongoDB 主键 |
| title | string | 书名 |
| author | string | 作者 |
| isbn | string | ISBN 编号 |
| publisher | string | 出版社 |
| publish_date | string | 出版日期 |
| price | float64 | 定价 |
| category | string | 主分类 |
| subcategory | string | 子分类 |
| description | string | 图书简介 |
| cover_url | string | 封面图片 URL |
| pages | int32 | 页数 |
| language | string | 语言 |
| tags | []string | 标签列表 |
| rating | float64 | 评分 |
| rating_count | int64 | 评分人数 |
| created_at / updated_at | time.Time | 时间戳 |

## 四、RPC 接口

| 方法 | 功能 | 关键逻辑 |
|------|------|----------|
| `GetBookDetail` | 图书详情 | 按 ObjectID 查询单本 |
| `GetBooksByIds` | 批量查询 | `$in` 批量查询，用于购物车、订单等场景 |
| `SearchBooks` | 多条件搜索 | 有关键词时优先 ES multi_match 全文检索，降级为 MongoDB `$or` + `$regex`；支持分类/作者/语言/价格区间过滤 |
| `ListByCategory` | 分类浏览 | 按 category/subcategory 分页查询 |
| `CreateBook` | 创建图书 | 自动设置 created_at/updated_at，同步索引到 ES |
| `UpdateBook` | 更新图书 | `$set` 仅更新非空字段，同步更新 ES 索引 |
| `DeleteBook` | 删除图书 | 按 ObjectID 删除，同步删除 ES 索引 |
| `ListCategories` | 分类统计 | MongoDB 聚合：`$group` + `$addToSet` + `$sum` |

### 搜索排序支持

| sort_by 值 | 排序规则 |
|------------|----------|
| `price_asc` | 价格升序 |
| `price_desc` | 价格降序 |
| `rating` | 评分降序 |
| `newest` | 创建时间降序（默认） |

## 五、技术选型

| 技术 | 用途 |
|------|------|
| go-micro v4 | 微服务框架，gRPC |
| MongoDB 7.0 | 主存储，文档型数据库 |
| mongo-driver | Go 官方 MongoDB 驱动 |
| Elasticsearch 8 | 全文检索（multi_match + fuzziness），图书索引自动同步 |
| Consul | 服务注册与发现 |

## 六、核心业务流程

### 多条件搜索

```
客户端 → SearchBooks(keyword, category, author, min_price, max_price, language, sort_by, page, page_size)
  ├─ [ES 路径] keyword 非空且 ES 可用：
  │   ├─ ES multi_match 全文检索（title^3/author^2/description/tags, fuzziness=AUTO）
  │   ├─ ES bool filter：category(term)、language(term)、price(range)
  │   ├─ 返回 bookID 列表 + total
  │   ├─ MongoDB FindByIDs 批量获取完整文档
  │   └─ 按 ES 相关性排序返回
  ├─ [MongoDB 降级路径] keyword 为空 / ES 不可用：
  │   ├─ 构建 MongoDB filter：
  │   │   ├─ keyword → $or: [{title: $regex}, {description: $regex}, {author: $regex}]
  │   │   ├─ category / author / language / price 过滤
  │   ├─ CountDocuments → total
  │   ├─ Find + Skip/Limit + Sort
  └─ 返回 BookListResponse {books, total, page, page_size}
```

### 分类聚合统计

```
客户端 → ListCategories()
  ├─ MongoDB Aggregate Pipeline:
  │   ├─ $group: {_id: "$category", subcategories: {$addToSet: "$subcategory"}, count: {$sum: 1}}
  │   └─ $sort: {count: -1}
  └─ 返回 [{name, subcategories, count}, ...]
```

## 七、Elasticsearch 集成

ES 已完整接入 `main.go`，启动时自动初始化客户端、创建索引 mapping。ES 不可用时服务正常启动，搜索降级为 MongoDB。

| 能力 | 说明 |
|------|------|
| **EnsureIndex** | 启动时自动创建 `books` 索引，mapping 含 title(text, boost=3)、author(text, boost=2)、description(text)、category(keyword) 等 |
| **IndexBook** | CreateBook / UpdateBook 时自动同步索引 |
| **SearchBooks** | multi_match 全文检索（title/author/description/tags），fuzziness=AUTO，支持分类/语言/价格 filter |
| **DeleteBook** | DeleteBook 时自动删除 ES 文档 |

ES 同步采用**同步写入 + 失败仅日志**策略，不影响主流程。

## 八、被依赖关系

- **Cart 服务**：调用 `GetBookDetail` 获取书籍信息（封面、标题、价格等）
- **AI 服务**：调用 `SearchBooks`、`GetBookDetail` 用于智能搜索与推荐
- **API Gateway**：转发图书查询请求
