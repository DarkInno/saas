# SaaS Test Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (\`- [ ]\`) syntax for tracking.

**Goal:** Turn existing tests into a measurable, reproducible coverage, integration, fuzz, and resiliency test system without adding host-owned retry or persistence behavior.

**Architecture:** Preserve the fast Go matrix. Add a PowerShell coverage gate and Compose-backed MySQL/PostgreSQL/Redis integration lane. Add native Go fuzz targets beside high-risk boundary code. Keep Toxiproxy fault injection behind a chaos build tag and a scheduled/manual workflow.

**Tech Stack:** Go testing/fuzzing, GitHub Actions, Docker Compose, PowerShell Core, MySQL 8.4, PostgreSQL 16, Redis 7, Toxiproxy 2.x HTTP API.

## Global Constraints

- Keep the Go 1.24/1.26 unit, race, lint, vulnerability, and example-smoke jobs intact.
- Use atomic coverage and an initial total threshold of exactly 65.0 percent.
- Only disposable Compose DSNs may be used; no shared or production service is ever contacted.
- Explicitly test both the root module and the nested tests/db module.
- Fuzz and chaos tests must use bounded contexts and must never panic.
- Do not add production retry, TLS, pooling, or persistence policy.
- The only production fix in this scope is a defensive resolver guard for a request with a nil URL.

---

### Task 1: Disposable integration environment and PR job

**Files:**
- Create: tests/compose.yaml
- Create: tests/run-integration.ps1
- Modify: .github/workflows/ci.yml
- Test: existing tests/db/sql_store_integration_test.go, data/gorm/plugin_integration_test.go, cache/redis_integration_test.go

**Interfaces:**
- Consumes: SAAS_MYSQL_DSN, SAAS_POSTGRES_DSN, SAAS_REDIS_ADDR, SAAS_REDIS_DB.
- Produces: one local/CI command that starts the three services, runs the existing tests without Skip, and always destroys the project volumes.

- [ ] **Step 1: Add the failing configuration check**

Run before creating the file:

~~~powershell
docker compose -f tests/compose.yaml config --quiet
~~~

Expected: FAIL because tests/compose.yaml does not exist.

- [ ] **Step 2: Create tests/compose.yaml**

~~~yaml
name: saas-tests

services:
  mysql:
    image: mysql:8.4
    environment:
      MYSQL_DATABASE: saas_test
      MYSQL_ROOT_PASSWORD: saas
    ports: ["127.0.0.1:33067:3306"]
    healthcheck:
      test: ["CMD-SHELL", "mysqladmin ping -h 127.0.0.1 -uroot -psaas --silent"]
      interval: 2s
      timeout: 5s
      retries: 30
      start_period: 10s

  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: saas_test
      POSTGRES_USER: saas
      POSTGRES_PASSWORD: saas
    ports: ["127.0.0.1:55432:5432"]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U saas -d saas_test"]
      interval: 2s
      timeout: 5s
      retries: 30
      start_period: 5s

  redis:
    image: redis:7
    ports: ["127.0.0.1:56379:6379"]
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 2s
      timeout: 5s
      retries: 30

  toxiproxy:
    image: ghcr.io/shopify/toxiproxy:2.12.0
    profiles: ["chaos"]
    ports:
      - "127.0.0.1:58474:8474"
      - "127.0.0.1:58666:8666"
      - "127.0.0.1:58667:8667"
      - "127.0.0.1:58668:8668"
    healthcheck:
      test: ["CMD", "/toxiproxy-cli", "list"]
      interval: 2s
      timeout: 5s
      retries: 30
~~~

- [ ] **Step 3: Create the PowerShell runner**

Create tests/run-integration.ps1 with this complete flow:

~~~powershell
param([switch]$KeepServices)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$composeFile = Join-Path $PSScriptRoot 'compose.yaml'

function Invoke-Checked {
    param([string]$File, [string[]]$Arguments)
    & $File @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$File $($Arguments -join ' ') failed with exit code $LASTEXITCODE"
    }
}

Push-Location $repoRoot
try {
    Invoke-Checked docker @('compose', '-f', $composeFile, 'up', '-d', '--wait')
    $env:SAAS_MYSQL_DSN = 'root:saas@tcp(127.0.0.1:33067)/saas_test?parseTime=true&timeout=3s&readTimeout=3s&writeTimeout=3s'
    $env:SAAS_POSTGRES_DSN = 'postgres://saas:saas@127.0.0.1:55432/saas_test?sslmode=disable'
    $env:SAAS_REDIS_ADDR = '127.0.0.1:56379'
    $env:SAAS_REDIS_DB = '15'

    Invoke-Checked go @('test', './data/gorm', '-run', '^TestMySQLIntegrationEnforcesTenantIsolation$', '-count=1')
    Push-Location (Join-Path $repoRoot 'tests/db')
    try {
        Invoke-Checked go @('test', './...', '-run', '^TestSQLStore(MySQL|Postgres)Integration$', '-count=1')
    } finally {
        Pop-Location
    }
    Invoke-Checked go @('test', './cache', '-run', '^TestRedisCacheIntegration$', '-count=1')
} catch {
    & docker compose -f $composeFile logs --no-color
    throw
} finally {
    Remove-Item Env:SAAS_MYSQL_DSN, Env:SAAS_POSTGRES_DSN, Env:SAAS_REDIS_ADDR, Env:SAAS_REDIS_DB -ErrorAction SilentlyContinue
    if (-not $KeepServices) {
        & docker compose -f $composeFile down --volumes --remove-orphans
    }
    Pop-Location
}
~~~

- [ ] **Step 4: Verify green integration behavior**

Run:

~~~powershell
docker compose -f tests/compose.yaml config --quiet
pwsh -File tests/run-integration.ps1
~~~

Expected: all four existing real-service tests pass rather than Skip; containers are removed at the end.

- [ ] **Step 5: Add the CI job**

Append to ci.yml without altering existing jobs:

~~~yaml
  integration:
    name: integration (mysql, postgres, redis)
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v7
      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version: "1.24.x"
          cache: true
      - name: Run disposable integration tests
        shell: pwsh
        run: ./tests/run-integration.ps1
~~~

- [ ] **Step 6: Commit**

~~~powershell
git add tests/compose.yaml tests/run-integration.ps1 .github/workflows/ci.yml
git diff --cached --check
git commit -m "ci: add disposable integration test job"
~~~

### Task 2: Atomic coverage gate and artifact

**Files:**
- Create: tests/check-coverage.ps1
- Modify: .github/workflows/ci.yml
- Test: temporary atomic profile generated from the root module

**Interfaces:**
- Consumes: a profile path and a numeric threshold.
- Produces: a sibling .txt summary, an optional GitHub summary line, and a nonzero status below the threshold.

- [ ] **Step 1: Write the failing command first**

~~~powershell
$profile = Join-Path $env:TEMP 'saas-coverage.out'
go test -count=1 -covermode=atomic -coverpkg=./... -coverprofile=$profile ./...
pwsh -File tests/check-coverage.ps1 -Profile $profile -Minimum 101
~~~

Expected: initially fails because the script is absent; after creation it fails because the actual total is below 101.

- [ ] **Step 2: Create tests/check-coverage.ps1**

~~~powershell
param(
    [Parameter(Mandatory = $true)]
    [string]$Profile,
    [double]$Minimum = 65.0
)

$ErrorActionPreference = 'Stop'
if (-not (Test-Path -LiteralPath $Profile -PathType Leaf)) {
    throw "coverage profile not found: $Profile"
}

$summary = & go tool cover "-func=$Profile"
if ($LASTEXITCODE -ne 0) {
    throw "go tool cover failed with exit code $LASTEXITCODE"
}

$summaryPath = "$Profile.txt"
$summary | Set-Content -LiteralPath $summaryPath -Encoding utf8
$totalLine = $summary | Where-Object { $_ -match '^total:' } | Select-Object -Last 1
if (-not $totalLine) {
    throw 'coverage summary did not contain a total line'
}

$match = [regex]::Match($totalLine, '(?<coverage>\d+(?:\.\d+)?)%')
if (-not $match.Success) {
    throw "unable to parse coverage from: $totalLine"
}

$coverage = [double]$match.Groups['coverage'].Value
$line = "Go statement coverage: {0:N1}% (minimum: {1:N1}%)" -f $coverage, $Minimum
Write-Output $line
if ($env:GITHUB_STEP_SUMMARY) {
    Add-Content -LiteralPath $env:GITHUB_STEP_SUMMARY -Value ([Environment]::NewLine + "## Coverage" + [Environment]::NewLine + $line)
}
if ($coverage -lt $Minimum) {
    throw $line
}
~~~

- [ ] **Step 3: Verify red and green threshold paths**

~~~powershell
pwsh -File tests/check-coverage.ps1 -Profile $profile -Minimum 85
pwsh -File tests/check-coverage.ps1 -Profile $profile -Minimum 101
Remove-Item $profile, "$profile.txt" -ErrorAction SilentlyContinue
~~~

Expected: the 85 invocation exits 0; the 101 invocation exits nonzero and reports the parsed percentage.

- [ ] **Step 4: Add the CI coverage job**

~~~yaml
  coverage:
    name: coverage
    runs-on: ubuntu-latest
    env:
      COVERAGE_PROFILE: ${{ runner.temp }}/saas-coverage.out
    steps:
      - name: Checkout
        uses: actions/checkout@v7
      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version: "1.24.x"
          cache: true
      - name: Generate atomic coverage profile
        shell: pwsh
        run: go test -count=1 -covermode=atomic -coverpkg=./... -coverprofile=$env:COVERAGE_PROFILE ./...
      - name: Enforce coverage floor
        shell: pwsh
        run: ./tests/check-coverage.ps1 -Profile $env:COVERAGE_PROFILE -Minimum 85
      - name: Upload coverage artifact
        uses: actions/upload-artifact@v4
        with:
          name: saas-coverage
          path: |
            ${{ runner.temp }}/saas-coverage.out
            ${{ runner.temp }}/saas-coverage.out.txt
          if-no-files-found: error
~~~

- [ ] **Step 5: Commit**

~~~powershell
git add tests/check-coverage.ps1 .github/workflows/ci.yml
git diff --cached --check
git commit -m "ci: enforce coverage baseline"
~~~

### Task 3: Fuzz tenant boundaries and fix malformed query requests

**Files:**
- Create: data/sqlx/fuzz_test.go
- Create: core/types/fuzz_test.go
- Create: core/resolver/fuzz_test.go
- Create: cache/fuzz_test.go
- Modify: core/resolver/errors.go
- Modify: core/resolver/query.go
- Modify: core/resolver/resolver_test.go

**Interfaces:**
- Consumes: QueryWithArgs, ParseTenantID, Composite resolver, KeyBuilder.Build, NewRedisFromURL.
- Produces: four native fuzz targets and an ErrNilURL result rather than a nil pointer panic.

- [ ] **Step 1: Write a failing request-shape regression test**

Add to resolver_test.go:

~~~go
func TestQueryContribRejectsRequestWithoutURL(t *testing.T) {
    request := &http.Request{Header: make(http.Header)}
    _, _, err := NewQueryContrib("", types.TenantIDStrategyString).Resolve(request)
    if !errors.Is(err, ErrNilURL) {
        t.Fatalf("Resolve(request without URL) error = %v, want ErrNilURL", err)
    }
}
~~~

Run:

~~~powershell
go test ./core/resolver -run '^TestQueryContribRejectsRequestWithoutURL$' -count=1
~~~

Expected: FAIL with a nil pointer panic from r.URL.Query.

- [ ] **Step 2: Implement only the guard needed for green**

In errors.go add:

~~~go
// ErrNilURL reports a request whose URL is unavailable to a query resolver.
ErrNilURL = errors.New("saas/resolver: nil request url")
~~~

In QueryContrib.Resolve, immediately after the nil request guard add:

~~~go
if r.URL == nil {
    return "", false, ErrNilURL
}
~~~

Re-run the target test. Expected: PASS.

- [ ] **Step 3: Add complete fuzz callbacks**

Implement these behaviors in same-package fuzz_test.go files:

~~~go
// data/sqlx: seed valid SELECT, valid DELETE-with-args, and multi-statement SQL.
// On nil error: require strings.Contains(rewritten, "tenant_id = ?"),
// exactly one appended tenant argument, and scanSQL(rewritten) succeeds.

// core/types: seed string, int, and uppercase UUID strategies.
// On nil error: require a nonempty, TrimSpace-normalized identifier.

// core/resolver: seed query/header combinations and construct request with
// httptest.NewRequest plus url.QueryEscape. Require query source precedence,
// header fallback, and ErrNoTenant only when both normalized values are empty.

// cache: seed a valid Redis URL and a credential-bearing malformed URL.
// NewRedisFromURL may succeed or return ErrInvalidRedisConfig, but an error
// must not contain a parsed password. KeyBuilder success must start with
// tenantPrefix; unsafe keys may return ErrUnsafeKey.
~~~

Use f.Add for every seed so normal go test executes the regression corpus.

- [ ] **Step 4: Run package and short fuzz verification**

~~~powershell
go test ./data/sqlx ./core/types ./core/resolver ./cache -count=1
go test ./data/sqlx -run=^$ -fuzz=FuzzQueryWithArgs -fuzztime=10s
go test ./core/types -run=^$ -fuzz=FuzzParseTenantID -fuzztime=10s
go test ./core/resolver -run=^$ -fuzz=FuzzCompositeResolverPriority -fuzztime=10s
go test ./cache -run=^$ -fuzz=FuzzRedisURLAndKeyBuilder -fuzztime=10s
~~~

Expected: all targets pass. If a minimized failure corpus appears, commit it and add a conventional regression assertion before any production change.

- [ ] **Step 5: Commit**

~~~powershell
git add core/resolver/errors.go core/resolver/query.go core/resolver/resolver_test.go core/resolver/fuzz_test.go core/types/fuzz_test.go cache/fuzz_test.go data/sqlx/fuzz_test.go
git diff --cached --check
git commit -m "test: add fuzz coverage for tenant boundaries"
~~~

### Task 4: Deterministic Toxiproxy contracts

**Files:**
- Create: internal/testtoxiproxy/client.go
- Create: internal/testtoxiproxy/client_test.go
- Create: cache/redis_chaos_test.go
- Create: tests/db/chaos_integration_test.go
- Create: tests/run-chaos.ps1
- Modify: tests/compose.yaml

**Interfaces:**
- Consumes: Toxiproxy on SAAS_TOXIPROXY_URL and Docker-service host names.
- Produces: a test-only HTTP client plus tagged Redis and SQLStore fault/recovery tests.

- [ ] **Step 1: Test the HTTP contract before creating the helper**

Create client_test.go with an httptest.Server. Assert that CreateProxy sends:

~~~json
{"name":"saas_redis","listen":"0.0.0.0:8668","upstream":"redis:6379","enabled":true}
~~~

and AddTimeout sends:

~~~json
{"name":"blocked","type":"timeout","stream":"downstream","toxicity":1.0,"attributes":{"timeout":0}}
~~~

Also assert DELETE paths for a toxic and proxy, and POST /proxies/{name} with enabled false. Run go test ./internal/testtoxiproxy -run '^TestClient' -count=1; expected initial compile failure.

- [ ] **Step 2: Implement the dependency-free helper**

Create a net/http + encoding/json client with exactly these signatures:

~~~go
type Client struct {
    endpoint   string
    httpClient *http.Client
}

type Proxy struct {
    Name   string
    client *Client
}

func New(endpoint string) *Client
func (client *Client) Wait(ctx context.Context) error
func (client *Client) CreateProxy(ctx context.Context, name, listen, upstream string) (*Proxy, error)
func (client *Client) SetEnabled(ctx context.Context, name string, enabled bool) error
func (client *Client) AddTimeout(ctx context.Context, proxy, name string) error
func (client *Client) RemoveToxic(ctx context.Context, proxy, name string) error
func (client *Client) DeleteProxy(ctx context.Context, name string) error
~~~

Wait polls GET /version using a ticker and respects ctx.Done. All non-2xx errors include only status and a bounded response body. AddTimeout uses timeout: 0 so traffic is dropped until removal.

- [ ] **Step 3: Add tagged Redis chaos test**

Create cache/redis_chaos_test.go with //go:build chaos. It must skip unless SAAS_CHAOS is 1, then:
1. wait for Toxiproxy;
2. create saas_redis at 0.0.0.0:8668 upstream redis:6379;
3. connect a short-timeout Redis client through SAAS_CHAOS_REDIS_ADDR;
4. prove tenant A/B key isolation;
5. add timeout toxic and assert a context-bounded Ping or Get error;
6. remove toxic, wait for Ping, and prove fresh isolated reads/writes;
7. delete proxy in cleanup.

- [ ] **Step 4: Add tagged MySQL/PostgreSQL CAS/recovery test**

Create tests/db/chaos_integration_test.go with //go:build chaos, package db_test. Reuse existing ping/reset helpers. For each configured backend:
1. create saas_mysql or saas_postgres proxy;
2. use a fresh sql.DB through its proxy and a matching store.WithSQLDialect;
3. create one tenant, launch two CompareAndSwap operations against the same expected snapshot, and require exactly one nil and one store.ErrTenantConflict;
4. verify the final tenant exactly equals one candidate update, never a hybrid;
5. disable the proxy; a newly opened db PingContext must fail before deadline;
6. re-enable it, wait for PingContext, then prove a new store.Get works;
7. remove proxy in cleanup.

- [ ] **Step 5: Create tests/run-chaos.ps1**

Mirror Task 1's Invoke-Checked, catch/log/finally teardown pattern. Start Compose with --profile chaos and export:

~~~powershell
$env:SAAS_CHAOS = '1'
$env:SAAS_TOXIPROXY_URL = 'http://127.0.0.1:58474'
$env:SAAS_CHAOS_MYSQL_DSN = 'root:saas@tcp(127.0.0.1:58666)/saas_test?parseTime=true&timeout=1s&readTimeout=1s&writeTimeout=1s'
$env:SAAS_CHAOS_POSTGRES_DSN = 'postgres://saas:saas@127.0.0.1:58667/saas_test?sslmode=disable&connect_timeout=1'
$env:SAAS_CHAOS_REDIS_ADDR = '127.0.0.1:58668'
~~~

Run:

~~~powershell
go test -tags=chaos ./cache -run '^TestRedisChaos' -count=1
Push-Location (Join-Path $repoRoot 'tests/db')
go test -tags=chaos ./... -run '^TestSQLStoreChaos' -count=1
Pop-Location
~~~

- [ ] **Step 6: Verify and commit**

~~~powershell
go test ./internal/testtoxiproxy -count=1
pwsh -File tests/run-chaos.ps1
git add internal/testtoxiproxy cache/redis_chaos_test.go tests/db/chaos_integration_test.go tests/run-chaos.ps1 tests/compose.yaml
git diff --cached --check
git commit -m "test: add deterministic dependency chaos coverage"
~~~

### Task 5: Scheduled fuzz/resilience workflow and documentation

**Files:**
- Create: .github/workflows/resilience.yml
- Modify: README.md
- Modify: README.zh-CN.md
- Modify: docs/compatibility.md
- Modify: docs/compatibility.zh-CN.md

**Interfaces:**
- Consumes: fuzz targets and run-chaos.ps1.
- Produces: separate scheduled/manual fuzz and chaos jobs, plus equivalent English/Chinese safe-run guidance.

- [ ] **Step 1: Add the scheduled/manual workflow**

~~~yaml
name: resilience

on:
  workflow_dispatch:
  schedule:
    - cron: "17 3 * * 3"

permissions:
  contents: read

jobs:
  fuzz:
    name: native fuzz
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v6
        with:
          go-version: "1.24.x"
          cache: true
      - run: go test ./data/sqlx -run=^$ -fuzz=FuzzQueryWithArgs -fuzztime=30s
      - run: go test ./core/types -run=^$ -fuzz=FuzzParseTenantID -fuzztime=30s
      - run: go test ./core/resolver -run=^$ -fuzz=FuzzCompositeResolverPriority -fuzztime=30s
      - run: go test ./cache -run=^$ -fuzz=FuzzRedisURLAndKeyBuilder -fuzztime=30s

  chaos:
    name: deterministic dependency chaos
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v6
        with:
          go-version: "1.24.x"
          cache: true
      - name: Run chaos contracts
        shell: pwsh
        run: ./tests/run-chaos.ps1
~~~

- [ ] **Step 2: Update documentation**

In both README languages:
- retain baseline go test, vet, and race examples;
- add pwsh -File tests/run-integration.ps1;
- show a temporary atomic coverage profile plus check-coverage.ps1 -Minimum 85;
- add pwsh -File tests/run-chaos.ps1;
- state that the scripts create/drop tables and must never target shared/production DSNs.

In both compatibility documents:
- replace the old claim that real DB tests are outside default CI;
- explain PR coverage/integration versus scheduled/manual fuzz/chaos;
- state that branch protection still must mark coverage and integration as required in GitHub settings.

- [ ] **Step 3: Run full verification**

~~~powershell
gofmt -w core/resolver/errors.go core/resolver/query.go core/resolver/resolver_test.go core/resolver/fuzz_test.go core/types/fuzz_test.go cache/fuzz_test.go cache/redis_chaos_test.go data/sqlx/fuzz_test.go internal/testtoxiproxy/client.go internal/testtoxiproxy/client_test.go tests/db/chaos_integration_test.go
go test ./... -count=1
go vet ./...
go test -race ./...
pwsh -File tests/run-integration.ps1
pwsh -File tests/run-chaos.ps1
go test ./data/sqlx -run=^$ -fuzz=FuzzQueryWithArgs -fuzztime=30s
go test ./core/types -run=^$ -fuzz=FuzzParseTenantID -fuzztime=30s
go test ./core/resolver -run=^$ -fuzz=FuzzCompositeResolverPriority -fuzztime=30s
go test ./cache -run=^$ -fuzz=FuzzRedisURLAndKeyBuilder -fuzztime=30s
docker compose -f tests/compose.yaml config --quiet
docker compose -f tests/compose.yaml --profile chaos config --quiet
git diff --check
~~~

Expected: every available lane exits 0. If Docker or the host race runtime is unavailable, record the exact environment error separately and do not claim the lane passed.

- [ ] **Step 4: Commit and hand off**

~~~powershell
git add .github/workflows/resilience.yml README.md README.zh-CN.md docs/compatibility.md docs/compatibility.zh-CN.md
git diff --cached --check
git commit -m "docs: document test hardening workflows"
git status --short --branch
git log --oneline --decorate -n 6
~~~

Expected: no untracked coverage profiles or generated test output remain. Do not push unless separately requested.
