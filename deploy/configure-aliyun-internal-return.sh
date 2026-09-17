#!/usr/bin/env bash
set -euo pipefail

MODE="${1:---check}"
IPTABLES_BIN="${IPTABLES_BIN:-iptables}"
IPTABLES_SAVE_BIN="${IPTABLES_SAVE_BIN:-iptables-save}"
CHAIN="${ALIYUN_INTERNAL_RETURN_CHAIN:-yb-aliyun-return}"
SERVER_INTERFACE="${ALIYUN_INTERNAL_INTERFACE:-eth0}"
INSTALL_PATH="${ALIYUN_INTERNAL_INSTALL_PATH:-/usr/local/sbin/yongbo-aliyun-internal-return}"
SYSTEMD_DIR="${ALIYUN_INTERNAL_SYSTEMD_DIR:-/etc/systemd/system}"
BACKUP_DIR="${ALIYUN_INTERNAL_BACKUP_DIR:-/root/ecommerce_ai/backups/network}"

OSS_HANGZHOU_CIDRS=(
  "100.118.28.0/24"
  "100.114.102.0/24"
  "100.98.170.0/24"
  "100.118.31.0/24"
)

die() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

require_root() {
  [[ "$(id -u)" -eq 0 ]] || die "root privileges are required"
}

chain_exists() {
  "$IPTABLES_BIN" -nL "$CHAIN" >/dev/null 2>&1
}

remove_input_jumps() {
  while "$IPTABLES_BIN" -C INPUT -j "$CHAIN" >/dev/null 2>&1; do
    "$IPTABLES_BIN" -D INPUT -j "$CHAIN"
  done
}

apply_rules() {
  require_root
  command -v "$IPTABLES_BIN" >/dev/null 2>&1 || die "iptables is unavailable"
  if ! chain_exists; then
    "$IPTABLES_BIN" -N "$CHAIN"
  fi
  "$IPTABLES_BIN" -F "$CHAIN"
  for cidr in "${OSS_HANGZHOU_CIDRS[@]}"; do
    "$IPTABLES_BIN" -A "$CHAIN" -i "$SERVER_INTERFACE" -s "$cidr" \
      -p tcp -m multiport --sports 80,443 \
      -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
  done
  # Tailscale's CGNAT anti-spoof rule also overlaps the ECS metadata service.
  "$IPTABLES_BIN" -A "$CHAIN" -i "$SERVER_INTERFACE" -s 100.100.100.200/32 \
    -p tcp --sport 80 -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
  "$IPTABLES_BIN" -A "$CHAIN" -j RETURN

  remove_input_jumps
  # The exception must precede Tailscale's INPUT -> ts-input jump.
  "$IPTABLES_BIN" -I INPUT 1 -j "$CHAIN"
}

backup_rules() {
  require_root
  command -v "$IPTABLES_SAVE_BIN" >/dev/null 2>&1 || die "iptables-save is unavailable"
  install -d -m 0700 "$BACKUP_DIR"
  local stamp backup
  stamp="$(date -u +%Y%m%dT%H%M%SZ)"
  backup="$BACKUP_DIR/iptables-before-aliyun-internal-$stamp.rules"
  "$IPTABLES_SAVE_BIN" >"$backup"
  chmod 0600 "$backup"
  printf 'BACKUP_PATH=%s\n' "$backup"
}

write_systemd_files() {
  require_root
  local unit_tmp dropin_dir dropin_tmp
  install -m 0755 "$0" "$INSTALL_PATH"

  unit_tmp="$(mktemp)"
  printf '%s\n' \
    '[Unit]' \
    'Description=Allow Alibaba Cloud internal return traffic before Tailscale anti-spoof filtering' \
    'After=network-online.target tailscaled.service' \
    'Wants=network-online.target' \
    '' \
    '[Service]' \
    'Type=oneshot' \
    "ExecStart=$INSTALL_PATH --apply-rules" \
    'RemainAfterExit=yes' \
    '' \
    '[Install]' \
    'WantedBy=multi-user.target' >"$unit_tmp"
  install -m 0644 "$unit_tmp" "$SYSTEMD_DIR/yongbo-aliyun-internal-return.service"
  rm -f "$unit_tmp"

  dropin_dir="$SYSTEMD_DIR/tailscaled.service.d"
  install -d -m 0755 "$dropin_dir"
  dropin_tmp="$(mktemp)"
  printf '%s\n' \
    '[Service]' \
    "ExecStartPost=$INSTALL_PATH --apply-rules" >"$dropin_tmp"
  install -m 0644 "$dropin_tmp" "$dropin_dir/yongbo-aliyun-internal-return.conf"
  rm -f "$dropin_tmp"

  systemctl daemon-reload
  systemctl enable yongbo-aliyun-internal-return.service >/dev/null
  systemctl start yongbo-aliyun-internal-return.service
}

check_rules() {
  printf 'CHAIN=%s\n' "$CHAIN"
  printf 'INTERFACE=%s\n' "$SERVER_INTERFACE"
  if ! chain_exists; then
    printf 'STATUS=missing\n'
    return 1
  fi
  if ! "$IPTABLES_BIN" -C INPUT -j "$CHAIN" >/dev/null 2>&1; then
    printf 'STATUS=chain_not_linked\n'
    return 1
  fi
  local input_first
  input_first="$($IPTABLES_BIN -S INPUT | sed -n '/^-A INPUT /{p;q;}')"
  printf 'INPUT_FIRST=%s\n' "$input_first"
  "$IPTABLES_BIN" -nL "$CHAIN" -v -x --line-numbers
  if [[ "$input_first" != "-A INPUT -j $CHAIN" ]]; then
    printf 'STATUS=wrong_order\n'
    return 1
  fi
  printf 'STATUS=ok\n'
}

case "$MODE" in
  --check)
    check_rules
    ;;
  --apply-rules)
    apply_rules
    check_rules
    ;;
  --install)
    backup_rules
    write_systemd_files
    check_rules
    ;;
  *)
    die "usage: $0 [--check|--apply-rules|--install]"
    ;;
esac
