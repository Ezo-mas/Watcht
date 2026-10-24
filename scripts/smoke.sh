#!/usr/bin/env bash
# Smoke-test a built watcht binary: start it on a spare loopback port, wait
# for the health check, and check that the embedded SPA and one of its
# hashed assets are served. Used by `make smoke` and CI.
set -euo pipefail

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
"$bin" healthcheck

index=$(curl -fsS "$base/movies/1")
asset=$(grep -o '/_app/immutable/[^"]*\.js' <<<"$index" | head -n 1)
if [[ -z $asset ]]; then
	echo "smoke: index.html references no /_app/immutable/ script; is the frontend embedded?" >&2
	exit 1
fi
type=$(curl -fsS -o /dev/null -w '%{content_type}' "$base$asset")
if [[ $type != text/javascript* ]]; then
	echo "smoke: $asset served as '$type', want text/javascript" >&2
	exit 1
fi
status=$(curl -s -o /dev/null -w '%{http_code}' "$base/_app/immutable/missing.js")
if [[ $status != 404 ]]; then
	echo "smoke: missing asset returned $status, want 404" >&2
	exit 1
fi
echo "smoke: ok ($asset)"
