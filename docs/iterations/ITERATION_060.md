# ITERATION_060

## Phase
- PHASE_AUTO_060 / narrow Bridge integration pass and local deployment packaging readiness

## Input Context
- Current CURRENT_STATE before execution: Step 59 complete
- Current OpenAPI version before execution: `0.59.0`
- Read latest iteration: `docs/iterations/ITERATION_059.md`
- Current execution phase file: `docs/phases/PHASE_AUTO_060.md`

## Goals
- Verify Bridge connectivity assumptions against the current upstream behavior.
- Re-test the Bridge-dependent mainline without adding platform features.
- Fix only narrow deployment-truthfulness or integration issues.
- Make local build/package deployment ready for the target same-host cloud server.

## Files Changed
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

## DB / Migration Changes
- No new migration in this phase.

## API Changes
- OpenAPI version advanced from `0.59.0` to `0.60.0`.
- No HTTP route or payload contract changed.
- ERP Bridge docs now state the same-host deployment assumption explicitly:
  - default runtime base URL is loopback
  - public-IP Bridge ingress is an explicit override, not the default packaging path

## Design Decisions
- Treated Bridge public-IP connectivity as a runtime environment concern because live probes still returned an empty HTTP reply rather than a stable JSON contract.
- Changed only the default Bridge base URL to same-host loopback because MAIN and Bridge are intended to run on the same cloud server.
- Added a packaging script and env example instead of introducing a deployment-platform redesign.

## Verification
- Live Bridge probe attempts against `223.4.249.11`:
  - TCP connect to `223.4.249.11:8081` succeeded
  - `GET /erp/products` and `GET /erp/categories` returned empty HTTP replies / closed connection from this environment
  - HTTPS probes failed TLS handshake
- Focused test coverage:
  - `go test ./service ./transport/handler -run 'ERPBridge|Task.*(BusinessInfo|Create|Warehouse|Audit)|AuditV7|Warehouse'`
  - `go test ./config`
- Full regression:
  - `go test ./...`
- Build/package verification:
  - `bash ./deploy/package-local.sh --version 0.60.0 --output-root dist/workflow-main-package-test`

## Risks / Known Gaps
- Live Bridge public-IP ingress still needs on-host verification because this environment only observed empty HTTP replies.
- The package remains dependent on runtime MySQL, Redis, and Bridge availability; this phase does not add process supervision, reverse proxy, or orchestration.
- No retry/outbox/callback layer was added around Bridge filing, by design.

## Suggested Next Step
- Copy the generated package to the target cloud server, set `.env` with the real DSN/Redis values, and keep `ERP_BRIDGE_BASE_URL` on same-host loopback unless Bridge is intentionally fronted by another local ingress.
