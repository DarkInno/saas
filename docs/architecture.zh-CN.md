# 架构

[EN](architecture.md) | [中文](architecture.zh-CN.md)

SaaS 是一个组装到宿主 Go 应用中的库；它本身不运行 HTTP/gRPC 服务，也不独立拥有部署。本图展示了该模块实现的集成边界以及常规的租户作用域请求路径。存储和外部系统节点由宿主选择和配置；它们是受支持的集成点，而不是本仓库部署的服务。

```mermaid
flowchart TB
    caller["HTTP/gRPC clients<br/>CLI and background jobs"]

    subgraph host["Host Go application"]
        web["HTTP integration<br/>web/http, Gin, Echo, Fiber, Kratos"]
        grpc["gRPC integration<br/>rpc/grpc interceptors"]
        direct["Direct invocation<br/>workers, CLI, application services"]
        app["Handlers and application services<br/>propagate context.Context"]
        modules["SaaS and business modules<br/>tenant、plan、subscription、quota、feature、onboarding 和 biz/*"]
        telemetry["Observability helpers<br/>obs; host configures exporters"]
    end

    subgraph boundary["Tenant boundary"]
        resolver["HTTP tenant resolver<br/>header, cookie, query, domain, token"]
        carrier["RPC metadata carrier<br/>rpc"]
        tenantStore["Tenant metadata store<br/>core/store.Store"]
        tenantContext["Tenant or explicit host context<br/>core/context"]
        tenantTypes["Tenant IDs, metadata, and state<br/>core/types"]
    end

    subgraph isolation["Tenant isolation and cache"]
        dataFilter["Context-derived data filter<br/>data"]
        gorm["GORM callbacks and guards<br/>data/gorm"]
        ent["Ent predicates, filters, and hooks<br/>data/ent"]
        sqlx["Safe simple-statement filtering<br/>data/sqlx"]
        cache["Tenant-scoped cache keys<br/>cache.TenantCache"]
        migration["Tenant column and index DDL planner<br/>migration"]
    end

    subgraph adapters["Host-selected storage and external adapters"]
        stores["Memory or host-provided database/sql stores<br/>core、生命周期模块和 biz"]
        database["Host-managed shared application database<br/>tenant-owned rows include tenant_id"]
        redis["Host-provided optional Redis cache"]
        idp["OIDC identity provider"]
        delivery["SMTP, SES, Resend, or webhook"]
    end

    caller --> web
    caller --> grpc
    caller --> direct
    web --> resolver
    grpc --> carrier
    resolver --> tenantStore
    carrier --> tenantStore
    direct --> tenantContext
    tenantStore --> tenantContext
    tenantTypes -. "defines" .-> resolver
    tenantTypes -. "defines" .-> tenantStore
    tenantTypes -. "defines" .-> tenantContext
    tenantContext --> app
    app --> modules
    app --> telemetry
    tenantContext --> dataFilter
    app -. "selected adapter" .-> gorm
    app -. "selected adapter" .-> ent
    app -. "selected adapter" .-> sqlx
    dataFilter -. "used by" .-> gorm
    dataFilter -. "used by" .-> ent
    dataFilter -. "used by" .-> sqlx
    app -. "optional" .-> cache
    tenantContext -. "scopes" .-> cache
    modules --> stores
    tenantStore --> stores
    stores --> database
    gorm --> database
    ent --> database
    sqlx --> database
    cache --> redis
    migration -. "plans; does not execute" .-> database
    modules -. "optional OIDC bridge" .-> idp
    modules -. "optional notifications" .-> delivery
```

## 租户作用域请求路径

```mermaid
flowchart LR
    inbound["Incoming HTTP request or gRPC metadata"]
    resolve["Resolve tenant ID"]
    lookup{"Tenant exists<br/>and is active?"}
    reject["Reject with tenant_required,<br/>tenant_forbidden, or tenant_inactive"]
    scoped["Attach tenant to context.Context"]
    handler["Host handler or service"]
    guard["Data adapter or cache wrapper"]
    protected["tenant_id predicate or<br/>tenant-prefixed cache key"]

    inbound --> resolve --> lookup
    lookup -->|no| reject
    lookup -->|yes| scoped --> handler --> guard --> protected
```

## 边界规则

- HTTP 和 gRPC 集成会解析租户、加载其元数据，并要求租户处于活跃状态，之后才将控制权交给宿主应用。
- `context.Context` 是作用域载体。后台任务必须显式建立租户上下文；全局主机操作必须使用有意为之的 `core/context.WithHost` 路径。
- GORM、Ent 和 sqlx 适配器从该上下文派生数据边界。在共享数据库模型中，租户所有的行都带有 `tenant_id`。
- 存储可以使用内存实现，也可以使用宿主提供的 SQL 连接。Redis 是可选的、由宿主提供的缓存适配器，而不是租户隔离的来源。
- `migration.Planner` 生成租户感知的 DDL 和 seed 语句；它从不执行迁移。

有关包级接口，请参阅 [API 参考](api.zh-CN.md)；有关详细的防护行为，请参阅[安全性](security.zh-CN.md)。
