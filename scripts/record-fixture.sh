#!/usr/bin/env bash
# Records one provider response body for a test fixture.
#
#   scripts/record-fixture.sh <provider> <out.json> <path> [name=value ...]
#
# The path is the API path after the provider's base URL, such as /movie/603.
# Each name=value is URL-encoded into the query. The credential is read from a
# file and sent as a header; it never appears in the URL, the output, or this
# shell's history. Prints the HTTP status and size; the body is written as
# recorded (pretty-printed), so trim it afterwards with jq.
set -euo pipefail

if [ $# -lt 3 ]; then
	echo "usage: $0 <provider> <out.json> <path> [name=value ...]" >&2
	exit 2
fi
provider=$1 out=$2 path=$3
shift 3

case $provider in
tmdb)
	base=https://api.themoviedb.org/3
	token_file=${WATCHT_TMDB_TOKEN_FILE:-$HOME/.config/watcht/tmdb-token}
	;;
*)
	echo "$0: unknown provider $provider (add it here)" >&2
	exit 2
	;;
esac

if [ ! -r "$token_file" ]; then
	echo "$0: no credential at $token_file" >&2
	exit 1
fi

args=()
for a in "$@"; do args+=(--data-urlencode "$a"); done

tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT
status=$(curl -sS -G -o "$tmp" -w '%{http_code}' \
	-H @<(printf 'Authorization: Bearer %s\n' "$(cat "$token_file")") \
	-H 'Accept: application/json' \
	"$base$path" ${args[@]+"${args[@]}"})
jq . "$tmp" >"$out"
echo "$out: HTTP $status, $(wc -c <"$out") bytes"
