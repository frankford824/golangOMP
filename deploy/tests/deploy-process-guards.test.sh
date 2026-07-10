#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TMP_ROOT="$(mktemp -d)"
TRACKED_PIDS=()

cleanup() {
  local pid
  for pid in "${TRACKED_PIDS[@]}"; do
    if [[ "$pid" =~ ^[1-9][0-9]*$ ]] && kill -0 "$pid" >/dev/null 2>&1; then
      kill "$pid" >/dev/null 2>&1 || true
      sleep 0.1
      kill -9 "$pid" >/dev/null 2>&1 || true
    fi
  done
  rm -rf "$TMP_ROOT"
}
trap cleanup EXIT

fail_test() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  [[ "$haystack" == *"$needle"* ]] || fail_test "expected output to contain: $needle"
}

free_port() {
  python3 - <<'PY'
import socket
with socket.socket() as sock:
    sock.bind(("127.0.0.1", 0))
    print(sock.getsockname()[1])
PY
}

port_is_listening() {
  local port="$1"
  ss -H -ltn "sport = :$port" 2>/dev/null | grep -q .
}

wait_for_port() {
  local port="$1"
  local expected="$2"
  local attempt
  for attempt in $(seq 1 50); do
    if port_is_listening "$port"; then
      [ "$expected" = "listening" ] && return 0
    else
      [ "$expected" = "free" ] && return 0
    fi
    sleep 0.1
  done
  fail_test "port $port did not become $expected"
}

SERVER_IMPL="$TMP_ROOT/fake_server.py"
cat >"$SERVER_IMPL" <<'PY'
#!/usr/bin/env python3
import http.server
import os

port = int(os.environ["PORT"])
health_status = int(os.environ.get("HEALTH_STATUS", "200"))

class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        status = health_status if self.path == "/health" else 404
        self.send_response(status)
        self.end_headers()
        self.wfile.write(b"ok" if status == 200 else b"not ready")

    def log_message(self, *_args):
        pass

http.server.HTTPServer(("127.0.0.1", port), Handler).serve_forever()
PY
chmod +x "$SERVER_IMPL"

FAKE_MAIN="$TMP_ROOT/fake-main"
cat >"$FAKE_MAIN" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [ "${FORK_SERVER:-false}" = "true" ]; then
  python3 "$SERVER_IMPL" &
  child=$!
  trap 'kill "$child" >/dev/null 2>&1 || true; wait "$child" >/dev/null 2>&1 || true' EXIT TERM INT
  wait "$child"
else
  exec python3 "$SERVER_IMPL"
fi
SH
chmod +x "$FAKE_MAIN"

write_main_env() {
  local path="$1"
  local port="$2"
  local status="${3:-200}"
  local fork_server="${4:-false}"
  cat >"$path" <<EOF
PORT=$port
SERVER_IMPL=$SERVER_IMPL
HEALTH_STATUS=$status
FORK_SERVER=$fork_server
EOF
}

test_start_main_guards() {
  local base="$TMP_ROOT/start-main"
  local port
  local output
  local pid

  mkdir -p "$base"
  port="$(free_port)"
  write_main_env "$base/main.env" "$port"
  output="$(START_MAIN_TIMEOUT_SECONDS=5 bash "$ROOT/deploy/start-main.sh" \
    --base-dir "$base" \
    --env-file "$base/main.env" \
    --binary-path "$FAKE_MAIN" \
    --pid-file "$base/main.pid" \
    --log-file "$base/main.log")"
  assert_contains "$output" "STARTED=true"
  assert_contains "$output" "LISTENER_OWNED=true"
  assert_contains "$output" "HEALTH_CODE=200"
  pid="$(cat "$base/main.pid")"
  TRACKED_PIDS+=("$pid")

  if output="$(START_MAIN_TIMEOUT_SECONDS=2 bash "$ROOT/deploy/start-main.sh" \
    --base-dir "$base" \
    --env-file "$base/main.env" \
    --binary-path "$FAKE_MAIN" \
    --pid-file "$base/second.pid" \
    --log-file "$base/second.log" 2>&1)"; then
    fail_test "start-main accepted an already occupied port"
  fi
  assert_contains "$output" "already listening before start"
  [ ! -e "$base/second.pid" ] || fail_test "rejected start wrote a pidfile"

  kill "$pid"
  wait_for_port "$port" free

  port="$(free_port)"
  write_main_env "$base/fork.env" "$port" 200 true
  if output="$(START_MAIN_TIMEOUT_SECONDS=3 bash "$ROOT/deploy/start-main.sh" \
    --base-dir "$base" \
    --env-file "$base/fork.env" \
    --binary-path "$FAKE_MAIN" \
    --pid-file "$base/fork.pid" \
    --log-file "$base/fork.log" 2>&1)"; then
    fail_test "start-main accepted a listener owned by a child process"
  fi
  assert_contains "$output" "not owned by launched pid"
  [ ! -e "$base/fork.pid" ] || fail_test "ownership failure left a pidfile"
  wait_for_port "$port" free

  port="$(free_port)"
  write_main_env "$base/unhealthy.env" "$port" 503 false
  if output="$(START_MAIN_TIMEOUT_SECONDS=2 bash "$ROOT/deploy/start-main.sh" \
    --base-dir "$base" \
    --env-file "$base/unhealthy.env" \
    --binary-path "$FAKE_MAIN" \
    --pid-file "$base/unhealthy.pid" \
    --log-file "$base/unhealthy.log" 2>&1)"; then
    fail_test "start-main accepted an unhealthy listener"
  fi
  assert_contains "$output" "did not become ready"
  assert_contains "$output" "health_code=503"
  [ ! -e "$base/unhealthy.pid" ] || fail_test "health failure left a pidfile"
  wait_for_port "$port" free
}

make_package() {
  local package_root="$1"
  mkdir -p "$package_root"
  cp -R "$ROOT/deploy" "$package_root/deploy"
  cp /bin/true "$package_root/ecommerce-api"
  cp /bin/true "$package_root/erp_bridge"
  printf 'PORT=8080\n' >"$package_root/.env.example"
  printf 'SERVER_PORT=8081\n' >"$package_root/bridge.env.example"
  printf 'new package\n' >"$package_root/package-marker"
}

write_shared_envs() {
  local base="$1"
  mkdir -p "$base/shared"
  printf 'PORT=8080\n' >"$base/shared/main.env"
  printf 'SERVER_PORT=8081\n' >"$base/shared/bridge.env"
}

test_remote_deploy_stops_exact_candidate() {
  local base="$TMP_ROOT/remote"
  local package_root="$TMP_ROOT/package"
  local version="vtest"
  local release="$base/releases/$version"
  local candidate_port
  local candidate_pid
  local live_pid

  make_package "$package_root"
  write_shared_envs "$base"
  mkdir -p "$release/runtime" "$base/run" "$base/live-release"
  candidate_port="$(free_port)"
  cp "$(command -v python3)" "$release/ecommerce-api"
  printf 'PORT=%s\n' "$candidate_port" >"$release/runtime/main.parallel.env"
  PORT="$candidate_port" HEALTH_STATUS=200 "$release/ecommerce-api" "$SERVER_IMPL" &
  candidate_pid=$!
  TRACKED_PIDS+=("$candidate_pid")
  printf '%s\n' "$candidate_pid" >"$base/run/ecommerce-api-${version}-parallel.pid"
  wait_for_port "$candidate_port" listening

  sleep 120 &
  live_pid=$!
  TRACKED_PIDS+=("$live_pid")
  printf '%s\n' "$live_pid" >"$base/run/ecommerce-api.pid"
  ln -s "$base/live-release" "$base/current"

  bash "$ROOT/deploy/remote-deploy.sh" \
    --package-root "$package_root" \
    --version "$version" \
    --remote-base-dir "$base" \
    --runtime-env-path "$base/shared/main.env" \
    --bridge-env-path "$base/shared/bridge.env" \
    --keep-releases 0 >/dev/null

  wait_for_port "$candidate_port" free
  kill -0 "$candidate_pid" >/dev/null 2>&1 && fail_test "same-version candidate was not stopped"
  kill -0 "$live_pid" >/dev/null 2>&1 || fail_test "stable MAIN pid was touched"
  [ ! -e "$base/run/ecommerce-api-${version}-parallel.pid" ] || fail_test "parallel pidfile was not removed"
  [ -f "$release/package-marker" ] || fail_test "release directory was not replaced"
}

test_remote_deploy_rejects_foreign_pidfile() {
  local base="$TMP_ROOT/foreign"
  local package_root="$TMP_ROOT/foreign-package"
  local version="vforeign"
  local release="$base/releases/$version"
  local foreign_pid
  local output

  make_package "$package_root"
  write_shared_envs "$base"
  mkdir -p "$release/runtime" "$base/run"
  cp /bin/true "$release/ecommerce-api"
  printf 'old release\n' >"$release/old-marker"
  printf 'PORT=%s\n' "$(free_port)" >"$release/runtime/main.parallel.env"

  sleep 120 &
  foreign_pid=$!
  TRACKED_PIDS+=("$foreign_pid")
  printf '%s\n' "$foreign_pid" >"$base/run/ecommerce-api-${version}-parallel.pid"

  if output="$(bash "$ROOT/deploy/remote-deploy.sh" \
    --package-root "$package_root" \
    --version "$version" \
    --remote-base-dir "$base" \
    --runtime-env-path "$base/shared/main.env" \
    --bridge-env-path "$base/shared/bridge.env" \
    --keep-releases 0 2>&1)"; then
    fail_test "remote-deploy accepted a foreign pid in the parallel pidfile"
  fi
  assert_contains "$output" "not"
  kill -0 "$foreign_pid" >/dev/null 2>&1 || fail_test "foreign pid was killed"
  [ -f "$release/old-marker" ] || fail_test "release was removed after pid ownership failure"
}

test_start_main_guards
test_remote_deploy_stops_exact_candidate
test_remote_deploy_rejects_foreign_pidfile
printf 'PASS: deploy process guards\n'
