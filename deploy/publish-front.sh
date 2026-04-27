#!/usr/bin/env bash
# 涓€閿皢鏈湴 dist/front 鍙戝竷鍒拌繙绔?/var/www/yongbo.cloud锛堜笌 publish-front.ps1 鍚屼竴 SOP锛夈€?
# 渚濊禆锛歴sh銆乻cp锛涜繙绔渶鏈?rsync銆乶ginx銆傚彲閫夋湰鏈?rsync 浠ュ姞閫熶笂浼犮€?
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
FRONT="$ROOT/dist/front"
SSH_HOST="${SSH_HOST:-jst_ecs}"
REMOTE_WEB="/var/www/yongbo.cloud"
REMOTE_BACKUP_PARENT="/var/www/backups"

SKIP_CHECKS=false
SKIP_VERIFY=false
DRY_RUN=false

usage() {
  echo "Usage: $0 [--skip-checks] [--skip-verify] [--dry-run] [--host HOST]"
  echo "  Env: SSH_HOST (default: jst_ecs)"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-checks) SKIP_CHECKS=true ;;
    --skip-verify) SKIP_VERIFY=true ;;
    --dry-run) DRY_RUN=true ;;
    --host) SSH_HOST="$2"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown arg: $1"; usage; exit 1 ;;
  esac
  shift
done

die() { echo "[publish-front] ERROR: $*" >&2; exit 1; }
step() { echo "[publish-front] $*"; }

if [[ "$SKIP_CHECKS" != "true" ]]; then
  step "鏈湴妫€鏌?dist/front ..."
  [[ -d "$FRONT" ]] || die "缂哄皯鐩綍: $FRONT"
  [[ -f "$FRONT/index.html" ]] || die "缂哄皯: $FRONT/index.html"
  [[ -d "$FRONT/assets" ]] || die "缂哄皯鐩綍: $FRONT/assets"
  if grep -qE 'localhost|127\.0\.0\.1' "$FRONT/index.html"; then
    die "index.html 鍚?localhost / 127.0.0.1"
  fi
  main_js=""
  main_js=$(grep -oE 'src="/assets/[^"]+\.js"' "$FRONT/index.html" | head -1 | sed 's/src="//;s/"$//' || true)
  if [[ -n "$main_js" ]]; then
    rel="${main_js#/}"
    path="$FRONT/$rel"
    if [[ -f "$path" ]]; then
      if grep -qE 'https?://localhost|https?://127\.0\.0\.1|127\.0\.0\.1:[0-9]+' "$path"; then
        echo "[publish-front] WARN: main bundle contains localhost/127.0.0.1 literals; confirm API is still /v1" >&2
      fi
      if ! grep -q '"/v1' "$path"; then
        echo "[publish-front] WARN: main bundle has no literal \"/v1\"; verify API base" >&2
      fi
    fi
  fi
  if ! grep -rEl 'beian\.miit\.gov\.cn|2026007026' "$FRONT" --include='*.html' --include='*.js' --include='*.css' -q 2>/dev/null; then
    echo "[publish-front] WARN: 鏈娴嬪埌澶囨鍙锋垨 beian 閾炬帴锛岃纭椤甸潰宸插睍绀? >&2
  fi
  step "鏈湴妫€鏌ラ€氳繃"
fi

command -v ssh >/dev/null || die "闇€瑕?ssh"
command -v scp >/dev/null || die "闇€瑕?scp"

if [[ "$DRY_RUN" == "true" ]]; then
  step "DryRun: Host=$SSH_HOST 婧?$FRONT 鐩爣=$REMOTE_WEB"
  exit 0
fi

step "鑾峰彇杩滅 UTC 鏃堕棿鎴?..."
TS="$(ssh "$SSH_HOST" 'date -u +%Y%m%dT%H%M%SZ' | tr -d '\r\n')"
[[ -n "$TS" ]] || die "鏃犳硶鍙栧緱杩滅鏃堕棿鎴?

BACKUP="$REMOTE_BACKUP_PARENT/yongbo.cloud_${TS}"
STAGING="/tmp/yongbo.cloud_dist_${TS}"

step "杩滅澶囦唤: $BACKUP"
ssh "$SSH_HOST" "mkdir -p \"$BACKUP\" && cp -a \"$REMOTE_WEB\"/. \"$BACKUP\"/ && mkdir -p \"$STAGING\""

step "涓婁紶鍒? $STAGING"
if command -v rsync >/dev/null 2>&1; then
  rsync -av --delete "$FRONT/" "$SSH_HOST:$STAGING/"
else
  scp -r "$FRONT"/* "$SSH_HOST:$STAGING/"
fi

step "鍚屾鑷虫寮忕洰褰曘€佹潈闄愩€乶ginx reload"
ssh "$SSH_HOST" "rsync -a --delete \"$STAGING\"/ \"$REMOTE_WEB\"/ && chmod -R a+rX \"$REMOTE_WEB\" && nginx -t && systemctl reload nginx"

step "鍙戝竷瀹屾垚銆傚浠? $BACKUP"

if [[ "$SKIP_VERIFY" != "true" ]] && command -v curl >/dev/null 2>&1; then
  step "HTTP 鎺㈡祴 yongbo.cloud ..."
  code="$(curl -sS -o /dev/null -w '%{http_code}' "https://yongbo.cloud/")"
  [[ "$code" == "200" ]] || echo "[publish-front] WARN: 棣栭〉 HTTP $code" >&2
  code="$(curl -sS -o /dev/null -w '%{http_code}' "https://yongbo.cloud/login")"
  [[ "$code" == "200" ]] || echo "[publish-front] WARN: /login HTTP $code" >&2
  code="$(curl -sS -o /dev/null -w '%{http_code}' -X POST "https://yongbo.cloud/v1/auth/login" -H "Content-Type: application/json" -d '{}')"
  [[ "$code" == "404" ]] || true
  step "楠岃瘉缁撴潫锛堣娴忚鍣ㄤ笌鐪熷疄璐﹀彿鎶芥煡锛?
fi
