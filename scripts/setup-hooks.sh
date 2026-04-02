#!/usr/bin/env bash
set -euo pipefail

HOOK_DIR="$(git rev-parse --show-toplevel)/.git/hooks"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

cp "$SCRIPT_DIR/pre-commit.sh" "$HOOK_DIR/pre-commit"
cp "$SCRIPT_DIR/pre-push.sh" "$HOOK_DIR/pre-push"

chmod +x "$HOOK_DIR/pre-commit"
chmod +x "$HOOK_DIR/pre-push"

echo "Git hooks installed to $HOOK_DIR"
