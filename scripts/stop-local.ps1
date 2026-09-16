[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$targetPorts = @(3000, 8080)
$stopped = New-Object System.Collections.Generic.HashSet[int]

function Get-ListeningProcessId([int]$Port) {
    try {
        return Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction Stop |
            Select-Object -ExpandProperty OwningProcess -Unique
    } catch {
        $rows = netstat -ano -p tcp | Select-String -Pattern "^\s*TCP\s+\S+:$Port\s+\S+\s+LISTENING\s+(\d+)\s*$"
        return @($rows | ForEach-Object { [int]$_.Matches[0].Groups[1].Value } | Select-Object -Unique)
    }
}

function Stop-ProcessTree([int]$ProcessId, [string]$Reason) {
    if ($ProcessId -le 0 -or $stopped.Contains($ProcessId)) {
        return
    }

    $process = Get-Process -Id $ProcessId -ErrorAction SilentlyContinue
    if (-not $process) {
        return
    }

    Write-Host "Stopping $Reason (PID $ProcessId, $($process.ProcessName))..." -ForegroundColor Yellow
    & taskkill.exe /PID $ProcessId /T /F 2>$null | Out-Null
    if ($LASTEXITCODE -ne 0) {
        Stop-Process -Id $ProcessId -Force -ErrorAction SilentlyContinue
    }
    [void]$stopped.Add($ProcessId)
}

foreach ($port in $targetPorts) {
    $processIds = @(Get-ListeningProcessId $port)
    if (-not $processIds.Count) {
        Write-Host "Port $port is not in use." -ForegroundColor DarkGray
        continue
    }

    foreach ($processId in $processIds) {
        Stop-ProcessTree -ProcessId $processId -Reason "service on port $port"
    }
}

# 清理由一键启动窗口留下的项目专属父进程，避免关闭服务后仍残留空白终端。
$repoPattern = [Regex]::Escape($repoRoot)
$projectProcesses = Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object {
    $_.ProcessId -ne $PID -and
    $_.Name -match '^(go|bun|node|npm|pwsh|powershell)(\.exe)?$' -and
    $_.CommandLine -match $repoPattern -and
    $_.CommandLine -match '(cmd/server|bun\s+run\s+dev|npm(?:\.cmd)?\s+run\s+dev|vite)'
}

foreach ($process in $projectProcesses) {
    Stop-ProcessTree -ProcessId ([int]$process.ProcessId) -Reason "open-ai-canvas helper"
}

Start-Sleep -Milliseconds 500
$remainingPorts = @($targetPorts | Where-Object { @(Get-ListeningProcessId $_).Count -gt 0 })
if ($remainingPorts.Count) {
    throw "Failed to release port(s): $($remainingPorts -join ', ')"
}

Write-Host "open-ai-canvas has stopped. Ports 3000 and 8080 are free." -ForegroundColor Green
