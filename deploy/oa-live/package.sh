#!/usr/bin/env bash
set -euo pipefail
root=$(git rev-parse --show-toplevel)
cd "$root"
sha=$(git rev-parse HEAD)
out=${1:?specify an absolute archive path}
[[ "$out" == /* ]]
stage=$(mktemp -d)
trap 'rm -rf -- "$stage"' EXIT
mkdir -p "$stage/web"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o "$stage/oa-extension" ./cmd/oa-extension
(cd vue && bash scripts/build-oa.sh)
cp -a vue/dist-oa/. "$stage/web/"
test -s "$stage/web/index.html"
cp deploy/oa-live/{site.py,yongbo-oa-live.service} "$stage/"
printf '%s\n' "$sha" > "$stage/REVISION"
printf '{"commit":"%s","frontend":"main-ops","backend":"shared-production","data":"formal"}\n' "$sha" > "$stage/web/oa-release.json"
tar -czf "$out" -C "$stage" .
sha256sum "$out"
