BACKEND_DIR := backend
DB_PATH ?= ./data/npms.db
CONFIG_PATH ?= ./config.yaml

.PHONY: test lint build build-standalone dev-up dev-down migrate-up

test:
	cd $(BACKEND_DIR) && go test ./...

lint:
	cd $(BACKEND_DIR) && go fmt ./... && go vet ./...

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
