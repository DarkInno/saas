# Architecture

[EN](architecture.md) | [中文](architecture.zh-CN.md)

SaaS is a library assembled into a host Go application; it does not run an
HTTP/gRPC service or own a deployment on its own. This diagram shows the
integration boundaries implemented by the module and the normal tenant-scoped
request path. The storage and external-system nodes are selected and configured
by the host; they are supported integration points, not services deployed by
this repository.

```mermaid
flowchart TB
    caller["HTTP/gRPC clients<br/>CLI and background jobs"]

    subgraph host["Host Go application"]
        web["HTTP integration<br/>web/http, Gin, Echo, Fiber, Kratos"]
        grpc["gRPC integration<br/>rpc/grpc interceptors"]
        direct["Direct invocation<br/>workers, CLI, application services"]
        app["Handlers and application services<br/>propagate context.Context"]
        modules["SaaS and business modules<br/>tenant, plan, subscription, quota, feature, onboarding, and biz/*"]
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
        stores["Memory or host-provided database/sql stores<br/>core, lifecycle modules, and biz"]
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

## Tenant-scoped request path

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

## Boundary rules

- HTTP and gRPC integrations resolve a tenant, load its metadata, and require
  it to be active before handing control to the host application.
- `context.Context` is the scope carrier. Background work must establish a
  tenant context explicitly; host-wide work must use the deliberate
  `core/context.WithHost` path.
- The GORM, Ent, and sqlx adapters derive their data boundary from that context.
  In the shared-database model, tenant-owned rows carry `tenant_id`.
- Stores can be in-memory or use a host-provided SQL connection. Redis is an
  optional host-provided cache adapter, not the source of tenant isolation.
- `migration.Planner` generates tenant-aware DDL and seed statements; it never
  executes migrations.

See [API Reference](api.md) for the package-level surface and
[Security](security.md) for the detailed guardrail behavior.
