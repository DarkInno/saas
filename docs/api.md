# API Reference

Public package overview.

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
| `data/gorm` | GORM plugin, tenant callbacks, `TenantScope`, host-only `SafeRaw`/`SafeExec`, `BulkCreate`, hard-delete guard, and MySQL soft-delete index planning. |
| `data/ent` | Ent selector predicates, query filters, mutation filters, and hooks that inject tenant and optional soft-delete filters. |
| `data/sqlx` | sqlx-compatible APIs for simple single-table SELECT/UPDATE/DELETE statements; complex SQL is rejected with `ErrUnsafeSQL`. |

## SaaS

| Package | Purpose |
|---|---|
| `saas/tenant` | Tenant lifecycle manager with create, activate, suspend, restore, soft-delete, and host-only hard-delete. |
| `saas/plan` | Plan, feature, and quota metadata with Store, memory implementation, list filters, and `database/sql` SQLStore. |
| `saas/subscription` | Subscription lifecycle with active/cancelled/expired states, renewal, grace-period expiration scans, billing hook, Store, memory implementation, and `database/sql` SQLStore. |
| `saas/quota` | Quota checking, atomic consuming, reset, memory implementation, nil-store guards, and `database/sql` SQLStore. |
| `saas/feature` | Plan default features plus tenant override resolution with memory implementation and `database/sql` SQLStore. |
| `saas/onboarding` | Cross-module tenant onboarding that creates a tenant, validates the plan, creates the subscription, initializes features and quotas, records audit metadata, sends an optional welcome notification, and activates the tenant. |

## Integration

| Package | Purpose |
|---|---|
| `web/gin` | Gin tenant middleware with default active-status enforcement, explicit active-status guard, host guard, and generic error handler. |
| `web/echo` | Echo tenant middleware with default active-status enforcement, explicit active-status guard, and host guard. |
| `web/fiber` | Fiber tenant middleware with default active-status enforcement, explicit active-status guard, and host guard. |
| `web/kratos` | Kratos tenant middleware with default active-status enforcement, explicit active-status guard, and host guard. |
| `web/http` | Standard-library HTTP tenant middleware with default active-status enforcement, explicit active-status guard, and host guard. |
| `migration` | DDL and seed statement planner for tenant columns and tenant-aware unique indexes. |
| `cache` | Tenant-scoped cache interface, key builder, wrapper, memory adapter, and bounded memory adapter. |
| `rpc` | Framework-neutral tenant metadata carriers. |
| `rpc/grpc` | gRPC unary and stream tenant interceptors with default active-status enforcement. |
| `obs` | Tenant observability fields and redaction. |

## Business Modules

| Package | Purpose |
|---|---|
| `biz/identity` | Post-auth tenant user mapping for verified external identity assertions, provider metadata presets, memory store, and `database/sql` SQLStore. |
| `biz/identity/oidc` | OIDC authorization-code bridge with PKCE, state, nonce, ID-token verification, optional userinfo, memory/SQL one-time login state storage, and assertion output. |
| `biz/user` | Users and tenant members with memory implementation and `database/sql` SQLStore. |
| `biz/rbac` | Tenant-scoped roles, `Role.HasPermission`, permission checks, memory Enforcer, and `database/sql` SQLStore. |
| `biz/audit` | Tenant-scoped audit event store with memory implementation and `database/sql` SQLStore. |
| `biz/notification` | Tenant-scoped notification interface, memory notifier, and SMTP email notifier. |

## Example

See [`examples/quickstart`](../examples/quickstart) for a compiling GORM DryRun example.
