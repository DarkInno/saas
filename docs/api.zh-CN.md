# API 参考

[EN](api.md) | [中文](api.zh-CN.md)

公共包概览。

## 核心

| 包 | 用途 |
|---|---|
| `core/types` | 租户 ID、租户元数据、生命周期状态，以及主机/租户侧常量。 |
| `core/context` | `WithTenant`、`FromContext`、`WithHost`、`IsHost`、`Detach` 和 `Switch`。 |
| `core/resolver` | Header、cookie、query、domain、token-claim 和组合式 HTTP 租户解析器。 |
| `core/store` | 租户元数据存储接口、分页列表过滤器、内存存储、TTL/有界缓存、缓存装饰器，以及 `database/sql` 存储。 |

## 数据隔离

| 包 | 用途 |
|---|---|
| `data` | 与 ORM 无关的参数化租户过滤条件。 |
| `data/gorm` | GORM 插件、租户回调、`TenantScope`、仅限主机的 `SafeRaw`/`SafeExec`、`BulkCreate`、硬删除防护以及 MySQL 软删除索引规划。 |
| `data/ent` | 注入租户和可选软删除过滤器的 Ent selector 谓词、查询过滤器、变更过滤器和 hook。 |
| `data/sqlx` | 用于简单单表 SELECT/UPDATE/DELETE 语句的 sqlx 兼容 API；复杂 SQL 会以 `ErrUnsafeSQL` 拒绝。 |

## SaaS

| 包 | 用途 |
|---|---|
| `tenant` | 租户生命周期管理器，支持创建、激活、暂停、恢复、软删除和仅限主机的硬删除。 |
| `plan` | 套餐、功能和配额元数据，提供 Store、内存实现、列表过滤器和 `database/sql` SQLStore。 |
| `subscription` | 订阅生命周期，提供活跃/已取消/已过期状态、续订、宽限期过期扫描、计费 hook、Store、内存实现和 `database/sql` SQLStore。 |
| `quota` | 配额检查、原子消耗、重置、内存实现、nil-store 防护和 `database/sql` SQLStore。 |
| `feature` | 套餐默认功能和租户覆盖解析，提供内存实现和 `database/sql` SQLStore。 |
| `onboarding` | 跨模块的租户开通流程：创建租户、校验套餐、创建订阅、初始化功能和配额、记录审计元数据、发送可选欢迎通知并激活租户。 |

## 集成

| 包 | 用途 |
|---|---|
| `web/gin` | Gin 租户中间件，默认强制活跃状态，提供显式活跃状态防护、主机防护和通用错误处理器。 |
| `web/echo` | Echo 租户中间件，默认强制活跃状态，提供显式活跃状态防护和主机防护。 |
| `web/fiber` | Fiber 租户中间件，默认强制活跃状态，提供显式活跃状态防护和主机防护。 |
| `web/kratos` | Kratos 租户中间件，默认强制活跃状态，提供显式活跃状态防护和主机防护。 |
| `web/http` | 标准库 HTTP 租户中间件，默认强制活跃状态，提供显式活跃状态防护和主机防护。 |
| `migration` | 用于租户列和租户感知唯一索引的 DDL 与 seed 语句规划器。 |
| `cache` | 租户作用域缓存接口、键构建器、包装器、内存适配器、有界内存适配器和 Redis 适配器。 |
| `rpc` | 与框架无关的租户元数据载体。 |
| `rpc/grpc` | 默认强制活跃状态的 gRPC unary 和 stream 租户拦截器。 |
| `obs` | 租户可观测性字段、脱敏、`slog` 辅助函数和 OpenTelemetry API 辅助函数。 |

## 业务模块

| 包 | 用途 |
|---|---|
| `biz/identity` | 用于已验证外部身份断言的认证后租户用户映射，提供供应商元数据预设、内存存储和 `database/sql` SQLStore。 |
| `biz/identity/oidc` | OIDC 授权码桥接，支持 PKCE、state、nonce、ID Token 验证、可选 userinfo、内存/SQL 一次性登录状态存储和断言输出。 |
| `biz/user` | 用户和租户成员，提供内存实现和 `database/sql` SQLStore。 |
| `biz/rbac` | 租户作用域角色、`Role.HasPermission`、权限检查、内存 Enforcer 和 `database/sql` SQLStore。 |
| `biz/audit` | 租户作用域审计事件存储，提供内存实现和 `database/sql` SQLStore。 |
| `biz/notification` | 租户作用域通知接口、内存 notifier、SMTP 邮件 notifier、Amazon SES API v2 notifier、Resend 邮件 API notifier、支持可选 HMAC 签名的 HTTP webhook notifier、渠道路由器、顺序扇出、重试和超时装饰器。 |

## 示例

请参阅 [`examples/quickstart`](../examples/quickstart) 中可编译的 GORM DryRun 示例。
