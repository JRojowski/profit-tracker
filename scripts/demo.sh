#!/usr/bin/env bash
# Start bin/profit-tracker with a fresh database of synthetic demo data.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
addr="${DEMO_ADDR:-127.0.0.1:8099}"
base="http://$addr"
work="${TMPDIR:-/tmp}/profit-tracker-demo"
db="$work/demo.db"
exports="$work/exports"

if curl -fsS "$base/api/health" >/dev/null 2>&1; then
	echo "A server already runs on $addr. Stop it, or set DEMO_ADDR to a free address." >&2
	exit 1
fi

rm -rf "$work"
mkdir -p "$exports"

(cd "$root/backend" && go run ./cmd/demo-exports -out "$exports" >/dev/null)

"$root/bin/profit-tracker" -db "$db" -addr "$addr" >"$work/server.log" 2>&1 &
pid=$!
# Stop the server when a later step fails. A successful run removes this trap.
trap 'kill "$pid" 2>/dev/null || true' EXIT

for _ in $(seq 1 50); do
	curl -fsS "$base/api/health" >/dev/null 2>&1 && break
	kill -0 "$pid" 2>/dev/null || { echo "server stopped, see $work/server.log" >&2; exit 1; }
	sleep 0.2
done
curl -fsS "$base/api/health" >/dev/null || { echo "server did not start, see $work/server.log" >&2; exit 1; }

import_file() {
	local name="$1" type="$2" currency="$3" file="$4" preview id
	preview="$(curl -fsS -F "file=@$file" "$base/api/imports")"
	id="$(printf '%s' "$preview" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)"
	[ -n "$id" ] || { echo "no import id for $name: $preview" >&2; exit 1; }
	if ! printf '%s' "$preview" | grep -q '"conflicts":\(null\|\[\]\)'; then
		echo "import conflicts for $name: $preview" >&2
		exit 1
	fi
	curl -fsS -H 'Content-Type: application/json' \
		-d "{\"account\":{\"name\":\"$name\",\"type\":\"$type\",\"currency\":\"$currency\"}}" \
		"$base/api/imports/$id/confirm" >/dev/null
	echo "imported $name"
}

import_file "XTB USD" regular USD "$(ls "$exports"/USD_56100001_*.xlsx)"
import_file "XTB PLN" regular PLN "$(ls "$exports"/PLN_56100002_*.xlsx)"
import_file "XTB IKZE" IKZE PLN "$(ls "$exports"/PLN_56100003_*.xlsx)"

# The refresh can fail for the ticker ZZQX.US. That is expected.
curl -sS -X POST "$base/api/prices/refresh" >/dev/null || echo "price refresh failed (expected for ZZQX.US)"

trap - EXIT

cat <<MSG

Demo server is running.
  URL:       $base
  Database:  $db
  PID:       $pid
  Stop:      kill $pid
  Vite:      PROFIT_TRACKER_ADDR=$addr make frontend-dev
MSG
