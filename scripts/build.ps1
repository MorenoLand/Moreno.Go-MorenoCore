param(
    [ValidateSet('moreno', 'auth', 'world', 'gameserver', 'all')]
    [string]$Target = 'moreno',
    [string]$OutputDirectory = 'bin'
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    New-Item -ItemType Directory -Force -Path $OutputDirectory | Out-Null
    switch ($Target) {
        'moreno' { & go build -trimpath -o (Join-Path $OutputDirectory 'MorenoCore.exe') . }
        'auth' { & go build -trimpath -o (Join-Path $OutputDirectory 'AuthServer.exe') ./server/authserver }
        'world' { & go build -trimpath -o (Join-Path $OutputDirectory 'WorldServer.exe') ./server/worldserver }
        'gameserver' { & go build -trimpath -o (Join-Path $OutputDirectory 'WorldServer.exe') ./server/worldserver }
        'all' {
            & go build -trimpath -o (Join-Path $OutputDirectory 'MorenoCore.exe') .
            if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
            & go build -trimpath -o (Join-Path $OutputDirectory 'AuthServer.exe') ./server/authserver
            if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
            & go build -trimpath -o (Join-Path $OutputDirectory 'WorldServer.exe') ./server/worldserver
        }
    }
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} finally {
    Pop-Location
}
