$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

Set-Location (Join-Path $root "frontend")
npm ci
npm run build

$assets = Join-Path $root "backend\web\dist\assets"
if (Test-Path $assets) { Remove-Item $assets -Recurse -Force }
Copy-Item (Join-Path (Get-Location) "dist\*") (Join-Path $root "backend\web\dist") -Recurse -Force

Set-Location (Join-Path $root "backend")
go mod download
go build -trimpath -ldflags="-s -w" -o (Join-Path $root "npms.exe") .\cmd\npms
Write-Host "Built $(Join-Path $root 'npms.exe')"
