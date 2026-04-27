# NAS SSH Access

Last updated: 2026-04-11

## Scope

This document is for ops/developer infrastructure access only.

- It is not part of the business upload/download/storage architecture.
- MAIN business asset runtime is OSS-only.
- Do not use this document to design frontend or business file flows.

## Current Aliases

- Server alias: `jst_ecs`
- NAS alias: `synology-dsm`

## Verified NAS SSH Entry

```bash
ssh synology-dsm
```

Preferred tmux entry:

```bash
ssh synology-dsm "source ~/.bashrc >/dev/null 2>&1; tmux new -As nas-upload"
```

## SSH Config Notes

- Windows local SSH config lives at `C:/Users/wsfwk/.ssh/config`
- Keep existing `IdentityFile` and `IdentitiesOnly yes`
- Keepalive settings remain recommended:
  - `ServerAliveInterval 30`
  - `ServerAliveCountMax 6`
  - `TCPKeepAlive yes`
- On the current Windows OpenSSH build, do not rely on `ControlMaster` multiplexing

## Supported Ops Uses

- SSH into NAS for inspection
- SCP or RSYNC code/scripts/logs for maintenance work
- Operate tmux/docker on the NAS host
- Troubleshoot future infra tasks unrelated to MAIN business file runtime

## Explicit Boundary

Allowed here:

- SSH
- SCP
- RSYNC
- tmux
- docker/service maintenance

Not allowed as business-runtime assumptions:

- NAS upload URLs
- NAS browser probes
- NAS multipart business contracts
- NAS download fallback
- NAS path-based asset resolution in MAIN

## Related Docs

- `docs/THREE_ENDPOINT_CONTROL_PLANE.md`
- `scripts/test_env_destructive_reset_keep_admin.sh`
