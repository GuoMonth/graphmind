#!/usr/bin/env bash
set -euo pipefail

echo "==> pre-push: building..."
go build ./cmd/gm

echo "==> pre-push: testing with race detector..."
go test -race -count=1 ./...

echo "==> pre-push: all checks passed ✓"
