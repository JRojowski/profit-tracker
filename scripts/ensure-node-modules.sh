#!/usr/bin/env bash
# Make sure frontend/node_modules matches frontend/package-lock.json.
# Usage: scripts/ensure-node-modules.sh [--force]
# --force runs `npm ci` again when node_modules is a real folder.
#
# In a git worktree, frontend/node_modules can be a link to the node_modules of the
# main checkout (setting worktree.symlinkDirectories). `npm ci` through the link
# empties the link target. This script never runs `npm ci` through the link.
set -euo pipefail

cd "$(dirname "$0")/.."

force=false
if [ "${1:-}" = "--force" ]; then force=true; fi

modules=frontend/node_modules
lock=frontend/package-lock.json

if [ -L "$modules" ]; then
  # The lockfile next to the link target belongs to the linked node_modules.
  target="$(readlink "$modules")"
  case "$target" in
    /*) ;;
    *) target="frontend/$target" ;;
  esac
  linked_lock="$(dirname "$target")/package-lock.json"
  # npm writes node_modules/.package-lock.json after a complete install. Without it, the
  # link target is missing or empty.
  if cmp -s "$lock" "$linked_lock" && [ -f "$modules/.package-lock.json" ]; then
    # The lockfiles are the same and the target is installed. Do nothing through the link.
    exit 0
  fi
  # The lockfiles are different, or the target is not installed. Remove only the link,
  # then install locally.
  echo "ensure-node-modules: the linked $modules does not match $lock or is not installed. Local npm ci." >&2
  rm "$modules"
elif ! $force && [ -d "$modules" ] && [ ! "$lock" -nt "$modules" ]; then
  exit 0
fi

(cd frontend && npm ci)
touch "$modules"
