#!/usr/bin/env bash
set -euo pipefail

echo "==> pre-commit: formatting..."
go fmt ./...

# Re-stage any Go files that were reformatted
CHANGED=$(git diff --name-only -- '*.go' || true)
if [ -n "$CHANGED" ]; then
    echo "$CHANGED" | xargs git add
    echo "    re-staged reformatted files"
fi

echo "==> pre-commit: vetting..."
go vet ./...

echo "==> pre-commit: linting..."
golangci-lint run ./...

echo "==> pre-commit: all checks passed ✓"
