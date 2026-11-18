#!/usr/bin/env bash
# OA host only. Never switch the original /root/ecommerce_ai or its web roots.
set -euo pipefail
archive=${1:?release archive}; expected=${2:?archive SHA256}
[[ "$(id -u)" = 0 && "$expected" =~ ^[a-f0-9]{64}$ ]]
actual=$(sha256sum "$archive" | cut -d' ' -f1)
[[ "$actual" == "$expected" ]]
base=/srv/yongbo-oa
test -f /etc/yongbo-oa/provisioned
test -s /etc/letsencrypt/live/oa.yongbo.cloud/fullchain.pem
exec 8>/run/lock/yongbo-oa-deploy.lock
flock -x 8
stage=$(mktemp -d "$base/releases/incoming-XXXXXXXX")
tar -xzf "$archive" --no-same-owner -C "$stage"
sha=$(cat "$stage/REVISION")
[[ "$sha" =~ ^[0-9a-f]{40}$ ]]
for binary in oa oa-core oa-store; do test -x "$stage/$binary"; done
for page in oa oa-work oa-assets; do test -s "$stage/web/$page.html"; done
old=$(readlink -f "$base/current")
[[ "$old" == "$base/releases/"* ]]
release="$base/releases/${sha:0:12}-${actual:0:10}"
if [[ -d "$release" ]]; then
  [[ "$(cat "$release/REVISION")" == "$sha" ]]
  [[ "$stage" == "$base/releases/incoming-"* && -d "$stage" ]]
  rm -rf -- "$stage"
else
  chmod 755 "$stage"
  mv "$stage" "$release"
fi
site=/etc/nginx/sites-available/oa.yongbo.cloud
backup=$(mktemp /run/oa-site-XXXXXXXX)
cp "$site" "$backup"
switch_to() { ln -sfn "$1" "$base/current.next"; mv -Tf "$base/current.next" "$base/current"; }
restore() {
  rc=$?; trap - ERR
  switch_to "$old"
  cp "$backup" "$site"
  systemctl restart yongbo-oa-core.service || true
  if [[ -x "$old/oa" ]]; then systemctl restart yongbo-oa.service || true; else systemctl stop yongbo-oa.service || true; fi
  if nginx -t; then systemctl reload nginx; fi
  echo 'OA validation release failed; previous OA release selected.' >&2
  exit "$rc"
}
install -m644 "$release"/units/* /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now yongbo-oa-mysql.socket yongbo-oa-gateway.socket yongbo-oa-objects.socket
switch_to "$release"
trap restore ERR
systemctl enable yongbo-oa-core.service yongbo-oa.service
systemctl restart yongbo-oa-core.service yongbo-oa.service
ready=0
for ((i=0;i<40;i++)); do
  if curl -fsS -m 2 http://127.0.0.1:18087/health >/dev/null 2>&1 && curl -fsS -m 2 http://127.0.0.1:18085/oa-api/health >/dev/null 2>&1; then ready=1; break; fi
  sleep 1
done
[[ "$ready" = 1 ]]
install -m644 "$release/oa.yongbo.cloud.conf" "$site"
nginx -t
systemctl reload nginx
for domain in oa.yongbo.cloud yongbo.cloud assets.yongbo.cloud; do
  curl -fsS --max-time 8 --resolve "$domain:443:127.0.0.1" "https://$domain/" -o /dev/null
done
curl -fsS --max-time 5 https://oa.yongbo.cloud/oa-version.json | grep -Fq "$sha"
printf '%s %s %s\n' "$(date -Is)" "$sha" "$actual" >> "$base/deployments.log"
trap - ERR
rm -f "$backup"
echo "OA active $sha"
