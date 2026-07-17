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
                Remove-Item -Path Function:\global:docker -ErrorAction SilentlyContinue
            }
            if ($previousGo) {
                Set-Item -Path Function:\global:go -Value $previousGo.ScriptBlock
            } else {
                Remove-Item -Path Function:\global:go -ErrorAction SilentlyContinue
            }
        }
    }

    It 'runs the quota SQL contracts for both disposable database dialects' {
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

            ($global:gotenancyDatabaseTestArguments -contains '^Test(SQLStore|QuotaSQLStore)(MySQL|Postgres)Integration$') | Should Be $true
        } finally {
            Remove-Variable -Name gotenancyDatabaseTestArguments -Scope Global -ErrorAction SilentlyContinue
            if ($previousDocker) {
                Set-Item -Path Function:\global:docker -Value $previousDocker.ScriptBlock
            } else {
                Remove-Item -Path Function:\global:docker -ErrorAction SilentlyContinue
            }
            if ($previousGo) {
                Set-Item -Path Function:\global:go -Value $previousGo.ScriptBlock
            } else {
                Remove-Item -Path Function:\global:go -ErrorAction SilentlyContinue
            }
        }
    }
}
