#!/usr/bin/env bash
# Format a Go file after Claude edits it. Uses the formatters in backend/.golangci.yml.
set -euo pipefail

file=$(jq -r '.tool_input.file_path // empty')
[[ "$file" == *.go ]] || exit 0
[[ -f "$file" ]] || exit 0

root="${CLAUDE_PROJECT_DIR:-$(pwd)}"
if ! out=$(golangci-lint fmt --config "$root/backend/.golangci.yml" "$file" 2>&1); then
  echo "golangci-lint fmt failed for $file: $out" >&2
  exit 2
fi
