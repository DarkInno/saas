param(
    [switch]$KeepServices,
    [string]$CoverageProfile
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$composeFile = Join-Path $PSScriptRoot 'compose.yaml'
$managedEnvironmentVariables = @(
    'GOTENANCY_MYSQL_DSN',
    'GOTENANCY_POSTGRES_DSN',
    'GOTENANCY_REDIS_ADDR',
    'GOTENANCY_REDIS_DB',
    'GOTENANCY_REDIS_PASSWORD'
)
$databaseCoveragePackages = @(
    'github.com/DarkInno/gotenancy/biz/audit',
    'github.com/DarkInno/gotenancy/biz/identity',
    'github.com/DarkInno/gotenancy/biz/identity/oidc',
    'github.com/DarkInno/gotenancy/biz/rbac',
    'github.com/DarkInno/gotenancy/biz/user',
    'github.com/DarkInno/gotenancy/core/store',
    'github.com/DarkInno/gotenancy/saas/feature',
    'github.com/DarkInno/gotenancy/saas/plan',
    'github.com/DarkInno/gotenancy/saas/quota',
    'github.com/DarkInno/gotenancy/saas/subscription'
) -join ','
$previousEnvironment = @{}
foreach ($name in $managedEnvironmentVariables) {
    $previousEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}

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
    $env:GOTENANCY_MYSQL_DSN = 'root:gotenancy@tcp(127.0.0.1:33067)/gotenancy_test?parseTime=true&timeout=3s&readTimeout=3s&writeTimeout=3s'
    $env:GOTENANCY_POSTGRES_DSN = 'postgres://gotenancy:gotenancy@127.0.0.1:55432/gotenancy_test?sslmode=disable'
    $env:GOTENANCY_REDIS_ADDR = '127.0.0.1:56379'
    $env:GOTENANCY_REDIS_DB = '15'
    Remove-Item -Path Env:GOTENANCY_REDIS_PASSWORD -ErrorAction SilentlyContinue

    Invoke-Checked go @('test', './data/gorm', '-run', '^TestMySQLIntegrationEnforcesTenantIsolation$', '-count=1')
    Push-Location (Join-Path $repoRoot 'tests/db')
    try {
        $databaseTestArguments = @('test', './...', '-run', '^Test(AuditSQLStore|SQLStore|QuotaSQLStore|RBACAndUserSQLStore|IdentitySQLStore|OIDCSQLLoginStore|FeatureSQLStore|PlanSQLStore|SubscriptionSQLStore)(MySQL|Postgres)Integration$', '-count=1')
        if ($CoverageProfile) {
            $databaseTestArguments += @('-covermode=atomic', "-coverpkg=$databaseCoveragePackages", "-coverprofile=$CoverageProfile")
        }
        Invoke-Checked -File go -Arguments $databaseTestArguments
    } finally {
        Pop-Location
    }
    Invoke-Checked go @('test', './cache', '-run', '^TestRedisCacheIntegration$', '-count=1')
} catch {
    & docker compose -f $composeFile logs --no-color
    throw
} finally {
    try {
        foreach ($name in $managedEnvironmentVariables) {
            [Environment]::SetEnvironmentVariable($name, $previousEnvironment[$name], 'Process')
        }
        if (-not $KeepServices) {
            Invoke-Checked docker @('compose', '-f', $composeFile, 'down', '--volumes', '--remove-orphans')
        }
    } finally {
        Pop-Location
    }
}
