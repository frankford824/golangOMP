#!/usr/bin/env bash
# Run the mysql client inside a named local A/B clone container.
#
# This adapter exists for WSL hosts where Docker Desktop is available but a
# host mysql client is not. The caller may still pass loopback host/port flags;
# they are rewritten to the container-local MySQL endpoint.
set -euo pipefail

container="${MYSQL_DOCKER_CONTAINER:-}"
docker_cli="${DOCKER_CLI:-/mnt/c/Progra~1/Docker/Docker/resources/bin/docker.exe}"

[[ -x "$docker_cli" ]] || {
  echo "Docker CLI is not executable: $docker_cli" >&2
  exit 69
}

rewritten=()
requested_port=""
while [[ $# -gt 0 ]]; do
  arg="$1"
  shift
  case "$arg" in
    --host=*) rewritten+=("--host=127.0.0.1") ;;
    --port=*) requested_port="${arg#--port=}"; rewritten+=("--port=3306") ;;
    --host)
      [[ $# -gt 0 ]] || { echo "--host requires a value" >&2; exit 64; }
      shift
      rewritten+=("--host" "127.0.0.1")
      ;;
    --port)
      [[ $# -gt 0 ]] || { echo "--port requires a value" >&2; exit 64; }
      requested_port="$1"
      shift
      rewritten+=("--port" "3306")
      ;;
    --defaults-extra-file=*)
      echo "--defaults-extra-file is not supported by the container adapter" >&2
      exit 64
      ;;
    *) rewritten+=("$arg") ;;
  esac
done

if [[ -z "$container" && "$requested_port" =~ ^[0-9]+$ ]]; then
  port_key="MYSQL_DOCKER_PORT_${requested_port}_CONTAINER"
  container="${!port_key:-}"
fi

case "$container" in
  codex-yongbo-ab-a-*|codex-yongbo-ab-b-*) ;;
  *)
    echo "set MYSQL_DOCKER_CONTAINER or MYSQL_DOCKER_PORT_<port>_CONTAINER to a dedicated codex-yongbo-ab-a-* or codex-yongbo-ab-b-* clone" >&2
    exit 64
    ;;
esac

exec "$docker_cli" exec -i "$container" mysql "${rewritten[@]}"
