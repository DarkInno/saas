# Security

## Tenant Context

- Tenant operations use `core/context.WithTenant`.
- Host operations use `core/context.WithHost`.
- Long-lived jobs should store tenant metadata and rebuild context explicitly.

## GORM Guardrails

- Query, update, delete, row, and count paths add `tenant_id = ?`.
- Create and bulk create fill tenant ID and reject mismatched tenant values.
- `Unscoped` panics in tenant context.
- Raw SQL is rejected in tenant context. `SafeRaw` and `SafeExec` require a context created with `core/context.WithHost`.
- Preload scopes are augmented with tenant filtering.

## Ent And sqlx Guardrails

- Ent integrations expose query filters, mutation filters, and mutation hooks.
- Ent create mutations set `tenant_id` from context and reject mismatched tenant values.
- Ent update and delete mutations receive storage-level tenant predicates.
- sqlx APIs only rewrite simple single-table SELECT/UPDATE/DELETE statements. Complex SQL such as joins, ordering, limits, returning clauses, comments, or multiple statements is rejected with `ErrUnsafeSQL`.

## Active Tenant Enforcement

- HTTP, Gin, Echo, Fiber, Kratos, and gRPC tenant middleware reject non-active tenants by default.
- Active-status guards are also available for trusted contexts created outside middleware.

## Post-Auth Identity Mapping

- `biz/identity` is not a complete OAuth/SSO implementation. It accepts only already verified provider assertions; OAuth/OIDC callback handling, token validation, magic-link delivery, and SAML XML validation stay in the application or IdP SDK layer.
- Identity providers must be explicitly allow-listed before an assertion is accepted.
- Email verification is required by default. Disable it only when the upstream IdP assertion is otherwise trusted, such as a controlled SAML connection.
- External subjects are linked by tenant, provider, and subject to prevent cross-tenant identity reuse from bypassing membership checks.
- Existing users are matched only when the assertion email equals the stored user email, and existing tenant member roles are not overwritten during sign-in.
- Generated user IDs are stable opaque hashes of provider and subject, avoiding raw provider subject leakage in normal user IDs.

## Cache Isolation

- Tenant cache keys are prefixed as `t:{tenant_id}:`.
- User-provided keys that already include tenant or global prefixes are rejected.
- Host global keys require explicit opt-in.
- In-memory cache adapters include bounded constructors.

## Error And Log Hygiene

- `web/gin.ErrorHandler` returns generic client errors.
- Web adapters return generic tenant error codes.
- gRPC interceptors return status errors with generic messages.
- `obs.Redact` masks common secret fields before log emission.
- Tenant IDs are emitted as structured observability fields, not embedded into error strings by framework adapters.
