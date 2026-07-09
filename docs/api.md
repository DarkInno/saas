# API Reference

Public package overview. The root module contains core, SaaS, sqlx-style, cache, migration, observability, and framework-neutral RPC APIs. ORM and framework integrations are split into their own modules.

## Core

| Package | Purpose |
|---|---|
| `core/types` | Tenant IDs, tenant metadata, lifecycle statuses, and host/tenant side constants. |
| `core/context` | `WithTenant`, `FromContext`, `WithHost`, `IsHost`, `Detach`, and `Switch`. |
| `core/resolver` | Header, cookie, query, domain, token-claim, and composite HTTP tenant resolvers. |
| `core/store` | Tenant metadata store interface, paginated list filters, memory store, TTL/bounded cache, cached decorator, and `database/sql` store. |

## Data Isolation

| Package | Purpose |
|---|---|
| `data` | ORM-independent parameterized tenant filter conditions. |
| `data/gorm` | Separate module for the GORM plugin, tenant callbacks, `TenantScope`, host-only `SafeRaw`/`SafeExec`, `BulkCreate`, hard-delete guard, and MySQL soft-delete index planning. |
| `data/ent` | Separate module for Ent selector predicates, query filters, mutation filters, and hooks that inject tenant and optional soft-delete filters. |
| `data/sqlx` | sqlx-compatible APIs for simple single-table SELECT/UPDATE/DELETE statements; complex SQL is rejected with `ErrUnsafeSQL`. |

## SaaS

| Package | Purpose |
|---|---|
| `saas/tenant` | Tenant lifecycle manager with create, activate, suspend, restore, soft-delete, and host-only hard-delete. |
| `saas/plan` | Plan, feature, quota metadata and in-memory CRUD service. |
| `saas/subscription` | Subscription lifecycle and billing hook. |
| `saas/quota` | In-memory quota checking, atomic consuming, and reset. |
| `saas/feature` | Plan default features plus tenant override resolution. |

## Integration

| Package | Purpose |
|---|---|
| `web/gin` | Separate module for Gin tenant middleware with default active-status enforcement, explicit active-status guard, host guard, and generic error handler. |
| `web/echo` | Separate module for Echo tenant middleware with default active-status enforcement, explicit active-status guard, and host guard. |
| `web/fiber` | Separate module for Fiber tenant middleware with default active-status enforcement, explicit active-status guard, and host guard. |
| `web/kratos` | Separate module for Kratos tenant middleware with default active-status enforcement, explicit active-status guard, and host guard. |
| `web/http` | Standard-library HTTP tenant middleware with default active-status enforcement, explicit active-status guard, and host guard. |
| `migration` | DDL and seed statement planner for tenant columns and tenant-aware unique indexes. |
| `cache` | Tenant-scoped cache interface, key builder, wrapper, memory adapter, and bounded memory adapter. |
| `rpc` | Framework-neutral tenant metadata carriers. |
| `rpc/grpc` | Separate module for gRPC unary and stream tenant interceptors with default active-status enforcement. |
| `obs` | Tenant observability fields and redaction. |

## Business Modules

| Package | Purpose |
|---|---|
| `biz/user` | Users and tenant members. |
| `biz/rbac` | Tenant-scoped roles and permission checks. |
| `biz/audit` | Tenant-scoped audit event store. |
| `biz/notification` | Tenant-scoped notification interface and memory notifier. |

## Example

See [`examples/quickstart`](../examples/quickstart) for a compiling GORM DryRun example.
