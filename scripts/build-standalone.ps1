$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

# 0. Terminate running NPMS processes to prevent file lock during compilation & copy
$processNames = @("npms", "npms-api", "npms-worker", "npms-db-migrate", "npms-snmp-debug", "npms-init")
foreach ($name in $processNames) {
    Get-Process -Name $name -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
}

# 1. Build frontend assets
Set-Location (Join-Path $root "frontend")
pnpm install --frozen-lockfile
pnpm run build

$assets = Join-Path $root "backend\web\dist\assets"
if (Test-Path $assets) { Remove-Item $assets -Recurse -Force }
Copy-Item (Join-Path (Get-Location) "dist\*") (Join-Path $root "backend\web\dist") -Recurse -Force

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

go run .\cmd\api-contract
if ((Get-Command git -ErrorAction SilentlyContinue) -and (Test-Path (Join-Path $root ".git"))) {
    git -C $root diff --exit-code -- docs/openapi.huma.generated.yaml
    if ($LASTEXITCODE -ne 0) {
        throw "Generated OpenAPI is out of date. Commit the regenerated docs/openapi.huma.generated.yaml before building standalone."
    }
}

go build -trimpath -ldflags="-s -w" -o (Join-Path $distBinDir "npms-api.exe") .\cmd\api
go build -trimpath -ldflags="-s -w" -o (Join-Path $distBinDir "npms-worker.exe") .\cmd\worker
go build -trimpath -ldflags="-s -w" -o (Join-Path $distBinDir "npms-db-migrate.exe") .\cmd\db-migrate
go build -trimpath -ldflags="-s -w" -o (Join-Path $distBinDir "npms-snmp-debug.exe") .\cmd\snmp-debug
go build -trimpath -ldflags="-s -w" -o (Join-Path $distBinDir "npms-init.exe") .\cmd\init


Copy-Item (Join-Path $distBinDir "*") $rootBinDir -Force
Copy-Item (Join-Path $distBinDir "npms-api.exe") (Join-Path $distDir "npms.exe") -Force
Copy-Item (Join-Path $distBinDir "npms-api.exe") (Join-Path $root "npms.exe") -Force

# 4. Copy config files
$cfgExample = Join-Path $root "config.example.yaml"
if (Test-Path $cfgExample) {
    Copy-Item $cfgExample (Join-Path $distDir "config.example.yaml") -Force
    
    # Do not create a runnable config from the example: it contains a placeholder token.
    # Run npms-init before first startup to generate secrets.
}

# 5. Copy profiles
$backendProfiles = Join-Path $root "backend\profiles"
if (Test-Path $backendProfiles) {
    Copy-Item (Join-Path $backendProfiles "*") $distProfilesDir -Recurse -Force
    Copy-Item (Join-Path $backendProfiles "*") $rootProfilesDir -Recurse -Force
}

Set-Location $root
Write-Host "Consolidated build completed successfully!"
Write-Host "Distribution bundle located at: $distDir"
