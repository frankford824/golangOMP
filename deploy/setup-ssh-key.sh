#!/usr/bin/env bash
# One-time setup: add deploy SSH public key to server authorized_keys.
# After this succeeds once, deploy.sh works without DEPLOY_PASSWORD / sshpass.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=deploy/lib.sh
. "$SCRIPT_DIR/lib.sh"

load_local_deploy_env "$ROOT"

: "${DEPLOY_HOST:?DEPLOY_HOST required}"
: "${DEPLOY_USER:?DEPLOY_USER required}"
DEPLOY_PORT="${DEPLOY_PORT:-22}"
SSH_TARGET="${DEPLOY_USER}@${DEPLOY_HOST}"
KEY_PATH="$HOME/.ssh/id_deploy_ecommerce"
PUB_PATH="$HOME/.ssh/id_deploy_ecommerce.pub"
CONFIG="$HOME/.ssh/config"
CONFIG_MARKER="# deploy-ecommerce-ai"

mkdir -p "$HOME/.ssh"
chmod 700 "$HOME/.ssh" 2>/dev/null || true

if [ ! -f "$PUB_PATH" ]; then
  log "Generating deploy key: $KEY_PATH"
  ssh-keygen -t ed25519 -f "$KEY_PATH" -N "" -C "deploy-ecommerce-ai"
fi

if [ ! -f "$CONFIG" ] || ! grep -q "$CONFIG_MARKER" "$CONFIG" 2>/dev/null; then
  {
    printf '\n%s\nHost %s\n  IdentityFile %s\n  IdentitiesOnly yes\n  User %s\n  Port %s\n' \
      "$CONFIG_MARKER" "$DEPLOY_HOST" "$KEY_PATH" "$DEPLOY_USER" "$DEPLOY_PORT"
  } >>"$CONFIG"
  chmod 600 "$CONFIG" 2>/dev/null || true
  log "Updated ~/.ssh/config for $DEPLOY_HOST"
fi

key_login_works() {
  ssh -o BatchMode=yes -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new -p "$DEPLOY_PORT" "$SSH_TARGET" "exit 0" >/dev/null 2>&1
}

shell_single_quote_escape() {
  printf '%s' "$1" | sed "s/'/'\\\\''/g"
}

password_ssh() {
  local remote_command="$1"

  if command -v sshpass >/dev/null 2>&1; then
    sshpass -p "$DEPLOY_PASSWORD" ssh \
      -o StrictHostKeyChecking=accept-new \
      -o PreferredAuthentications=password \
      -o PubkeyAuthentication=no \
      -p "$DEPLOY_PORT" \
      "$SSH_TARGET" \
      "$remote_command"
    return
  fi

  if command -v powershell.exe >/dev/null 2>&1 && command -v wslpath >/dev/null 2>&1; then
    local ps_script
    ps_script="$(mktemp --suffix=.ps1)"
    cat >"$ps_script" <<'POWERSHELL'
param(
  [string]$DeployPassword,
  [string]$DeployPort,
  [string]$SshTarget,
  [string]$RemoteCommand
)
$escapedPassword = $DeployPassword -replace '([()%!^"<>&|])', '^$1'
$ask = Join-Path $env:TEMP 'askpass-deploy.cmd'
Set-Content -Path $ask -Value "@echo off`r`necho $escapedPassword`r`n"
$env:SSH_ASKPASS = $ask
$env:SSH_ASKPASS_REQUIRE = 'force'
$env:DISPLAY = 'dummy'
& ssh.exe `
  -o StrictHostKeyChecking=accept-new `
  -o PreferredAuthentications=password `
  -o PubkeyAuthentication=no `
  -p $DeployPort `
  $SshTarget `
  $RemoteCommand
$code = $LASTEXITCODE
Remove-Item $ask -Force
exit $code
POWERSHELL
    powershell.exe -NoProfile -NonInteractive -File "$(wslpath -w "$ps_script")" "$DEPLOY_PASSWORD" "$DEPLOY_PORT" "$SSH_TARGET" "$remote_command"
    local status=$?
    rm -f "$ps_script"
    return "$status"
  fi

  command -v setsid >/dev/null 2>&1 || fail "Password setup requires sshpass, Windows OpenSSH askpass, or setsid + SSH_ASKPASS support."
  local askpass_script
  askpass_script="$(mktemp)"
  printf '#!/bin/sh\nprintf "%%s\\n" "$DEPLOY_PASSWORD"\n' >"$askpass_script"
  chmod 700 "$askpass_script"

  DISPLAY="${DISPLAY:-:0}" SSH_ASKPASS="$askpass_script" SSH_ASKPASS_REQUIRE=force DEPLOY_PASSWORD="$DEPLOY_PASSWORD" \
    setsid ssh \
      -o StrictHostKeyChecking=accept-new \
      -o PreferredAuthentications=password \
      -o PubkeyAuthentication=no \
      -p "$DEPLOY_PORT" \
      "$SSH_TARGET" \
      "$remote_command" </dev/null
  local status=$?
  rm -f "$askpass_script"
  return "$status"
}

install_public_key() {
  local public_key
  local quoted_public_key
  local remote_command
  public_key="$(cat "$PUB_PATH")"
  quoted_public_key="$(shell_single_quote_escape "$public_key")"
  remote_command="umask 077; mkdir -p ~/.ssh; touch ~/.ssh/authorized_keys; chmod 700 ~/.ssh; chmod 600 ~/.ssh/authorized_keys; grep -qxF '$quoted_public_key' ~/.ssh/authorized_keys || printf '%s\n' '$quoted_public_key' >> ~/.ssh/authorized_keys"

  password_ssh "$remote_command"
}

log "Deploy key: $KEY_PATH"
log "Target: $SSH_TARGET"

if key_login_works; then
  log "SSH key already authorized on remote host."
else
  [ -n "${DEPLOY_PASSWORD:-}" ] || fail "DEPLOY_PASSWORD is required for first-time key setup when passwordless SSH is not yet available."
  log "Installing deploy public key into remote authorized_keys..."
  install_public_key
fi

log "Verifying passwordless SSH..."
ssh -o BatchMode=yes -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new -p "$DEPLOY_PORT" "$SSH_TARGET" "echo SSH_OK"
log "SSH key setup complete. Deploy now defaults to SSH key authentication."
