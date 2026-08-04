#!/usr/bin/env bash
# Quick Tunnel evidence helper. It is plan-only unless --execute is explicit.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMPOSE_FILE="$ROOT/deploy/ab-browser/compose.yaml"
VALIDATOR="$ROOT/scripts/ab/validate_browser_ab_stack.py"
ACTION="plan"
EXECUTE=false
ENV_FILE=""
RUN_DIR=""
CLOUDFLARED_BIN="cloudflared"
WAIT_SECONDS=45

usage() {
  cat <<'USAGE'
Usage: browser_quick_tunnels.sh [plan|start|status|stop] --env-file FILE --run-dir DIR [options]

Default is plan-only. Starting or stopping processes requires --execute.

Options:
  --compose-file FILE       dedicated Browser A/B compose file
  --cloudflared-bin FILE    WSL/Linux cloudflared binary (default: cloudflared)
  --wait-seconds N          maximum URL discovery wait (default: 45)
  --execute                 permit local cloudflared process start/stop
USAGE
}

die() { printf 'ERROR: %s\n' "$*" >&2; exit 2; }

if [[ $# -gt 0 && "$1" =~ ^(plan|start|status|stop)$ ]]; then
  ACTION="$1"
  shift
fi
while [[ $# -gt 0 ]]; do
  case "$1" in
    --env-file) ENV_FILE="$2"; shift 2 ;;
    --run-dir) RUN_DIR="$2"; shift 2 ;;
    --compose-file) COMPOSE_FILE="$2"; shift 2 ;;
    --cloudflared-bin) CLOUDFLARED_BIN="$2"; shift 2 ;;
    --wait-seconds) WAIT_SECONDS="$2"; shift 2 ;;
    --execute) EXECUTE=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done

[[ -f "$ENV_FILE" ]] || die "--env-file must name an existing file"
[[ -n "$RUN_DIR" ]] || die "--run-dir is required"
[[ "$WAIT_SECONDS" =~ ^[1-9][0-9]*$ ]] || die "--wait-seconds must be positive"

env_value() {
  python3 - "$ENV_FILE" "$1" <<'PY'
import pathlib, sys
path, wanted = pathlib.Path(sys.argv[1]), sys.argv[2]
for raw in path.read_text(encoding="utf-8").splitlines():
    line = raw.strip()
    if not line or line.startswith("#"):
        continue
    if line.startswith("export "):
        line = line[7:].strip()
    if "=" not in line:
        continue
    key, value = line.split("=", 1)
    if key.strip() == wanted:
        value = value.strip()
        if len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'":
            value = value[1:-1]
        print(value)
        raise SystemExit(0)
raise SystemExit(1)
PY
}

labels=(external-external devplus-devplus external-devplus devplus-external)
port_keys=(
  AB_EDGE_EXTERNAL_EXTERNAL_PORT
  AB_EDGE_DEVPLUS_DEVPLUS_PORT
  AB_EDGE_EXTERNAL_DEVPLUS_PORT
  AB_EDGE_DEVPLUS_EXTERNAL_PORT
)
ports=()
for key in "${port_keys[@]}"; do
  ports+=("$(env_value "$key")")
done
frontend_hashes=(
  "$(env_value AB_EXTERNAL_FRONTEND_SHA256)"
  "$(env_value AB_DEVPLUS_FRONTEND_SHA256)"
  "$(env_value AB_EXTERNAL_FRONTEND_SHA256)"
  "$(env_value AB_DEVPLUS_FRONTEND_SHA256)"
)
backend_hashes=(
  "$(env_value AB_EXTERNAL_BACKEND_SHA256)"
  "$(env_value AB_DEVPLUS_BACKEND_SHA256)"
  "$(env_value AB_DEVPLUS_BACKEND_SHA256)"
  "$(env_value AB_EXTERNAL_BACKEND_SHA256)"
)

python3 "$VALIDATOR" --compose-file "$COMPOSE_FILE" --env-file "$ENV_FILE" >/dev/null

TUNNEL_DIR="$RUN_DIR/browser-tunnels"

write_evidence_hashes() {
  local include_logs="$1"
  python3 - "$TUNNEL_DIR" "$include_logs" <<'PY'
import hashlib, pathlib, sys
root, include_logs = pathlib.Path(sys.argv[1]), sys.argv[2] == "true"
names = []
for path in sorted(item for item in root.iterdir() if item.is_file()):
    if path.name == "evidence.sha256":
        continue
    if not include_logs and path.suffix == ".log":
        continue
    names.append(path)
lines = [f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}" for path in names]
(root / "evidence.sha256").write_text("\n".join(lines) + "\n", encoding="utf-8")
PY
}

verify_evidence_hashes() {
  [[ -f "$TUNNEL_DIR/evidence.sha256" ]] || die "missing tunnel evidence hash manifest"
  (cd "$TUNNEL_DIR" && sha256sum --check --status evidence.sha256) || die "tunnel evidence hash mismatch"
}

process_cmdline() {
  tr '\0' ' ' <"/proc/$1/cmdline" 2>/dev/null || true
}

verify_process_identity() {
  local label="$1" expected_url="$2" pid_file="$TUNNEL_DIR/$1.pid" pid cmdline actual_hash expected_hash
  [[ -f "$pid_file" ]] || die "missing pid evidence for $label"
  pid="$(tr -d '[:space:]' <"$pid_file")"
  [[ "$pid" =~ ^[1-9][0-9]*$ ]] || die "invalid pid for $label: $pid"
  kill -0 "$pid" 2>/dev/null || return 1
  cmdline="$(process_cmdline "$pid")"
  [[ "$cmdline" == *cloudflared* && "$cmdline" == *tunnel* && "$cmdline" == *--url* && "$cmdline" == *"$expected_url"* ]] || \
    die "process identity mismatch for $label pid=$pid"
  actual_hash="$(printf '%s' "$cmdline" | sha256sum | awk '{print $1}')"
  expected_hash="$(tr -d '[:space:]' <"$TUNNEL_DIR/$label.cmdline.sha256")"
  [[ "$actual_hash" == "$expected_hash" ]] || die "process command hash mismatch for $label pid=$pid"
}

probe_edge() {
  local label="$1" base_url="$2" mode="$3" front_hash="$4" backend_hash="$5" retry_args=()
  [[ "$mode" == "public" ]] && retry_args=(--retry 8 --retry-delay 1 --retry-all-errors)
  local prefix="$TUNNEL_DIR/$label.$mode"
  curl "${retry_args[@]}" --fail --silent --show-error --max-time 10 \
    --output "$prefix.health.body" --write-out '%{http_code}' "$base_url/health" >"$prefix.health.code"
  [[ "$(<"$prefix.health.code")" == "200" ]] || die "$label $mode health probe did not return 200"
  curl "${retry_args[@]}" --fail --silent --show-error --max-time 10 \
    --output "$prefix.identity.json" "$base_url/__ab/identity"
  python3 - "$prefix.identity.json" "$label" "$front_hash" "$backend_hash" <<'PY'
import json, pathlib, sys
path, edge, frontend, backend = pathlib.Path(sys.argv[1]), sys.argv[2], sys.argv[3], sys.argv[4]
payload = json.loads(path.read_text(encoding="utf-8"))
expected = {"edge": edge, "frontend_sha256": frontend, "backend_sha256": backend}
for key, value in expected.items():
    if payload.get(key) != value:
        raise SystemExit(f"identity mismatch for {edge}: {key} expected={value!r} actual={payload.get(key)!r}")
PY
  curl "${retry_args[@]}" --fail --silent --show-error --max-time 10 \
    --dump-header "$prefix.ping.headers" --output "$prefix.ping.body" \
    --write-out '%{http_code}' "$base_url/ping" >"$prefix.ping.code"
  [[ "$(<"$prefix.ping.code")" == "200" ]] || die "$label $mode API probe did not return 200"
  python3 - "$prefix.ping.headers" "$backend_hash" <<'PY'
import pathlib, sys
headers = {}
for raw in pathlib.Path(sys.argv[1]).read_text(encoding="iso-8859-1").splitlines():
    if ":" in raw:
        key, value = raw.split(":", 1)
        headers[key.strip().lower()] = value.strip()
if headers.get("x-ab-backend-fingerprint") != sys.argv[2]:
    raise SystemExit("API backend fingerprint header mismatch")
PY
}
if [[ "$ACTION" == "plan" || ("$ACTION" == "start" && "$EXECUTE" != true) ]]; then
  printf 'PLAN only: no cloudflared process will be started.\n'
  for index in "${!labels[@]}"; do
    printf '%s -> http://127.0.0.1:%s\n' "${labels[$index]}" "${ports[$index]}"
  done
  printf 'Evidence directory on explicit start: %s\n' "$TUNNEL_DIR"
  exit 0
fi

if [[ "$ACTION" == "status" ]]; then
  [[ -d "$TUNNEL_DIR" ]] || die "tunnel evidence directory not found: $TUNNEL_DIR"
  verify_evidence_hashes
  for index in "${!labels[@]}"; do
    label="${labels[$index]}"
    pid="$(tr -d '[:space:]' <"$TUNNEL_DIR/$label.pid" 2>/dev/null || true)"
    if verify_process_identity "$label" "http://127.0.0.1:${ports[$index]}"; then
      printf '%s RUNNING pid=%s\n' "$label" "$pid"
    else
      printf '%s STOPPED\n' "$label"
    fi
  done
  exit 0
fi

[[ "$EXECUTE" == true ]] || die "$ACTION requires --execute"

safe_stop_pid() {
  local label="$1" expected_url="$2" pid_file="$TUNNEL_DIR/$1.pid" pid
  [[ -f "$pid_file" ]] || return 0
  pid="$(tr -d '[:space:]' <"$pid_file")"
  [[ "$pid" =~ ^[1-9][0-9]*$ ]] || die "invalid pid for $label: $pid"
  kill -0 "$pid" 2>/dev/null || return 0
  verify_process_identity "$label" "$expected_url" || die "recorded process is not running for $label"
  kill "$pid"
}

if [[ "$ACTION" == "stop" ]]; then
  [[ -d "$TUNNEL_DIR" ]] || die "tunnel evidence directory not found: $TUNNEL_DIR"
  verify_evidence_hashes
  for index in "${!labels[@]}"; do
    safe_stop_pid "${labels[$index]}" "http://127.0.0.1:${ports[$index]}"
  done
  for label in "${labels[@]}"; do
    pid="$(tr -d '[:space:]' <"$TUNNEL_DIR/$label.pid")"
    for _ in $(seq 1 50); do
      kill -0 "$pid" 2>/dev/null || break
      sleep 0.1
    done
    kill -0 "$pid" 2>/dev/null && die "cloudflared pid $pid did not stop for $label"
  done
  python3 - "$TUNNEL_DIR/tunnels.json" <<'PY'
import datetime as dt, json, pathlib, sys
path = pathlib.Path(sys.argv[1])
data = json.loads(path.read_text(encoding="utf-8"))
data["stopped_at"] = dt.datetime.now(dt.timezone.utc).isoformat()
path.write_text(json.dumps(data, ensure_ascii=False, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
  write_evidence_hashes true
  printf 'Stopped recorded Quick Tunnels under %s\n' "$TUNNEL_DIR"
  exit 0
fi

[[ "$ACTION" == "start" ]] || die "unsupported action: $ACTION"
command -v curl >/dev/null 2>&1 || die "curl is required"
if [[ "$CLOUDFLARED_BIN" == */* ]]; then
  [[ -x "$CLOUDFLARED_BIN" ]] || die "cloudflared binary is not executable: $CLOUDFLARED_BIN"
else
  command -v "$CLOUDFLARED_BIN" >/dev/null 2>&1 || die "cloudflared is not installed"
fi
[[ ! -e "$TUNNEL_DIR" ]] || die "refusing to reuse tunnel evidence directory: $TUNNEL_DIR"

for index in "${!labels[@]}"; do
  code="$(curl --max-time 3 --silent --output /dev/null --write-out '%{http_code}' "http://127.0.0.1:${ports[$index]}/health" || true)"
  [[ "$code" == "200" ]] || die "edge ${labels[$index]} is not healthy on loopback port ${ports[$index]} (HTTP $code)"
done

mkdir -p "$TUNNEL_DIR"
python3 "$VALIDATOR" --compose-file "$COMPOSE_FILE" --env-file "$ENV_FILE" \
  --output "$TUNNEL_DIR/browser-stack-preflight.json" >/dev/null
version="$($CLOUDFLARED_BIN version 2>&1 | head -n 1)"
printf '%s\n' "$version" >"$TUNNEL_DIR/cloudflared.version.txt"
for index in "${!labels[@]}"; do
  probe_edge "${labels[$index]}" "http://127.0.0.1:${ports[$index]}" local \
    "${frontend_hashes[$index]}" "${backend_hashes[$index]}"
done
started_pids=()
cleanup_failed_start() {
  local pid
  for pid in "${started_pids[@]}"; do
    kill "$pid" 2>/dev/null || true
  done
}
trap cleanup_failed_start ERR INT TERM

for index in "${!labels[@]}"; do
  label="${labels[$index]}"
  local_url="http://127.0.0.1:${ports[$index]}"
  "$CLOUDFLARED_BIN" tunnel --no-autoupdate --loglevel info \
    --logfile "$TUNNEL_DIR/$label.log" --url "$local_url" \
    >"$TUNNEL_DIR/$label.stdout.log" 2>&1 &
  pid=$!
  started_pids+=("$pid")
  printf '%s\n' "$pid" >"$TUNNEL_DIR/$label.pid"
  cmdline=""
  for _ in $(seq 1 20); do
    cmdline="$(process_cmdline "$pid")"
    [[ "$cmdline" == *cloudflared* && "$cmdline" == *"$local_url"* ]] && break
    sleep 0.05
  done
  [[ "$cmdline" == *cloudflared* && "$cmdline" == *"$local_url"* ]] || die "could not attest cloudflared command line for $label"
  printf '%s' "$cmdline" | sha256sum | awk '{print $1}' >"$TUNNEL_DIR/$label.cmdline.sha256"
done

deadline=$((SECONDS + WAIT_SECONDS))
while (( SECONDS < deadline )); do
  ready=0
  for label in "${labels[@]}"; do
    if grep -Eho 'https://[A-Za-z0-9.-]+\.trycloudflare\.com' \
      "$TUNNEL_DIR/$label.log" "$TUNNEL_DIR/$label.stdout.log" 2>/dev/null | head -n 1 \
      >"$TUNNEL_DIR/$label.url" && [[ -s "$TUNNEL_DIR/$label.url" ]]; then
      ready=$((ready + 1))
    fi
  done
  (( ready == 4 )) && break
  sleep 1
done

for index in "${!labels[@]}"; do
  label="${labels[$index]}"
  [[ -s "$TUNNEL_DIR/$label.url" ]] || die "Quick Tunnel URL was not discovered for $label"
  pid="$(<"$TUNNEL_DIR/$label.pid")"
  kill -0 "$pid" 2>/dev/null || die "cloudflared exited before evidence capture for $label"
done

url_count="$(cat "$TUNNEL_DIR"/*.url | sort -u | wc -l | tr -d '[:space:]')"
[[ "$url_count" == "4" ]] || die "Quick Tunnel public URLs must be unique (unique=$url_count)"
for index in "${!labels[@]}"; do
  label="${labels[$index]}"
  probe_edge "$label" "$(<"$TUNNEL_DIR/$label.url")" public \
    "${frontend_hashes[$index]}" "${backend_hashes[$index]}"
done

python3 - "$TUNNEL_DIR" "$version" "${labels[*]}" "${ports[*]}" "${frontend_hashes[*]}" "${backend_hashes[*]}" <<'PY'
import datetime as dt, hashlib, json, pathlib, sys
root = pathlib.Path(sys.argv[1])
version, labels, ports = sys.argv[2], sys.argv[3].split(), sys.argv[4].split()
frontends, backends = sys.argv[5].split(), sys.argv[6].split()
items = []
for label, port, frontend, backend in zip(labels, ports, frontends, backends, strict=True):
    public_url = (root / f"{label}.url").read_text(encoding="utf-8").strip()
    items.append({
        "label": label,
        "local_url": f"http://127.0.0.1:{port}",
        "public_url": public_url,
        "pid": int((root / f"{label}.pid").read_text(encoding="utf-8").strip()),
        "cmdline_sha256": (root / f"{label}.cmdline.sha256").read_text(encoding="utf-8").strip(),
        "frontend_sha256": frontend,
        "backend_sha256": backend,
        "local_health_status": int((root / f"{label}.local.health.code").read_text(encoding="utf-8")),
        "public_health_status": int((root / f"{label}.public.health.code").read_text(encoding="utf-8")),
        "local_identity_sha256": hashlib.sha256((root / f"{label}.local.identity.json").read_bytes()).hexdigest(),
        "public_identity_sha256": hashlib.sha256((root / f"{label}.public.identity.json").read_bytes()).hexdigest(),
        "local_api_sha256": hashlib.sha256((root / f"{label}.local.ping.body").read_bytes()).hexdigest(),
        "public_api_sha256": hashlib.sha256((root / f"{label}.public.ping.body").read_bytes()).hexdigest(),
    })
    if items[-1]["local_identity_sha256"] != items[-1]["public_identity_sha256"]:
        raise SystemExit(f"public/local identity response drift for {label}")
    if items[-1]["local_api_sha256"] != items[-1]["public_api_sha256"]:
        raise SystemExit(f"public/local API response drift for {label}")
payload = {
    "schema": 2,
    "kind": "cloudflare_quick_tunnel",
    "started_at": dt.datetime.now(dt.timezone.utc).isoformat(),
    "cloudflared_version": version,
    "tunnels": items,
}
(root / "tunnels.json").write_text(json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
write_evidence_hashes false
trap - ERR INT TERM
cat "$TUNNEL_DIR/tunnels.json"
