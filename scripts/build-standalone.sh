#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# 0. Terminate running NPMS processes to prevent file lock during compilation & copy
pkill -f npms-api 2>/dev/null || true
pkill -f npms-worker 2>/dev/null || true
pkill -f npms-db-migrate 2>/dev/null || true
pkill -f npms-snmp-debug 2>/dev/null || true
pkill -f npms-init 2>/dev/null || true
pkill -x npms 2>/dev/null || true
taskkill //F //IM npms.exe 2>/dev/null || true
taskkill //F //IM npms-api.exe 2>/dev/null || true
taskkill //F //IM npms-worker.exe 2>/dev/null || true
taskkill //F //IM npms-init.exe 2>/dev/null || true

# 0.5 Version and build metadata
VERSION_FILE="$ROOT_DIR/VERSION"
if [ -f "$VERSION_FILE" ]; then
    VERSION="$(tr -d ' \r\n' < "$VERSION_FILE")"
else
    VERSION="1.0.0"
fi
GIT_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo "")"
BUILD_TIME="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

echo "Building NPMS version: $VERSION (commit: $GIT_COMMIT, time: $BUILD_TIME)"

# 1. Build frontend assets

cd "$ROOT_DIR/frontend"
export VITE_APP_VERSION="$VERSION"
export VITE_APP_COMMIT="$GIT_COMMIT"
export VITE_APP_BUILD_TIME="$BUILD_TIME"
pnpm install --frozen-lockfile
pnpm run build

rm -rf "$ROOT_DIR/backend/web/dist/assets"
cp -R dist/. "$ROOT_DIR/backend/web/dist/"

# 2. Setup distribution directories
DIST_DIR="$ROOT_DIR/dist"
mkdir -p "$DIST_DIR/bin" "$DIST_DIR/profiles" "$DIST_DIR/data" "$ROOT_DIR/bin" "$ROOT_DIR/profiles" "$ROOT_DIR/data"

# 3. Build backend Go executables
cd "$ROOT_DIR/backend"
go mod download
go run ./cmd/api-contract
if command -v git >/dev/null 2>&1 && [ -d "$ROOT_DIR/.git" ]; then
    git -C "$ROOT_DIR" diff --exit-code -- docs/openapi.huma.generated.yaml
fi

LDFLAGS="-s -w -X 'npms/backend/internal/version.Version=$VERSION' -X 'npms/backend/internal/version.GitCommit=$GIT_COMMIT' -X 'npms/backend/internal/version.BuildDate=$BUILD_TIME'"

go build -trimpath -ldflags="$LDFLAGS" -o "$DIST_DIR/bin/npms-api" ./cmd/api
go build -trimpath -ldflags="$LDFLAGS" -o "$DIST_DIR/bin/npms-worker" ./cmd/worker
go build -trimpath -ldflags="$LDFLAGS" -o "$DIST_DIR/bin/npms-db-migrate" ./cmd/db-migrate
go build -trimpath -ldflags="$LDFLAGS" -o "$DIST_DIR/bin/npms-snmp-debug" ./cmd/snmp-debug
go build -trimpath -ldflags="$LDFLAGS" -o "$DIST_DIR/bin/npms-init" ./cmd/init


cp "$DIST_DIR/bin/"* "$ROOT_DIR/bin/"
cp "$DIST_DIR/bin/npms-api" "$DIST_DIR/npms"
cp "$DIST_DIR/bin/npms-api" "$ROOT_DIR/npms"

# 4. Copy config & MIB files
if [ -f "$ROOT_DIR/config.example.yaml" ]; then
    cp "$ROOT_DIR/config.example.yaml" "$DIST_DIR/config.example.yaml"
    # Do not create a runnable config from the example: it contains a placeholder token.
    # Run npms-init before first startup to generate secrets.
fi

if [ -f "$ROOT_DIR/Printer-MIB.yaml" ]; then
    cp "$ROOT_DIR/Printer-MIB.yaml" "$DIST_DIR/Printer-MIB.yaml"
fi

# 5. Copy profiles & Printer-MIB sample fixtures
if [ -d "$ROOT_DIR/backend/profiles" ]; then
    cp -R "$ROOT_DIR/backend/profiles/." "$DIST_DIR/profiles/"
    cp -R "$ROOT_DIR/backend/profiles/." "$ROOT_DIR/profiles/"
fi

if [ -d "$ROOT_DIR/backend/testdata/printer-mib" ]; then
    mkdir -p "$DIST_DIR/testdata/printer-mib"
    cp -R "$ROOT_DIR/backend/testdata/printer-mib/." "$DIST_DIR/testdata/printer-mib/"
fi

cd "$ROOT_DIR"
echo "Consolidated build completed successfully!"
echo "Distribution bundle located at: $DIST_DIR"
