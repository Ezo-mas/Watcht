#!/usr/bin/env bash
# Smoke-test a watcht image: run it, wait for Docker's HEALTHCHECK to report
# healthy, check it runs as a non-root user, then run scripts/smoke.sh
# against it. Used by `make docker-smoke` and CI.
set -euo pipefail

image=${1:-watcht:local}
port=${2:-18081}
name="watcht-smoke-$$"

docker run -d --name "$name" -p "127.0.0.1:$port:8080" "$image" >/dev/null
trap 'docker rm -f "$name" >/dev/null 2>&1 || true' EXIT

status=starting
for _ in $(seq 1 60); do
	status=$(docker inspect -f '{{.State.Health.Status}}' "$name")
	[[ $status == starting ]] || break
	sleep 1
done
if [[ $status != healthy ]]; then
	docker logs "$name" >&2 || true
	echo "docker-smoke: container health is '$status', want healthy" >&2
	exit 1
fi

user=$(docker inspect -f '{{.Config.User}}' "$image")
if [[ -z $user || $user == root || $user == 0* ]]; then
	echo "docker-smoke: image runs as '${user:-root}', want a non-root user" >&2
	exit 1
fi

"$(dirname "$0")/smoke.sh" --url "http://127.0.0.1:$port"
echo "docker-smoke: ok ($image, user $user)"
