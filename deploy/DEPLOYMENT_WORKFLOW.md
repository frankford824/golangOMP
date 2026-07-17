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
- The preferred production automation path is
  `.github/workflows/production-self-hosted-release.yml`, executed by the
  repository self-hosted runner labeled `yongbo-production` on the ECS host.
- `deploy/deploy-on-host.sh` packages and deploys directly on that host without
  an SSH/SCP round trip. `deploy/deploy.sh` remains the emergency workstation
  path.
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

## GitHub Self-hosted Production Workflow

The repository-level runner on the production ECS host carries these labels:

- `self-hosted`
- `linux`
- `x64`
- `yongbo-production`

The persistent host toolchain is part of the runner contract: Go must match the
`go` directive in `go.mod`, Node.js major version must be 22, and the matching
Playwright Chromium revision is cached under root's standard Playwright cache.
The workflow validates these versions instead of downloading a fresh Go/Node
toolchain for every release run. Go modules use `https://goproxy.cn,direct`
with checksum verification through `sum.golang.google.cn`, because the
production ECS cannot reliably reach `proxy.golang.org`; checksum validation is
not disabled.

Run **Production self-hosted release** manually from GitHub Actions and select:

- `validate`: all backend/frontend gates only.
- `package`: gates plus a versioned Linux package, persisted outside the
  ephemeral runner workspace at
  `/root/ecommerce_ai/packages/<artifact>-<version>-linux-amd64.tar.gz` with a
  SHA-256 sidecar. Reusing a version with different bytes is rejected.
- `candidate`: gates, package, and parallel MAIN startup on port `18080` (or
  the supplied port). Live MAIN/Bridge and web roots are unchanged.
- `production`: requires `confirm_production=PRODUCTION`, a matching
  `/root/ecommerce_ai/shared/v8-cutover-approved.env`, then performs the normal
  backend cutover and optionally publishes both frontends.

The cutover marker must contain the exact reviewed commit:

```text
APPROVED_COMMIT=<40-character-git-sha>
```

This marker deliberately keeps the large v8 database/data migration outside
ordinary CI. It may be created only after the production-equivalent snapshot,
workflow-groups dry-run/apply/rollback rehearsal, and operator review are
complete.

Immediately before a production cutover, `deploy/backup-production-db.sh`
creates and verifies a consistent compressed MySQL dump under
`/root/ecommerce_ai/backups/production-release`. The three newest release
backups are retained by default.

Frontend publication on the host uses
`deploy/publish-front-on-host.sh`. It validates both static artifact manifests,
backs up both web roots, synchronizes the artifacts, validates/reloads Nginx,
probes both public sites, and restores the backups if publication fails.

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
  - refuses to start when the candidate port is already listening; a stale or foreign listener is never accepted as the new candidate
  - reports startup success only after the launched PID owns the listening socket and `GET /health` returns `200`
- Purpose:
  - safe warm-up and verification before any manual cutover

### Same-version Candidate Cleanup
- Before replacing an existing `releases/<version>` directory, deploy reads only the version-qualified parallel pidfile and verifies that `/proc/<pid>/exe` is the candidate binary inside that exact release.
- A verified candidate is stopped before the release directory is removed, and deploy waits for its candidate port to become free.
- A malformed pidfile, a foreign/reused PID, a port that remains occupied, or an attempt to replace the current live release aborts deployment without deleting the release directory.
- The stable live MAIN pidfile (`run/ecommerce-api.pid`) is not part of this cleanup path, so same-version candidate cleanup does not stop the live `8080` process.

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
- `external_asset_nas_watcher` (static Linux AMD64 sidecar for Synology `/p3` event ingestion)
- `.env.example`
- `bridge.env.example`
- `config/*.json`
- `db/migrations/`
- `docs/openapi.yaml`
- `docs/API_USAGE_GUIDE.md` (auto-generated per release since v0.4)
- `docs/API_INTEGRATION_GUIDE.md` (auto-generated per release since v0.4)
- `deploy/*.sh` (includes `check-remote-db.sh` for database integration readiness check and `run-org-master-convergence.sh` for the v1.0 org-master-data release flow)
- `deploy/docker-compose.external-asset-watcher.yml` (NAS-side watcher lifecycle; requires a dedicated event token and writable state directory)
- `PACKAGE_INFO.json`

## NAS External-Asset Watcher

The packaged `external_asset_nas_watcher` is deployed on the Synology host with
`deploy/docker-compose.external-asset-watcher.yml`. It watches
`/volume1/image_lib/仓库素材区/徐凯`, persists a local snapshot, debounces writes,
waits for file-size/mtime stability, and posts idempotent batches to
`POST /v1/integration/external-assets/events`.

The Synology tree currently exceeds its per-user inotify watch budget. The
compose file therefore runs two non-privileged shards under distinct host UIDs.
Both watch the root; stable FNV-1a ownership assigns each complete top-level
subtree to exactly one shard, including top-level directories created later.

NAS-side environment (keep the real token out of Git):

```dotenv
WATCH_BACKEND_URL=https://yongbo.cloud
WATCH_EVENT_TOKEN=<same value as backend EXTERNAL_ASSETS_EVENT_TOKEN>
WATCH_AGENT_ID=synology-p3-xukai
WATCH_RECONCILE_INTERVAL=6h
```

Backend runtime requirements:

- `EXTERNAL_ASSETS_EVENT_TOKEN` must be a dedicated random secret.
- `EXTERNAL_ASSETS_EVENT_ROOTS=/p3/仓库素材区/徐凯` restricts accepted events.
- `EXTERNAL_ASSETS_PREPARE_LEASE_TTL=2h` reclaims interrupted OSS uploads.
- Include `/p3` and `/p3/仓库素材区/徐凯` in `EXTERNAL_ASSETS_FULL_SYNC_MOUNTS`
  and `EXTERNAL_ASSETS_FULL_SYNC_ROOTS`; the existing full-sync interval is the
  authoritative periodic calibration if inotify delivery or watcher state is lost.
- Keep `EXTERNAL_ASSETS_FULL_SYNC_MAX_FILES_PER_MOUNT` above the measured root
  size (the deployment default is `100000`) so calibration completes instead of
  stopping in a safe-but-partial state.

On first start, `WATCH_BOOTSTRAP_EMIT=false` snapshots existing files without
flooding the event endpoint. Run one scoped full sync after deployment to establish
the backend baseline. Subsequent create, close-write, move, and delete events are
sent after the stability delay; a six-hour local snapshot reconciliation and the
backend full sync independently repair missed events.

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
- `deploy/start-main.sh` requires `curl` plus either `ss` or `lsof`; it fails closed when listener ownership cannot be proven or `/health` cannot be confirmed as `200` within `START_MAIN_TIMEOUT_SECONDS` (default `30`).

## Deploy Process Guard Tests
- Run `bash deploy/tests/deploy-process-guards.test.sh` locally.
- The test covers occupied-port rejection, exact listener ownership, unhealthy startup cleanup, same-version parallel candidate cleanup, and foreign pidfile refusal. It uses temporary directories and local ephemeral ports only; it does not connect to or modify production.

## Remote Database Integration Readiness Check (since v0.4)
- After deploy, run on the server: `bash /root/ecommerce_ai/current/deploy/check-remote-db.sh`
- Or with custom base dir: `bash deploy/check-remote-db.sh --base-dir /root/ecommerce_ai`
- Requires: `main.env` (or `shared/main.env`) with DB_HOST, DB_USER, DB_NAME; mysql client on server
- From local IDE: `ssh user@host "cd /root/ecommerce_ai/current && bash deploy/check-remote-db.sh"`

## Performance Migration Release Checklist
- Apply pending database migrations before starting the new `cmd/server` binary. Migrations 114-119 add columns and read-model tables used by hot read paths; starting the binary first can produce `unknown column` errors on product management, asset center, search, and report endpoints.
- After migrations 117 and 119, run `cmd/tools/search-reindex` to rebuild `asset_search_documents` and `product_search_documents`; run `cmd/tools/search-semantic-enrich` only when AI enrichment is intentionally enabled.
- After migration 118, keep `ENABLE_CRON_REPORT_L1_DAILY=true` in `shared/main.env`. The default `CRON_SCHEDULE_REPORT_L1_DAILY="*/10 * * * *"` and `REPORT_L1_DAILY_REFRESH_DAYS=3` keep recent L1 throughput aggregates fresh. If `report_task_daily.updated_at` is more than 2 hours stale, the runtime falls back to realtime throughput SQL instead of serving old aggregates.
- Capture pre/post `EXPLAIN` and `performance_schema.events_statements_summary_by_digest` snapshots for the changed hot queries before claiming the performance rollout complete.

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
- DB migrations are packaged and cutover deploy runs `deploy/run-pending-migrations.sh` before restarting services. The first run creates a `schema_migrations` baseline without replaying historical files; later releases apply only newly added migration files and strip any `ROLLBACK` block before execution.
