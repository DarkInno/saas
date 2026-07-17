# GoTenancy 测试保障补全设计

## 目标

把现有的单元、回归和可选真实依赖测试，提升为可量化、可复现且分层执行的质量体系：

- 每个 PR 都能看到并守住主模块覆盖率基线；
- MySQL、PostgreSQL 和 Redis 的既有真实集成测试不再因缺少环境变量而静默跳过；
- 输入边界通过原生 Go fuzz 持续检验；
- 依赖故障通过可重复的韧性契约验证，而不是随机性很高的“杀进程测试”。

本设计以库的职责边界为准：GoTenancy 接收宿主提供的数据库和 Redis 客户端，不承诺替宿主重试、持久化或恢复连接；测试验证库在依赖故障时的边界行为与租户隔离不变量。

## 当前基线与问题

- 根模块默认 CI 执行 `go vet ./...`、`go test ./...`、示例 smoke 和 `go test -race ./...`，但没有覆盖率 profile、报告、阈值或真实服务容器。
- 本设计确认时的主模块聚合语句覆盖率为 66.5%。这是一次快照，不是现有门禁；嵌套 `tests/db` 模块不在根目录 `go test ./...` 的扫描范围内。
- SQL Store 的 MySQL/PostgreSQL、GORM MySQL 和 Redis 集成测试均已存在，但都要求显式环境变量，且未配置时会跳过。
- 仓库没有原生 `Fuzz*` 目标、fuzz corpus、Compose 编排、Toxiproxy 或其他真实依赖故障注入工具。

## 方案选择

采用分层方案，而不是仅增加覆盖率，或把长时 fuzz 和混沌演练塞进每个 PR：

| 层级 | PR 门禁 | 定时或手动门禁 |
|---|---|---|
| 单元与回归 | 既有 `go test`、`go test -race`、示例 smoke | 保持同一命令集 |
| 覆盖率 | 原子模式 profile、摘要、85% 当前总覆盖率门槛 | 保存趋势并在稳定后提高门槛 |
| 真实集成 | Compose 启动 MySQL、PostgreSQL、Redis 后运行既有集成测试 | 同 PR 门禁 |
| Fuzz | 运行已提交的 seed/corpus 回归输入 | 每个目标原生 fuzz 30 秒 |
| 韧性/混沌 | 不运行 | Toxiproxy 注入断连和延迟，按计划或手动触发 |

65% 曾是有意低于 66.5% 快照的初始防回退阈值；在补齐租户业务行为与依赖故障契约后，当前门槛已提高到 85%。覆盖率用于发现明显回退，不以覆盖率数字代替租户隔离、事务一致性或故障恢复的行为测试。

## 设计

### 1. 覆盖率门禁和报告

现有 Go 版本矩阵保持不变。Go 1.24 测试路径额外执行：

```text
go test -count=1 -covermode=atomic -coverpkg=./... -coverprofile=<temporary-profile> ./...
go tool cover -func=<temporary-profile>
```

CI 以 PowerShell 脚本解析 `total:` 行，并在总覆盖率低于 85.0% 时失败。脚本还生成文本摘要；profile 和摘要作为 GitHub Actions artifact 上传。PowerShell 是该仓库 Windows 用户的本地入口，GitHub 的 Ubuntu runner 也可运行 PowerShell Core，因此不需要维护两套阈值逻辑。

profile 写入 runner 的临时目录，避免把生成文件留在工作树。覆盖率 job 不读取、记录或上传任何 DSN、Redis 密码或其他连接字符串。

### 2. 可复现的 MySQL、PostgreSQL、Redis 集成环境

增加一个只用于测试的 Compose 定义，固定以下服务主版本：

- MySQL 8.4；
- PostgreSQL 16；
- Redis 7。

每个服务配置 healthcheck。CI 和本地都以相同命令启动和关闭：

```text
docker compose -f tests/compose.yaml up -d --wait
docker compose -f tests/compose.yaml down --volumes --remove-orphans
```

集成 job 使用一次性的本地容器地址和以下既有测试入口：

```text
go test ./data/gorm -run '^TestMySQLIntegrationEnforcesTenantIsolation$' -count=1
(cd tests/db && go test ./... -run '^TestSQLStore(MySQL|Postgres)Integration$' -count=1)
go test ./cache -run '^TestRedisCacheIntegration$' -count=1
```

job 在命令前设置 `GOTENANCY_MYSQL_DSN`、`GOTENANCY_POSTGRES_DSN`、`GOTENANCY_REDIS_ADDR` 和专用 Redis DB。测试会删建 `tenants` 或 `tenant_orders`，因此文档和脚本必须明确：这些变量只能指向 Compose 创建的可销毁测试实例，绝不能指向共享或生产服务。

### 3. 原生 Go fuzz 层

第一批 fuzz 目标覆盖租户隔离的高风险输入边界：

1. `data/sqlx`：任意查询和参数输入不得 panic；接受的查询必须保留单语句限制、不能绕过注释或 NUL 检查，并且只能以受控方式追加租户参数；拒绝的输入必须返回安全错误。
2. `core/types` 和 resolver：任意租户 ID、header、cookie、query 输入不得 panic；成功结果遵循既有 ID 策略和来源优先级。
3. `cache`：任意缓存键与 Redis URL 输入不得 panic；Redis URL 的错误路径不得在错误文本中泄露凭据；成功的缓存键仍保持租户隔离。

每个 fuzz 目标先提交已有单测中的合法与非法 seed。常规 `go test` 会把这些 seed 作为回归用例执行；定时/手动 workflow 再执行 `-run=^$ -fuzz=<name> -fuzztime=30s`。发现的最小崩溃输入必须提交到相应 `testdata/fuzz` corpus，并先以普通回归方式复现。

Fuzz 不擅自改变公开 API 语义。例如若 fuzz 暴露 `nil` HTTP URL 或 JSON `null` 的边界，先把期望行为写成回归测试和明确设计决定，再做最小兼容性修改。

### 4. 韧性契约与混沌演练

新增带 `chaos` build tag 的测试，避免让每个 PR 承受网络故障注入的时序风险。专用 Compose 环境增加 Toxiproxy，测试客户端经代理而不是直连 MySQL、PostgreSQL 或 Redis。

韧性断言统一为：

- 故障期间，操作在其 `context.Context` deadline 内返回明确错误，不 panic；
- 故障不得造成跨租户读取、写入或缓存键泄漏；
- 对 SQL CAS/事务，失败后不能产生部分可见或撕裂的租户配置；
- 清除 toxic 或恢复代理后，新建操作可以重新 Ping、读写或执行事务；
- 不断言该库自动重试，也不假设 Redis 重启后数据必然保留。

优先场景是 Redis 的连接中断、延迟和恢复，以及 MySQL/PostgreSQL 的连接中断和并发 CAS 冲突。所有测试使用 context deadline 和明确状态轮询，不使用固定 `Sleep` 作为故障是否生效的判断。

### 5. 文档、本地入口与 CI 分工

README 和兼容性文档增加覆盖率、Compose 集成和 Windows PowerShell 等价命令，并解释两类工作流：

- PR workflow：快速、确定性的单元、race、覆盖率和三服务集成；
- scheduled/manual workflow：短时原生 fuzz 与 Toxiproxy 韧性演练。

CI workflow 负责仓库内测试，GitHub 分支保护仍需要仓库管理员将 coverage 与 integration job 显式设为 required checks；workflow 文件本身不能强制该平台设置。

## 非目标

- 不引入生产重试、连接池、TLS 或 Redis 持久化策略；它们由宿主客户端配置负责。
- 不把性能 benchmark 的绝对数字作为 PR 门禁；先保留现有 benchmark，后续如需性能基线另行设计。
- 不把混沌测试伪装成生产灾备演练，也不访问任何外部、共享或生产依赖。

## 验收标准

1. CI 能输出覆盖率摘要和 profile，并拒绝低于 85% 的主模块覆盖率。
2. 一个干净的 Compose 环境中，MySQL、PostgreSQL、Redis 三类既有真实集成测试均实际执行而非 Skip。
3. 三个高风险输入边界都有 seed corpus 和可重复的 Go fuzz 目标。
4. 手动或定时 chaos workflow 能验证断连/延迟/恢复期间的错误边界与租户隔离不变量。
5. 文档提供 Linux/macOS Docker 与 Windows PowerShell 的安全运行说明，且明确禁止使用生产 DSN。
