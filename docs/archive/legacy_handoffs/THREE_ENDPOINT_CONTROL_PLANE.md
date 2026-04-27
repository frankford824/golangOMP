# Three-Endpoint Control Plane

## Scope

This document is ops/developer infrastructure guidance only.

- It is not part of the MAIN business upload/download/storage runtime contract.
- MAIN business asset runtime is OSS-only.
- NAS references here are operational access facts, not business file-flow design.

## Authoritative Baseline

- Local MAIN repo is the only control plane and the only long-term coordination entrypoint.
- Server alias: `jst_ecs`
- NAS alias: `synology-dsm`
- Daily operations must start from the local MAIN workspace and use these aliases instead of raw IPs.
- Current conclusion: the three-endpoint tunnel and collaboration plan is formally established, not pending.

## Host Facts

### Server (`jst_ecs`)

- Passwordless `root` SSH login is verified.
- `authorized_keys` is correct and reusable.
- `tmux`, `rsync`, `scp`, and `ssh` are available.
- Live release directory: `/root/ecommerce_ai/releases/v0.8`
- Live deploy script directory: `/root/ecommerce_ai/releases/v0.8/deploy`
- Live log directory: `/root/ecommerce_ai/logs`
- Standard tmux session: `main-live`

### NAS (`synology-dsm`)

- Upload/download service is verified.
- `tmux 3.6a` is installed at `~/bin/tmux`.
- The stable way to enter tmux is:

```bash
ssh synology-dsm "source ~/.bashrc >/dev/null 2>&1; tmux new -As nas-upload"
```

- Standard tmux session: `nas-upload`

## SSH Policy

### Windows local control node

- Formal SSH config file: `C:/Users/wsfwk/.ssh/config`
- Keep:
  - `IdentityFile`
  - `IdentitiesOnly yes`
  - `ServerAliveInterval 30`
  - `ServerAliveCountMax 6`
  - `TCPKeepAlive yes`
- Do not enable:
  - `ControlMaster`
  - `ControlPersist`
  - `ControlPath`
- Reason:
  - current Windows OpenSSH build is `OpenSSH_for_Windows_9.5p2`
  - SSH multiplexing was verified to fail with:
    - `getsockname failed: Not a socket`
    - `Unknown error`
- Baseline rule:
  - Windows local MAIN control node must disable SSH connection reuse.
  - Keepalive stays enabled.

### Linux or macOS control node

- The same aliases may be reused.
- SSH multiplexing is optional there:
  - `ControlMaster auto`
  - `ControlPersist 10m`
  - `ControlPath ~/.ssh/cm-%r@%h:%p`
- This is an optimization, not a required baseline.

## Active Aliases

- `jst_ecs` -> `root@223.4.249.11` via `C:/Users/wsfwk/.ssh/id_deploy_ecommerce`
- `synology-dsm` -> `yongbo@100.111.214.38` via `C:/Users/wsfwk/.ssh/id_rsa`
- Raw-IP daily entry is deprecated.

## Local MAIN Responsibilities

- Unified build, packaging, deploy, and runtime verification entrypoint for the server side.
- Unified SSH/rsync/scp control point for server and NAS.
- Unified verification source for:
  - live process executable path
  - live SHA256
  - live health
  - NAS upload/download health
- Unified collaboration entry for future server and NAS maintenance.

## Current Deploy Baseline

- Live release line remains `v0.8`.
- Current production practice is overwrite publish onto the existing `v0.8` directory.
- Do not document or imply `v0.9` as the current live line.
- MAIN live binary target:
  - `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
- Preferred verification methods:
  - `/proc/<pid>/exe`
  - `sha256sum`
  - `curl /health`
- Preferred deploy path:
  - use the existing MAIN repo deploy scripts first
  - use direct overwrite replacement only when explicitly performing the current `v0.8` hotfix workflow

## Command Templates

### Connect

```bash
ssh jst_ecs
ssh synology-dsm
```

### Server tmux

```bash
ssh jst_ecs "tmux new -As main-live"
```

### NAS tmux

```bash
ssh synology-dsm "source ~/.bashrc >/dev/null 2>&1; tmux new -As nas-upload"
```

### MAIN-managed deploy

```bash
bash ./deploy/deploy.sh --version v0.8 --release-note "overwrite v0.8 from local MAIN control plane"
```

### MAIN overwrite deploy to current `v0.8`

```bash
GOOS=linux GOARCH=amd64 go build -o dist/ecommerce-api-v0.8 ./cmd/server
scp dist/ecommerce-api-v0.8 jst_ecs:/root/ecommerce_ai/releases/v0.8/ecommerce-api.new
ssh jst_ecs '
  set -e
  TS=$(date +%Y%m%dT%H%M%S)
  cp /root/ecommerce_ai/releases/v0.8/ecommerce-api /root/ecommerce_ai/backups/ecommerce-api-v0.8-$TS
  mv /root/ecommerce_ai/releases/v0.8/ecommerce-api.new /root/ecommerce_ai/releases/v0.8/ecommerce-api
  chmod +x /root/ecommerce_ai/releases/v0.8/ecommerce-api
  bash /root/ecommerce_ai/releases/v0.8/deploy/stop-main.sh --base-dir /root/ecommerce_ai
  bash /root/ecommerce_ai/releases/v0.8/deploy/start-main.sh \
    --base-dir /root/ecommerce_ai \
    --env-file /root/ecommerce_ai/shared/main.env \
    --binary-path /root/ecommerce_ai/releases/v0.8/ecommerce-api
  MAIN_PID=$(cat /root/ecommerce_ai/run/ecommerce-api.pid)
  echo MAIN_PID=$MAIN_PID
  echo MAIN_EXE=$(readlink /proc/$MAIN_PID/exe)
  sha256sum /root/ecommerce_ai/releases/v0.8/ecommerce-api
  sha256sum $(readlink /proc/$MAIN_PID/exe)
'
```

### Server runtime verification

```bash
ssh jst_ecs "bash /root/ecommerce_ai/releases/v0.8/deploy/check-three-services.sh --base-dir /root/ecommerce_ai --auto-recover-8082"
```

### NAS code sync

```bash
rsync -avz --delete \
  --exclude .git \
  --exclude tmp \
  --exclude data \
  /local/path/to/asset-upload-service/ \
  synology-dsm:/volume1/homes/yongbo/asset-upload-service/
```

### NAS rebuild

```bash
ssh synology-dsm '
  source ~/.bashrc >/dev/null 2>&1
  cd /volume1/homes/yongbo/asset-upload-service &&
  /usr/local/bin/docker compose up -d --build --force-recreate &&
  /usr/local/bin/docker compose ps &&
  curl -fsS http://127.0.0.1:8089/health
'
```

## Collaboration Conclusion

- Local MAIN repo = the only control plane.
- Server coordination uses `jst_ecs`.
- NAS coordination uses `synology-dsm`.
- Future three-endpoint work must be coordinated from the local MAIN workspace.
- This control-plane plan is the formal baseline.
