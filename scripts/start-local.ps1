[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$backendDir = Join-Path $repoRoot "backend"
$webDir = Join-Path $repoRoot "web"
$dataDir = Join-Path $repoRoot ".local\project-workbench-debug"
$goBuildCache = Join-Path $repoRoot ".local\cache\go-build"
$goModuleCache = Join-Path $repoRoot ".local\cache\go-mod"
$bundledGccBin = Join-Path $repoRoot ".local\tools\w64devkit\bin"

foreach ($commandName in @("go", "bun")) {
    if (-not (Get-Command $commandName -ErrorAction SilentlyContinue)) {
        throw "$commandName was not found. Install the required runtime first."
    }
}

foreach ($directory in @($dataDir, $goBuildCache, $goModuleCache)) {
    New-Item -ItemType Directory -Force -Path $directory | Out-Null
}

$viteBinary = Join-Path $webDir "node_modules\.bin\vite.exe"
if (-not (Test-Path -LiteralPath $viteBinary)) {
    Write-Host "web/node_modules is missing. Running bun install --frozen-lockfile..." -ForegroundColor Yellow
    Push-Location $webDir
    try {
        & bun install --frozen-lockfile
        if ($LASTEXITCODE -ne 0) {
            throw "bun install failed. The frontend cannot be started."
        }
    } finally {
        Pop-Location
    }
}

$gccCommand = Get-Command gcc -ErrorAction SilentlyContinue
if ($gccCommand) {
    $gccBin = Split-Path -Parent $gccCommand.Source
} elseif (Test-Path -LiteralPath (Join-Path $bundledGccBin "gcc.exe")) {
    $gccBin = $bundledGccBin
} else {
    throw "GCC was not found. SQLite requires a C compiler on Windows."
}

function Test-ListeningPort([int]$Port) {
    try {
        return $null -ne (Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction Stop | Select-Object -First 1)
    } catch {
        return $false
    }
}

foreach ($port in @(3000, 8080)) {
    if (Test-ListeningPort $port) {
        throw "Port $port is already in use. Stop the process and try again."
    }
}

$powerShellPath = (Get-Command pwsh -ErrorAction SilentlyContinue).Source
if (-not $powerShellPath) {
    $powerShellPath = (Get-Command powershell.exe -ErrorAction SilentlyContinue).Source
}
if (-not $powerShellPath) {
    throw "PowerShell was not found. Cannot open service windows."
}

function ConvertTo-PowerShellLiteral([string]$Value) {
    return "'" + $Value.Replace("'", "''") + "'"
}

$backendDirLiteral = ConvertTo-PowerShellLiteral $backendDir
$webDirLiteral = ConvertTo-PowerShellLiteral $webDir
$dataDirLiteral = ConvertTo-PowerShellLiteral $dataDir
$gccPathPrefixLiteral = ConvertTo-PowerShellLiteral ($gccBin + ";")

$backendCommand = @"
`$ErrorActionPreference = 'Stop'
Set-Location -LiteralPath $backendDirLiteral
`$env:CANVAS_BACKEND_ADDR = '127.0.0.1:8080'
`$env:CANVAS_BACKEND_DATA_DIR = $dataDirLiteral
`$env:GOCACHE = $(ConvertTo-PowerShellLiteral $goBuildCache)
`$env:GOMODCACHE = $(ConvertTo-PowerShellLiteral $goModuleCache)
`$env:CGO_ENABLED = '1'
`$env:CC = 'gcc'
`$env:Path = $gccPathPrefixLiteral + `$env:Path
Write-Host 'Backend: http://127.0.0.1:8080' -ForegroundColor Cyan
go run ./cmd/server
"@

$webCommand = @"
`$ErrorActionPreference = 'Stop'
Set-Location -LiteralPath $webDirLiteral
`$env:VITE_API_PROXY_TARGET = 'http://127.0.0.1:8080'
Write-Host 'Frontend: http://localhost:3000' -ForegroundColor Cyan
bun run dev
"@

$backendProcess = Start-Process -FilePath $powerShellPath -WindowStyle Normal -WorkingDirectory $backendDir -PassThru -ArgumentList @("-NoLogo", "-NoExit", "-NoProfile", "-Command", $backendCommand)
$webProcess = Start-Process -FilePath $powerShellPath -WindowStyle Normal -WorkingDirectory $webDir -PassThru -ArgumentList @("-NoLogo", "-NoExit", "-NoProfile", "-Command", $webCommand)

Write-Host "Frontend and backend windows opened." -ForegroundColor Green
Write-Host "Backend PID: $($backendProcess.Id); Frontend PID: $($webProcess.Id)"
Write-Host "Open http://localhost:3000. Press Ctrl+C in each window to stop."
