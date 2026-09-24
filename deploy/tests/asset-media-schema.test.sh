#!/usr/bin/env bash
set -euo pipefail
# Inputs: repository-shaped fixture directory and a prebuilt Linux test binary.
# No production credentials, mounts, published ports, or network access.
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
binary="${1:?usage: asset-media-schema.test.sh /absolute/path/mysql.test}"
[[ "$binary" = /* && -f "$binary" ]] || exit 2
fixture="asset-media-fixture-$(date +%s)-$$"
docker image inspect mysql:8.0 >/dev/null
cleanup() { docker rm -f -v "$fixture" >/dev/null 2>&1 || true; }
trap cleanup EXIT
docker run --detach --name "$fixture" --network none --memory 1g --cpus 1 \
  --label yongbo.test=asset-media --env MYSQL_ALLOW_EMPTY_PASSWORD=yes \
  --env MYSQL_DATABASE=asset_media_test mysql:8.0 >/dev/null
ready=false
for ((i=0;i<60;i++)); do
  if docker exec "$fixture" mysqladmin -h127.0.0.1 ping --silent >/dev/null 2>&1; then ready=true; break; fi
  sleep 1
done
[[ "$ready" = true ]] || { docker logs --tail 30 "$fixture"; exit 1; }
awk '/^-- ROLLBACK-BEGIN/{exit} {print}' "$root/db/migrations/113_external_asset_source_modified_at.sql" | docker exec -i "$fixture" mysql asset_media_test
awk '/^-- ROLLBACK-BEGIN/{exit} {print}' "$root/db/migrations/081_v1_3_external_asset_index.sql" | docker exec -i "$fixture" mysql asset_media_test
docker exec -i "$fixture" mysql asset_media_test < "$root/db/migrations/142_asset_media_delivery.sql"
docker cp "$binary" "$fixture:/tmp/asset-media-mysql.test"
docker exec --env 'ASSET_MEDIA_TEST_DSN=root@unix(/var/run/mysqld/mysqld.sock)/asset_media_test?parseTime=true&loc=UTC' \
  "$fixture" /tmp/asset-media-mysql.test -test.run '^TestAssetMediaMySQLIntegration$' -test.v
