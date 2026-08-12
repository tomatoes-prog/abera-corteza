#!/bin/sh

# Copyright 2026 Abera/Corteza contributors
# Licensed under the Apache License, Version 2.0.

set -eu

image=${1:?usage: docker-postgres-smoke.sh IMAGE}
run_id="abera-corteza-ci-$$"
network="${run_id}-network"
database="${run_id}-postgres"
application="${run_id}-app"
data_volume="${run_id}-data"
runtime_dir=$(mktemp -d)
key_dir=$(mktemp -d)
runtime_mount=$runtime_dir
windows_docker=false

# Git Bash otherwise rewrites container paths and misreads the colon in a
# Windows bind mount. Convert only the host path and leave Docker arguments
# untouched. Linux runners continue to use the original POSIX path.
case $(uname -s) in
	MINGW*|MSYS*)
		runtime_mount=$(cygpath -w "$runtime_dir")
		windows_docker=true
		;;
esac

cleanup() {
	docker rm -f "$application" "$database" >/dev/null 2>&1 || true
	docker volume rm "$data_volume" >/dev/null 2>&1 || true
	docker network rm "$network" >/dev/null 2>&1 || true
	rm -rf "$runtime_dir"
	rm -rf "$key_dir"
}
trap cleanup EXIT INT TERM

docker network create "$network" >/dev/null
docker volume create "$data_volume" >/dev/null

openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:3072 \
	-out "${key_dir}/private-key.pem" >/dev/null 2>&1
openssl pkey -in "${key_dir}/private-key.pem" -pubout \
	-out "${runtime_dir}/public-key.pem" >/dev/null 2>&1

if "$windows_docker"; then
	export MSYS_NO_PATHCONV=1
fi

docker run --detach --name "$database" --network "$network" \
	--env POSTGRES_DB=corteza \
	--env POSTGRES_USER=corteza \
	--env POSTGRES_PASSWORD=corteza-ci-password \
	postgres:15.18-bookworm@sha256:e8db9bd3e9e1751eb639fb17be53cc6d1b62a322adf75b99e791767a7a16ce69 >/dev/null

deadline=$(( $(date +%s) + 120 ))
until docker exec "$database" pg_isready --username corteza --dbname corteza >/dev/null 2>&1; do
	[ "$(date +%s)" -lt "$deadline" ] || {
		docker logs "$database" >&2
		exit 1
	}
	sleep 2
done

docker run --detach --name "$application" --network "$network" \
	--memory 1g --memory-swap 1g \
	--volume "${data_volume}:/data" \
	--mount "type=bind,source=${runtime_mount},target=/run/abera" \
	--env 'DB_DSN=postgres://corteza:corteza-ci-password@'"${database}"':5432/corteza?sslmode=disable' \
	--env 'AUTH_JWT_SECRET=ci-only-jwt-secret-with-at-least-thirty-two-characters' \
	--env ABERA_MODE=demo \
	--env ABERA_DEMO_BUNDLE=showroom-co \
	--env ABERA_INITIAL_ADMIN_EMAIL=admin-ci@abera.invalid \
	--env 'ABERA_INITIAL_ADMIN_NAME=Administrador CI' \
	--env ABERA_INITIAL_ADMIN_HANDLE=admin-ci \
	--env ABERA_MCP_MODE=basic \
	--env ABERA_BOOTSTRAP_PUBLIC_KEY_FILE=/run/abera/public-key.pem \
	--env ABERA_BOOTSTRAP_OUTPUT_FILE=/run/abera/bootstrap.enc.json \
	--env DOMAIN=localhost \
	--env DOMAIN_WEBAPP=localhost \
	--env HTTP_SSL_TERMINATED=false \
	"$image" >/dev/null

wait_for_health() {
	deadline=$(( $(date +%s) + 600 ))
	while [ "$(date +%s)" -lt "$deadline" ]; do
		status=$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$application")
		[ "$status" = healthy ] && return 0
		[ "$status" != exited ] || break
		sleep 5
	done
	docker logs "$application" >&2
	return 1
}

wait_for_health
docker exec "$application" test -s /run/abera/bootstrap.enc.json
docker exec "$application" grep -q \
	'"algorithm":"RSA-OAEP-256+A256GCM"' \
	/run/abera/bootstrap.enc.json
test "$(docker inspect --format '{{.State.OOMKilled}}' "$application")" = false

before=$(docker exec "$application" sha256sum \
	/run/abera/bootstrap.enc.json | cut -d ' ' -f 1)
docker restart "$application" >/dev/null
wait_for_health
after=$(docker exec "$application" sha256sum \
	/run/abera/bootstrap.enc.json | cut -d ' ' -f 1)

test "$before" = "$after"
test "$(docker inspect --format '{{.State.OOMKilled}}' "$application")" = false
docker exec "$application" sh -c 'test "$(cat /data/.abera-deployment)" = demo:showroom-co'

echo "docker-postgres-smoke: OK"
