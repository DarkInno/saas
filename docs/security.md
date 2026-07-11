# Security

## Tenant Context

- Tenant operations use `core/context.WithTenant`.
- Host operations use `core/context.WithHost`.
- Long-lived jobs should store tenant metadata and rebuild context explicitly.

## GORM Guardrails

- Query, update, delete, row, and count paths add `tenant_id = ?`.
- Create and bulk create fill tenant ID and reject mismatched tenant values.
- `Unscoped` reports an error in tenant context.
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

## Identity And OIDC

- `biz/identity` accepts only already verified provider assertions.
- `biz/identity/oidc` handles standard OIDC authorization-code callbacks, token exchange, ID-token verification, nonce checks, PKCE verifier use, form-post callback parsing, and optional one-time login state storage. Application sessions, Magic Link delivery, SAML XML validation, MFA, and WebAuthn stay outside this package.
- Applications can pass the expected `State`, `Nonce`, and `PKCEVerifier` directly, or use `LoginStore` / `MemoryLoginStore` to consume pending login state exactly once.
- `MemoryLoginStore` is bounded and TTL-based, but it is process-local; horizontally scaled applications should implement `LoginStore` with their secure session or shared cache layer.
- ID tokens are verified against provider keys, issuer, audience, expiry, nonce, and `at_hash` when present.
- Provider endpoints and redirect URLs must use HTTPS unless they are loopback HTTP URLs for local development.
- Identity providers must be explicitly allow-listed before an assertion is accepted.
- OIDC issuer validation is strict; avoid multi-tenant issuer shortcuts such as Microsoft `common` unless the application owns the issuer policy explicitly.
- Email verification is required by default. Disable it only when the upstream IdP assertion is otherwise trusted, such as a controlled SAML connection.
- External subjects are linked by tenant, provider, and subject to prevent cross-tenant identity reuse from bypassing membership checks.
- Existing users are matched only when the assertion email equals the stored user email, and existing tenant member roles are not overwritten during sign-in.
- Generated user IDs are stable opaque hashes of provider and subject, avoiding raw provider subject leakage in normal user IDs.

## Cache Isolation

- Tenant cache keys use `t2:{base64url(tenant_id)}:{key}`. The unpadded Base64URL tenant component makes the tenant/key boundary unambiguous, including when either value contains colons.
- The versioned `t2:` namespace prevents ambiguous legacy `t:` entries from overlapping new tenant keys. User-provided keys that already include either tenant prefix or the global prefix are rejected.
- Host global keys remain `g:{key}` and require explicit opt-in.
- In-memory cache adapters include bounded constructors.
- The Redis adapter stores only exact keys produced by the cache layer and does not use broad key scans; wrap it with `TenantCache` for tenant isolation.
- Production Redis clients should be configured through `go-redis` options with TLS, command timeouts, retry limits, and OpenTelemetry instrumentation appropriate to the deployment.

## Error And Log Hygiene

- `web/gin.ErrorHandler` returns generic client errors.
- Web adapters return generic tenant error codes.
- gRPC interceptors return status errors with generic messages.
- `obs.Redact` masks common secret fields before log emission.
- `obs.RedactSlogAttrs` redacts sensitive `slog` attributes, including nested groups, before structured log emission.
- `obs.RecordSpanError` records sanitized OpenTelemetry error events with the original error type and a caller-provided or generic status description, avoiding accidental tenant or secret leakage through telemetry messages.
- Tenant IDs are emitted as structured observability fields, not embedded into error strings by framework adapters.
- `biz/notification.SESNotifier` uses the official AWS SDK v2 `sesv2.SendEmail` client path instead of hand-written SigV4 signing, maps explicit message tags to SES tags, returns safe delivery errors, and treats throttling/server faults as retryable.
- `biz/notification.ResendNotifier` uses Resend's HTTPS email API with bearer authentication, a required `User-Agent`, optional idempotency key, safe status errors, and retry classification for `429` and `5xx` responses.
- `biz/notification.WebhookNotifier` requires HTTPS unless the endpoint is loopback or insecure HTTP is explicitly allowed, rejects URL userinfo, emits JSON payloads, supports HMAC signatures, and does not include provider response bodies in error strings.
