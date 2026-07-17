param(
    [Parameter(Mandatory = $true)]
    [string]$Profile,
    [double]$Minimum = 85.0
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
