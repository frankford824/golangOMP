#!/usr/bin/env bash
set -euo pipefail

script_dir() {
  cd "$(dirname "${BASH_SOURCE[0]}")" && pwd
}

repo_root() {
  cd "$(script_dir)/.." && pwd
}

load_local_deploy_env() {
  local root="${1:-$(repo_root)}"
  local env_path="$root/.vscode/deploy.local.env"
  local -a inherited_vars=()
  local -A inherited_values=()
  local va

  [ -f "$env_path" ] || return 0

  while IFS='=' read -r var _; do
    case "$var" in
      DEPLOY_*)
        inherited_vars+=("$var")
        inherited_values["$var"]="${!var}"
        ;;
    esac
  done < <(env)

  set -a
  # shellcheck source=/dev/null
  . "$env_path"
  set +a

  for var in "${inherited_vars[@]}"; do
    printf -v "$var" '%s' "${inherited_values[$var]}"
    export "$var"
  done
}

utc_now() {
  date -u +"%Y-%m-%dT%H:%M:%SZ"
}

log() {
  printf '%s\n' "$*" >&2
}

fail() {
  log "ERROR: $*"
  exit 1
}

sanitize_field() {
  printf '%s' "${1:-}" | tr '\r\n|' '   '
}

resolve_path() {
  local root="$1"
  local target="$2"
  if [[ "$target" = /* ]]; then
    printf '%s\n' "$target"
    return
  fi
  printf '%s\n' "$root/$target"
}

require_cmd() {
  local tool
  for tool in "$@"; do
    command -v "$tool" >/dev/null 2>&1 || fail "$tool is required."
  done
}

go_cmd() {
  if command -v go >/dev/null 2>&1; then
    printf '%s\n' go
    return
  fi
  if command -v go.exe >/dev/null 2>&1; then
    printf '%s\n' go.exe
    return
  fi
  fail "go is required."
}

sha256_of() {
  local file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
    return
  fi
  if command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 "$file" | awk '{print $NF}'
    return
  fi
  fail "No SHA-256 tool available."
}

ensure_release_history_file() {
  local history_path="$1"
  if [ -f "$history_path" ]; then
    return
  fi
  mkdir -p "$(dirname "$history_path")"
  cat >"$history_path" <<EOF
# managed_release_history_v1
baseline_version=v0.1
version_rule=increment_minor_by_one
artifact_prefix=ecommerce-ai
bridge_runtime_base_url=http://127.0.0.1:8081
last_updated_utc=$(utc_now)
# format: release|version|created_at_utc|updated_at_utc|status|artifact_name|artifact_sha256|deploy_host|remote_base_dir|summary|notes
EOF
}

history_value() {
  local history_path="$1"
  local key="$2"
  awk -F= -v key="$key" '
    $1 == key {
      sub(/^[^=]*=/, "", $0)
      print $0
      exit
    }
  ' "$history_path"
}

version_sort_key() {
  local version="$1"
  if [[ ! "$version" =~ ^v([0-9]+)\.([0-9]+)$ ]]; then
    fail "Unsupported managed release version: $version"
  fi
  printf '%010d%010d\n' "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}"
}

next_managed_release_version() {
  local history_path="$1"
  local baseline
  local latest_version=""
  local latest_key=""
  local version
  ensure_release_history_file "$history_path"
  baseline="$(history_value "$history_path" baseline_version)"
  while IFS= read -r version; do
    [ -n "$version" ] || continue
    local key
    key="$(version_sort_key "$version")"
    if [ -z "$latest_key" ] || [[ "$key" > "$latest_key" ]]; then
      latest_key="$key"
      latest_version="$version"
    fi
  done < <(awk -F'|' '$1 == "release" { print $2 }' "$history_path")

  if [ -z "$latest_version" ]; then
    printf '%s\n' "$baseline"
    return
  fi

  if [[ ! "$latest_version" =~ ^v([0-9]+)\.([0-9]+)$ ]]; then
    fail "Latest recorded version cannot be incremented: $latest_version"
  fi
  printf 'v%s.%s\n' "${BASH_REMATCH[1]}" "$((BASH_REMATCH[2] + 1))"
}

append_release_record() {
  local history_path="$1"
  local version="$2"
  local status="$3"
  local summary="$4"
  local artifact_name="$5"
  local artifact_sha="$6"
  local deploy_host="$7"
  local remote_base_dir="$8"
  local notes="${9:-}"
  local created_at="${10:-$(utc_now)}"
  local updated_at="${11:-$created_at}"
  local tmp

  ensure_release_history_file "$history_path"
  tmp="$(mktemp)"
  awk -F= -v ts="$updated_at" '
    BEGIN { updated = 0 }
    /^last_updated_utc=/ {
      print "last_updated_utc=" ts
      updated = 1
      next
    }
    { print }
    END {
      if (!updated) {
        print "last_updated_utc=" ts
      }
    }
  ' "$history_path" >"$tmp"
  mv "$tmp" "$history_path"
  printf 'release|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s\n' \
    "$(sanitize_field "$version")" \
    "$(sanitize_field "$created_at")" \
    "$(sanitize_field "$updated_at")" \
    "$(sanitize_field "$status")" \
    "$(sanitize_field "$artifact_name")" \
    "$(sanitize_field "$artifact_sha")" \
    "$(sanitize_field "$deploy_host")" \
    "$(sanitize_field "$remote_base_dir")" \
    "$(sanitize_field "$summary")" \
    "$(sanitize_field "$notes")" >>"$history_path"
}

read_env_value() {
  local env_path="$1"
  local key="$2"
  local default_value="${3:-}"
  if [ ! -f "$env_path" ]; then
    printf '%s\n' "$default_value"
    return
  fi
  awk -F= -v key="$key" '
    $0 ~ "^[[:space:]]*#" { next }
    $1 == key {
      sub(/^[^=]*=/, "", $0)
      print $0
      found = 1
      exit
    }
    END {
      if (!found) {
        exit 1
      }
    }
  ' "$env_path" 2>/dev/null || printf '%s\n' "$default_value"
}

env_has_key() {
  local env_path="$1"
  local key="$2"
  [ -f "$env_path" ] || return 1
  awk -F= -v key="$key" '
    $0 ~ "^[[:space:]]*#" { next }
    $1 == key { found = 1; exit }
    END { exit found ? 0 : 1 }
  ' "$env_path"
}

upsert_env_value() {
  local env_path="$1"
  local key="$2"
  local value="$3"
  local tmp

  mkdir -p "$(dirname "$env_path")"
  if [ ! -f "$env_path" ]; then
    printf '%s=%s\n' "$key" "$value" >"$env_path"
    return
  fi

  tmp="$(mktemp)"
  awk -F= -v key="$key" -v value="$value" '
    BEGIN { updated = 0 }
    $0 ~ "^[[:space:]]*#" {
      print
      next
    }
    $1 == key {
      if (!updated) {
        print key "=" value
        updated = 1
      }
      next
    }
    { print }
    END {
      if (!updated) {
        print key "=" value
      }
    }
  ' "$env_path" >"$tmp"
  mv "$tmp" "$env_path"
}

remove_env_key() {
  local env_path="$1"
  local key="$2"
  local tmp
  [ -f "$env_path" ] || return 0

  tmp="$(mktemp)"
  awk -F= -v key="$key" '
    $0 ~ "^[[:space:]]*#" { print; next }
    $1 == key { next }
    { print }
  ' "$env_path" >"$tmp"
  mv "$tmp" "$env_path"
}

main_env_uses_db_field_model() {
  local env_path="$1"
  env_has_key "$env_path" "DB_HOST" &&
    env_has_key "$env_path" "DB_USER" &&
    env_has_key "$env_path" "DB_NAME"
}

read_main_port_from_env() {
  local env_path="$1"
  local default_value="${2:-8080}"
  if env_has_key "$env_path" "PORT"; then
    read_env_value "$env_path" "PORT" "$default_value"
    return
  fi
  read_env_value "$env_path" "SERVER_PORT" "$default_value"
}

write_parallel_main_env_template() {
  local env_path="$1"
  local candidate_port="$2"
  local bridge_url="${3:-http://127.0.0.1:8081}"

  mkdir -p "$(dirname "$env_path")"
  cat >"$env_path" <<EOF
PORT=$candidate_port
DB_HOST=127.0.0.1
DB_PORT=3306
DB_USER=
DB_PASS=
DB_NAME=
ERP_BRIDGE_BASE_URL=$bridge_url
EOF
  if [ -n "${TZ:-}" ]; then
    printf 'TZ=%s\n' "$TZ" >>"$env_path"
  fi
}

tcp_ready() {
  local host="$1"
  local port="$2"
  if ! command -v timeout >/dev/null 2>&1; then
    return 1
  fi
  timeout 2 bash -c "cat < /dev/null > /dev/tcp/$host/$port" >/dev/null 2>&1
}

json_escape() {
  printf '%s' "${1:-}" | sed \
    -e 's/\\/\\\\/g' \
    -e 's/"/\\"/g' \
    -e ':a' -e 'N' -e '$!ba' -e 's/\n/\\n/g'
}

resolve_go_entrypoint() {
  local root="$1"
  local main_entrypoint_dir="$root/cmd/server"
  local main_entrypoint_file="$main_entrypoint_dir/main.go"

  if [ ! -d "$main_entrypoint_dir" ]; then
    fail "MAIN packaging entrypoint is locked to ./cmd/server, but $main_entrypoint_dir is missing. cmd/api fallback is disabled."
  fi

  if [ ! -f "$main_entrypoint_file" ]; then
    fail "MAIN packaging entrypoint is locked to ./cmd/server, but $main_entrypoint_file is missing. cmd/api fallback is disabled."
  fi

  printf '%s\n' "./cmd/server"
}

native_path_for_tool() {
  local tool="$1"
  local path="$2"
  if [[ "$tool" = *.exe ]] && command -v wslpath >/dev/null 2>&1; then
    wslpath -w "$path"
    return
  fi
  printf '%s\n' "$path"
}

ps_single_quote_escape() {
  printf '%s' "${1:-}" | sed "s/'/''/g"
}

go_build_linux_amd64() {
  local root="$1"
  local go_tool="$2"
  local output_path="$3"
  local entrypoint="$4"

  if [[ "$go_tool" = *.exe ]]; then
    command -v powershell.exe >/dev/null 2>&1 || fail "powershell.exe is required when using go.exe from bash."

    local go_tool_path
    local native_root
    local native_output
    go_tool_path="$(command -v "$go_tool" || printf '%s' "$go_tool")"
    go_tool_path="$(native_path_for_tool "$go_tool" "$go_tool_path")"
    native_root="$(native_path_for_tool "$go_tool" "$root")"
    native_output="$(native_path_for_tool "$go_tool" "$output_path")"

    powershell.exe -NoProfile -NonInteractive -Command "& {
      Set-Location -LiteralPath '$(ps_single_quote_escape "$native_root")'
      \$env:CGO_ENABLED = '0'
      \$env:GOOS = 'linux'
      \$env:GOARCH = 'amd64'
      & '$(ps_single_quote_escape "$go_tool_path")' build -o '$(ps_single_quote_escape "$native_output")' '$entrypoint'
      if (\$LASTEXITCODE -ne 0) { exit \$LASTEXITCODE }
    }"
    return
  fi

  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 "$go_tool" build -o "$output_path" "$entrypoint"
}

package_release() {
  local root="$1"
  local version="$2"
  local output_root="$3"
  local skip_tests="$4"
  local artifact_prefix="$5"
  local bridge_base_url="$6"
  local dist_root
  local artifact_dir_name
  local stage_root
  local artifact_path
  local deploy_root
  local entrypoint
  local go_tool
  local main_output
  local bridge_output

  dist_root="$(resolve_path "$root" "$output_root")"
  artifact_dir_name="${artifact_prefix}-${version}-linux-amd64"
  stage_root="$dist_root/$artifact_dir_name"
  artifact_path="$dist_root/${artifact_dir_name}.tar.gz"
  deploy_root="$stage_root/deploy"
  entrypoint="$(resolve_go_entrypoint "$root")"
  go_tool="$(go_cmd)"
  main_output="$stage_root/ecommerce-api"
  bridge_output="$stage_root/erp_bridge"

  rm -rf "$stage_root" "$artifact_path"
  mkdir -p "$stage_root" "$stage_root/config" "$stage_root/db" "$stage_root/docs" "$deploy_root"

  (
    cd "$root"
    if [ "$skip_tests" != "true" ]; then
      "$go_tool" test ./...
    fi
    go_build_linux_amd64 "$root" "$go_tool" "$main_output" "$entrypoint"
    go_build_linux_amd64 "$root" "$go_tool" "$bridge_output" "$entrypoint"
  )

  [ -f "$stage_root/ecommerce-api" ] || fail "Linux build output missing: $stage_root/ecommerce-api"
  [ -f "$stage_root/erp_bridge" ] || fail "Linux build output missing: $stage_root/erp_bridge"

  cp "$root"/config/*.json "$stage_root/config/"
  cp -R "$root/db/migrations" "$stage_root/db/"
  cp "$root/docs/api/openapi.yaml" "$stage_root/docs/openapi.yaml"
  if [ -f "$root/deploy/generate-release-docs.sh" ]; then
    # Tolerate accidental CRLF checkout on Windows hosts.
    bash <(tr -d '\r' <"$root/deploy/generate-release-docs.sh") "$version" "$stage_root/docs"
  fi
  cp "$root/deploy/main.env.example" "$stage_root/.env.example"
  cp "$root/deploy/bridge.env.example" "$stage_root/bridge.env.example"
  cp "$root/deploy/DEPLOYMENT_WORKFLOW.md" "$stage_root/README_DEPLOY.md"

  local helpe
  for helper in lib.sh remote-deploy.sh run-with-env.sh run-migrations-v05.sh run-org-master-convergence.sh verify-v05-acceptance.sh start-main.sh stop-main.sh start-bridge.sh stop-bridge.sh start-sync.sh stop-sync.sh verify-runtime.sh check-three-services.sh check-remote-db.sh; do
    cp "$root/deploy/$helper" "$deploy_root/$helper"
  done
  # Normalize packaged shell helpers to LF to avoid CRLF parse failures on Linux.
  local deploy_script
  local normalized_script
  for deploy_script in "$deploy_root/"*.sh; do
    [ -f "$deploy_script" ] || continue
    normalized_script="${deploy_script}.normalized"
    tr -d '\r' <"$deploy_script" >"$normalized_script"
    mv "$normalized_script" "$deploy_script"
  done
  chmod +x "$stage_root/ecommerce-api" "$stage_root/erp_bridge" "$deploy_root/"*.sh

  cat >"$stage_root/PACKAGE_INFO.json" <<EOF
{
  "version": "$(json_escape "$version")",
  "artifact_directory": "$(json_escape "$artifact_dir_name")",
  "artifact_archive": "$(json_escape "$(basename "$artifact_path")")",
  "main_binary": "ecommerce-api",
  "bridge_binary": "erp_bridge",
  "resolved_entrypoint": "$(json_escape "$entrypoint")",
  "main_build_command": "CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ecommerce-api $(json_escape "$entrypoint")",
  "bridge_build_command": "CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o erp_bridge $(json_escape "$entrypoint")",
  "runtime_bridge_base_url": "$(json_escape "$bridge_base_url")",
  "suggested_remote_base_dir": "/root/ecommerce_ai",
  "runtime_env_example": ".env.example",
  "bridge_env_example": "bridge.env.example",
  "package_created_at_utc": "$(utc_now)"
}
EOF

  (
    cd "$dist_root"
    tar -czf "$artifact_path" "$artifact_dir_name"
  )

  PACKAGE_VERSION="$version"
  PACKAGE_ENTRYPOINT="$entrypoint"
  PACKAGE_STAGE_ROOT="$stage_root"
  PACKAGE_ARTIFACT_DIR_NAME="$artifact_dir_name"
  PACKAGE_ARTIFACT_PATH="$artifact_path"
  PACKAGE_ARTIFACT_NAME="$(basename "$artifact_path")"
  PACKAGE_ARTIFACT_SHA256="$(sha256_of "$artifact_path")"
}

ssh_runner() {
  local target="$1"
  shift
  local auth_mode="${DEPLOY_AUTH_MODE:-key}"
  local ssh_tool="ssh"
  if command -v wslpath >/dev/null 2>&1 && command -v ssh.exe >/dev/null 2>&1; then
    ssh_tool="ssh.exe"
  fi
  case "$auth_mode" in
    key)
      "$ssh_tool" -o BatchMode=yes -o StrictHostKeyChecking=accept-new -p "${DEPLOY_PORT:-22}" "$target" "$@"
      ;;
    password)
      [ -n "${DEPLOY_PASSWORD:-}" ] || fail "DEPLOY_AUTH_MODE=password requires DEPLOY_PASSWORD."
      if command -v sshpass >/dev/null 2>&1; then
        sshpass -p "$DEPLOY_PASSWORD" ssh \
          -o StrictHostKeyChecking=accept-new \
          -o PreferredAuthentications=password \
          -o PubkeyAuthentication=no \
          -p "${DEPLOY_PORT:-22}" \
          "$target" "$@"
        return
      fi
      command -v setsid >/dev/null 2>&1 || fail "Password auth requires sshpass or setsid + SSH_ASKPASS support."
      local askpass_script
      askpass_script="$(mktemp)"
      printf '#!/bin/sh\nprintf "%%s\\n" "$DEPLOY_PASSWORD"\n' >"$askpass_script"
      chmod 700 "$askpass_script"
      DISPLAY="${DISPLAY:-:0}" SSH_ASKPASS="$askpass_script" SSH_ASKPASS_REQUIRE=force DEPLOY_PASSWORD="$DEPLOY_PASSWORD" \
        setsid ssh \
          -o StrictHostKeyChecking=accept-new \
          -o PreferredAuthentications=password \
          -o PubkeyAuthentication=no \
          -p "${DEPLOY_PORT:-22}" \
          "$target" "$@" </dev/null
      local status=$?
      rm -f "$askpass_script"
      return "$status"
      ;;
    *)
      fail "Unsupported DEPLOY_AUTH_MODE=$auth_mode. Use key or password."
      ;;
  esac
}

scp_runner() {
  local source_path="$1"
  local target="$2"
  local auth_mode="${DEPLOY_AUTH_MODE:-key}"
  local scp_tool="scp"
  local normalized_source="$source_path"
  if command -v wslpath >/dev/null 2>&1 && command -v scp.exe >/dev/null 2>&1; then
    scp_tool="scp.exe"
    normalized_source="$(native_path_for_tool "$scp_tool" "$source_path")"
  fi
  case "$auth_mode" in
    key)
      "$scp_tool" -o BatchMode=yes -o StrictHostKeyChecking=accept-new -P "${DEPLOY_PORT:-22}" "$normalized_source" "$target"
      ;;
    password)
      [ -n "${DEPLOY_PASSWORD:-}" ] || fail "DEPLOY_AUTH_MODE=password requires DEPLOY_PASSWORD."
      if command -v sshpass >/dev/null 2>&1; then
        sshpass -p "$DEPLOY_PASSWORD" scp \
          -o StrictHostKeyChecking=accept-new \
          -o PreferredAuthentications=password \
          -o PubkeyAuthentication=no \
          -P "${DEPLOY_PORT:-22}" \
          "$source_path" "$target"
        return
      fi
      command -v setsid >/dev/null 2>&1 || fail "Password auth requires sshpass or setsid + SSH_ASKPASS support."
      local askpass_script
      askpass_script="$(mktemp)"
      printf '#!/bin/sh\nprintf "%%s\\n" "$DEPLOY_PASSWORD"\n' >"$askpass_script"
      chmod 700 "$askpass_script"
      DISPLAY="${DISPLAY:-:0}" SSH_ASKPASS="$askpass_script" SSH_ASKPASS_REQUIRE=force DEPLOY_PASSWORD="$DEPLOY_PASSWORD" \
        setsid scp \
          -o StrictHostKeyChecking=accept-new \
          -o PreferredAuthentications=password \
          -o PubkeyAuthentication=no \
          -P "${DEPLOY_PORT:-22}" \
          "$source_path" "$target" </dev/null
      local status=$?
      rm -f "$askpass_script"
      return "$status"
      ;;
    *)
      fail "Unsupported DEPLOY_AUTH_MODE=$auth_mode. Use key or password."
      ;;
  esac
}
