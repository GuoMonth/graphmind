.PHONY: build test test-cover lint fmt vet fix check validate clean setup-hooks

BINARY_NAME = gm
BUILD_DIR   = ./bin
CMD_DIR     = ./cmd/gm
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     = -ldflags "-X main.version=$(VERSION)"

## Build ──────────────────────────────────────────────────────────────────────

build:
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

## Test ───────────────────────────────────────────────────────────────────────

test:
	go test -race -count=1 ./...

test-cover:
	go test -race -count=1 -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@echo "─────────────────────────────────────────────────"
	@echo "HTML report: go tool cover -html=coverage.out"

## Quality ────────────────────────────────────────────────────────────────────

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

fix:
	go fix ./...

## Combined gates ─────────────────────────────────────────────────────────────

# Pre-commit gate: fast checks
check: fmt vet lint

# Pre-push gate: thorough validation
validate: build test

## Setup ──────────────────────────────────────────────────────────────────────

setup-hooks:
	@chmod +x scripts/*.sh
	@./scripts/setup-hooks.sh

## Cleanup ────────────────────────────────────────────────────────────────────

clean:
	rm -rf $(BUILD_DIR) coverage.out
