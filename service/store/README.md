# Store 服务（门店服务）

> 服务名：`bookhive.store` | 端口：9002 | 数据库：MySQL + Redis

## 一、服务概述

门店服务管理连锁书店的门店信息，核心亮点是利用 MySQL 空间索引（`SPATIAL INDEX`）实现基于地理位置的最近门店查询与半径范围搜索，并使用 Redis 缓存门店坐标以加速高频查询。

## 二、目录结构

```
service/store/
├── main.go                 # 服务入口：MySQL + Redis 连接、服务注册
├── model/
│   └── store.go            # 数据模型：Store、StoreWithDistance
├── repository/
│   └── store_repo.go       # 数据访问层（GORM + 原生 SQL 空间查询）
└── handler/
    └── store_handler.go    # gRPC 接口实现（6 个 RPC）
```

## 三、数据模型

### 3.1 stores 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT PK | 主键自增 |
| name | VARCHAR(200) | 门店名称 |
| description | TEXT | 门店描述 |
| address | VARCHAR(500) | 详细地址 |
| city | VARCHAR(50) INDEX | 城市 |
| district | VARCHAR(50) | 区/县 |
| phone | VARCHAR(20) | 联系电话 |
| location | POINT SRID 4326 | 空间坐标（WGS84），有 SPATIAL INDEX |
| business_hours | VARCHAR(100) | 营业时间（默认 09:00-21:00） |
| status | TINYINT INDEX | 状态（1=营业，0=关闭） |
| image_url | VARCHAR(500) | 门店图片 |

> GORM 模型使用 `Latitude`/`Longitude` 字段，`location` 通过原生 SQL `ST_GeomFromText` 写入。

## 四、RPC 接口

| 方法 | 功能 | 关键逻辑 |
|------|------|----------|
| `ListStores` | 门店列表 | 支持按城市筛选，分页，仅返回 status=1 |
| `GetStoreDetail` | 门店详情 | 按 ID 查询 |
| `GetNearestStore` | 最近门店 | `ST_Distance_Sphere` 计算距离，返回最近一家 |
| `GetStoresInRadius` | 半径内门店 | 指定经纬度和半径(km)，返回范围内门店列表 |
| `CreateStore` | 创建门店 | GORM 创建 + 原生 SQL 写入 POINT + 缓存坐标 |
| `UpdateStore` | 更新门店 | 支持部分字段更新，自动清除 Redis 缓存 |

## 五、技术选型

| 技术 | 用途 |
|------|------|
| go-micro v4 | 微服务框架，gRPC |
| MySQL | 持久化，InnoDB 引擎 |
| GORM | ORM 读写 |
| MySQL 空间函数 | `ST_GeomFromText`、`ST_Distance_Sphere` 地理距离计算 |
| Redis | 门店坐标缓存（Key: `store:coords:{id}`，TTL: 30 分钟） |
| Consul | 服务注册与发现 |

## 六、核心业务流程

### 最近门店查询

```
客户端 → GetNearestStore(latitude, longitude)
  ├─ SQL: SELECT *, ST_Distance_Sphere(location, POINT(lng, lat)) / 1000 AS distance
  ├─ WHERE status = 1
  ├─ ORDER BY distance ASC LIMIT 1
  └─ 返回 Store + distance_km
```

### 半径范围搜索

```
客户端 → GetStoresInRadius(lat, lng, radius_km, limit)
  ├─ SQL: SELECT *, ST_Distance_Sphere(...) / 1000 AS distance
  ├─ HAVING distance <= radius_km
  ├─ ORDER BY distance ASC LIMIT limit（默认 20）
  └─ 返回 StoreListResponse
```

### 缓存策略

```
创建门店 → 写入 Redis 坐标缓存
更新门店 → 删除 Redis 缓存（下次读取时 miss 回源）
读取坐标 → 优先 Redis，miss 则查 MySQL
```

## 七、被依赖关系

- **API Gateway**：转发门店查询请求
- **前端**：基于地理位置展示附近门店
