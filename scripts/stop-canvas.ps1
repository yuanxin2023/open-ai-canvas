[CmdletBinding()]
param(
    [int]$ShutdownTimeoutSeconds = 30,
    [string]$PreviewComposeFile = $env:CANVAS_PREVIEW_COMPOSE_FILE,
    [string]$PreviewProjectName = $env:CANVAS_PREVIEW_PROJECT_NAME
)

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$previewHelpers = Join-Path $PSScriptRoot "canvas-preview.ps1"
$localStopper = Join-Path $PSScriptRoot "stop-local.ps1"

. $previewHelpers

$previewContext = Get-CanvasPreviewContext `
    -RepoRoot $repoRoot `
    -ComposeFile $PreviewComposeFile `
    -ProjectName $PreviewProjectName

if (-not $previewContext) {
    & $localStopper
    exit $LASTEXITCODE
}

Assert-CanvasDockerReady
$previewUrl = Get-CanvasPreviewUrl -Context $previewContext
$previewPort = ([Uri]$previewUrl).Port

Write-Host "Stopping the local preview services..." -ForegroundColor Cyan
Invoke-CanvasPreviewCompose -Context $previewContext -Arguments @("stop", "--timeout", "$ShutdownTimeoutSeconds")

$deadline = (Get-Date).AddSeconds([Math]::Min($ShutdownTimeoutSeconds + 10, 120))
do {
    $listener = Get-NetTCPConnection -LocalPort $previewPort -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    $runningServices = @(
        Invoke-CanvasPreviewCompose -Context $previewContext -Arguments @("ps", "--status", "running", "--services") -CaptureOutput |
            Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_) }
    )
    if (-not $listener -and -not $runningServices.Count) {
        Write-Host "AI Canvas has stopped. Port $previewPort is free." -ForegroundColor Green
        exit 0
    }
    Start-Sleep -Seconds 1
} while ((Get-Date) -lt $deadline)

throw "The local preview did not stop completely. Run docker compose ps to inspect the remaining services."
