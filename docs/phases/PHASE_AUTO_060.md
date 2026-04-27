# PHASE_AUTO_060

## Why This Phase Now
- Step A-E mainline was already minimally usable.
- The next required work was not another feature increment; it was a truthful narrow integration pass against the current Bridge behavior plus deployment packaging readiness.
- MAIN and Bridge are expected to run on the same cloud server, so Bridge base-url and packaging assumptions needed to reflect same-host deployment rather than vague public ingress defaults.

## Current Context
- Current CURRENT_STATE before this phase: Step 59 complete
- Current OpenAPI version before this phase: `0.59.0`
- Latest iteration before this phase: `docs/iterations/ITERATION_059.md`
- Mainline focus: Bridge connectivity assumptions, Bridge-dependent task flow verification, and local build/package deployment readiness

## Goals
- Verify current Bridge connectivity assumptions honestly.
- Keep Bridge-dependent task create/business-info/audit/warehouse flows truthful without expanding platform scope.
- Fix only narrow config or integration issues required for deployment-ready packaging.
- Prepare a local package path that can be copied to the cloud server and started with explicit env assumptions.

## Allowed Scope
- `ERP_BRIDGE_BASE_URL` config hardening for same-host deployment
- Focused Bridge/client/task integration verification
- Narrow packaging/build/deploy preparation files
- Focused docs and tests

## Forbidden Scope
- New platform feature work
- Broad architecture redesign
- New finance/order/aftersale/cross-border modules
- Broad admin-center or workflow-center expansion
- Any change that treats packaging work as a pretext for wider product scope

## Expected File Changes
- `config/config.go`
- `config/config_test.go`
- `deploy/package-local.sh`
- `deploy/main.env.example`
- `deploy/LOCAL_PACKAGE_DEPLOY.md`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/phases/PHASE_AUTO_060.md`
- `docs/iterations/ITERATION_060.md`

## Required API / DB Changes
- API:
  - keep existing Bridge and task HTTP contracts stable
  - document the same-host Bridge runtime assumption more explicitly
- DB:
  - no new migration in this phase

## Success Criteria
- Focused integration-oriented verification passes.
- `go test ./...` passes.
- Local build/package command is verified.
- Deployment env assumptions and startup command are explicit.
- Docs stay honest about the unresolved runtime dependency on Bridge ingress behavior.
