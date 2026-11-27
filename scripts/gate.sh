#!/usr/bin/env bash
# Run the build or the tests of each stack that changed since the merge base with origin/main.
# The quality gate calls this script through `make gate-build` and `make gate-test`.
# A change in backend/ selects the backend. A change in frontend/ selects the frontend.
# A change in another path selects both stacks. The script ignores docs/ and *.md files.
# Usage: scripts/gate.sh build|test|stacks
#
# Two bases are in use, on purpose:
# - quality-gate.sh skips the whole run when the code is the same as at the last green run.
#   It uses the last green commit as its base.
# - This script selects the stacks from all changes since the merge base with origin/main.
#   Thus a run after a green run still checks each stack that the branch changed.
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

# Run `make frontend-check`. In a worktree, frontend/node_modules can be a link to the
# node_modules of the main checkout (setting worktree.symlinkDirectories).
frontend_check() {
  local modules=frontend/node_modules
  if [ ! -L "$modules" ]; then
    make frontend-check
    return
  fi
  # The lockfile next to the link target belongs to the linked node_modules.
  local linked_lock
  linked_lock="$(dirname "$(readlink "$modules")")/package-lock.json"
  if cmp -s frontend/package-lock.json "$linked_lock"; then
    # The lockfiles are the same. Do not let make run `npm ci` through the link.
    make -o "$modules" frontend-check
  else
    # The lockfiles are different. Remove only the link and install locally.
    echo "gate: package-lock.json differs from $linked_lock. Local npm ci." >&2
    rm "$modules"
    make frontend-check
  fi
}

case "$stage" in
  build)
    if $backend; then go -C backend build ./...; fi
    if $frontend; then frontend_check; fi
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
