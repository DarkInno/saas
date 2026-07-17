# Compatibility

[EN](compatibility.md) | [中文](compatibility.zh-CN.md)

## Go

- Module language version: Go `1.24`.
- `go.mod` records this as `go 1.24.0`.
- CI test jobs should cover Go `1.24.x` and Go `1.26.x`; lint and vulnerability scans run on a patched Go `1.26.5+` toolchain.

Go `1.23` support was dropped because the patched `github.com/go-jose/go-jose/v4` release required by the OIDC path requires Go `1.24.0`.

The module should not require Go `1.25+` dependencies without an explicit compatibility decision.

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
| Redis cache | `github.com/redis/go-redis/v9` v9.21.0 |

`core/` remains free of GORM, Ent, sqlx, Redis, and web-framework imports.

## SQLStore

`core/store.SQLStore` supports:

- MySQL/SQLite placeholders: `?`
- PostgreSQL placeholders: `$1`, `$2`, ...

Use `WithSQLDialect(SQLDialectPostgres)` for PostgreSQL.

The disposable integration runner owns the local MySQL, PostgreSQL, and Redis endpoints used by SQLStore, GORM, and Redis cache tests. It must never be configured with shared or production DSNs because the database tests create and drop tables.

## CI and resilience

Pull requests run the existing Go version matrix, lint, vulnerability scan, and example smoke tests, plus two dedicated gates:

- `coverage` creates an atomic root-module profile, enforces at least 65.0% statements, and uploads the profile and summary artifact.
- `integration (mysql, postgres, redis)` runs the disposable Compose-backed MySQL, PostgreSQL, GORM, and Redis contracts.

The separate `resilience` workflow runs every Wednesday at 03:17 UTC and on manual dispatch. It executes bounded native fuzz targets and the deterministic Toxiproxy fault/recovery contracts; these longer-running checks are intentionally outside the pull-request path.

Repository branch protection must independently mark `coverage` and `integration (mysql, postgres, redis)` as required checks in GitHub settings.

## Verification

```bash
go test ./...
go vet ./...
go test -race ./...
go list -m -f '{{.Path}} {{.GoVersion}}' all
go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...
```

On Windows without local cgo, run race tests in Docker:

```bash
docker run --rm -v "${PWD}:/workspace" -w /workspace -e CGO_ENABLED=1 -e GOFLAGS=-mod=readonly golang:1.24 go test -race ./...
```
