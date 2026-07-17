# API Reference

[EN](api.md) | [中文](api.zh-CN.md)

Public package overview.

## Core

| Package | Purpose |
|---|---|
| `core/types` | Tenant and deployment-unit IDs, metadata, lifecycle statuses, and host/tenant side constants. |
| `core/context` | `WithTenant`, `WithTenantDeployment`, `DeploymentFromContext`, `FromContext`, `WithHost`, `IsHost`, `Detach`, and `Switch`. |
| `core/resolver` | Header, cookie, query, domain, token-claim, and composite HTTP tenant resolvers. |
| `core/store` | Tenant metadata store interface, paginated list filters, memory store, TTL/bounded cache, cached decorator, and `database/sql` store. |

## Data Isolation

SaaS supports one topology: a shared application database and shared tables, with a required `tenant_id` on every tenant-owned row. It does not route tenant connections or switch schemas, so database-per-tenant, schema-per-tenant, and hybrid models are outside this module's contract.

| Package | Purpose |
|---|---|
| `data` | ORM-independent parameterized tenant filter conditions. |
| `data/gorm` | GORM plugin, tenant callbacks, `TenantScope`, host-only `SafeRaw`/`SafeExec`, `BulkCreate`, hard-delete guard, and MySQL soft-delete index planning. |
| `data/ent` | Ent selector predicates, query filters, mutation filters, and hooks that inject tenant and optional soft-delete filters. |
| `data/sqlx` | sqlx-compatible APIs for simple single-table SELECT/UPDATE/DELETE statements; complex SQL is rejected with `ErrUnsafeSQL`. |

## SaaS

| Package | Purpose |
|---|---|
| `tenant` | Tenant lifecycle manager with create, activate, suspend, restore, soft-delete, and host-only hard-delete. |
| `plan` | Plan, feature, and quota metadata with Store, memory implementation, list filters, and `database/sql` SQLStore. |
| `subscription` | Subscription lifecycle with active/cancelled/expired states, renewal, grace-period expiration scans, billing hook, Store, memory implementation, and `database/sql` SQLStore. |
| `quota` | Quota checking, atomic consuming, reset, memory implementation, nil-store guards, and `database/sql` SQLStore. |
| `feature` | Plan default features plus tenant override resolution with memory implementation and `database/sql` SQLStore. |
| `deployment` | Logical tenant-to-deployment-unit directory with memory and `database/sql` stores, host-defined policy/audit hooks, and controlled moves; it never performs physical routing or data movement. |
| `onboarding` | Cross-module tenant onboarding that creates a tenant, validates the plan, optionally assigns a deployment unit, creates the subscription, initializes features and quotas, records audit metadata, sends an optional welcome notification, and activates the tenant. |

## Integration

| Package | Purpose |
|---|---|
| `web/gin` | Gin tenant middleware with default active-status enforcement, explicit active-status guard, host guard, and generic error handler. |
| `web/echo` | Echo tenant middleware with default active-status enforcement, explicit active-status guard, and host guard. |
| `web/fiber` | Fiber tenant middleware with default active-status enforcement, explicit active-status guard, and host guard. |
| `web/kratos` | Kratos tenant middleware with default active-status enforcement, explicit active-status guard, and host guard. |
| `web/http` | Standard-library HTTP tenant middleware with default active-status enforcement, explicit active-status guard, and host guard. |
| `migration` | DDL and seed statement planner for tenant columns and tenant-aware unique indexes. |
| `cache` | Tenant-scoped cache interface, key builder, wrapper, memory adapter, bounded memory adapter, and Redis adapter. |
| `rpc` | Framework-neutral tenant metadata carriers. |
| `rpc/mq` | SDK-free NATS, RabbitMQ, and Kafka message-header carrier interfaces and adapters for `rpc.InjectTenant` and `rpc.ExtractTenant`; broker clients remain host-owned. |
| `rpc/grpc` | gRPC unary and stream tenant interceptors with default active-status enforcement. |
| `obs` | Tenant and deployment-unit-ID observability fields, redaction, `slog` helpers, and OpenTelemetry API helpers. |

All web adapters and gRPC interceptors accept an optional
`WithDeploymentResolver`. When configured, it resolves a tenant's logical
deployment unit after active-tenant lookup and attaches it to the request
context; failures are returned as generic deployment-unavailable denials.

## Business Modules

| Package | Purpose |
|---|---|
| `biz/identity` | Post-auth tenant user mapping for verified external identity assertions, provider metadata presets, memory store, and `database/sql` SQLStore. |
| `biz/identity/oidc` | OIDC authorization-code bridge with PKCE, state, nonce, ID-token verification, optional userinfo, memory/SQL one-time login state storage, and assertion output. |
| `biz/user` | Users and tenant members with memory implementation and `database/sql` SQLStore. |
| `biz/rbac` | Tenant-scoped roles, `Role.HasPermission`, permission checks, memory Enforcer, and `database/sql` SQLStore. |
| `biz/audit` | Tenant-scoped audit event store with memory implementation and `database/sql` SQLStore. |
| `biz/commission` | Optional tenant-scoped commission domain with versioned platform templates, constrained tenant programs, tier calculation, immutable decision/earning/journal/outbox records, and host-adapted settlement state. Every Service command is authorized; provider submission distinguishes pending from verified terminal settlement outcomes. It provides memory and `database/sql` stores but never executes migrations or payouts. |
| `biz/notification` | Tenant-scoped notification interface, memory notifier, SMTP email notifier, Amazon SES API v2 notifier, Resend email API notifier, HTTP webhook notifier with optional HMAC signing, channel router, sequential fanout, retry, and timeout decorators. |

## Example

See [`examples/quickstart`](../examples/quickstart) for a compiling GORM DryRun example.
