#!/usr/bin/env bash
# Smoke-test watcht: check the health endpoint, the SPA fallback, one hashed
# asset, a 404 for a missing asset, and, on a fresh instance, first-run
# setup and sign-in.
#
#   scripts/smoke.sh [BIN [PORT]]          start BIN on a spare loopback port
#                                          with a throwaway data directory
#   scripts/smoke.sh --url URL [--setup]   test a server that is already running
#
# The setup and sign-in checks create an admin account with a published
# password, so with --url they run only when --setup is given. Use it only
# on a throwaway instance, never a real one.
#
# Used by `make smoke`, `make docker-smoke`, and CI.
set -euo pipefail

setup=yes
if [[ ${1:-} == --url ]]; then
	base=${2:?--url needs a URL}
	[[ ${3:-} == --setup ]] || setup=
else
	bin=${1:-bin/watcht}
	port=${2:-18080}
	base="http://127.0.0.1:$port"
	data=$(mktemp -d)
	export WATCHT_LISTEN="127.0.0.1:$port" WATCHT_DATA_DIR="$data"
	"$bin" serve >"$data/server.log" 2>&1 &
	pid=$!
	trap 'kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; rm -rf "$data"' EXIT
	for _ in $(seq 1 40); do
		"$bin" healthcheck 2>/dev/null && break
		sleep 0.25
	done
fi

fail() {
	echo "smoke: $*" >&2
	if [[ -n ${data:-} && -f $data/server.log ]]; then
		echo "--- server log ---" >&2
		cat "$data/server.log" >&2
	fi
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

if [[ -z $setup ]]; then
	echo "smoke: ok ($base, $asset; setup and sign-in not checked without --setup)"
	exit 0
fi

# Auth, on a fresh instance where setup is still open.
api="$base/api/v1/auth"
session=$(curl -fsS "$api/session")
grep -q '"setup_required":true' <<<"$session" || fail "fresh instance: /api/v1/auth/session = $session, want setup_required"

creds='{"username":"smoke","password":"smoke-test-password"}'
headers=$(curl -s -o /dev/null -D - -H 'Content-Type: application/json' -d "$creds" "$api/setup")
grep -q '^HTTP/[0-9.]* 201' <<<"$headers" || fail "setup didn't return 201: $(head -n 1 <<<"$headers")"
cookie=$(grep -io '^set-cookie: watcht_session=[^;]*' <<<"$headers" | cut -d' ' -f2)
[[ -n $cookie ]] || fail "setup set no session cookie"
curl -fsS -b "$cookie" "$api/session" | grep -q '"username":"smoke"' || fail "not signed in after setup"

status=$(curl -s -o /dev/null -w '%{http_code}' -H 'Content-Type: application/json' -d "$creds" \
	-H 'Sec-Fetch-Site: cross-site' -H 'Origin: https://evil.example' "$api/login")
[[ $status == 403 ]] || fail "cross-site login returned $status, want 403"
status=$(curl -s -o /dev/null -w '%{http_code}' -H 'Content-Type: application/json' -d "$creds" "$api/login")
[[ $status == 200 ]] || fail "login returned $status, want 200"

# An API token, created with the session, works on the data API.
created=$(curl -fsS -b "$cookie" -H 'Content-Type: application/json' -d "{\"name\":\"smoke\",\"password\":\"smoke-test-password\"}" "$base/api/v1/tokens")
secret=$(grep -o '"secret":"wt_[A-Za-z0-9_-]*"' <<<"$created" | cut -d'"' -f4)
[[ -n $secret ]] || fail "creating an API token returned no secret: $created"
curl -fsS -H "Authorization: Bearer $secret" "$base/api/v1/me" | grep -q '"username":"smoke"' ||
	fail "GET /api/v1/me with the API token didn't return the user"

echo "smoke: ok ($base, $asset)"
