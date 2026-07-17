# SaaS

[EN](README.md) | [中文](README.zh-CN.md)

[![Go Reference](https://pkg.go.dev/badge/github.com/DarkInno/saas.svg)](https://pkg.go.dev/github.com/DarkInno/saas)
[![CI](https://github.com/DarkInno/saas/actions/workflows/ci.yml/badge.svg)](https://github.com/DarkInno/saas/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

SaaS 是一个与 ORM 无关的 Go 工具包，用于实现以必需 `tenant_id` 边界为基础的共享数据库多租户。

它提供租户上下文、租户解析、数据防护、Web/RPC 中间件、租户元数据存储和常见 SaaS 模块。默认模型很简单：每一行租户所有的数据都带有 `tenant_id`，适配器从 `context.Context` 派生当前活跃租户。

## 范围

- 使用必需 `tenant_id` 边界的共享数据库隔离。
- 只能通过显式主机上下文进行全局主机访问。
- 用于租户感知数据访问的 GORM、Ent 和 sqlx 适配器。
- HTTP、Gin、Echo、Fiber、Kratos 和 gRPC 中间件。
- 租户生命周期、套餐、订阅、配额、功能、认证后身份关联、RBAC、审计、用户和通知。

尚未实现独立数据库和混合隔离模型。
未来可选的扩展能力可位于单独模块中，但核心接入适配器随主模块提供。

## 架构

有关宿主集成、租户边界、数据隔离和外部适配器图，请参阅 [docs/architecture.zh-CN.md](docs/architecture.zh-CN.md)。

## 从 GoTenancy 迁移

SaaS v0.2.0 是一次破坏性改名。Go 模块路径现在是
`github.com/DarkInno/saas`，生命周期包也提升为顶级路径，例如
`github.com/DarkInno/saas/tenant`。已有应用升级前请阅读
[v0.2 迁移指南](docs/migration-v0.2.zh-CN.md)。

## 要求

- Go `1.24+`。

## 安装

```bash
go mod init your-app
go get github.com/DarkInno/saas
```

## 完整示例

此可复制粘贴的示例会创建内存中的租户、安装 GORM 插件、在 GORM DryRun 模式下执行租户作用域查询，并打印生成的 SQL。它不需要真实数据库。

```go
package main

import (
	"context"
	"fmt"
	"log"

	tenantctx "github.com/DarkInno/saas/core/context"
	"github.com/DarkInno/saas/core/store"
	"github.com/DarkInno/saas/core/types"
	gormtenant "github.com/DarkInno/saas/data/gorm"

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

## 接入示例

请从仓库根目录运行示例：

```bash
go run ./examples/quickstart
go run ./examples/gin-gorm
go run ./examples/grpc
go run ./examples/ent
```

- [examples/quickstart](examples/quickstart)：最小化 GORM 创建流程。
- [examples/gin-gorm](examples/gin-gorm)：Gin header 解析器、租户存储校验、请求上下文注入和 GORM 查询防护。
- [examples/grpc](examples/grpc)：解析租户元数据并注入租户上下文的 unary gRPC 拦截器。
- [examples/ent](examples/ent)：使用存储层接口（由生成的 builder 暴露）的 Ent 查询和变更过滤器。

## 常见模式

在启动时只注册一次 GORM 插件：

```go
if err := db.Use(gormtenant.New(gormtenant.Config{})); err != nil {
	log.Fatal(err)
}
```

在入口处解析租户，然后让 `context.Context` 贯穿应用层和数据层：

```go
tenantResolver := resolver.NewComposite(
	resolver.NewHeaderContrib("", types.TenantIDStrategyString),
)
router.Use(ginsaas.TenantMiddleware(tenantResolver, tenants))
```

在执行前过滤 Ent 查询：

```go
query := client.Order.Query()
if err := enttenant.FilterQuery(ctx, query, enttenant.Config{}); err != nil {
	return err
}
orders, err := query.All(ctx)
```

将 Ent 变更 hook 注册到生成的客户端：

```go
client.Use(enttenant.Hook(enttenant.Config{}))
```

使用租户元数据保护 gRPC handler：

```go
server := grpc.NewServer(
	grpc.UnaryInterceptor(grpcsaas.TenantUnaryServerInterceptor(tenants)),
)
```

为全局主机操作使用显式主机上下文：

```go
ctx := tenantctx.WithHost(context.Background())
```

## 包

- `core/types`：租户 ID、租户元数据、状态和侧类型。
- `core/context`：租户和主机上下文、detach 和 switch。
- `core/resolver`：header、cookie、query、domain、token-claim 和组合式解析器。
- `core/store`：内存存储、分页列表过滤器、内存缓存、缓存存储装饰器以及 `database/sql` 存储。
- `data`：与 ORM 无关的租户过滤条件。
- `data/gorm`：GORM 插件、防护套件、仅限主机的 `SafeRaw`/`SafeExec`、`BulkCreate` 和删除 API。
- `data/ent`：Ent selector 谓词、查询过滤器、变更过滤器和 hook API。
- `data/sqlx`：用于简单单表 SELECT/UPDATE/DELETE 语句的租户过滤 API。
- `tenant`：租户生命周期状态机。
- `plan`：套餐 Store、内存实现和 `database/sql` SQLStore。
- `subscription`：订阅生命周期、续订、过期、宽限期处理、计费 hook、Store、内存实现和 `database/sql` SQLStore。
- `quota`：配额检查、原子消耗、重置、内存实现和 `database/sql` SQLStore。
- `feature`：套餐默认功能、租户级功能覆盖、内存实现和 `database/sql` SQLStore。
- `onboarding`：涵盖租户、套餐、订阅、功能、配额、审计和通知服务的租户开通流程。
- `biz/identity`：针对已验证外部身份断言的认证后租户用户映射，提供内存和 `database/sql` 存储。
- `biz/identity/oidc`：OIDC 授权码桥接，支持 PKCE、state、nonce、ID Token 验证、一次性登录状态存储、SQL 支持的登录状态存储和断言输出。
- `web/*`：net/http、Gin、Echo、Fiber 和 Kratos 的租户中间件和防护。
- `rpc/grpc`：gRPC unary 和 stream 租户拦截器。
- `migration`：租户列和索引规划。
- `cache`：租户作用域缓存包装器、内存适配器和 Redis 适配器。
- `obs`：租户可观测性字段、脱敏、`slog` 辅助函数和 OpenTelemetry API 辅助函数。
- `biz/*`：身份、用户、RBAC、审计和通知模块，提供内存存储、当持久化属于模块契约时提供 SQL 存储、SMTP/SES/Resend/webhook 投递、渠道路由、扇出、重试和超时辅助函数。

## 认证后身份映射

`biz/identity` 将已验证的外部身份映射为租户用户和成员资格。`biz/identity/oidc` 为回调处理、一次性登录状态和 ID Token 验证增加标准 OIDC 授权码桥接。它仍不是完整的 IAM 平台：应用会话、账户页面、Magic Link 投递、SAML 验证、MFA 和 WebAuthn 仍是应用或 IdP 的职责。

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

将浏览器重定向到 `login.URL`。`MemoryLoginStore` 是进程本地的，并且每个 state 只消费一次；多实例部署应在其安全会话或共享缓存层之上实现 `LoginStore`。

## 验证

```bash
go test ./...
go vet ./...
go test -race ./...
```

在 Windows 上，`go test -race` 需要 cgo 和 C 编译器。如果本地没有 cgo，请在 Docker 中运行 race 测试：

```bash
docker run --rm -v "${PWD}:/workspace" -w /workspace -e CGO_ENABLED=1 -e GOFLAGS=-mod=readonly golang:1.24 go test -race ./...
```

### 一次性集成、覆盖率与混沌测试

PowerShell 运行脚本会为 MySQL、PostgreSQL、Redis 以及（混沌测试时）Toxiproxy 启动本地一次性 Docker Compose 环境。脚本会创建和删除测试表及卷，因此绝不能将其 DSN 指向共享或生产服务。

```powershell
# Windows PowerShell；PowerShell 7 用户可将其替换为 pwsh。
# 由一次性 MySQL/PostgreSQL 测试生成 SQL 契约覆盖率。
$sqlProfile = Join-Path $env:TEMP 'saas-sql-contract-coverage.out'
powershell.exe -NoProfile -ExecutionPolicy Bypass -File tests/run-integration.ps1 -CoverageProfile $sqlProfile

$unitProfile = Join-Path $env:TEMP 'saas-unit-coverage.out'
$profile = Join-Path $env:TEMP 'saas-coverage.out' # 合并单元测试和数据库集成测试覆盖率
go test -count=1 -covermode=atomic -coverpkg=./... "-coverprofile=$unitProfile" ./...
./tests/merge-coverage.ps1 -Profiles @($unitProfile, $sqlProfile) -Output $profile
powershell.exe -NoProfile -ExecutionPolicy Bypass -File tests/check-coverage.ps1 -Profile $profile -Minimum 85
Remove-Item -LiteralPath $unitProfile, $sqlProfile, $profile, "$profile.txt" -Force -ErrorAction SilentlyContinue

powershell.exe -NoProfile -ExecutionPolicy Bypass -File tests/run-chaos.ps1
```

对于生产 Redis，请在将 `go-redis` 客户端传递给 `cache.NewRedis` 前，配置 TLS、超时、重试限制和 OpenTelemetry。

## 项目布局

```text
core/          Tenant context, resolver, store, and types
data/          Data filtering contracts and adapters
tenant/        租户生命周期模块
plan/          套餐元数据和存储
subscription/  订阅生命周期和计费 hook
quota/         配额检查和消耗
feature/       功能默认值和租户覆盖
onboarding/    跨模块租户开通
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

## 兼容性

请参阅 [docs/compatibility.zh-CN.md](docs/compatibility.zh-CN.md)。

## 许可证

[Apache License 2.0](LICENSE)
