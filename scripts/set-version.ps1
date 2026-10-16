param(
    [Parameter(Mandatory = $true, Position = 0)]
    [string]$NewVersion,
    [switch]$SkipSync
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

# Clean up version string (strip leading 'v' if present)
$version = $NewVersion.TrimStart("v").Trim()
if (-not ($version -match '^\d+\.\d+\.\d+.*$')) {
    throw "Invalid semantic version: '$NewVersion'. Format must be like 1.0.0, 1.1.0, 1.0.1-beta.1"
}

Write-Host "Setting NPMS version to: $version"

# 1. Update VERSION file
$versionFile = Join-Path $root "VERSION"
Set-Content -Path $versionFile -Value $version -NoNewline
Write-Host "Updated $versionFile"

# 2. Update frontend/package.json
$pkgJsonPath = Join-Path $root "frontend\package.json"
if (Test-Path $pkgJsonPath) {
    $pkgContent = Get-Content $pkgJsonPath -Raw
    $updatedPkg = $pkgContent -replace '"version":\s*"[^"]+"', ('"version": "' + $version + '"')
    Set-Content -Path $pkgJsonPath -Value $updatedPkg -NoNewline
    Write-Host "Updated $pkgJsonPath"
}

# 3. Update backend/internal/version/version.go default fallback
$goVersionPath = Join-Path $root "backend\internal\version\version.go"
if (Test-Path $goVersionPath) {
    $goContent = Get-Content $goVersionPath -Raw
    $updatedGo = $goContent -replace 'Version\s*=\s*"[^"]+"', ('Version = "' + $version + '"')
    Set-Content -Path $goVersionPath -Value $updatedGo -NoNewline
    Write-Host "Updated $goVersionPath"
}

# 4. If not SkipSync, regenerate api-contract and sync frontend types
if (-not $SkipSync) {
    Write-Host "Regenerating API contract & frontend types..."
    Set-Location (Join-Path $root "backend")
    go run .\cmd\api-contract
    
    Set-Location (Join-Path $root "frontend")
    pnpm run api:generate
    
    Set-Location $root
}

Write-Host "Version bump to $version completed successfully!"
