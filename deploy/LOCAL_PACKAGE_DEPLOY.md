# Local Package Deploy

## Purpose
- Keep a local packaging shortcut next to the standardized Linux deploy flow.
- Use the same tar.gz package layout as `deploy/deploy.sh` without touching the managed release history file.
- MAIN packaging entrypoint is locked to `./cmd/server`; `cmd/api` is not a packaging fallback.
- `cmd/api` remains on disk only as a deprecated compatibility entrypoint and is not a valid production packaging target.

## Command
- `bash ./deploy/package-local.sh --version v0.1`

## Local VS Code Deploy Settings
- `deploy/package-local.sh` auto-loads `.vscode/deploy.local.env` when that file exists
- This keeps the local packaging entrypoint aligned with the main deploy workflow for future `DEPLOY_*` usage

## Output
- Package directory:
  - `dist/ecommerce-ai-v0.1-linux-amd64/`
- Tarball artifact:
  - `dist/ecommerce-ai-v0.1-linux-amd64.tar.gz`

## Package Contents
- `ecommerce-api`
- `erp_bridge`
- `.env.example`
- `bridge.env.example`
- `config/*.json`
- `db/migrations/`
- `docs/openapi.yaml`
- `deploy/remote-deploy.sh`
- `deploy/start-main.sh`
- `deploy/stop-main.sh`
- `deploy/start-bridge.sh`
- `deploy/stop-bridge.sh`
- `deploy/verify-runtime.sh`
- `PACKAGE_INFO.json`

## Runtime Default
- Same-host Bridge loopback remains the default runtime dependency:
  - `ERP_BRIDGE_BASE_URL=http://127.0.0.1:8081`
- `live MAIN` means the public business application service on `8080`
- `candidate MAIN` means the side-by-side validation instance of that same MAIN service on `18080`
- `Bridge` means the ERP/JST adapter runtime on `8081`; side-by-side validation keeps candidate MAIN pointed at the live Bridge dependency rather than introducing a second Bridge role

## Deployment Modes
- Real cutover deployment:
  - `bash ./deploy/deploy.sh`
- Safe side-by-side validation deployment:
  - `bash ./deploy/deploy.sh --parallel`
- Side-by-side validation leaves live MAIN and Bridge untouched, starts only a candidate MAIN on a separate port, and does not switch live symlinks.
- Side-by-side candidate env generation now prefers the live MAIN env shape, preserves DB-style fields and `TZ` when present, forces Bridge loopback to `http://127.0.0.1:8081`, and only overrides the candidate port.
- When no live MAIN env exists yet, the generated parallel env is a minimal DB-style skeleton rather than the packaged `MYSQL_DSN` template.

## Standard Workflow
- The managed release entrypoint is:
  - `bash ./deploy/deploy.sh`
- Full instructions live in:
  - `deploy/DEPLOYMENT_WORKFLOW.md`
