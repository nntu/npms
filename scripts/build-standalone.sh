#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR/frontend"
npm ci
npm run build

rm -rf "$ROOT_DIR/backend/web/dist/assets"
cp -R dist/. "$ROOT_DIR/backend/web/dist/"

cd "$ROOT_DIR/backend"
go mod download
go build -trimpath -ldflags="-s -w" -o "$ROOT_DIR/npms" ./cmd/npms
echo "Built $ROOT_DIR/npms"
