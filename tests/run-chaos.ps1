param([switch]$KeepServices)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$composeFile = Join-Path $PSScriptRoot 'compose.yaml'
$managedEnvironmentVariables = @(
    'GOTENANCY_CHAOS',
    'GOTENANCY_TOXIPROXY_URL',
    'GOTENANCY_CHAOS_MYSQL_DSN',
    'GOTENANCY_CHAOS_POSTGRES_DSN',
    'GOTENANCY_CHAOS_REDIS_ADDR'
)
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
    Invoke-Checked docker @('compose', '--profile', 'chaos', '-f', $composeFile, 'up', '-d', '--wait')
    $env:GOTENANCY_CHAOS = '1'
    $env:GOTENANCY_TOXIPROXY_URL = 'http://127.0.0.1:58474'
    $env:GOTENANCY_CHAOS_MYSQL_DSN = 'root:gotenancy@tcp(127.0.0.1:58666)/gotenancy_test?parseTime=true&timeout=1s&readTimeout=1s&writeTimeout=1s'
    $env:GOTENANCY_CHAOS_POSTGRES_DSN = 'postgres://gotenancy:gotenancy@127.0.0.1:58667/gotenancy_test?sslmode=disable&connect_timeout=1'
    $env:GOTENANCY_CHAOS_REDIS_ADDR = '127.0.0.1:58668'

    Invoke-Checked go @('test', '-tags=chaos', './cache', '-run', '^TestRedisChaos', '-count=1')
    Push-Location (Join-Path $repoRoot 'tests/db')
    try {
        Invoke-Checked go @('test', '-tags=chaos', './...', '-run', '^TestSQLStoreChaos', '-count=1')
    } finally {
        Pop-Location
    }
} catch {
    & docker compose --profile chaos -f $composeFile logs --no-color
    throw
} finally {
    try {
        foreach ($name in $managedEnvironmentVariables) {
            [Environment]::SetEnvironmentVariable($name, $previousEnvironment[$name], 'Process')
        }
        if (-not $KeepServices) {
            Invoke-Checked docker @('compose', '--profile', 'chaos', '-f', $composeFile, 'down', '--volumes', '--remove-orphans')
        }
    } finally {
        Pop-Location
    }
}
