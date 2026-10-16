param(
    [switch]$SkipFrontend
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

# 0. Terminate running NPMS processes to prevent file lock during compilation & copy
$processNames = @("npms", "npms-api", "npms-worker", "npms-db-migrate", "npms-snmp-debug", "npms-init")
foreach ($name in $processNames) {
    Get-Process -Name $name -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
}

# 0.5 Version and build metadata
$versionFile = Join-Path $root "VERSION"
$version = if (Test-Path $versionFile) { (Get-Content $versionFile -Raw).Trim() } else { "1.0.0" }
$gitCommit = try { (git rev-parse --short HEAD 2>$null).Trim() } catch { "" }
$buildTime = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")

Write-Host "Building NPMS version: $version (commit: $gitCommit, time: $buildTime)"

# 1. Build frontend assets
$frontendDir = Join-Path $root "frontend"
$frontendDist = Join-Path $frontendDir "dist"
if (-not $SkipFrontend -or -not (Test-Path (Join-Path $frontendDist "index.html"))) {
    Set-Location $frontendDir
    $env:VITE_APP_VERSION = $version
    $env:VITE_APP_COMMIT = $gitCommit
    $env:VITE_APP_BUILD_TIME = $buildTime
    pnpm install --frozen-lockfile
    pnpm run build
}

$webDist = Join-Path $root "backend\web\dist"
if (!(Test-Path $webDist)) { New-Item -ItemType Directory -Path $webDist -Force | Out-Null }
$assets = Join-Path $webDist "assets"
if (Test-Path -LiteralPath $assets) {
    Remove-Item -LiteralPath $assets -Recurse -Force -ErrorAction SilentlyContinue
}
Copy-Item (Join-Path $frontendDist "*") $webDist -Recurse -Force

# 2. Setup distribution directories
$distDir = Join-Path $root "dist"
$distBinDir = Join-Path $distDir "bin"
$distProfilesDir = Join-Path $distDir "profiles"
$distDataDir = Join-Path $distDir "data"
$rootBinDir = Join-Path $root "bin"
$rootProfilesDir = Join-Path $root "profiles"
$rootDataDir = Join-Path $root "data"

foreach ($dir in @($distDir, $distBinDir, $distProfilesDir, $distDataDir, $rootBinDir, $rootProfilesDir, $rootDataDir)) {
    if (!(Test-Path $dir)) { New-Item -ItemType Directory -Path $dir | Out-Null }
}

# 3. Build backend Go executables
Set-Location (Join-Path $root "backend")
go mod download

$openApiFile = Join-Path $root "docs\openapi.huma.generated.yaml"
$beforeHash = if (Test-Path $openApiFile) { (Get-FileHash $openApiFile -Algorithm SHA256).Hash } else { "" }
go run .\cmd\api-contract
$afterHash = if (Test-Path $openApiFile) { (Get-FileHash $openApiFile -Algorithm SHA256).Hash } else { "" }
if ($beforeHash -ne $afterHash) {
    throw "Generated OpenAPI was out of date and has been updated. Run 'pnpm run api:generate' in frontend/ and rebuild."
}

$ldflags = "-s -w -X npms/backend/internal/version.Version=$version -X npms/backend/internal/version.GitCommit=$gitCommit -X npms/backend/internal/version.BuildDate=$buildTime"

go build -trimpath -ldflags "$ldflags" -o (Join-Path $distBinDir "npms-api.exe") .\cmd\api
go build -trimpath -ldflags "$ldflags" -o (Join-Path $distBinDir "npms-worker.exe") .\cmd\worker
go build -trimpath -ldflags "$ldflags" -o (Join-Path $distBinDir "npms-db-migrate.exe") .\cmd\db-migrate
go build -trimpath -ldflags "$ldflags" -o (Join-Path $distBinDir "npms-snmp-debug.exe") .\cmd\snmp-debug
go build -trimpath -ldflags "$ldflags" -o (Join-Path $distBinDir "npms-init.exe") .\cmd\init


Copy-Item (Join-Path $distBinDir "*") $rootBinDir -Force
Copy-Item (Join-Path $distBinDir "npms-api.exe") (Join-Path $distDir "npms.exe") -Force
Copy-Item (Join-Path $distBinDir "npms-api.exe") (Join-Path $root "npms.exe") -Force

# 4. Copy config & MIB files
$cfgExample = Join-Path $root "config.example.yaml"
if (Test-Path $cfgExample) {
    Copy-Item $cfgExample (Join-Path $distDir "config.example.yaml") -Force
    
    # Do not create a runnable config from the example: it contains a placeholder token.
    # Run npms-init before first startup to generate secrets.
}

$printerMIB = Join-Path $root "Printer-MIB.yaml"
if (Test-Path $printerMIB) {
    Copy-Item $printerMIB (Join-Path $distDir "Printer-MIB.yaml") -Force
}

# 5. Copy profiles & Printer-MIB sample fixtures
$backendProfiles = Join-Path $root "backend\profiles"
if (Test-Path $backendProfiles) {
    Copy-Item (Join-Path $backendProfiles "*") $distProfilesDir -Recurse -Force
    Copy-Item (Join-Path $backendProfiles "*") $rootProfilesDir -Recurse -Force
}

$printerMIBFixtures = Join-Path $root "backend\testdata\printer-mib"
$distFixturesDir = Join-Path $distDir "testdata\printer-mib"
if (Test-Path $printerMIBFixtures) {
    if (!(Test-Path $distFixturesDir)) { New-Item -ItemType Directory -Path $distFixturesDir -Force | Out-Null }
    Copy-Item (Join-Path $printerMIBFixtures "*") $distFixturesDir -Recurse -Force
}

Set-Location $root
Write-Host "Consolidated build completed successfully!"
Write-Host "Distribution bundle located at: $distDir"
