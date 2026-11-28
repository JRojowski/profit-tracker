#!/usr/bin/env bash
# PreToolUse hook for Bash.
# Run the quality gate only when the command really runs "git push".
# The "if" filter in settings.json is best-effort, so this script checks again.
# If jq is missing or the JSON has no command, allow the call.

input=$(cat)

command -v jq >/dev/null 2>&1 || exit 0
cmd=$(printf '%s' "$input" | jq -r '.tool_input.command // empty' 2>/dev/null) || exit 0
[ -n "$cmd" ] || exit 0

# "git", then global options, then "push".
# "git" must start a command: line start, ";", "&", "|", "(" or "$(".
# Leading VAR=value assignments are allowed.
opt='(-C[[:space:]]+[^[:space:]]+|-c[[:space:]]+[^[:space:]]+|--[A-Za-z-]+(=[^[:space:]]*)?|-[pP])'
re="(^|[;&|(])[[:space:]]*([A-Za-z_][A-Za-z0-9_]*=[^[:space:]]*[[:space:]]+)*git([[:space:]]+${opt})*[[:space:]]+push([[:space:]]|\$|[;&|)])"

printf '%s\n' "$cmd" | grep -Eq -- "$re" || exit 0

printf '%s' "$input" | "${CLAUDE_PROJECT_DIR:?}"/.claude/hooks/quality-gate.sh
exit $?
