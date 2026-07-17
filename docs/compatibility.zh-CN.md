# 兼容性

[EN](compatibility.md) | [中文](compatibility.zh-CN.md)

## Go

- 模块语言版本：Go `1.24`。
- `go.mod` 将其记录为 `go 1.24.0`。
- CI 测试任务应覆盖 Go `1.24.x` 和 Go `1.26.x`；lint 和漏洞扫描在已修复的 Go `1.26.5+` 工具链上运行。

由于 OIDC 路径所需的已修复 `github.com/go-jose/go-jose/v4` 版本要求 Go `1.24.0`，因此不再支持 Go `1.23`。

除非作出明确的兼容性决策，模块不应引入要求 Go `1.25+` 的依赖。

## 隔离模型

SaaS 仅支持共享数据库、共享 Schema 隔离；租户数据必须具有 `tenant_id` 边界。

按租户独立数据库、独立 Schema 和混合隔离模型不属于当前 API 的范围。该模块不会创建租户数据库或 Schema、路由租户连接，或在运行时切换 Schema。

## 适配器

| 适配器 | 依赖 |
|---|---|
| GORM v2 | `gorm.io/gorm` v1.31.2 |
| Ent | `entgo.io/ent` v0.14.1 |
| Gin | `github.com/gin-gonic/gin` v1.9.1 |
| Echo | `github.com/labstack/echo/v4` v4.13.4 |
| Fiber | `github.com/gofiber/fiber/v2` v2.52.13 |
| Kratos | `github.com/go-kratos/kratos/v2` v2.9.2 |
| gRPC | `google.golang.org/grpc` v1.75.1 |
| OIDC | `github.com/coreos/go-oidc/v3` v3.15.0 和 `golang.org/x/oauth2` v0.30.0 |
| Redis 缓存 | `github.com/redis/go-redis/v9` v9.21.0 |

`core/` 保持不导入 GORM、Ent、sqlx、Redis 和 web 框架。

## SQLStore

`core/store.SQLStore` 支持：

- MySQL/SQLite 占位符：`?`
- PostgreSQL 占位符：`$1`、`$2`、...

对于 PostgreSQL，请使用 `WithSQLDialect(SQLDialectPostgres)`。

一次性集成运行脚本负责 SQLStore、GORM 和 Redis 缓存测试所使用的本地 MySQL、PostgreSQL 与 Redis 端点。数据库测试会创建和删除表，因此绝不能将脚本配置为使用共享或生产 DSN。

## CI 与韧性测试

Pull Request 会运行既有的 Go 版本矩阵、lint、漏洞扫描和示例 smoke 测试，并额外包含两个独立门禁：

- `coverage` 生成根模块的原子覆盖率 profile，强制至少 85.0% statements，并上传 profile 与摘要 artifact。
- `integration (mysql, postgres, redis)` 运行基于一次性 Compose 的 MySQL、PostgreSQL、GORM 与 Redis 契约测试。

独立的 `resilience` 工作流会在每周三 03:17 UTC 运行，也可手动触发。它执行有时间上限的原生 fuzz target 和确定性的 Toxiproxy 故障/恢复契约；这些耗时更长的检查有意不放在 Pull Request 路径中。

仓库分支保护还必须在 GitHub 设置中独立将 `coverage` 与 `integration (mysql, postgres, redis)` 设为 required checks。

## 验证

```bash
go test ./...
go vet ./...
go test -race ./...
go list -m -f '{{.Path}} {{.GoVersion}}' all
go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...
```

在 Windows 上如果本地没有 cgo，请在 Docker 中运行 race 测试：

```bash
docker run --rm -v "${PWD}:/workspace" -w /workspace -e CGO_ENABLED=1 -e GOFLAGS=-mod=readonly golang:1.24 go test -race ./...
```
