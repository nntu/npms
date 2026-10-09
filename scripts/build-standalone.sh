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

# 1. Build frontend assets

cd "$ROOT_DIR/frontend"
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

go build -trimpath -ldflags="-s -w" -o "$DIST_DIR/bin/npms-api" ./cmd/api
go build -trimpath -ldflags="-s -w" -o "$DIST_DIR/bin/npms-worker" ./cmd/worker
go build -trimpath -ldflags="-s -w" -o "$DIST_DIR/bin/npms-db-migrate" ./cmd/db-migrate
go build -trimpath -ldflags="-s -w" -o "$DIST_DIR/bin/npms-snmp-debug" ./cmd/snmp-debug
go build -trimpath -ldflags="-s -w" -o "$DIST_DIR/bin/npms-init" ./cmd/init


cp "$DIST_DIR/bin/"* "$ROOT_DIR/bin/"
cp "$DIST_DIR/bin/npms-api" "$DIST_DIR/npms"
cp "$DIST_DIR/bin/npms-api" "$ROOT_DIR/npms"

# 4. Copy config files
if [ -f "$ROOT_DIR/config.example.yaml" ]; then
    cp "$ROOT_DIR/config.example.yaml" "$DIST_DIR/config.example.yaml"
    [ -f "$DIST_DIR/config.yaml" ] || cp "$ROOT_DIR/config.example.yaml" "$DIST_DIR/config.yaml"
    sed -i 's|\./backend/profiles|\./profiles|g' "$DIST_DIR/config.yaml" 2>/dev/null || true
    [ -f "$ROOT_DIR/config.yaml" ] || cp "$ROOT_DIR/config.example.yaml" "$ROOT_DIR/config.yaml"
fi

# 5. Copy profiles
if [ -d "$ROOT_DIR/backend/profiles" ]; then
    cp -R "$ROOT_DIR/backend/profiles/." "$DIST_DIR/profiles/"
    cp -R "$ROOT_DIR/backend/profiles/." "$ROOT_DIR/profiles/"
fi

cd "$ROOT_DIR"
echo "Consolidated build completed successfully!"
echo "Distribution bundle located at: $DIST_DIR"
