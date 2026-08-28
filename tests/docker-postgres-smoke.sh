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
script_dir=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
repo_dir=$(CDPATH= cd -- "${script_dir}/.." && pwd)
runtime_dir="${repo_dir}/.tmp-docker-smoke-runtime-${run_id}"
key_dir="${repo_dir}/.tmp-docker-smoke-key-${run_id}"
mkdir "$runtime_dir" "$key_dir"
runtime_mount=$runtime_dir
private_key_host=${key_dir}/private-key.pem
envelope_host=${runtime_dir}/bootstrap.enc.json
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
	-out "$private_key_host" >/dev/null 2>&1
openssl pkey -in "$private_key_host" -pubout \
	-out "${runtime_dir}/public-key.pem" >/dev/null 2>&1

if "$windows_docker"; then
	export MSYS_NO_PATHCONV=1
	private_key_host=$(cygpath -w "$private_key_host")
	envelope_host=$(cygpath -w "$envelope_host")
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
	--publish 127.0.0.1::80 \
	--volume "${data_volume}:/data" \
	--mount "type=bind,source=${runtime_mount},target=/run/abera" \
	--env 'DB_DSN=postgres://corteza:corteza-ci-password@'"${database}"':5432/corteza?sslmode=disable' \
	--env 'AUTH_JWT_SECRET=ci-only-jwt-secret-with-at-least-thirty-two-characters' \
	--env ABERA_MODE=demo \
	--env ABERA_DEMO_BUNDLE=showroom-co \
	--env ABERA_INITIAL_ADMIN_EMAIL=admin-ci@abera.invalid \
	--env 'ABERA_INITIAL_ADMIN_NAME=Administrador CI' \
	--env ABERA_INITIAL_ADMIN_HANDLE=admin-ci \
	--env ABERA_MCP_MODE=admin \
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

pwsh_bin=pwsh
command -v "$pwsh_bin" >/dev/null 2>&1 || pwsh_bin=pwsh.exe
command -v "$pwsh_bin" >/dev/null 2>&1 || {
	echo "docker-postgres-smoke: pwsh is required for the API verification" >&2
	exit 1
}
application_port=$(docker port "$application" 80/tcp | sed -n 's/.*:\([0-9][0-9]*\)$/\1/p' | head -n 1)
[ -n "$application_port" ] || {
	echo "docker-postgres-smoke: application port was not published" >&2
	exit 1
}
verify_private_key=$private_key_host
verify_envelope=$envelope_host

windows_path_from_posix() {
	case "$1" in
		/mnt/[A-Za-z]/*)
			drive=${1#/mnt/}
			drive=${drive%%/*}
			rest=${1#/mnt/$drive/}
			printf '%s:\\%s\n' "$(printf '%s' "$drive" | tr '[:lower:]' '[:upper:]')" \
				"$(printf '%s' "$rest" | sed 's|/|\\\\|g')"
			;;
		/[A-Za-z]/*)
			drive=${1#/}
			drive=${drive%%/*}
			rest=${1#/$drive/}
			printf '%s:\\%s\n' "$(printf '%s' "$drive" | tr '[:lower:]' '[:upper:]')" \
				"$(printf '%s' "$rest" | sed 's|/|\\\\|g')"
			;;
		*) return 1 ;;
	esac
}

case "$pwsh_bin" in
	*pwsh.exe)
		if [ "$windows_docker" = true ] && command -v cygpath >/dev/null 2>&1; then
			verify_private_key=$(cygpath -w "$private_key_host")
			verify_envelope=$(cygpath -w "$envelope_host")
		elif command -v wslpath >/dev/null 2>&1; then
			verify_private_key=$(wslpath -w "$private_key_host" 2>/dev/null || windows_path_from_posix "$private_key_host")
			verify_envelope=$(wslpath -w "$envelope_host" 2>/dev/null || windows_path_from_posix "$envelope_host")
		elif command -v cygpath >/dev/null 2>&1; then
			verify_private_key=$(cygpath -w "$private_key_host")
			verify_envelope=$(cygpath -w "$envelope_host")
		else
			verify_private_key=$(windows_path_from_posix "$private_key_host")
			verify_envelope=$(windows_path_from_posix "$envelope_host")
		fi
		;;
esac
"$pwsh_bin" -NoLogo -NoProfile -NonInteractive -File tests/verify-demo-api.ps1 \
	-BaseUrl "http://127.0.0.1:${application_port}" \
	-PrivateKeyFile "$verify_private_key" \
	-EnvelopeFile "$verify_envelope"

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
