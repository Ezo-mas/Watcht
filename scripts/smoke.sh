#!/usr/bin/env bash
# Smoke-test watcht: check the health endpoint, the SPA fallback, one hashed
# asset, and a 404 for a missing asset.
#
#   scripts/smoke.sh [BIN [PORT]]   start BIN on a spare loopback port first
#   scripts/smoke.sh --url URL      test a server that is already running
#
# Used by `make smoke`, `make docker-smoke`, and CI.
set -euo pipefail

if [[ ${1:-} == --url ]]; then
	base=${2:?--url needs a URL}
else
	bin=${1:-bin/watcht}
	port=${2:-18080}
	base="http://127.0.0.1:$port"
	export WATCHT_LISTEN="127.0.0.1:$port"
	"$bin" serve >/dev/null 2>&1 &
	pid=$!
	trap 'kill "$pid" 2>/dev/null || true' EXIT
	for _ in $(seq 1 40); do
		"$bin" healthcheck 2>/dev/null && break
		sleep 0.25
	done
fi

fail() {
	echo "smoke: $*" >&2
	exit 1
}

curl -fsS "$base/healthz" | grep -q '"status":"ok"' || fail "/healthz is not ok"

index=$(curl -fsS "$base/movies/1")
asset=$(grep -o '/_app/immutable/[^"]*\.js' <<<"$index" | head -n 1)
[[ -n $asset ]] || fail "index.html references no /_app/immutable/ script; is the frontend embedded?"

type=$(curl -fsS -o /dev/null -w '%{content_type}' "$base$asset")
[[ $type == text/javascript* ]] || fail "$asset served as '$type', want text/javascript"

status=$(curl -s -o /dev/null -w '%{http_code}' "$base/_app/immutable/missing.js")
[[ $status == 404 ]] || fail "missing asset returned $status, want 404"

echo "smoke: ok ($base, $asset)"
