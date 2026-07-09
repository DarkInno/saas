# Compatibility

## Go

- Module language version: Go `1.23`.
- Root and adapter `go.mod` files record this as `go 1.23.0`.
- CI test jobs should cover Go `1.23.x` and Go `1.26.x`; lint runs on Go `1.26.x`.

Root and adapter modules should not require Go `1.24+` dependencies without an explicit compatibility decision.

## Isolation Model

GoTenancy supports shared-database isolation with a required `tenant_id` boundary.

Independent database and hybrid isolation models are not part of the current API.

## Adapters

| Adapter | Module | Dependency |
|---|---|---|
| GORM v2 | `github.com/DarkInno/gotenancy/data/gorm` | `gorm.io/gorm` v1.31.2 |
| Ent | `github.com/DarkInno/gotenancy/data/ent` | `entgo.io/ent` v0.14.1 |
| Gin | `github.com/DarkInno/gotenancy/web/gin` | `github.com/gin-gonic/gin` v1.9.1 |
| Echo | `github.com/DarkInno/gotenancy/web/echo` | `github.com/labstack/echo/v4` v4.13.4 |
| Fiber | `github.com/DarkInno/gotenancy/web/fiber` | `github.com/gofiber/fiber/v2` v2.52.13 |
| Kratos | `github.com/DarkInno/gotenancy/web/kratos` | `github.com/go-kratos/kratos/v2` v2.9.2 |
| gRPC | `github.com/DarkInno/gotenancy/rpc/grpc` | `google.golang.org/grpc` v1.75.1 |

The root module remains free of GORM, Ent, Gin, Echo, Fiber, Kratos, and gRPC imports.
`data/sqlx` stays in the root module because it only depends on small structural interfaces and `database/sql`.

## SQLStore

`core/store.SQLStore` supports:

- MySQL/SQLite placeholders: `?`
- PostgreSQL placeholders: `$1`, `$2`, ...

Use `WithSQLDialect(SQLDialectPostgres)` for PostgreSQL.

Optional integration tests use:

- `GOTENANCY_MYSQL_DSN`
- `GOTENANCY_POSTGRES_DSN`

Database service containers are not part of the default CI gate.

## Verification

```bash
go test ./...
go vet ./...
go test -race ./...
go list -m -f '{{.Path}} {{.GoVersion}}' all

for module in data/gorm data/ent web/gin web/echo web/fiber web/kratos rpc/grpc; do
  (cd "$module" && go test ./... && go vet ./... && go list -m -f '{{.Path}} {{.GoVersion}}' all)
done
```

On Windows without local cgo, run race tests in Docker:

```bash
docker run --rm -v "${PWD}:/workspace" -w /workspace -e CGO_ENABLED=1 -e GOFLAGS=-mod=readonly golang:1.23 go test -race ./...
```
