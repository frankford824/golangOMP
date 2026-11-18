#!/usr/bin/env bash
# Switch only the OA hostname. The core backend and both established sites keep serving.
set -euo pipefail
archive=${1:?release archive}; digest=${2:?expected SHA256}
[[ $(id -u) = 0 && "$digest" =~ ^[0-9a-f]{64}$ && "$(sha256sum "$archive" | cut -d' ' -f1)" == "$digest" ]]
base=/srv/yongbo-oa-live
install -d -m755 "$base/releases"
exec 8>/run/lock/yongbo-oa-live-deploy.lock; flock -x 8
stage=$(mktemp -d "$base/releases/build-XXXXXXXX")
tar -xzf "$archive" --no-same-owner -C "$stage"
sha=$(cat "$stage/REVISION")
[[ "$sha" =~ ^[0-9a-f]{40}$ && -x "$stage/oa-extension" && -s "$stage/web/index.html" ]]
release="$base/releases/${sha:0:12}-${digest:0:10}"
if [[ ! -d "$release" ]]; then chmod 755 "$stage";mv "$stage" "$release";else [[ "$stage" == "$base/releases/build-"* ]];rm -rf -- "$stage";fi
getent passwd yongbo-oa-live >/dev/null || useradd --system --user-group --home-dir /var/lib/yongbo-oa-live --shell /usr/sbin/nologin yongbo-oa-live
test -r /etc/yongbo-oa-live/service.env
install -m644 "$release/yongbo-oa-live.service" /etc/systemd/system/
site=/etc/nginx/sites-available/oa.yongbo.cloud
main=/etc/nginx/sites-enabled/yongbo.cloud
test -e /etc/letsencrypt/live/oa.yongbo.cloud/fullchain.pem
if [[ ! -L "$base/current" ]];then
  # An emergency frontend fallback is a real main-ops static release, not a test site.
  fallback="$base/releases/base-$(date -u +%Y%m%dT%H%M%SZ)"
  install -d -m755 "$fallback/web"
  cp -a /var/www/yongbo.cloud/. "$fallback/web/"
  ln -s "$fallback" "$base/current"
fi
prior=$(readlink -f "$base/current")
[[ "$prior" == "$base/releases/"* ]]
select_release(){ ln -sfn "$1" "$base/current.next";mv -Tf "$base/current.next" "$base/current"; }
render_site(){ python3 "$release/site.py" "$main" > "$site.next";chmod 644 "$site.next";mv -Tf "$site.next" "$site"; }
restore(){
  code=$?;trap - ERR
  select_release "$prior"
  if [[ -x "$prior/oa-extension" ]]; then systemctl restart yongbo-oa-live || true; else systemctl stop yongbo-oa-live || true; fi
  render_site
  if nginx -t;then systemctl reload nginx;fi
  echo 'OA reverted to its previous formal frontend; original sites were not switched.' >&2; exit "$code"
}
select_release "$release"
trap restore ERR
systemctl daemon-reload
systemctl enable yongbo-oa-live.service
systemctl restart yongbo-oa-live.service
ready=0
for ((i=0;i<25;i++));do if curl -fsS -m 2 http://127.0.0.1:18095/oa-api/health >/dev/null 2>&1; then ready=1;break;fi;sleep 1;done
[[ "$ready" = 1 ]]
render_site
if [[ ! -e /etc/nginx/sites-enabled/oa.yongbo.cloud ]];then ln -s "$site" /etc/nginx/sites-enabled/oa.yongbo.cloud;fi
nginx -t
systemctl reload nginx
served=0
for ((i=0;i<25;i++));do if curl -fsS -m 3 --resolve oa.yongbo.cloud:443:127.0.0.1 https://oa.yongbo.cloud/oa-release.json 2>/dev/null | grep -Fq "$sha";then served=1;break;fi;sleep .5;done
[[ "$served" = 1 ]]
for h in oa.yongbo.cloud yongbo.cloud assets.yongbo.cloud;do curl -fsS --max-time 8 --resolve "$h:443:127.0.0.1" "https://$h/" -o /dev/null;done
trap - ERR
printf '%s %s %s\n' "$(date -Is)" "$sha" "$digest" >> "$base/deployments.log"
echo "OA main-ops frontend active: $sha"
