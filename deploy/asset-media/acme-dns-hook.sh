#!/usr/bin/env bash
set -euo pipefail
umask 077
# Dedicated to this one NAS gateway name. Never replace domain-wide records.
[[ "${CERTBOT_DOMAIN:-}" == media-cache.yongbo.cloud ]] || exit 2
[[ "${CERTBOT_VALIDATION:-}" =~ ^[A-Za-z0-9_-]+$ ]] || exit 2
mode="${1:?auth or cleanup required}"
[[ "$mode" == auth || "$mode" == cleanup ]] || exit 2
set -a
. /root/ecommerce_ai/shared/main.env
set +a
export ALIBABA_CLOUD_ACCESS_KEY_ID="$OSS_ACCESS_KEY_ID"
export ALIBABA_CLOUD_ACCESS_KEY_SECRET="$OSS_ACCESS_KEY_SECRET"
state_dir=/root/ecommerce_ai/shared/media-acme
mkdir -p "$state_dir"
state="$state_dir/$CERTBOT_VALIDATION.json"
if [[ "$mode" == cleanup ]]; then
  [[ -f "$state" ]] || exit 0
  record_id="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["RecordId"])' "$state")"
  [[ "$record_id" =~ ^[0-9]+$ ]] || exit 2
  aliyun alidns DeleteDomainRecord --region cn-hangzhou --RecordId "$record_id" >/dev/null
  rm -- "$state"
  exit 0
fi
if [[ ! -f "$state" ]]; then
  aliyun alidns AddDomainRecord --region cn-hangzhou --DomainName yongbo.cloud \
    --RR _acme-challenge.media-cache --Type TXT --Value "$CERTBOT_VALIDATION" --TTL 600 > "$state.tmp"
  mv -- "$state.tmp" "$state"
fi
mapfile -t servers < <(dig +short NS yongbo.cloud)
[[ ${#servers[@]} -gt 0 ]] || exit 1
for ((attempt=0;attempt<30;attempt++)); do
  ready=true
  for server in "${servers[@]}"; do
    if ! dig +time=3 +tries=1 +short TXT _acme-challenge.media-cache.yongbo.cloud "@$server" | grep -Fxq "\"$CERTBOT_VALIDATION\""; then ready=false; fi
  done
  if [[ "$ready" == true ]]; then sleep 20; exit 0; fi
  sleep 5
done
printf 'DNS proof did not propagate; no network security bypass attempted\n' >&2
exit 1
