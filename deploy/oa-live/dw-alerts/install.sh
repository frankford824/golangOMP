#!/usr/bin/env bash
# Run on jst_ecs as root. On first install pipe the sender's raw key to --key-stdin.
set -euo pipefail
test "$(id -u)" = 0
src=$(cd "$(dirname "$0")" && pwd)
cfg=/etc/yongbo-oa-live/dw-alerts.env
getent group yongbo-oa-live >/dev/null
if [[ ${1:-} = --key-stdin ]]; then
  address=$(tailscale ip -4)
  umask 077
  install -d -m700 /etc/yongbo-oa-live
  python3 -c '
import base64, ipaddress, os, sys, tempfile
from pathlib import Path
address = ipaddress.IPv4Address(sys.argv[1])
if address not in ipaddress.IPv4Network("100.64.0.0/10"):
    sys.exit("expected a tailnet address")
# Sender load_secret reads bytes then strips ASCII whitespace.
raw = sys.stdin.buffer.read(4097)
key = raw.strip()
if len(raw) > 4096 or len(key) < 16:
    sys.exit("missing or invalid sender key")
path = Path(sys.argv[2])
fd, temp = tempfile.mkstemp(dir=path.parent)
try:
    with os.fdopen(fd, "w") as f:
        f.write("DW_ALERT_ADDRESS=" + str(address) + "\nDW_ALERT_SECRET_B64=" + base64.b64encode(key).decode() + "\n")
    os.replace(temp, path)
finally:
    Path(temp).unlink(missing_ok=True)
' "$address" "$cfg"
elif [[ $# != 0 ]]; then
  exit 2
fi
test -s "$cfg"
getent passwd oa-dw-alerts >/dev/null || useradd --system --no-create-home --gid yongbo-oa-live --shell /usr/sbin/nologin oa-dw-alerts
install -d -m755 /opt/yongbo-dw-alerts
install -m644 "$src/receiver.py" /opt/yongbo-dw-alerts/receiver.py
install -m644 "$src/oa-dw-alerts.service" /etc/systemd/system/oa-dw-alerts.service
systemctl daemon-reload
systemctl enable oa-dw-alerts.service
systemctl restart oa-dw-alerts.service
for ((i=0;i<20;i++)); do
  if curl -fsS --max-time 1 --unix-socket /run/oa-dw-alerts/feed.sock http://localhost/summary >/dev/null 2>&1; then
    systemctl is-active --quiet oa-dw-alerts
    echo "Order-data webhook ready."
    exit 0
  fi
  sleep 0.25
done
exit 1
