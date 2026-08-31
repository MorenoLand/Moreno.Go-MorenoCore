param(
    [string]$AuthDump,
    [string]$CharactersDump,
    [string]$WorldDump,
    [string]$DataArchive,
    [string]$CustomConfigDirectory,
    [switch]$Force
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$bin = Join-Path $root 'bin'
New-Item -ItemType Directory -Force -Path $bin | Out-Null
if ($CustomConfigDirectory) {
    Copy-Item -LiteralPath (Join-Path $CustomConfigDirectory 'authserver.conf') -Destination (Join-Path $bin 'authserver.conf') -Force
    Copy-Item -LiteralPath (Join-Path $CustomConfigDirectory 'worldserver.conf') -Destination (Join-Path $bin 'worldserver.conf') -Force
} else {
    if (-not (Test-Path -LiteralPath (Join-Path $bin 'authserver.conf'))) {
        Copy-Item -LiteralPath (Join-Path $root 'configs\authserver.conf.dist') -Destination (Join-Path $bin 'authserver.conf')
        $authConfig = Get-Content -Raw -LiteralPath (Join-Path $bin 'authserver.conf')
        $authConfig = [regex]::Replace($authConfig, '(?m)^DataDir\s*=.*$', 'DataDir = "bin"')
        Set-Content -LiteralPath (Join-Path $bin 'authserver.conf') -Value $authConfig -NoNewline
    }
    if (-not (Test-Path -LiteralPath (Join-Path $bin 'worldserver.conf'))) {
        Copy-Item -LiteralPath (Join-Path $root 'configs\worldserver.conf.dist') -Destination (Join-Path $bin 'worldserver.conf')
        $worldConfig = Get-Content -Raw -LiteralPath (Join-Path $bin 'worldserver.conf')
        $worldConfig = [regex]::Replace($worldConfig, '(?m)^DataDir\s*=.*$', 'DataDir = "bin"')
        $worldConfig = [regex]::Replace($worldConfig, '(?m)^Eluna\.ScriptPath\s*=.*$', 'Eluna.ScriptPath = "bin/lua_scripts"')
        Set-Content -LiteralPath (Join-Path $bin 'worldserver.conf') -Value $worldConfig -NoNewline
    }
}
if ($DataArchive) { Expand-Archive -LiteralPath $DataArchive -DestinationPath $root -Force }
$dumps = @($AuthDump, $CharactersDump, $WorldDump) | Where-Object { $_ }
if ($dumps.Count -ne 0 -and $dumps.Count -ne 3) { throw 'AuthDump, CharactersDump, and WorldDump must be supplied together.' }
if ($dumps.Count -eq 3) {
    $args = @('./tools/dbtool', 'import-sql', '--output-dir', $bin, '--auth', $AuthDump, '--characters', $CharactersDump, '--world', $WorldDump)
    if ($Force) { $args += '--force' }
    & go run @args
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
Write-Output "Setup files are in $bin"
