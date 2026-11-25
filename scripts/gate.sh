#!/usr/bin/env bash
# Run the build or the tests of each stack that changed since the merge base with origin/main.
# The quality gate calls this script through `make gate-build` and `make gate-test`.
# A change in backend/ selects the backend. A change in frontend/ selects the frontend.
# A change in another path selects both stacks. The script ignores docs/ and *.md files.
# Usage: scripts/gate.sh build|test|stacks
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

stage="${1:-}"
backend=false
frontend=false

if base=$(git merge-base HEAD origin/main 2>/dev/null); then
  # Committed and uncommitted changes since the base, and the untracked files.
  changed=$(
    { git diff --name-only "$base"; git ls-files --others --exclude-standard; } \
      | grep -v -e '^docs/' -e '\.md$' || true
  )
else
  # With no merge base, the committed changes are unknown. Select both stacks.
  echo "gate: no merge base with origin/main. All stacks selected." >&2
  backend=true
  frontend=true
  changed=""
fi

while IFS= read -r path; do
  case "$path" in
    "") ;;
    backend/*) backend=true ;;
    frontend/*) frontend=true ;;
    *) backend=true; frontend=true ;;
  esac
done <<< "$changed"

echo "gate: backend=$backend frontend=$frontend" >&2

case "$stage" in
  build)
    if $backend; then go -C backend build ./...; fi
    if $frontend; then make frontend-check; fi
    ;;
  test)
    # The frontend has no unit tests. `make frontend-check` in the build stage runs its lint.
    if $backend; then make test && make lint; fi
    ;;
  stacks)
    ;;
  *)
    echo "Usage: scripts/gate.sh build|test|stacks" >&2
    exit 2
    ;;
esac
