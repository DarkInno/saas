# Changelog

All notable changes to GoTenancy are documented in this file.

## Unreleased

- Added SaaS SQL stores for plans, subscriptions, feature flags, and quota usage, including safe table-name validation and MySQL/SQLite/PostgreSQL placeholder rendering.
- Added plan and subscription Store/List APIs plus Store-oriented memory constructors while keeping existing service constructors.
- Added `Role.HasPermission`, RBAC `Enforcer`, `MemoryEnforcer`, and nil-store guards for quota services.

## v0.1.5 - 2026-07-09

- Added subscription renewal, direct expiration, grace-period-based `ExpireDue` scans, and current-period tracking.
- Added `saas/onboarding` to coordinate tenant creation, plan validation, subscription creation, feature/quota initialization, audit records, welcome notifications, and activation.
- Added `biz/identity` as a post-auth identity mapping layer with provider metadata presets and verified identity-to-tenant user/member linking.
- Added `biz/identity/oidc` for standard OIDC authorization-code callback processing, PKCE, ID-token verification, optional userinfo loading, one-time login state storage, and assertion output.
- Fixed OIDC authorization URL generation so caller options cannot override nonce or PKCE security parameters.
- Fixed OIDC memory login store capacity handling so expired logins are reclaimed before returning `ErrLoginStoreFull`.

## v0.1.4 - 2026-07-09

- Restored ORM, framework, gRPC, and example packages to the main Go module so `go get github.com/DarkInno/gotenancy` provides the full package surface.
- Reverted CI and lint verification to root-module checks while keeping SQLStore database integration tests outside the default gate.
- Clarified that future extension capabilities, not the core adoption adapters, are the right split boundary.

## v0.1.3 - 2026-07-09

- Split ORM, framework, gRPC, and example packages into separate Go modules so the root module no longer pulls adapter dependencies.
- Kept database integration checks outside the default CI gate.
- Added CI smoke coverage for quickstart, Gin + GORM, gRPC, and Ent examples.
- Added regression tests for HTTP, Echo, Fiber, Kratos, and gRPC tenant guards and stream/request context behavior.
- Added this changelog for release traceability.

## v0.1.2 - 2026-07-09

- Added runnable adoption examples for quickstart, Gin + GORM, gRPC, and Ent.
- Expanded README guidance with install, package overview, verification, and integration examples.
- Added package documentation comments for public example, web, SaaS, data, RPC, observability, and business modules.

## v0.1.1 - 2026-07-08

- Replaced the license file with the standard Apache License 2.0 text.

## v0.1.0 - 2026-07-08

- Published the initial GoTenancy module.
- Added shared-database tenant context, tenant resolution, stores, GORM/Ent/sqlx data guards, web and gRPC middleware, SaaS modules, cache isolation, migration planning, observability helpers, and security tests.
- Added CI coverage for the minimum supported Go version and the latest Go toolchain line.
