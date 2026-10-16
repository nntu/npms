BACKEND_DIR := backend
DB_PATH ?= ./data/npms.db
CONFIG_PATH ?= ./config.yaml

.PHONY: test lint build api-contract api-contract-check frontend-install frontend-check ci build-standalone dev-up dev-down migrate-up init-config set-version

test:
	cd $(BACKEND_DIR) && go test ./...

lint:
	cd $(BACKEND_DIR) && go fmt ./... && go vet ./...

api-contract:
	cd $(BACKEND_DIR) && go run ./cmd/api-contract

api-contract-check: api-contract
	git diff --exit-code -- docs/openapi.huma.generated.yaml

frontend-install:
	cd frontend && pnpm install --frozen-lockfile

frontend-check:
	cd frontend && pnpm run check

ci: test lint api-contract-check frontend-check

VERSION ?= $(shell cat ./VERSION 2>/dev/null || echo "1.0.0")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || echo "unknown")
LDFLAGS ?= -s -w -X 'npms/backend/internal/version.Version=$(VERSION)' -X 'npms/backend/internal/version.GitCommit=$(COMMIT)' -X 'npms/backend/internal/version.BuildDate=$(BUILD_DATE)'

build:
	cd $(BACKEND_DIR) && go build -ldflags="$(LDFLAGS)" ./cmd/...

build-standalone:
	./scripts/build-standalone.sh

dev-up:
	@echo "No container is required. SQLite runs at $(DB_PATH)."

dev-down:
	@echo "No container process to stop."

migrate-up:
	cd $(BACKEND_DIR) && go run ./cmd/db-migrate --config "../$(CONFIG_PATH)"

init-config:
	cd $(BACKEND_DIR) && go run ./cmd/init --config "../$(CONFIG_PATH)"

set-version:
	@powershell -ExecutionPolicy Bypass -File ./scripts/set-version.ps1 $(v)

