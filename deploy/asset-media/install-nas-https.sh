#!/bin/sh
set -eu
umask 077
# Execute only through an explicitly approved NAS administrator action.
# No account changes, port mappings, arbitrary configuration sourcing or deletes.
[ "$(id -u)" = 0 ] || { echo 'NAS administrator execution required' >&2; exit 1; }
incoming=/volume1/docker/yongbo-media/tls-incoming
store=/usr/local/etc/yongbo-media-tls
program=/usr/local/libexec/yongbo-media
config=/usr/local/etc/nginx/sites-enabled/yongbo-media.conf
mkdir -p "$store" "$program"
chmod 700 "$store" "$program"
if [ -f "$config" ] && ! grep -q '^# Managed by Yongbo asset media v1$' "$config"; then
  echo 'Refusing an unmanaged Nginx configuration' >&2; exit 1
fi
stage="$(mktemp -d "$store/.stage.XXXXXX")"
cleanup() {
  case "$stage" in /usr/local/etc/yongbo-media-tls/.stage.*) rm -rf -- "$stage" ;; *) exit 1 ;; esac
}
trap cleanup EXIT HUP INT TERM
cp -P "$incoming/fullchain.pem" "$stage/fullchain.pem"
cp -P "$incoming/privkey.pem" "$stage/privkey.pem"
for name in fullchain.pem privkey.pem; do
  [ -f "$stage/$name" ] && [ ! -L "$stage/$name" ] || exit 1
done
openssl x509 -in "$stage/fullchain.pem" -out "$stage/cert.pem"
openssl x509 -in "$stage/cert.pem" -noout -checkhost media-cache.yongbo.cloud >/dev/null
openssl x509 -in "$stage/cert.pem" -noout -checkend 86400 >/dev/null
openssl verify -CAfile /etc/ssl/certs/ca-certificates.crt -untrusted "$stage/fullchain.pem" "$stage/cert.pem" >/dev/null
openssl pkey -in "$stage/privkey.pem" -pubout -outform DER -out "$stage/key-public.der"
openssl x509 -in "$stage/cert.pem" -pubkey -noout > "$stage/cert-public.pem"
openssl pkey -pubin -in "$stage/cert-public.pem" -outform DER -out "$stage/cert-public.der"
cmp -s "$stage/key-public.der" "$stage/cert-public.der" || { echo 'Certificate/key mismatch' >&2; exit 1; }
cat > "$stage/nginx.conf" <<'NGINX'
# Managed by Yongbo asset media v1
server {
    listen 443 ssl;
    server_name media-cache.yongbo.cloud;
    ssl_certificate /usr/local/etc/yongbo-media-tls/fullchain.pem;
    ssl_certificate_key /usr/local/etc/yongbo-media-tls/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    access_log off;
    error_log /var/log/yongbo-media-nginx.log crit;
    allow 127.0.0.1;
    allow 192.168.0.0/24;
    allow 192.168.2.0/24;
    allow 100.125.196.22;
    deny all;
    location /edge/v1/ {
        proxy_pass http://127.0.0.1:18089;
        proxy_http_version 1.1;
        proxy_buffering off;
        proxy_request_buffering off;
        proxy_read_timeout 6h;
        proxy_send_timeout 6h;
        proxy_set_header Host $host;
        proxy_set_header Connection "";
    }
    location / { return 404; }
}
NGINX
if [ "$0" != "$program/refresh-tls.sh" ]; then
  cp "$0" "$program/refresh-tls.sh"
  chown 0:0 "$program/refresh-tls.sh"
  chmod 700 "$program/refresh-tls.sh"
fi
if [ -f "$store/fullchain.pem" ] && [ -f "$store/privkey.pem" ] && [ -f "$config" ] &&
   cmp -s "$stage/fullchain.pem" "$store/fullchain.pem" &&
   cmp -s "$stage/privkey.pem" "$store/privkey.pem" && cmp -s "$stage/nginx.conf" "$config"; then
  echo 'Yongbo media TLS/configuration unchanged'; exit 0
fi
if [ -f "$config" ]; then cp -p "$config" "$store/nginx.previous.conf"; fi
if [ -f "$store/fullchain.pem" ]; then cp -p "$store/fullchain.pem" "$store/fullchain.previous.pem"; fi
if [ -f "$store/privkey.pem" ]; then cp -p "$store/privkey.pem" "$store/privkey.previous.pem"; fi
mv -f "$stage/fullchain.pem" "$store/fullchain.pem"
mv -f "$stage/privkey.pem" "$store/privkey.pem"
cp "$stage/nginx.conf" "$config"
chmod 600 "$store/fullchain.pem" "$store/privkey.pem"
chmod 644 "$config"
if /usr/bin/nginx -t && /usr/syno/bin/synosystemctl reload nginx; then
  echo 'Yongbo media HTTPS configuration installed; no WAN mapping changed'
  openssl x509 -in "$store/fullchain.pem" -noout -enddate
else
  if [ -f "$store/nginx.previous.conf" ]; then cp -p "$store/nginx.previous.conf" "$config"; else rm -f -- /usr/local/etc/nginx/sites-enabled/yongbo-media.conf; fi
  if [ -f "$store/fullchain.previous.pem" ]; then cp -p "$store/fullchain.previous.pem" "$store/fullchain.pem"; fi
  if [ -f "$store/privkey.previous.pem" ]; then cp -p "$store/privkey.previous.pem" "$store/privkey.pem"; fi
  /usr/bin/nginx -t && /usr/syno/bin/synosystemctl reload nginx
  echo 'New configuration rejected; prior configuration restored' >&2
  exit 1
fi
