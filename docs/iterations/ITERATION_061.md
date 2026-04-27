# ITERATION_061

## Phase
- PHASE_AUTO_061 / unified deployment and packaging workflow standardization

## Input Context
- Current CURRENT_STATE before execution: Step 60 complete
- Current OpenAPI version before execution: `0.60.0`
- Read latest iteration: `docs/iterations/ITERATION_060.md`
- Current execution phase file: `docs/phases/PHASE_AUTO_061.md`

## Goals
- Add one reusable Linux deployment entrypoint for future iterations.
- Standardize managed package versioning from baseline `v0.1`.
- Establish one fixed release-history file as the source of truth.
- Keep runtime assumptions explicit for same-host Bridge deployment under `/root/ecommerce_ai`.

## Files Changed
- `deploy/deploy.sh`
- `deploy/lib.sh`
- `deploy/package-local.sh`
- `deploy/remote-deploy.sh`
- `deploy/run-with-env.sh`
- `deploy/start-main.sh`
- `deploy/stop-main.sh`
- `deploy/start-bridge.sh`
- `deploy/stop-bridge.sh`
- `deploy/verify-runtime.sh`
- `deploy/release-history.log`
- `deploy/deploy.env.example`
- `deploy/bridge.env.example`
- `deploy/DEPLOYMENT_WORKFLOW.md`
- `deploy/LOCAL_PACKAGE_DEPLOY.md`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/phases/PHASE_AUTO_061.md`
- `docs/iterations/ITERATION_061.md`
- `ITERATION_INDEX.md`

## DB / Migration Changes
- No new migration in this phase.

## API Changes
- No HTTP route or payload contract changed.
- OpenAPI version remains `0.60.0`.

## Design Decisions
- Replaced the Step 61 PowerShell/Windows baseline with bash-first helpers that package `.tar.gz` artifacts and deploy into Linux paths under `/root/ecommerce_ai`.
- Kept the managed version source of truth in one fixed repository file, now `deploy/release-history.log`, with baseline `v0.1` and minor auto-increment.
- Kept the same-host Bridge runtime default at `http://127.0.0.1:8081`.
- Moved deployment credentials/config to local environment variables through `deploy/deploy.env.example`; secrets are not stored in repo files.

## Verification
- Build and regression:
  - `go test ./...`
- Local package verification:
  - `bash ./deploy/package-local.sh --version v0.1 --output-root dist/package-check`
- Local-only managed release verification:
  - `bash ./deploy/deploy.sh --local-only --release-history-path dist/release-history-test.log --output-root dist/deploy-check --release-note "workflow verification" --skip-tests`
- Package layout verification:
  - confirmed versioned package directories and tar.gz artifacts are created
  - confirmed packages contain env examples, config, migrations, OpenAPI, and Linux deploy helpers

## Risks / Known Gaps
- Real SSH upload/deploy still requires user-provided server access outside this environment.
- `DEPLOY_PASSWORD` is read from the local shell environment, but non-interactive password auth still depends on local `sshpass`; otherwise the workflow falls back to normal SSH authentication.
- The repo still does not ship an automatic migration runner; migrations are packaged only.

## Suggested Next Step
- Export the real deployment variables locally and run `bash ./deploy/deploy.sh` as the standard managed release command from this version forward.

## Current Priority Update
- current-stage priority has since shifted to:
  - mainline feature development first
  - integration, verification, release, and deployment first
  - compatibility retirement deferred behind mainline delivery
- existing v0.4 convergence and retirement docs remain the governance baseline, but short-term work should advance retirement only in review / close-out / governance windows
