# GoTenancy

[![Go Reference](https://pkg.go.dev/badge/github.com/DarkInno/gotenancy.svg)](https://pkg.go.dev/github.com/DarkInno/gotenancy)
[![CI](https://github.com/DarkInno/gotenancy/actions/workflows/ci.yml/badge.svg)](https://github.com/DarkInno/gotenancy/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

GoTenancy is an ORM-independent Go toolkit for shared-database multi-tenancy with a required `tenant_id` boundary.

It provides tenant context, tenant resolution, data guards, web/RPC middleware, tenant metadata storage, and common SaaS modules. The default model is simple: every tenant-owned row carries `tenant_id`, and adapters derive the active tenant from `context.Context`.

## Scope

- Shared-database isolation with a required `tenant_id` boundary.
- Host-wide access only through explicit host context.
- Root module APIs for tenant context, tenant resolution, sqlx-style filtering, stores, cache, migration planning, SaaS helpers, and framework-neutral RPC carriers.
- Split adapter modules for GORM, Ent, Gin, Echo, Fiber, Kratos, and gRPC.
- Tenant lifecycle, plans, subscriptions, quotas, features, RBAC, audit, users, and notifications.

Independent database and hybrid isolation models are not implemented.

## Requirements

- Go `1.23+`.

## Install

```bash
go mod init your-app
go get github.com/DarkInno/gotenancy
```

Adapters are separate modules so core users do not pull framework or ORM dependencies:

```bash
go get github.com/DarkInno/gotenancy/data/gorm
go get github.com/DarkInno/gotenancy/data/ent
go get github.com/DarkInno/gotenancy/web/gin
go get github.com/DarkInno/gotenancy/rpc/grpc
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
(cd examples/quickstart && go run .)
(cd examples/gin-gorm && go run .)
(cd examples/grpc && go run .)
(cd examples/ent && go run .)
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
- `data/gorm`: separate module for the GORM plugin, guard suite, host-only `SafeRaw`/`SafeExec`, `BulkCreate`, and delete APIs.
- `data/ent`: separate module for Ent selector predicate, query filter, mutation filter, and hook APIs.
- `data/sqlx`: tenant-filtered APIs for simple single-table SELECT/UPDATE/DELETE statements.
- `saas/tenant`: tenant lifecycle state machine.
- `saas/plan`: plan CRUD.
- `saas/subscription`: subscription lifecycle and billing hook.
- `saas/quota`: quota checking and atomic consumption.
- `saas/feature`: plan defaults plus tenant-level feature overrides.
- `web/http`: root module middleware for the standard library.
- `web/gin`, `web/echo`, `web/fiber`, `web/kratos`: separate framework adapter modules.
- `rpc/grpc`: separate module for gRPC unary and stream tenant interceptors.
- `migration`: tenant column and index planning.
- `cache`: tenant-scoped cache wrapper and memory adapters.
- `obs`: observability fields and redaction.
- `biz/*`: user, RBAC, audit, and notification modules.

## Verification

```bash
go test ./...
go vet ./...
go test -race ./...

for module in data/gorm data/ent web/gin web/echo web/fiber web/kratos rpc/grpc; do
  (cd "$module" && go test ./... && go vet ./...)
done
```

On Windows, `go test -race` requires cgo and a C compiler. Without local cgo, run race tests in Docker:

```bash
docker run --rm -v "${PWD}:/workspace" -w /workspace -e CGO_ENABLED=1 -e GOFLAGS=-mod=readonly golang:1.23 go test -race ./...
```

## Project Layout

```text
core/          Tenant context, resolver, store, and types
data/          Data filtering contracts plus split GORM and Ent adapter modules
saas/          Tenant lifecycle, plan, subscription, quota, and feature modules
web/           Standard-library middleware plus split framework adapter modules
migration/     Tenant schema migration planning
cache/         Tenant-aware cache abstractions
rpc/           RPC metadata propagation plus split gRPC adapter module
obs/           Observability fields and redaction
biz/           User, RBAC, audit, and notification modules
examples/      Runnable example modules
tests/         Cache, concurrency, and local-only DB integration modules
docs/          API, security, and compatibility notes
```

## Compatibility

See [docs/compatibility.md](docs/compatibility.md).

## License

[Apache License 2.0](LICENSE)
