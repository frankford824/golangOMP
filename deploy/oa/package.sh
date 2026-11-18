#!/usr/bin/env bash
# Build all three applications from one checked-out commit. No production release directory is used.
set -euo pipefail
root=$(git rev-parse --show-toplevel)
cd "$root"
sha=$(git rev-parse HEAD)
out=${1:?absolute output archive path required}
[[ "$out" == /* ]]
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp"/{web,config,units}
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o "$tmp/oa" ./cmd/oa
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o "$tmp/oa-core" ./cmd/server
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o "$tmp/oa-store" ./cmd/oa-store
(cd vue && npm run build:oa)
cp -a vue/dist-oa/. "$tmp/web/"
for entry in oa.html oa-work.html oa-assets.html; do test -s "$tmp/web/$entry"; done
cp config/frontend_access.json config/category_seed.json config/cost_rule_seed.json config/category_erp_mapping_seed.json config/erp_short_name_rules.json "$tmp/config/"
cp deploy/oa/start-core.sh deploy/oa/oa.yongbo.cloud.conf "$tmp/"
cp deploy/oa/*.service deploy/oa/*.socket "$tmp/units/"
printf '%s\n' "$sha" > "$tmp/REVISION"
printf '{"commit":"%s","environment":"validation"}\n' "$sha" > "$tmp/web/oa-version.json"
tar -czf "$out" -C "$tmp" .
sha256sum "$out"
