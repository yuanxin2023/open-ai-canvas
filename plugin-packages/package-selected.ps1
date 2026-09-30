$ErrorActionPreference = 'Stop'

Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem

$root = Split-Path -Parent $PSCommandPath
$ids = if ($args.Count -gt 0) { $args } else { throw 'Pass one or more plugin directory names.' }

foreach ($id in $ids) {
  if ($id -like 'src-*') {
    throw "Source mirror directories cannot be packaged directly: $id"
  }
  $source = Join-Path $root $id
  $manifestPath = Join-Path $source 'manifest.json'
  if (-not (Test-Path -LiteralPath $manifestPath)) {
    throw "Missing manifest.json for $id"
  }
  $manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
  $extension = switch -Wildcard ([string]$manifest.apiVersion) {
    'yingce.plugin/*' { '.yingce-plugin'; break }
    'lovwow.plugin/*' { '.lovwow-plugin'; break }
    default { '.canvas-plugin' }
  }
  $output = Join-Path $root ($id + $extension)
  $temporary = Join-Path $root ('.' + $id + $extension + '.tmp')
  if (Test-Path -LiteralPath $temporary) { Remove-Item -LiteralPath $temporary -Force }

  $archive = [System.IO.Compression.ZipFile]::Open($temporary, [System.IO.Compression.ZipArchiveMode]::Create)
  try {
    Get-ChildItem -LiteralPath $source -Recurse -File | Where-Object {
      $_.Name -in @('manifest.json', 'README.md', 'LICENSE') -or
      $_.FullName -match '\\(docs|assets|web|backend)\\'
    } | ForEach-Object {
      $entryName = $_.FullName.Substring($source.Length + 1).Replace('\', '/')
      [System.IO.Compression.ZipFileExtensions]::CreateEntryFromFile(
        $archive, $_.FullName, $entryName, [System.IO.Compression.CompressionLevel]::Optimal
      ) | Out-Null
    }
  } finally {
    $archive.Dispose()
  }
  Move-Item -LiteralPath $temporary -Destination $output -Force
  @('.yingce-plugin', '.lovwow-plugin', '.canvas-plugin') | Where-Object { $_ -ne $extension } | ForEach-Object {
    $staleOutput = Join-Path $root ($id + $_)
    if (Test-Path -LiteralPath $staleOutput) { Remove-Item -LiteralPath $staleOutput -Force }
  }
  Write-Output "$id -> $output"
}
