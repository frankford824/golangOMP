# PHASE_AUTO_061

## Why This Phase Now
- The current minimal mainline and narrow Bridge integration pass were already complete.
- The next required work was operational, not business-facing: standardize versioned packaging, deployment, and release bookkeeping from this version forward.
- MAIN and Bridge are intended to run on the same cloud host, so deployment workflow and runtime verification needed to preserve the same-host loopback assumption without widening architecture.

## Current Context
- Current CURRENT_STATE before this phase: Step 60 complete
- Current OpenAPI version before this phase: `0.60.0`
- Latest iteration before this phase: `docs/iterations/ITERATION_060.md`
- Mainline focus: one-command deploy workflow, version auto-increment, fixed release history, runtime verification guidance

## Goals
- Add one standard deploy entrypoint that future iterations can reuse.
- Make managed package versions auto-increment from one fixed repository file starting at `v0.1`.
- Keep release history readable and script-friendly.
- Add remote deploy/start/stop/verify helpers suited to the Linux package shape and same-host Bridge runtime.
- Update docs and handover/state bookkeeping without changing business flow.

## Allowed Scope
- Deploy/package scripts
- Release-history/version bookkeeping
- Runtime verification helpers and docs
- Env/config examples for deployment
- CURRENT_STATE / MODEL_HANDOVER / iteration bookkeeping updates

## Forbidden Scope
- New business features
- Workflow or auth redesign
- Bridge contract redesign
- CI/CD platform rollout
- Kubernetes / Terraform / large infra additions

## Expected File Changes
- `deploy/deploy.sh`
- `deploy/lib.sh`
- `deploy/package-local.sh`
- `deploy/remote-deploy.sh`
- `deploy/start-main.sh`
- `deploy/stop-main.sh`
- `deploy/start-bridge.sh`
- `deploy/stop-bridge.sh`
- `deploy/run-with-env.sh`
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

## Required API / DB Changes
- API:
  - no HTTP route or payload contract change
- DB:
  - no new migration

## Success Criteria
- `deploy/deploy.sh` can resolve the next managed version, build, package, and prepare for remote deployment.
- `deploy/release-history.log` is the fixed version source of truth with baseline `v0.1`.
- Deployment docs cover local build/package, upload, remote deploy/start, and runtime verification.
- Build/package assumptions are verified locally against the Linux/tar workflow.
