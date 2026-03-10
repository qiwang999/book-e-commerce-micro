# User 服务（用户服务）

> 服务名：`bookhive.user` | 端口：9001 | 数据库：MySQL

## 一、服务概述

用户服务是系统的基础身份模块，负责用户注册与登录、JWT 令牌签发与校验、个人资料与阅读偏好管理、收货地址管理等核心能力。所有需要身份认证的请求均通过 API Gateway 调用本服务的 `ValidateToken` 接口完成鉴权。

## 二、目录结构

```
service/user/
├── main.go                 # 服务入口：配置加载、DB 连接、服务注册
├── model/
│   └── user.go             # 数据模型：User、UserProfile、UserAddress
├── repository/
│   └── user_repo.go        # 数据访问层（GORM）
└── handler/
    └── user_handler.go     # gRPC 接口实现（10 个 RPC）
```

## 三、数据模型

### 3.1 users 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT PK | 主键自增 |
| email | VARCHAR(128) UNIQUE | 登录邮箱 |
| password_hash | VARCHAR(256) | bcrypt 哈希密码 |
| name | VARCHAR(64) | 用户昵称 |
| avatar_url | VARCHAR(512) | 头像地址 |
| role | VARCHAR(32) | 角色（默认 user） |
| status | INT | 账号状态（1=正常） |
| created_at / updated_at | TIMESTAMP | 时间戳 |

### 3.2 user_profiles 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT PK | 主键自增 |
| user_id | BIGINT UNIQUE | 关联用户 |
| phone | VARCHAR(32) | 手机号 |
| gender | TINYINT | 性别（0=未知） |
| birthday | VARCHAR(16) | 生日 |
| favorite_categories | JSON | 偏好分类列表 |
| favorite_authors | JSON | 偏好作者列表 |
| reading_preferences | JSON | 阅读偏好标签 |

### 3.3 user_addresses 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT PK | 主键自增 |
| user_id | BIGINT INDEX | 所属用户 |
| name | VARCHAR(64) | 收件人姓名 |
| phone | VARCHAR(32) | 联系电话 |
| province / city / district | VARCHAR(64) | 省 / 市 / 区 |
| detail | VARCHAR(256) | 详细地址 |
| is_default | BOOL | 是否默认地址 |

## 四、RPC 接口

| 方法 | 功能 | 关键逻辑 |
|------|------|----------|
| `Register` | 用户注册 | 邮箱唯一校验 → bcrypt 哈希 → 创建 User + Profile → 签发 JWT |
| `Login` | 用户登录 | 邮箱查询 → bcrypt 比对 → 签发 JWT |
| `GetProfile` | 获取个人资料 | 联合查询 User + UserProfile |
| `UpdateProfile` | 更新个人资料 | 支持姓名、头像、手机、性别、生日 |
| `GetUserPreferences` | 获取阅读偏好 | 返回 favorite_categories/authors/reading_preferences |
| `UpdateUserPreferences` | 更新阅读偏好 | Profile 不存在时自动创建 |
| `GetAddress` | 获取单个地址 | 按地址 ID 查询 |
| `ListAddresses` | 获取地址列表 | 按 is_default DESC, id DESC 排序 |
| `CreateAddress` | 创建收货地址 | 事务内处理默认地址互斥 |
| `ValidateToken` | 校验 JWT | 返回 valid、user_id、role |

## 五、技术选型

| 技术 | 用途 |
|------|------|
| go-micro v4 | 微服务框架，gRPC 服务端 |
| GORM | ORM，自动迁移 |
| MySQL | 持久化存储 |
| Consul | 服务注册与发现 |
| bcrypt | 密码安全哈希 |
| golang-jwt/jwt/v5 | JWT 签发与校验 |
| Viper + Consul KV | 配置管理（支持热更新） |

## 六、核心业务流程

### 注册流程

```
客户端 → Register(email, password, name)
  ├─ 邮箱已存在？→ 返回 400 "email already registered"
  ├─ bcrypt.GenerateFromPassword(password)
  ├─ 创建 User 记录
  ├─ 创建空 UserProfile
  ├─ JWTManager.GenerateToken(userID, role)
  └─ 返回 {token, user_info}
```

### 登录流程

```
客户端 → Login(email, password)
  ├─ 按邮箱查询 User（不存在 → 401）
  ├─ bcrypt.CompareHashAndPassword（失败 → 401）
  ├─ JWTManager.GenerateToken(userID, role)
  └─ 返回 {token, user_info}
```

### 地址创建（默认地址互斥）

```
客户端 → CreateAddress(user_id, ..., is_default=true)
  ├─ 开启事务
  ├─ UPDATE user_addresses SET is_default=false WHERE user_id=? AND is_default=true
  ├─ INSERT 新地址
  └─ 提交事务
```

## 七、被依赖关系

- **API Gateway**：调用 `ValidateToken` 实现统一鉴权
- **AI 服务**：调用 `GetUserPreferences` 获取用户偏好用于推荐
