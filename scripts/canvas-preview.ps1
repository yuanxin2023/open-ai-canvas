function Get-CanvasPreviewContext {
    param(
        [Parameter(Mandatory = $true)]
        [string]$RepoRoot,

        [string]$ComposeFile = $env:CANVAS_PREVIEW_COMPOSE_FILE,

        [string]$ProjectName = $env:CANVAS_PREVIEW_PROJECT_NAME
    )

    if ([string]::IsNullOrWhiteSpace($ComposeFile)) {
        $preferredComposeFile = Join-Path $RepoRoot ".local\cache\upstream-v171-preview\preview-compose.yml"
        if (Test-Path -LiteralPath $preferredComposeFile) {
            $ComposeFile = $preferredComposeFile
        } else {
            $ComposeFile = Get-ChildItem -LiteralPath (Join-Path $RepoRoot ".local\cache") `
                -Filter "preview-compose.yml" -File -Recurse -ErrorAction SilentlyContinue |
                Sort-Object LastWriteTime -Descending |
                Select-Object -First 1 -ExpandProperty FullName
        }
    }

    if ([string]::IsNullOrWhiteSpace($ComposeFile) -or -not (Test-Path -LiteralPath $ComposeFile)) {
        return $null
    }

    $ComposeFile = (Resolve-Path -LiteralPath $ComposeFile).Path
    $environmentFile = Join-Path (Split-Path -Parent $ComposeFile) "preview.env"
    if (-not (Test-Path -LiteralPath $environmentFile)) {
        throw "Preview environment file was not found: $environmentFile"
    }

    if ([string]::IsNullOrWhiteSpace($ProjectName)) {
        $ProjectName = "codex-canvas-v171-preview"
    }

    return [pscustomobject]@{
        ComposeFile = $ComposeFile
        EnvironmentFile = $environmentFile
        ProjectName = $ProjectName
    }
}

function Assert-CanvasDockerReady {
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
        throw "Docker was not found. Start Docker Desktop and try again."
    }

    & docker info *> $null
    if ($LASTEXITCODE -ne 0) {
        throw "Docker is not ready. Start Docker Desktop and try again."
    }
}

function Invoke-CanvasPreviewCompose {
    param(
        [Parameter(Mandatory = $true)]
        [psobject]$Context,

        [Parameter(Mandatory = $true)]
        [string[]]$Arguments,

        [switch]$CaptureOutput
    )

    $dockerArguments = @(
        "compose",
        "--env-file", $Context.EnvironmentFile,
        "--project-name", $Context.ProjectName,
        "--file", $Context.ComposeFile
    ) + $Arguments

    if ($CaptureOutput) {
        $output = & docker @dockerArguments
        if ($LASTEXITCODE -ne 0) {
            throw "Docker Compose failed with exit code $LASTEXITCODE."
        }
        return $output
    }

    & docker @dockerArguments
    if ($LASTEXITCODE -ne 0) {
        throw "Docker Compose failed with exit code $LASTEXITCODE."
    }
}

function Get-CanvasPreviewUrl {
    param(
        [Parameter(Mandatory = $true)]
        [psobject]$Context
    )

    $configText = Invoke-CanvasPreviewCompose -Context $Context -Arguments @("config", "--format", "json") -CaptureOutput
    $config = ($configText -join [Environment]::NewLine) | ConvertFrom-Json
    $webPort = @($config.services.web.ports | Where-Object { $_.target -eq 3000 } | Select-Object -First 1)
    if (-not $webPort.Count -or [string]::IsNullOrWhiteSpace([string]$webPort[0].published)) {
        throw "The preview Compose file does not publish the web service port 3000."
    }

    $hostName = if ($webPort[0].host_ip -in @("0.0.0.0", "::", "")) { "localhost" } else { $webPort[0].host_ip }
    return "http://${hostName}:$($webPort[0].published)"
}

function Test-CanvasHealth {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Url
    )

    try {
        $response = Invoke-RestMethod -Uri $Url -TimeoutSec 5
        return $response.code -eq 0 -and $response.data.ready -eq $true -and $response.data.schema.ready -eq $true
    } catch {
        return $false
    }
}
