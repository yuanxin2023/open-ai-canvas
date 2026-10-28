[CmdletBinding()]
param(
    [switch]$NoBrowser,
    [int]$StartupTimeoutSeconds = 180,
    [string]$PreviewComposeFile = $env:CANVAS_PREVIEW_COMPOSE_FILE,
    [string]$PreviewProjectName = $env:CANVAS_PREVIEW_PROJECT_NAME
)

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$previewHelpers = Join-Path $PSScriptRoot "canvas-preview.ps1"
$localStarter = Join-Path $PSScriptRoot "start-local.ps1"
$healthUrl = "http://127.0.0.1:3000/api/health"
$canvasUrl = "http://localhost:3000/canvas"

. $previewHelpers

function Test-CanvasHttp([string]$Url) {
    try {
        $response = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 3
        return $response.StatusCode -ge 200 -and $response.StatusCode -lt 400
    } catch {
        return $false
    }
}

function Test-ListeningPort([int]$Port) {
    try {
        return $null -ne (Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction Stop | Select-Object -First 1)
    } catch {
        return $false
    }
}

$previewContext = Get-CanvasPreviewContext `
    -RepoRoot $repoRoot `
    -ComposeFile $PreviewComposeFile `
    -ProjectName $PreviewProjectName

if ($previewContext) {
    Assert-CanvasDockerReady
    $previewUrl = Get-CanvasPreviewUrl -Context $previewContext
    $previewHealthUrl = "$previewUrl/api/health"

    Write-Host "Starting the local preview services..." -ForegroundColor Cyan
    Invoke-CanvasPreviewCompose -Context $previewContext -Arguments @("up", "--detach", "--remove-orphans")

    $deadline = (Get-Date).AddSeconds($StartupTimeoutSeconds)
    do {
        if (Test-CanvasHealth -Url $previewHealthUrl) {
            Write-Host "AI Canvas is ready: $previewUrl" -ForegroundColor Green
            if (-not $NoBrowser) {
                Start-Process $previewUrl
            }
            exit 0
        }
        Start-Sleep -Seconds 2
    } while ((Get-Date) -lt $deadline)

    Write-Host "The services did not become healthy within $StartupTimeoutSeconds seconds. Recent logs:" -ForegroundColor Red
    Invoke-CanvasPreviewCompose -Context $previewContext -Arguments @("logs", "--tail", "80")
    throw "The local preview timed out: $previewHealthUrl"
}

if (Test-CanvasHttp $healthUrl) {
    Write-Host "AI Canvas is already running." -ForegroundColor Green
    if (-not $NoBrowser) {
        Start-Process $canvasUrl
    }
    exit 0
}

$occupiedPorts = @(3000, 8080) | Where-Object { Test-ListeningPort $_ }
if ($occupiedPorts.Count -gt 0) {
    throw "Port(s) $($occupiedPorts -join ', ') are already in use. AI Canvas cannot start safely."
}

if (-not (Test-Path -LiteralPath $localStarter)) {
    throw "The local startup script was not found: $localStarter"
}

Write-Host "Starting the AI Canvas frontend and backend..." -ForegroundColor Cyan
& $localStarter

$deadline = (Get-Date).AddMinutes(3)
do {
    Start-Sleep -Seconds 2
    if (Test-CanvasHttp $healthUrl) {
        Write-Host "AI Canvas is ready." -ForegroundColor Green
        if (-not $NoBrowser) {
            Start-Process $canvasUrl
        }
        exit 0
    }
} while ((Get-Date) -lt $deadline)

throw "AI Canvas was launched but did not pass its health check within 3 minutes."
