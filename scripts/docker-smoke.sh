#!/usr/bin/env bash
# Smoke-test a watcht image: run it, wait for Docker's HEALTHCHECK to report
# healthy, check it runs as a non-root user, then run scripts/smoke.sh
# against it. Used by `make docker-smoke` and CI.
set -euo pipefail

image=${1:-watcht:local}
port=${2:-18081}
name="watcht-smoke-$$"

if [[ -z $(docker inspect -f '{{if .Config.Healthcheck}}yes{{end}}' "$image") ]]; then
	echo "docker-smoke: $image has no HEALTHCHECK" >&2
	exit 1
fi

# Set before docker run: a failed run (e.g. port in use) can still leave a
# created container behind.
trap 'docker rm -f "$name" >/dev/null 2>&1 || true' EXIT
docker run -d --name "$name" -p "127.0.0.1:$port:8080" "$image" >/dev/null

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
case $user in
"" | root | root:* | 0 | 0:*)
	echo "docker-smoke: image runs as '${user:-root}', want a non-root user" >&2
	exit 1
	;;
esac
# Also check the process that's actually running.
uid=$(docker top "$name" -o pid,uid | awk 'NR == 2 { print $2 }')
if [[ -z $uid || $uid == 0 ]]; then
	echo "docker-smoke: container process runs as uid '${uid:-unknown}', want non-root" >&2
	exit 1
fi

"$(dirname "$0")/smoke.sh" --url "http://127.0.0.1:$port"
echo "docker-smoke: ok ($image, user $user, uid $uid)"
