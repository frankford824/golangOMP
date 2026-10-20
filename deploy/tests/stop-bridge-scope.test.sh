#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/../.." && pwd)"
base="$(mktemp -d /tmp/bridge-stop-scope-XXXXXX)"
bridge_pid="";sync_pid=""
cleanup(){ kill "$bridge_pid" "$sync_pid" 2>/dev/null || true; rm -rf -- "$base"; }
trap cleanup EXIT
mkdir -p "$base/run"
bash -c 'exec -a "$1/erp_bridge_sync" sleep 60' _ "$base" & sync_pid=$!
bash -c 'exec -a "$1/erp_bridge" sleep 60' _ "$base" & bridge_pid=$!
echo "$bridge_pid" > "$base/run/erp_bridge.pid"
sleep .1
bash "$root/deploy/stop-bridge.sh" --base-dir "$base"
kill -0 "$sync_pid"
if kill -0 "$bridge_pid" 2>/dev/null;then echo 'Bridge still running' >&2;exit 1;fi
echo 'PASS: stopping Bridge preserves the separate sync role'
