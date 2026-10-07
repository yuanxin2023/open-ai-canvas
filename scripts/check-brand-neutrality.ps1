[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$latinMarker = -join ([char[]](121, 105, 110, 103, 99, 101))
$hanMarker = -join ([char[]](24433, 31574))
$violations = [System.Collections.Generic.List[string]]::new()

function Test-ForbiddenContent([byte[]]$Bytes) {
    $text = [System.Text.Encoding]::UTF8.GetString($Bytes)
    @(
        ($latinMarker + ".plugin/v1"),
        ($latinMarker + ".plugin/v2"),
        ($latinMarker + ".plugin/"),
        ($latinMarker + "\.plugin/"),
        ("." + $latinMarker + "-plugin")
    ) | ForEach-Object {
        $text = $text.Replace($_, "", [System.StringComparison]::OrdinalIgnoreCase)
    }
    return $text.IndexOf($latinMarker, [System.StringComparison]::OrdinalIgnoreCase) -ge 0 -or $text.Contains($hanMarker)
}

function Test-PackageEntries([string]$Path, [string]$DisplayPath) {
    $stream = [System.IO.File]::OpenRead($Path)
    try {
        $archive = [System.IO.Compression.ZipArchive]::new($stream, [System.IO.Compression.ZipArchiveMode]::Read)
        try {
            foreach ($entry in $archive.Entries) {
                if ([string]::IsNullOrEmpty($entry.Name)) { continue }
                $entryStream = $entry.Open()
                try {
                    $memory = [System.IO.MemoryStream]::new()
                    try {
                        $entryStream.CopyTo($memory)
                        if (Test-ForbiddenContent $memory.ToArray()) {
                            $violations.Add("压缩包内容: $DisplayPath!$($entry.FullName)")
                        }
                    } finally {
                        $memory.Dispose()
                    }
                } finally {
                    $entryStream.Dispose()
                }
            }
        } finally {
            $archive.Dispose()
        }
    } finally {
        $stream.Dispose()
    }
}

Push-Location $repoRoot
try {
    $pluginRoot = Join-Path $repoRoot "plugin-packages"
    $artifactOnlyPluginIds = @("autodl-comfyui")
    $sourcePluginIds = @(
        Get-ChildItem -LiteralPath $pluginRoot -Directory |
            Where-Object {
                -not $_.Name.StartsWith("src-", [System.StringComparison]::OrdinalIgnoreCase) -and
                (Test-Path -LiteralPath (Join-Path $_.FullName "manifest.json") -PathType Leaf)
            } |
            ForEach-Object { $_.Name }
    )
    $expectedArtifactIds = @($sourcePluginIds + $artifactOnlyPluginIds | Sort-Object -Unique)
    $actualArtifactIds = @(
        Get-ChildItem -LiteralPath $pluginRoot -File |
            Where-Object { $_.Name -like "*.canvas-plugin" -or $_.Name -like "*.lovwow-plugin" -or $_.Name -like "*.yingce-plugin" } |
            ForEach-Object { $_.BaseName } |
            Sort-Object -Unique
    )
    $artifactComparison = @(Compare-Object -ReferenceObject $expectedArtifactIds -DifferenceObject $actualArtifactIds)
    foreach ($difference in $artifactComparison) {
        if ($difference.SideIndicator -eq "<=") {
            $violations.Add("缺少插件包: plugin-packages/$($difference.InputObject).canvas-plugin 或兼容格式")
        } elseif ($difference.SideIndicator -eq ">=") {
            $violations.Add("多余插件包: plugin-packages/$($difference.InputObject)")
        }
    }

    $trackedAndUntracked = @(git ls-files --cached --others --exclude-standard)
    foreach ($relativePath in $trackedAndUntracked) {
        if ([string]::IsNullOrWhiteSpace($relativePath)) { continue }
        $fullPath = Join-Path $repoRoot $relativePath
        if (-not (Test-Path -LiteralPath $fullPath -PathType Leaf)) { continue }
        if ($relativePath.IndexOf($latinMarker, [System.StringComparison]::OrdinalIgnoreCase) -ge 0 -or $relativePath.Contains($hanMarker)) {
            $violations.Add("文件名: $relativePath")
        }
        $bytes = [System.IO.File]::ReadAllBytes($fullPath)
        if (Test-ForbiddenContent $bytes) {
            $violations.Add("文件内容: $relativePath")
        }
        if ($relativePath.EndsWith(".canvas-plugin", [System.StringComparison]::OrdinalIgnoreCase) -or $relativePath.EndsWith(".lovwow-plugin", [System.StringComparison]::OrdinalIgnoreCase) -or $relativePath.EndsWith(".yingce-plugin", [System.StringComparison]::OrdinalIgnoreCase)) {
            Test-PackageEntries $fullPath $relativePath
        }
    }
} finally {
    Pop-Location
}

if ($violations.Count -gt 0) {
    $violations | Sort-Object -Unique | ForEach-Object { Write-Error $_ }
    throw "发现禁止的第一方标识。"
}

Write-Host "品牌中性检查通过。" -ForegroundColor Green
