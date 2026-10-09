BACKEND_DIR := backend
DB_PATH ?= ./data/npms.db
CONFIG_PATH ?= ./config.yaml

.PHONY: test lint build api-contract api-contract-check frontend-install frontend-check ci build-standalone dev-up dev-down migrate-up init-config

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

build:
	cd $(BACKEND_DIR) && go build ./cmd/...

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

