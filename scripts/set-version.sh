#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [ $# -lt 1 ]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 1.1.0"
    exit 1
fi

NEW_VERSION="${1#v}"
if ! [[ "$NEW_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+ ]]; then
    echo "Error: Invalid semantic version '$1'. Format must be like 1.0.0, 1.1.0"
    exit 1
fi

echo "Setting NPMS version to: $NEW_VERSION"

# 1. Update VERSION file
printf "%s" "$NEW_VERSION" > "$ROOT_DIR/VERSION"
echo "Updated $ROOT_DIR/VERSION"

# 2. Update frontend/package.json
PKG_JSON="$ROOT_DIR/frontend/package.json"
if [ -f "$PKG_JSON" ]; then
    sed -i -E "s/\"version\": \"[^\"]+\"/\"version\": \"$NEW_VERSION\"/" "$PKG_JSON"
    echo "Updated $PKG_JSON"
fi

# 3. Update backend/internal/version/version.go
GO_VERSION="$ROOT_DIR/backend/internal/version/version.go"
if [ -f "$GO_VERSION" ]; then
    sed -i -E "s/Version = \"[^\"]+\"/Version = \"$NEW_VERSION\"/" "$GO_VERSION"
    echo "Updated $GO_VERSION"
fi

# 4. Regenerate API contract & frontend types
echo "Regenerating API contract & frontend types..."
cd "$ROOT_DIR/backend"
go run ./cmd/api-contract

cd "$ROOT_DIR/frontend"
pnpm run api:generate

cd "$ROOT_DIR"
echo "Version bump to $NEW_VERSION completed successfully!"
