param(
    [ValidateSet('moreno', 'auth', 'world')]
    [string]$Target = 'moreno',
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$ExtraArguments
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    $arguments = @()
    if ($Target -eq 'auth') { $arguments += '--auth' }
    if ($Target -eq 'world') { $arguments += '--world' }
    & go run . @arguments @ExtraArguments
    exit $LASTEXITCODE
} finally {
    Pop-Location
}
