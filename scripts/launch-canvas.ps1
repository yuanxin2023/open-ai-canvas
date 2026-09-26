# AI 创作工作台一键启动器
# 默认启动后端和前端；提供隧道配置时再启动 cloudflared。

[CmdletBinding()]
param(
    [string]$PublicBaseUrl = $env:CANVAS_PUBLIC_BASE_URL,
    [string]$CloudflaredConfig = $env:CLOUDFLARED_CONFIG
)

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$backendDir = Join-Path $repoRoot "backend"
$webDir = Join-Path $repoRoot "web"
$dataDir = Join-Path $repoRoot ".local\project-workbench-debug"
$goBuildCache = Join-Path $repoRoot ".local\cache\go-build"
$goModuleCache = Join-Path $repoRoot ".local\cache\go-mod"
$cloudflaredCommand = Get-Command cloudflared -ErrorAction SilentlyContinue
$cloudflaredExe = if ($cloudflaredCommand) { $cloudflaredCommand.Source } else { "C:\Program Files (x86)\cloudflared\cloudflared.exe" }
$bunCommand = Get-Command bun -ErrorAction SilentlyContinue
if (-not $bunCommand) { throw "未找到 Bun，请先安装 Bun 后再启动项目。" }
$bunExe = $bunCommand.Source
$localUrl = "http://localhost:3000"
$healthUrl = "http://127.0.0.1:3000/api/health"

function Test-PortListen([int]$Port) {
    try {
        return $null -ne (Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction Stop | Select-Object -First 1)
    } catch {
        return $false
    }
}

function Test-Http([string]$Url) {
    try {
        $r = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 3
        return $r.StatusCode -ge 200 -and $r.StatusCode -lt 400
    } catch {
        return $false
    }
}

# 0) 已全部就绪则直接打开
if ((Test-PortListen 8080) -and (Test-PortListen 3000)) {
    Write-Host "AI 创作工作台已在运行，正在打开..." -ForegroundColor Green
    Start-Process $localUrl
    exit 0
}

# 1) 后端：8080
if (-not (Test-PortListen 8080)) {
    Write-Host "启动后端 (8080)..." -ForegroundColor Cyan
    New-Item -ItemType Directory -Force -Path $dataDir, $goBuildCache, $goModuleCache | Out-Null
    $env:CANVAS_BACKEND_ADDR = "127.0.0.1:8080"
    $env:CANVAS_BACKEND_DATA_DIR = $dataDir
    if (-not [string]::IsNullOrWhiteSpace($PublicBaseUrl)) {
        $env:CANVAS_PUBLIC_BASE_URL = $PublicBaseUrl.TrimEnd("/")
    }
    $env:GOPROXY = "https://goproxy.cn,direct"
    $env:GOCACHE = $goBuildCache
    $env:GOMODCACHE = $goModuleCache
    Start-Process -FilePath "go" -ArgumentList "run", "./cmd/server" `
        -WorkingDirectory $backendDir `
        -RedirectStandardOutput (Join-Path $repoRoot ".local\backend.stdout.log") `
        -RedirectStandardError (Join-Path $repoRoot ".local\backend.stderr.log") `
        -WindowStyle Hidden | Out-Null
}

# 2) 前端：3000
if (-not (Test-PortListen 3000)) {
    Write-Host "启动前端 (3000)..." -ForegroundColor Cyan
    $viteBinary = Join-Path $webDir "node_modules\.bin\vite"
    if (-not (Test-Path -LiteralPath $viteBinary)) {
        Write-Host "安装前端依赖..." -ForegroundColor Yellow
        Push-Location $webDir
        try { & $bunExe install --frozen-lockfile } finally { Pop-Location }
    }
    $env:VITE_API_PROXY_TARGET = "http://127.0.0.1:8080"
    Start-Process -FilePath $bunExe -ArgumentList "run", "dev" `
        -WorkingDirectory $webDir `
        -RedirectStandardOutput (Join-Path $repoRoot ".local\web.stdout.log") `
        -RedirectStandardError (Join-Path $repoRoot ".local\web.stderr.log") `
        -WindowStyle Hidden | Out-Null
}

# 3) 可选的 cloudflared 隧道
if (-not [string]::IsNullOrWhiteSpace($CloudflaredConfig) -and -not (Get-Process -Name cloudflared -ErrorAction SilentlyContinue)) {
    if ((Test-Path -LiteralPath $cloudflaredExe) -and (Test-Path -LiteralPath $CloudflaredConfig)) {
        Write-Host "启动公网隧道 cloudflared..." -ForegroundColor Cyan
        Start-Process -FilePath $cloudflaredExe -ArgumentList "--config", $CloudflaredConfig, "tunnel", "run" -WindowStyle Hidden | Out-Null
    } else {
        Write-Host "未找到 cloudflared 或隧道配置，跳过公网隧道（本地仍可访问）" -ForegroundColor Yellow
    }
} elseif (Get-Process -Name cloudflared -ErrorAction SilentlyContinue) {
    Write-Host "公网隧道 cloudflared 已在运行" -ForegroundColor DarkGray
}

# 4) 等待健康检查并打开浏览器
$deadline = (Get-Date).AddMinutes(3)
do {
    Start-Sleep -Seconds 2
    if (Test-Http $healthUrl) {
        Write-Host "AI 创作工作台已启动，正在打开浏览器..." -ForegroundColor Green
        Start-Sleep -Seconds 1
        Start-Process $localUrl
        Write-Host "本地: $localUrl" -ForegroundColor Green
        if (-not [string]::IsNullOrWhiteSpace($PublicBaseUrl)) {
            Write-Host "公网: $($PublicBaseUrl.TrimEnd('/'))" -ForegroundColor Green
        }
        exit 0
    }
} while ((Get-Date) -lt $deadline)

Write-Host "启动超时，请查看日志：.local\backend.stderr.log / .local\web.stderr.log" -ForegroundColor Red
