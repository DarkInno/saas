$runnerPath = Join-Path $PSScriptRoot 'run-integration.ps1'

Describe 'run-integration.ps1 environment isolation' {
    It 'does not pass an inherited Redis password to the disposable cache test and restores it afterward' {
        $name = 'GOTENANCY_REDIS_PASSWORD'
        $previous = [Environment]::GetEnvironmentVariable($name, 'Process')
        $previousDocker = Get-Command docker -CommandType Function -ErrorAction SilentlyContinue
        $previousGo = Get-Command go -CommandType Function -ErrorAction SilentlyContinue
        try {
            [Environment]::SetEnvironmentVariable($name, 'host-inherited-password', 'Process')
            $global:gotenancyObservedRedisPassword = $null

            Set-Item -Path Function:\global:docker -Value { $global:LASTEXITCODE = 0 }
            Set-Item -Path Function:\global:go -Value {
                if ($args -contains './cache') {
                    $global:gotenancyObservedRedisPassword = [Environment]::GetEnvironmentVariable('GOTENANCY_REDIS_PASSWORD', 'Process')
                }
                $global:LASTEXITCODE = 0
            }

            . $runnerPath -KeepServices

            $global:gotenancyObservedRedisPassword | Should Be $null
            [Environment]::GetEnvironmentVariable($name, 'Process') | Should Be 'host-inherited-password'
        } finally {
            [Environment]::SetEnvironmentVariable($name, $previous, 'Process')
            Remove-Variable -Name gotenancyObservedRedisPassword -Scope Global -ErrorAction SilentlyContinue
            if ($previousDocker) {
                Set-Item -Path Function:\global:docker -Value $previousDocker.ScriptBlock
            } else {
                Remove-Item -Path Function:\docker -ErrorAction SilentlyContinue
            }
            if ($previousGo) {
                Set-Item -Path Function:\global:go -Value $previousGo.ScriptBlock
            } else {
                Remove-Item -Path Function:\go -ErrorAction SilentlyContinue
            }
        }
    }

    It 'runs the added SQL store contracts for both disposable database dialects' {
        $previousDocker = Get-Command docker -CommandType Function -ErrorAction SilentlyContinue
        $previousGo = Get-Command go -CommandType Function -ErrorAction SilentlyContinue
        try {
            $global:gotenancyDatabaseTestArguments = @()
            Set-Item -Path Function:\global:docker -Value { $global:LASTEXITCODE = 0 }
            Set-Item -Path Function:\global:go -Value {
                if ($args -contains './...') {
                    $global:gotenancyDatabaseTestArguments = @($args)
                }
                $global:LASTEXITCODE = 0
            }

            . $runnerPath -KeepServices

            ($global:gotenancyDatabaseTestArguments -contains '^Test(AuditSQLStore|SQLStore|QuotaSQLStore|RBACAndUserSQLStore|IdentitySQLStore|OIDCSQLLoginStore|FeatureSQLStore|PlanSQLStore|SubscriptionSQLStore)(MySQL|Postgres)Integration$') | Should Be $true
        } finally {
            Remove-Variable -Name gotenancyDatabaseTestArguments -Scope Global -ErrorAction SilentlyContinue
            if ($previousDocker) {
                Set-Item -Path Function:\global:docker -Value $previousDocker.ScriptBlock
            } else {
                Remove-Item -Path Function:\docker -ErrorAction SilentlyContinue
            }
            if ($previousGo) {
                Set-Item -Path Function:\global:go -Value $previousGo.ScriptBlock
            } else {
                Remove-Item -Path Function:\go -ErrorAction SilentlyContinue
            }
        }
    }

    It 'can emit target-package coverage for the database contracts' {
        $previousDocker = Get-Command docker -CommandType Function -ErrorAction SilentlyContinue
        $previousGo = Get-Command go -CommandType Function -ErrorAction SilentlyContinue
        try {
            $profile = Join-Path $TestDrive 'gotenancy-db-coverage.out'
            $global:gotenancyDatabaseCoverageArguments = @()
            Set-Item -Path Function:\global:docker -Value { $global:LASTEXITCODE = 0 }
            Set-Item -Path Function:\global:go -Value {
                if ($args -contains './...') {
                    $global:gotenancyDatabaseCoverageArguments = @($args)
                }
                $global:LASTEXITCODE = 0
            }

            . $runnerPath -KeepServices -CoverageProfile $profile

            $expectedCoveragePackages = @(
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
            ($global:gotenancyDatabaseCoverageArguments -contains '-covermode=atomic') | Should Be $true
            ($global:gotenancyDatabaseCoverageArguments -contains "-coverpkg=$expectedCoveragePackages") | Should Be $true
            ($global:gotenancyDatabaseCoverageArguments -contains "-coverprofile=$profile") | Should Be $true
        } finally {
            Remove-Variable -Name gotenancyDatabaseCoverageArguments -Scope Global -ErrorAction SilentlyContinue
            if ($previousDocker) {
                Set-Item -Path Function:\global:docker -Value $previousDocker.ScriptBlock
            } else {
                Remove-Item -Path Function:\docker -ErrorAction SilentlyContinue
            }
            if ($previousGo) {
                Set-Item -Path Function:\global:go -Value $previousGo.ScriptBlock
            } else {
                Remove-Item -Path Function:\go -ErrorAction SilentlyContinue
            }
        }
    }
}
