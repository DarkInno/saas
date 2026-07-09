# Changelog

All notable changes to GoTenancy are documented in this file.

## Unreleased

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
