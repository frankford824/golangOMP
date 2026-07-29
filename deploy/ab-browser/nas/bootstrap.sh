#!/usr/bin/env bash
set -euo pipefail

ROOT="${ROOT:-/volume1/docker/yongbo-v8-cloneb}"
DOCKER="${DOCKER:-/usr/local/bin/docker}"
COMPOSE=("$DOCKER" compose --env-file "$ROOT/.env" -f "$ROOT/compose.yaml")
ACTION="${1:-status}"
SOURCE_ALL_TABLE_COUNTS_SHA256="abeb6d2f74af6843474b504bd2233f7bf410afd5d5b7aac95e0e5bab0c433b01"
SOURCE_FIXTURE_ROOT_TREE_SHA256="ad70053456c5a6503e22b08e98d97dfac92da3a54592f075296fdd7823976a45"
SOURCE_FIXTURE_SEED_TREE_SHA256="f9e4d02c308aec2e3918ef5bed37b7c9bfb1dbccf775d50df9f90119fededd56"

cd "$ROOT"

random_hex() {
  openssl rand -hex 24
}

prepare_env() {
  if [[ -f .env ]]; then
    chmod 600 .env
    return
  fi

  umask 077
  {
    printf 'MYSQL_ROOT_PASSWORD=%s\n' "$(random_hex)"
    printf 'MYSQL_DATABASE=%s\n' 'cloneb_v8_6858cee'
    printf 'MYSQL_APP_USER=%s\n' 'cloneb_app'
    printf 'MYSQL_APP_PASSWORD=%s\n' "$(random_hex)"
    printf 'REDIS_PASSWORD=%s\n' "$(random_hex)"
    printf 'WEB_PORT=%s\n' '18180'
  } >.env
}

load_env() {
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
}

tag_images() {
  local pairs=(
    'sha256:dd989d4015a54d99c8cb677d9e0d2bd624c4114cb86c3b7d3a723f7d686d6041 yb-v8-cloneb-backend:d9275bd88f90'
    'sha256:812d47f806db497c53f9b47e76bdab38bcf5d724c69d3986df5ee4336c210559 yb-v8-cloneb-nginx:5616878291a2'
    'sha256:6cd09145362dfe6831b14545de3d5fd6cc75c37cfd6ef8561429c1fc73518b39 yb-v8-cloneb-mysql:7dcddc01f13b'
    'sha256:487efc0616382465781b8fdc3d6d1db449e6fd80ae23bf48432a2da6b6929908 yb-v8-cloneb-redis:6ab0b6e73817'
    'sha256:b50b146cafc14e1dd217a955108727508ba2012cedddf4787a181aaa15585f2d yb-v8-cloneb-fixture-upload:be4f501b1b94'
    'sha256:9f619c50d56e45dab935222813765494ed4a68123de57da8b56cf125031d3e98 yb-v8-cloneb-fixture-object:fd2e163c776e'
  )
  local pair source target
  for pair in "${pairs[@]}"; do
    read -r source target <<<"$pair"
    "$DOCKER" image inspect "$source" >/dev/null
    "$DOCKER" tag "$source" "$target"
  done
}

wait_healthy() {
  local service="$1"
  local deadline=$((SECONDS + 300))
  local container_id status
  while (( SECONDS < deadline )); do
    container_id="$("${COMPOSE[@]}" ps -q "$service")"
    if [[ -n "$container_id" ]]; then
      status="$("$DOCKER" inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container_id")"
      if [[ "$status" == "healthy" || "$status" == "running" ]]; then
        return 0
      fi
    fi
    sleep 3
  done
  echo "Service did not become ready: $service" >&2
  "${COMPOSE[@]}" logs --tail 80 "$service" >&2 || true
  return 1
}

verify_artifacts() {
  (
    cd artifacts
    sha256sum -c manifest.sha256
  )
}

import_database_once() {
  if [[ -f evidence/database-imported.sha256 ]]; then
    return
  fi

  local dump_hash
  local import_status
  dump_hash="$(sha256sum artifacts/cloneb.sql.gz | awk '{print $1}')"

  "${COMPOSE[@]}" exec -T -e "MYSQL_PWD=$MYSQL_ROOT_PASSWORD" mysql \
    mysql --user=root \
    -e "SET GLOBAL innodb_flush_log_at_trx_commit=2; SET GLOBAL sync_binlog=0"

  set +e
  gzip -dc artifacts/cloneb.sql.gz |
    "${COMPOSE[@]}" exec -T -e "MYSQL_PWD=$MYSQL_ROOT_PASSWORD" mysql \
      mysql --user=root "$MYSQL_DATABASE"
  import_status=$?
  set -e

  "${COMPOSE[@]}" exec -T -e "MYSQL_PWD=$MYSQL_ROOT_PASSWORD" mysql \
    mysql --user=root \
    -e "SET GLOBAL innodb_flush_log_at_trx_commit=1; SET GLOBAL sync_binlog=1"
  if [[ "$import_status" -ne 0 ]]; then
    return "$import_status"
  fi

  printf '%s\n' "$dump_hash" >evidence/database-imported.sha256
}

query_count() {
  local table="$1"
  "${COMPOSE[@]}" exec -T -e "MYSQL_PWD=$MYSQL_ROOT_PASSWORD" mysql \
    mysql --user=root --batch --skip-column-names "$MYSQL_DATABASE" \
    -e "SELECT COUNT(*) FROM ${table}" | tr -d '\r'
}

verify_database() {
  local failures=0
  local checks=(
    'tasks 2382'
    'task_asset_groups 3195'
    'task_asset_group_revisions 2919'
    'task_assets 27954'
    'users 148'
    'task_event_logs 66710'
  )
  local check table expected actual
  for check in "${checks[@]}"; do
    read -r table expected <<<"$check"
    actual="$(query_count "$table")"
    printf '%s expected=%s actual=%s\n' "$table" "$expected" "$actual"
    if [[ "$actual" != "$expected" ]]; then
      failures=$((failures + 1))
    fi
  done
  [[ "$failures" -eq 0 ]]
}

fixture_tree_hash() {
  local path="$1"
  "${COMPOSE[@]}" exec -T fixture-upload sh -c \
    "find '$path' -type f -print0 | sort -z | xargs -0 sha256sum | sha256sum" |
    awk '{print $1}'
}

verify_fixture_trees() {
  local root_hash
  local seed_hash
  seed_hash="$(fixture_tree_hash /run/ab/seed)"
  if [[ "$seed_hash" != "$SOURCE_FIXTURE_SEED_TREE_SHA256" ]]; then
    echo "Fixture upload seed mismatch: expected=$SOURCE_FIXTURE_SEED_TREE_SHA256 actual=$seed_hash" >&2
    return 1
  fi
  if [[ ! -f evidence/fixture-tree-hashes.txt ]]; then
    root_hash="$(fixture_tree_hash /run/ab/root)"
    if [[ "$root_hash" != "$SOURCE_FIXTURE_ROOT_TREE_SHA256" ]]; then
      echo "Initial fixture upload root mismatch: expected=$SOURCE_FIXTURE_ROOT_TREE_SHA256 actual=$root_hash" >&2
      return 1
    fi
    {
      printf 'initial_fixture_upload_root_tree_sha256=%s\n' "$root_hash"
      printf 'fixture_upload_seed_tree_sha256=%s\n' "$seed_hash"
    } >evidence/fixture-tree-hashes.txt
    sha256sum evidence/fixture-tree-hashes.txt >evidence/fixture-tree-hashes.sha256
  fi
}

write_full_table_counts() {
  local actual_hash
  "${COMPOSE[@]}" exec -T -e "MYSQL_PWD=$MYSQL_ROOT_PASSWORD" mysql \
    mysql --user=root --batch --skip-column-names "$MYSQL_DATABASE" \
    <full-table-counts.sql >evidence/all-table-counts.tsv
  actual_hash="$(sha256sum evidence/all-table-counts.tsv | awk '{print $1}')"
  if [[ "$actual_hash" != "$SOURCE_ALL_TABLE_COUNTS_SHA256" ]]; then
    echo "Full table-count fingerprint mismatch: expected=$SOURCE_ALL_TABLE_COUNTS_SHA256 actual=$actual_hash" >&2
    return 1
  fi
  sha256sum evidence/all-table-counts.tsv >evidence/all-table-counts.sha256
}

probe_runtime() {
  curl --fail --silent --show-error --max-time 10 http://127.0.0.1:18180/health
  echo
  curl --fail --silent --show-error --max-time 10 http://127.0.0.1:18180/__ab/identity
  echo
  curl --fail --silent --show-error --max-time 10 \
    --output /dev/null http://127.0.0.1:18180/tasks/1264
}

write_receipt() {
  {
    printf 'deployed_at=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    printf 'source_candidate=%s\n' '6858cee'
    printf 'source_backend_sha256=%s\n' 'd9275bd88f90f6dbff1663f6dd42f3807b289855eb4d9fc08fbdbe79a050a574'
    printf 'source_frontend_sha256=%s\n' '1bdc33088b3dec096eb226a783c66f9e19c198fd4ffb6b8f137d2715e4a15ae0'
    printf 'database_dump_sha256=%s\n' "$(cat evidence/database-imported.sha256)"
    printf 'all_table_counts_sha256=%s\n' "$(awk '{print $1}' evidence/all-table-counts.sha256)"
    printf 'fixture_tree_hashes_sha256=%s\n' "$(awk '{print $1}' evidence/fixture-tree-hashes.sha256)"
    printf 'entrypoint=%s\n' 'http://192.168.0.125:18180'
    "$DOCKER" image inspect \
      yb-v8-cloneb-backend:d9275bd88f90 \
      yb-v8-cloneb-nginx:5616878291a2 \
      --format 'image={{.RepoTags}} id={{.Id}}'
  } >evidence/deployment-receipt.txt
  sha256sum evidence/deployment-receipt.txt >evidence/deployment-receipt.sha256
}

start() {
  prepare_env
  load_env
  verify_artifacts
  tag_images
  mkdir -p evidence
  chmod 644 config/auth-b.json

  "${COMPOSE[@]}" up -d mysql redis fixture-upload fixture-object
  wait_healthy mysql
  wait_healthy redis
  wait_healthy fixture-upload
  wait_healthy fixture-object
  verify_fixture_trees
  import_database_once
  verify_database
  write_full_table_counts

  "${COMPOSE[@]}" up -d erp-bridge backend edge
  wait_healthy erp-bridge
  wait_healthy edge
  probe_runtime
  write_receipt
  echo "Clone B NAS environment ready: http://192.168.0.125:18180"
}

status() {
  prepare_env
  load_env
  "${COMPOSE[@]}" ps -a
  if "${COMPOSE[@]}" ps -q edge >/dev/null 2>&1; then
    probe_runtime
  fi
}

case "$ACTION" in
  start)
    start
    ;;
  status)
    status
    ;;
  stop)
    prepare_env
    load_env
    "${COMPOSE[@]}" stop
    ;;
  *)
    echo "Usage: $0 {start|status|stop}" >&2
    exit 2
    ;;
esac
