# GoTenancy

[EN](README.md) | [中文](README.zh-CN.md)

[![Go Reference](https://pkg.go.dev/badge/github.com/DarkInno/gotenancy.svg)](https://pkg.go.dev/github.com/DarkInno/gotenancy)
[![CI](https://github.com/DarkInno/gotenancy/actions/workflows/ci.yml/badge.svg)](https://github.com/DarkInno/gotenancy/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

GoTenancy is an ORM-independent Go toolkit for shared-database multi-tenancy with a required `tenant_id` boundary.

It provides tenant context, tenant resolution, data guards, web/RPC middleware, tenant metadata storage, and common SaaS modules. The default model is simple: every tenant-owned row carries `tenant_id`, and adapters derive the active tenant from `context.Context`.

## Scope

- Shared-database isolation with a required `tenant_id` boundary.
- Host-wide access only through explicit host context.
- GORM, Ent, and sqlx adapters for tenant-aware data access.
- HTTP, Gin, Echo, Fiber, Kratos, and gRPC middleware.
- Tenant lifecycle, plans, subscriptions, quotas, features, post-auth identity links, RBAC, audit, users, and notifications.

Independent database and hybrid isolation models are not implemented.
Future optional extension capabilities can live in separate modules, but the core adoption adapters ship with the main module.

## Architecture

See [docs/architecture.md](docs/architecture.md) for the host-integration,
tenant-boundary, data-isolation, and external-adapter diagram.

## Requirements

- Go `1.24+`.

## Install

```bash
go mod init your-app
go get github.com/DarkInno/gotenancy
```

## Complete Example

This copy-paste example creates an in-memory tenant, installs the GORM plugin, runs a tenant-scoped query in GORM DryRun mode, and prints the generated SQL. It does not require a live database.

```go
package main

import (
	"context"
	"fmt"
	"log"

	tenantctx "github.com/DarkInno/gotenancy/core/context"
	"github.com/DarkInno/gotenancy/core/store"
	"github.com/DarkInno/gotenancy/core/types"
	gormtenant "github.com/DarkInno/gotenancy/data/gorm"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Order struct {
	ID       uint
	TenantID string `gorm:"column:tenant_id"`
	Number   string `gorm:"column:number"`
}

func main() {
	ctx := context.Background()
	tenants := store.NewMemoryStore()
	if err := tenants.Create(ctx, types.Tenant{
		ID:     "tenant-a",
		Name:   "Tenant A",
		Status: types.TenantStatusActive,
	}); err != nil {
		log.Fatal(err)
	}

	tenant, err := tenants.Get(ctx, "tenant-a")
	if err != nil {
		log.Fatal(err)
	}
	ctx = tenantctx.WithTenant(ctx, tenant)

	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "user:pass@tcp(localhost:3306)/app?parseTime=true",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		DryRun:                 true,
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Use(gormtenant.New(gormtenant.Config{})); err != nil {
		log.Fatal(err)
	}

	var orders []Order
	result := db.WithContext(ctx).Find(&orders)
	if result.Error != nil {
		log.Fatal(result.Error)
	}

	fmt.Println(result.Statement.SQL.String())
	fmt.Println(result.Statement.Vars)
}
```

## Adoption Examples

Run the examples from the repository root:

```bash
go run ./examples/quickstart
go run ./examples/gin-gorm
go run ./examples/grpc
go run ./examples/ent
```

- [examples/quickstart](examples/quickstart): minimal GORM create flow.
- [examples/gin-gorm](examples/gin-gorm): Gin header resolver, tenant store validation, request context injection, and GORM query guard.
- [examples/grpc](examples/grpc): unary gRPC interceptor that resolves tenant metadata and injects tenant context.
- [examples/ent](examples/ent): Ent query and mutation filters using the storage-level interfaces generated builders expose.

## Common Patterns

Register the GORM plugin once on startup:

```go
if err := db.Use(gormtenant.New(gormtenant.Config{})); err != nil {
	log.Fatal(err)
}
```

Resolve tenants at the edge, then pass `context.Context` through application and data layers:

```go
tenantResolver := resolver.NewComposite(
	resolver.NewHeaderContrib("", types.TenantIDStrategyString),
)
router.Use(gingotenancy.TenantMiddleware(tenantResolver, tenants))
```

Filter Ent queries before execution:

```go
query := client.Order.Query()
if err := enttenant.FilterQuery(ctx, query, enttenant.Config{}); err != nil {
	return err
}
orders, err := query.All(ctx)
```

Register the Ent mutation hook with generated clients:

```go
client.Use(enttenant.Hook(enttenant.Config{}))
```

Protect gRPC handlers with tenant metadata:

```go
server := grpc.NewServer(
	grpc.UnaryInterceptor(grpcgotenancy.TenantUnaryServerInterceptor(tenants)),
)
```

Use explicit host context for host-wide operations:

```go
ctx := tenantctx.WithHost(context.Background())
```

## Packages

- `core/types`: tenant IDs, tenant metadata, status, and side types.
- `core/context`: tenant and host context, detach, and switch.
- `core/resolver`: header, cookie, query, domain, token-claim, and composite resolvers.
- `core/store`: memory store, paginated list filters, memory cache, cached store decorator, and `database/sql` store.
- `data`: ORM-independent tenant filter condition.
- `data/gorm`: GORM plugin, guard suite, host-only `SafeRaw`/`SafeExec`, `BulkCreate`, and delete APIs.
- `data/ent`: Ent selector predicate, query filter, mutation filter, and hook APIs.
- `data/sqlx`: tenant-filtered APIs for simple single-table SELECT/UPDATE/DELETE statements.
- `saas/tenant`: tenant lifecycle state machine.
- `saas/plan`: plan Store, memory implementation, and `database/sql` SQLStore.
- `saas/subscription`: subscription lifecycle, renewal, expiration, grace-period handling, billing hook, Store, memory implementation, and `database/sql` SQLStore.
- `saas/quota`: quota checking, atomic consumption, reset, memory implementation, and `database/sql` SQLStore.
- `saas/feature`: plan defaults, tenant-level feature overrides, memory implementation, and `database/sql` SQLStore.
- `saas/onboarding`: tenant onboarding flow across tenant, plan, subscription, feature, quota, audit, and notification services.
- `biz/identity`: post-auth tenant user mapping for verified external identity assertions, with memory and `database/sql` stores.
- `biz/identity/oidc`: OIDC authorization-code bridge with PKCE, state, nonce, ID-token verification, one-time login state storage, SQL-backed login state storage, and assertion output.
- `web/*`: tenant middleware and guards for net/http, Gin, Echo, Fiber, and Kratos.
- `rpc/grpc`: gRPC unary and stream tenant interceptors.
- `migration`: tenant column and index planning.
- `cache`: tenant-scoped cache wrapper, memory adapters, and Redis adapter.
- `obs`: tenant observability fields, redaction, `slog` helpers, and OpenTelemetry API helpers.
- `biz/*`: identity, user, RBAC, audit, and notification modules with memory stores, SQL stores where persistence is part of the module contract, SMTP/SES/Resend/webhook delivery, channel routing, fanout, retry, and timeout helpers.

## Post-Auth Identity Mapping

`biz/identity` maps verified external identities into tenant users and memberships. `biz/identity/oidc` adds a standard OIDC authorization-code bridge for callback processing, one-time login state, and ID-token verification. It is still not a full IAM platform: application sessions, account screens, Magic Link delivery, SAML validation, MFA, and WebAuthn remain application or IdP responsibilities.

```go
oidcClient, err := oidc.New(ctx, oidc.Config{
	Provider:    identity.GoogleOIDC(),
	ClientID:    "client-id",
	RedirectURL: "https://app.example.com/auth/callback",
})

loginStore := oidc.NewMemoryLoginStore(10 * time.Minute)
login, err := oidcClient.BeginLogin(ctx, loginStore, oidc.LoginRequest{
	TenantID: "tenant-a",
	Roles:    []string{"member"},
})
result, err := oidcClient.HandleLoginCallback(ctx, request, loginStore)

users := user.NewMemoryService()
identityService := identity.NewService(users, identity.WithProviders(identity.GoogleOIDC()))
session, err := identityService.Authenticate(ctx, result.Assertion)
```

Redirect the browser to `login.URL`. `MemoryLoginStore` is process-local and consumes states once; multi-instance deployments should implement `LoginStore` on top of their secure session or shared cache layer.

## Verification

```bash
go test ./...
go vet ./...
go test -race ./...
```

On Windows, `go test -race` requires cgo and a C compiler. Without local cgo, run race tests in Docker:

```bash
docker run --rm -v "${PWD}:/workspace" -w /workspace -e CGO_ENABLED=1 -e GOFLAGS=-mod=readonly golang:1.24 go test -race ./...
```

### Disposable integration, coverage, and chaos

The PowerShell runners start a local disposable Docker Compose environment for MySQL, PostgreSQL, Redis, and (for chaos) Toxiproxy. They create and drop test tables and volumes, so never point their DSNs at shared or production services.

```powershell
# Windows PowerShell; PowerShell 7 users may substitute pwsh.
powershell.exe -NoProfile -ExecutionPolicy Bypass -File tests/run-integration.ps1

$profile = Join-Path $env:TEMP 'gotenancy-coverage.out'
go test -count=1 -covermode=atomic -coverpkg=./... "-coverprofile=$profile" ./...
powershell.exe -NoProfile -ExecutionPolicy Bypass -File tests/check-coverage.ps1 -Profile $profile -Minimum 65
Remove-Item -LiteralPath $profile, "$profile.txt" -Force -ErrorAction SilentlyContinue

powershell.exe -NoProfile -ExecutionPolicy Bypass -File tests/run-chaos.ps1
```

For production Redis, configure TLS, timeouts, retry limits, and OpenTelemetry on the `go-redis` client before passing it to `cache.NewRedis`.

## Project Layout

```text
core/          Tenant context, resolver, store, and types
data/          Data filtering contracts and adapters
saas/          Tenant lifecycle, plan, subscription, quota, feature, and onboarding modules
web/           Web framework and net/http integration
migration/     Tenant schema migration planning
cache/         Tenant-aware cache abstractions
rpc/           RPC metadata propagation
obs/           Observability fields, redaction, slog, and OpenTelemetry helpers
biz/           Identity, user, RBAC, audit, and notification modules
examples/      Runnable examples
tests/         Security, cache, concurrency, and local-only DB integration tests
docs/          API, security, and compatibility notes
```

## Compatibility

See [docs/compatibility.md](docs/compatibility.md).

## License

[Apache License 2.0](LICENSE)
