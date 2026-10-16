#!/usr/bin/env sh
# Uses only cached images and temporary test-owned resources on the dedicated daemon.
set -eu
container=${LEFORGE_TEST_SANDBOX_CONTAINER:-leforge-sandbox-daemon-1}
service=$(docker inspect --format '{{index .Config.Labels "com.docker.compose.service"}}' "$container")
[ "$service" = sandbox-daemon ] || { echo 'Expected dedicated sandbox-daemon' >&2; exit 1; }
task_tmp=$(mktemp -d)
trap 'rm -f "$task_tmp/docker-sandbox"; rmdir "$task_tmp"' EXIT HUP INT TERM
cat > "$task_tmp/docker-sandbox" <<'WRAPPER'
#!/usr/bin/env sh
exec docker exec -i "$LEFORGE_TEST_SANDBOX_CONTAINER" docker "$@"
WRAPPER
chmod 700 "$task_tmp/docker-sandbox"
export LEFORGE_TEST_SANDBOX_CONTAINER="$container"
export LEFORGE_TEST_ENV_DOCKER_BIN="$task_tmp/docker-sandbox"
cd "$(dirname "$0")/../server"
go test ./internal/environment -run '^TestEnvironmentPolicyRealSandbox$' -count=1 -v -timeout 4m
