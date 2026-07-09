# Compatibility

## Go

- Module language version: Go `1.23`.
- `go.mod` records this as `go 1.23.0`.
- CI test jobs should cover Go `1.23.x` and Go `1.26.x`; lint runs on Go `1.26.x`.

The module should not require Go `1.24+` dependencies without an explicit compatibility decision.

## Isolation Model

GoTenancy supports shared-database isolation with a required `tenant_id` boundary.

Independent database and hybrid isolation models are not part of the current API.

## Adapters

| Adapter | Dependency |
|---|---|
| GORM v2 | `gorm.io/gorm` v1.31.2 |
| Ent | `entgo.io/ent` v0.14.1 |
| Gin | `github.com/gin-gonic/gin` v1.9.1 |
| Echo | `github.com/labstack/echo/v4` v4.13.4 |
| Fiber | `github.com/gofiber/fiber/v2` v2.52.13 |
| Kratos | `github.com/go-kratos/kratos/v2` v2.9.2 |
| gRPC | `google.golang.org/grpc` v1.75.1 |
| OIDC | `github.com/coreos/go-oidc/v3` v3.15.0 and `golang.org/x/oauth2` v0.30.0 |

`core/` remains free of GORM, Ent, sqlx, Redis, and web-framework imports.

## SQLStore

`core/store.SQLStore` supports:

- MySQL/SQLite placeholders: `?`
- PostgreSQL placeholders: `$1`, `$2`, ...

Use `WithSQLDialect(SQLDialectPostgres)` for PostgreSQL.

Optional integration tests use:

- `GOTENANCY_MYSQL_DSN`
- `GOTENANCY_POSTGRES_DSN`

SQLStore database integration tests live in `tests/db` and are not part of the default CI gate.

## Verification

```bash
go test ./...
go vet ./...
go test -race ./...
go list -m -f '{{.Path}} {{.GoVersion}}' all
```

On Windows without local cgo, run race tests in Docker:

```bash
docker run --rm -v "${PWD}:/workspace" -w /workspace -e CGO_ENABLED=1 -e GOFLAGS=-mod=readonly golang:1.23 go test -race ./...
```
