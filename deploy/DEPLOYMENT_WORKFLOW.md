# Unified Deployment Workflow

## Scope
- Standard release entrypoint: `deploy/deploy.sh`
- Local package helper: `deploy/package-local.sh`
- MAIN packaging entrypoint is locked to `./cmd/server`; `cmd/api` is not a production packaging fallback
- `cmd/api` remains in the repo only as a deprecated compatibility entrypoint and must not be used for production build or deploy flows
- `live MAIN` means the public business application service bound to the cutover port `8080`
- `candidate MAIN` means a side-by-side validation instance of that same MAIN service, usually on `18080`
- `Bridge` means the ERP/JST adapter runtime on `8081`; side-by-side validation keeps candidate MAIN pointed at the live Bridge dependency instead of creating a second Bridge role
- compatibility-only legacy `8080` surfaces remain rollback-safe continuity only; they are not a second target deployment model
- Fixed release history source of truth: `deploy/release-history.log`
- Managed release versions start at `v0.1`
- MAIN runtime keeps same-host Bridge access at `http://127.0.0.1:8081`
- Remote Linux base path defaults to `/root/ecommerce_ai`

## Release History Rule
- `deploy/release-history.log` is append-only and script-friendly.
- `baseline_version=v0.1` is the first managed release.
- Local-only managed packaging may auto-increment by minor version:
  - `v0.1`
  - `v0.2`
  - `v0.3`
- Remote deploys must pass `--version`; live deploy is no longer allowed to silently choose the next managed version.
- Each lifecycle step appends one `release|...` record, so the file remains the source of truth for both versioning and deploy status.
- Historical packaged records that show `entrypoint=./cmd/api` on 2026-03-12 are legacy pre-convergence history, not a current production entrypoint option.

## SSH Key Passwordless Deployment (Recommended)
- **Default**: Deploy uses SSH key authentication. No password or `sshpass` required.
- **One-time setup**:
  - Windows IDE / PowerShell: `powershell -ExecutionPolicy Bypass -File .\deploy\setup-ssh-key.ps1`
  - Bash shell: `bash deploy/setup-ssh-key.sh`
- Either setup helper will:
  1. Generate (or reuse) `~/.ssh/id_deploy_ecommerce` key pair
  2. Add `~/.ssh/config` for the deploy host with `IdentityFile`
  3. Append the public key to the remote `authorized_keys` file with correct permissions
  4. Verify `ssh -o BatchMode=yes` succeeds before the first real deploy
- The setup helpers accept the host key with `StrictHostKeyChecking=accept-new`; they do not disable host verification.
- After setup succeeds, `deploy.sh` runs without `DEPLOY_PASSWORD` or `sshpass`.
- **Fallback**: Password auth is compatibility-only. Use `DEPLOY_AUTH_MODE=password` plus `DEPLOY_PASSWORD` if you must keep the old path temporarily.

## Required Local Environment Variables
- `DEPLOY_HOST`
- `DEPLOY_USER`
- `DEPLOY_PORT`
- `DEPLOY_BASE_DIR`

## Optional Local Environment Variables
- `DEPLOY_AUTH_MODE` - optional auth mode. Default `key`; set `password` only for compatibility fallback.
- `DEPLOY_PASSWORD` - only needed when using password-based auth; not required for SSH key deploy
- `DEPLOY_APP_NAME`
- `DEPLOY_KEEP_RELEASES`
- `DEPLOY_MAIN_PORT`
- `DEPLOY_PARALLEL_PORT`
- `DEPLOY_BRIDGE_BASE_URL`
- `DEPLOY_RUNTIME_ENV_FILE`
- `DEPLOY_BRIDGE_ENV_FILE`

## Recommended VS Code Local Workflow
- Create or edit `.vscode/deploy.local.env`
- The file is local-only and should stay out of git
- `deploy/deploy.sh` and `deploy/package-local.sh` automatically load it when present
- Existing shell-exported `DEPLOY_*` values still win if you set them explicitly

Starter file:
- `.vscode/deploy.local.env`

Copy `deploy/deploy.env.example` to another local-only shell snippet if needed, or export the variables directly in your terminal. Do not commit real secrets.

## MAIN Runtime Upload-Service Env
- The MAIN runtime env file should now carry OSS upload-service settings when the design asset center is enabled:
  - `UPLOAD_SERVICE_ENABLED`
  - `UPLOAD_SERVICE_BASE_URL`
  - `UPLOAD_SERVICE_BROWSER_MULTIPART_BASE_URL=/`
  - `UPLOAD_SERVICE_TIMEOUT`
  - `UPLOAD_SERVICE_INTERNAL_TOKEN`
  - `UPLOAD_STORAGE_PROVIDER=oss`
- `UPLOAD_SERVICE_AUTH_TOKEN` remains a backward-compatible legacy alias only.
- Recommended deployment value for `UPLOAD_SERVICE_BASE_URL` is the upload service's backend-only internal address.
- Browser multipart traffic should be exposed through same-origin `/upload/*` reverse proxying so both deployed dist and local Vite dev-proxy mode can use the same returned paths.

## Deployment Modes

### Normal Cutover Mode
- Command:
  - `bash ./deploy/deploy.sh --version v0.9`
- Behavior:
  - uploads the package
  - deploys into `/root/ecommerce_ai/releases/<version>`
  - refreshes shared scripts under `/root/ecommerce_ai/scripts`
  - refreshes stable symlinks:
    - `/root/ecommerce_ai/current`
    - `/root/ecommerce_ai/ecommerce-api`
    - `/root/ecommerce_ai/erp_bridge`
  - reuses stable env files under:
    - `/root/ecommerce_ai/shared/main.env`
    - `/root/ecommerce_ai/shared/bridge.env`
  - stops the current MAIN and Bridge, then starts the new release on the live ports

### Side-by-Side Validation Mode
- Command:
  - `bash ./deploy/deploy.sh --version v0.9 --parallel`
- Optional port override:
  - `bash ./deploy/deploy.sh --version v0.9 --parallel --parallel-port 19080`
- Behavior:
  - uploads the package
  - deploys into a new isolated `/root/ecommerce_ai/releases/<version>` directory
  - leaves live MAIN and live Bridge untouched
  - does not stop the live MAIN service
  - does not stop the live Bridge service
  - does not rewrite:
    - `/root/ecommerce_ai/current`
    - `/root/ecommerce_ai/ecommerce-api`
    - `/root/ecommerce_ai/erp_bridge`
  - does not overwrite live shared env files in place
  - creates a candidate env file at:
    - `/root/ecommerce_ai/releases/<version>/runtime/main.parallel.env`
  - derives the candidate env from the live MAIN env file when available
  - preserves live-style DB fields such as:
    - `DB_HOST`
    - `DB_PORT`
    - `DB_USER`
    - `DB_PASS`
    - `DB_NAME`
  - preserves `TZ` when it already exists in the source env
  - keeps only candidate-specific overrides in the generated env:
    - `PORT=<parallel-port>` when the live env uses `PORT`
    - `SERVER_PORT=<parallel-port>` only when the source env already uses `SERVER_PORT`
    - `ERP_BRIDGE_BASE_URL=http://127.0.0.1:8081`
  - removes stale template-only `MYSQL_DSN` from the candidate env when DB-style fields are present
  - starts only the candidate MAIN instance from the version directory
  - uses an isolated candidate port:
    - default `18080`
    - override with `--parallel-port` or `DEPLOY_PARALLEL_PORT`
  - keeps Bridge dependency pinned to:
    - `http://127.0.0.1:8081`
  - writes isolated candidate runtime files:
    - pid: `/root/ecommerce_ai/run/ecommerce-api-<version>-parallel.pid`
    - log: `/root/ecommerce_ai/logs/ecommerce-api-<version>-parallel.log`
    - state: `/root/ecommerce_ai/releases/<version>/runtime/deploy-state.parallel.env`
- Purpose:
  - safe warm-up and verification before any manual cutover

## What `deploy.sh` Does
1. Validates the explicit `--version` for remote deploys and records lifecycle steps in `deploy/release-history.log`
2. Runs `go test ./...` unless `--skip-tests` is used
3. Builds static Linux AMD64 binaries (`GOOS=linux GOARCH=amd64`) named `ecommerce-api` and `erp_bridge`
4. Creates a versioned package directory and `.tar.gz` artifact under `dist/`
5. Uploads the tarball over `scp`
6. Extracts on the remote host under `/root/ecommerce_ai/incoming`
7. Runs `deploy/remote-deploy.sh` in either cutover mode or side-by-side mode
8. Appends release status records back into `deploy/release-history.log`

## First Deploy Behavior
- In normal cutover mode:
  - if `shared/main.env` or `shared/bridge.env` is missing, deploy seeds it from the packaged example file
  - deploy stops at `deployed_waiting_for_env` and does not auto-start either process
- In side-by-side validation mode:
  - if the requested candidate source env file exists, deploy copies that live env shape into `releases/<version>/runtime/main.parallel.env` and overrides only the candidate port and loopback Bridge URL
  - if the requested candidate source env file does not exist, deploy seeds `releases/<version>/runtime/main.parallel.env` with a minimal live-style skeleton:
    - `PORT`
    - `DB_HOST`
    - `DB_PORT`
    - `DB_USER`
    - `DB_PASS`
    - `DB_NAME`
    - `ERP_BRIDGE_BASE_URL`
  - the minimal parallel skeleton intentionally does not inject template-only values such as `MYSQL_DSN`, Redis, or ERP sync placeholders
  - deploy stops at `deployed_parallel_waiting_for_env` and does not auto-start the candidate process

## Repeated Deploys
- Future cutover deploys reuse the same `bash ./deploy/deploy.sh --version <target>` command.
- Future safe validation deploys reuse:
  - `bash ./deploy/deploy.sh --version <target> --parallel`
- The Linux release layout remains:
  - `/root/ecommerce_ai/incoming`
  - `/root/ecommerce_ai/releases/<version>`
  - `/root/ecommerce_ai/shared`
  - `/root/ecommerce_ai/logs`
  - `/root/ecommerce_ai/run`
  - `/root/ecommerce_ai/scripts`

## Package Contents
- `ecommerce-api`
- `erp_bridge`
- `.env.example`
- `bridge.env.example`
- `config/*.json`
- `db/migrations/`
- `docs/openapi.yaml`
- `docs/API_USAGE_GUIDE.md` (auto-generated per release since v0.4)
- `docs/API_INTEGRATION_GUIDE.md` (auto-generated per release since v0.4)
- `deploy/*.sh` (includes `check-remote-db.sh` for database integration readiness check and `run-org-master-convergence.sh` for the v1.0 org-master-data release flow)
- `PACKAGE_INFO.json`

## Local Packaging Only
- `bash ./deploy/package-local.sh --version v0.1`

## Local-Only Managed Release Verification
- `bash ./deploy/deploy.sh --local-only --release-history-path dist/release-history-test.log --output-root dist/deploy-check --release-note "workflow verification"`

## Runtime Verification Helper
- Live default example:
  - `bash /root/ecommerce_ai/scripts/verify-runtime.sh --base-url http://127.0.0.1:8080 --bridge-url http://127.0.0.1:8081`
- Side-by-side candidate example:
  - `bash /root/ecommerce_ai/releases/<version>/deploy/verify-runtime.sh --base-url http://127.0.0.1:18080`
- If `curl` is installed, the helper also reports HTTP status codes for the auth/task checks.

## Remote Database Integration Readiness Check (since v0.4)
- After deploy, run on the server: `bash /root/ecommerce_ai/current/deploy/check-remote-db.sh`
- Or with custom base dir: `bash deploy/check-remote-db.sh --base-dir /root/ecommerce_ai`
- Requires: `main.env` (or `shared/main.env`) with DB_HOST, DB_USER, DB_NAME; mysql client on server
- From local IDE: `ssh user@host "cd /root/ecommerce_ai/current && bash deploy/check-remote-db.sh"`

## v1.0 Org-Master-Data Release Flow
- The org-master-data convergence is not part of the generic deploy auto-start path; run it explicitly before claiming the v1.0 org baseline is live.
- Server-side helper:
  - `bash /root/ecommerce_ai/current/deploy/run-org-master-convergence.sh --base-dir /root/ecommerce_ai`
- What it does:
  - backs up `users`, `user_roles`, `org_departments`, and `org_teams`
  - applies `058_v1_0_org_team_department_scoped_uniqueness.sql`
  - seeds the official v1.0 departments/teams from packaged `config/auth_identity.json`
  - applies `057_v1_0_org_master_convergence.sql`
  - prints postcheck SQL output for legacy-row and official-baseline verification

## Notes
- **Deploy authentication**: SSH key passwordless deploy is the default and recommended. Run the matching setup helper once (`deploy/setup-ssh-key.ps1` for Windows IDE / PowerShell, `deploy/setup-ssh-key.sh` for bash); after that `deploy.sh` requires neither `DEPLOY_PASSWORD` nor `sshpass`.
- `deploy.sh` now defaults to `DEPLOY_AUTH_MODE=key` and uses batch-mode SSH/SCP so it never falls back to manual password prompts.
- Packaging normalizes deployed `deploy/*.sh` helpers to LF line endings to avoid CRLF parsing failures on Linux hosts.
- Password-based deploy is compatibility-only and opt-in: set `DEPLOY_AUTH_MODE=password` plus `DEPLOY_PASSWORD`.
- `DEPLOY_PASSWORD` may be kept in `.vscode/deploy.local.env` for one-time setup or fallback; it is never committed.
- The runtime launch helper accepts either `PORT` or `SERVER_PORT`. When DB-style fields are present and `MYSQL_DSN` is absent, it derives `MYSQL_DSN` in-memory at process start instead of writing a template DSN into the candidate env file.
- This workflow now distinguishes safe validation from real cutover; side-by-side mode does not perform final cutover.
- Historical migrations `001` through `004` previously used `TEXT ... DEFAULT ''` clauses. MySQL 8 strict mode rejects defaults on `TEXT`/`BLOB`, so fresh bootstrap had to be corrected in-repo by removing those defaults from the source migrations instead of relying on server-side manual edits.
- The repository migration pack now also includes `028_v7_runtime_distribution_event_tables.sql` so the legacy runtime tables still required by the binary are repository-owned: `event_logs`, `sku_sequences`, `distribution_jobs`, and `job_attempts`.
- DB migrations are packaged, but there is still no automatic migration runner in this repo.
