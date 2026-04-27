# MAIN / Bridge v0.4 Convergence Plan

Date: 2026-03-13
Source of truth: `docs/MAIN_BRIDGE_RESPONSIBILITY_MATRIX.md`

## Executive Summary

v0.4 is the minimal convergence release that removes production ambiguity between legacy 8080 behavior, canonical MAIN, and Bridge 8081 by locking production MAIN to `./cmd/server`, keeping MAIN as the public business app, keeping Bridge as the ERP query/mutation adapter, preserving only the compatibility surfaces needed for rollback, and aligning packaging, deploy, runtime, and smoke docs to the same-host `8080` MAIN plus `8081` Bridge model. v0.4 is not an architecture rewrite, does not extract the ERP sync runtime, does not add new ERP mutation domains, and does not broaden Bridge beyond the current query facade plus existing `business-info` filing path; legacy 8080 sync behavior remains in MAIN for v0.4, `cmd/api` is demoted/deprecated as a production entrypoint in v0.4 rather than broadly reworked, and Bridge write boundaries remain as-is in v0.4.

Current priority decision after v0.4 close-out:
- keep this plan as the governance baseline for responsibility, convergence, and later cleanup decisions
- short-term development priority is now mainline feature delivery plus integration/verification/release/deployment work
- compatibility retirement no longer drives the current mainline execution order and should continue only in review, version close-out, or post-release governance windows

## Terminology Guardrails

- current runtime reality:
  - `live MAIN` means the public business application service on `8080`
  - `candidate MAIN` means a side-by-side validation instance of that same MAIN service on `18080`, not a new runtime split
  - `Bridge` means the ERP/JST adapter runtime on `8081`; it owns adapter query semantics and ERP mutation execution, but not MAIN business routes or generic sync/background ownership in v0.4
  - MAIN keeps the current sync/runtime ownership needed for the smallest safe convergence path in v0.4, including `ERPSyncWorker` and `/v1/products/sync/*`
- target architecture:
  - the target direction is clearer MAIN-versus-Bridge language, not a new runtime split in this turn
- compatibility-only behavior:
  - legacy `8080` compatibility surfaces and same-host loopback Bridge assumptions remain rollback-safe continuity only; they are not the target architecture
- deferred future extraction work:
  - any later extraction of MAIN-owned sync/runtime/background concerns remains explicitly deferred until after v0.4

## Scope In

- [x] Lock production packaging and deploy resolution to `./cmd/server`; remove the current auto-fallback that previously packaged `./cmd/api`. Matrix mapping: `MAIN live runtime on port 8080`, `Same-host loopback dependency`, `Rollback-sensitive Bridge-coupled items`.
- [x] Demote `cmd/api` to non-production status in scripts and docs for v0.4. Decision for v0.4: retire `cmd/api` as a production entrypoint now; physical deletion or stub consolidation may happen after v0.4. Matrix mapping: `MAIN live runtime on port 8080`.
- [x] Keep `/v1/erp/*` as the Bridge-backed query facade and keep `GET /v1/products/search` plus `GET /v1/products/{id}` as local-cache compatibility routes only. Matrix mapping: `MAIN Bridge facade`, `Local cached ERP product read surface`, `Legacy 8080 ERP-facing responsibility that tries to be both cache and live ERP`.
- [x] Keep `PATCH /v1/tasks/{id}/business-info` as the only MAIN-side ERP mutation trigger and keep Bridge `POST /erp/products/upsert` as the mutation executor. Decision for v0.4: Bridge write boundaries do not change. Matrix mapping: `Product filing trigger boundary`, `Product upsert execution via Bridge`, `Local cache/binding of Bridge-selected products`.
- [x] Keep legacy 8080 sync behavior in MAIN for v0.4, including `ERPSyncWorker`, `GET /v1/products/sync/status`, and `POST /v1/products/sync/run`; do not extract or move it. Matrix mapping: `Incremental ERP cache sync worker`, `Local ERP cache / product sync`, `Local ERP sync admin surface`.
- [x] Make deployment, runtime, verification, and cutover documentation describe one production model: MAIN on `8080`, Bridge on `8081`, MAIN using `ERP_BRIDGE_BASE_URL=http://127.0.0.1:8081`, and side-by-side MAIN validation on `18080` before cutover. Matrix mapping: `Same-host loopback dependency`, `Compatibility surfaces for safe cutover and fallback`, `Rollback-sensitive Bridge-coupled items`.

## Scope Out

- [ ] Do not extract ERP sync into a separate runtime or introduce a second daily full-sync job in v0.4; the current MAIN-owned sync loop remains the only owned sync runtime after v0.4.
- [ ] Do not move MAIN-owned business routes, workflow state, warehouse flow, audit flow, export center, integration-center visibility, or user/auth concerns into Bridge.
- [ ] Do not add new Bridge mutation domains in v0.4, including shelve/unshelve, virtual quantity, procurement docking, WMS docking, finance writeback, or generic ERP writeback APIs.
- [ ] Do not remove `GET /v1/products/search`, `GET /v1/products/{id}`, or `POST /v1/integration/call-logs/{id}/advance` in v0.4; they stay only as compatibility surfaces and retirement candidates after v0.4.
- [ ] Do not broadly clean up unrelated V6 legacy surfaces such as `/v1/sku/*`, `/v1/agent/*`, `/v1/incidents`, or `/v1/policies` unless they directly block the MAIN/Bridge convergence tasks above.
- [ ] Do not redesign Bridge runtime ownership, public ingress, or topology beyond the existing same-host loopback assumption; public-IP ingress troubleshooting remains an ops/runtime issue outside v0.4 scope.
- [ ] Do not perform broad `cmd/api` deletion or entrypoint refactoring if a narrow production demotion removes the packaging/deploy ambiguity.

## Required Workstreams

1. Entrypoint convergence around `cmd/server` and the deploy/package scripts.
2. ERP route-surface convergence across router registration, OpenAPI wording, and MAIN-versus-Bridge ownership language.
3. MAIN/Bridge mutation-boundary enforcement around `PATCH /v1/tasks/{id}/business-info`.
4. Compatibility-surface labeling and retirement handling for legacy 8080 behavior that must remain temporarily.
5. Deployment, runtime, smoke-check, and cutover documentation alignment to the same production model.

## Task Breakdown

| Workstream | Task Name | Purpose | Repo Areas Likely Affected | Expected Output | Risk If Skipped |
|---|---|---|---|---|---|
| Entrypoint convergence | Lock `resolve_go_entrypoint` to `./cmd/server` only | Remove the already-observed production packaging drift where releases were built from `./cmd/api`. | `deploy/lib.sh`, `deploy/package-local.sh`, `deploy/deploy.sh`, `deploy/release-history.log` | Packaging and managed deploy fail fast unless `./cmd/server` is present; package metadata resolves only to `./cmd/server`. | A future release can silently ship the wrong entrypoint again. |
| Entrypoint convergence | Demote `cmd/api` to non-production status | Make repo intent explicit without forcing a broad code deletion in the same release. | `cmd/api/main.go`, `deploy/DEPLOYMENT_WORKFLOW.md`, `deploy/LOCAL_PACKAGE_DEPLOY.md`, `CURRENT_STATE.md` | `cmd/api` is documented as compatibility-only or deprecated; no production script or workflow points to it. | Engineers keep treating `cmd/api` as equivalent to canonical MAIN. |
| Route-surface convergence | Mark `/v1/erp/*` as the only live ERP query facade on MAIN | Stop the old 8080 ambiguity where cache-backed product search could be mistaken for live ERP lookup. | `transport/http.go`, `docs/api/openapi.yaml`, `docs/MAIN_BRIDGE_RESPONSIBILITY_MATRIX.md` | Router/docs/OpenAPI consistently describe `/v1/erp/*` as Bridge-backed query facade. | Frontend or ops will keep sending live-ERP expectations to the wrong route family. |
| Route-surface convergence | Mark `/v1/products/search` and `/v1/products/{id}` as compatibility-only local cache routes | Preserve rollback-safe behavior without letting those routes grow into the future ERP contract. | `transport/http.go`, `docs/api/openapi.yaml`, product handler comments/tests | Docs and route descriptions explicitly say local cache, compatibility-only, not real-time ERP. | New work will drift back onto the wrong surface and re-create the mixed-responsibility model. |
| Route-surface convergence | Retire mixed "cache or live ERP" language | Remove wording that still implies MAIN owns both live ERP queries and local product-cache lookup. | `docs/api/openapi.yaml`, `CURRENT_STATE.md`, deploy/readme docs | One consistent ownership story across docs and release notes. | Docs will continue to contradict the responsibility matrix. |
| Boundary enforcement | Freeze `business-info` as the only ERP mutation trigger | Keep the rollback-sensitive write boundary narrow and prevent spread into create/procurement/warehouse flows. | `service/task_service.go`, `transport/handler/task.go`, `service/task_erp_bridge_test.go`, `docs/api/openapi.yaml` | Tests and docs show that only `PATCH /v1/tasks/{id}/business-info` with valid filing conditions can call Bridge upsert. | MAIN will accrete hidden ERP write paths that are hard to roll back safely. |
| Boundary enforcement | Keep Bridge execution ownership unchanged | Ensure MAIN prepares payloads, validates business rules, and logs traces, but Bridge still executes ERP writes. | `service/erp_bridge_service.go`, `service/task_service.go`, `service/task_erp_bridge_test.go` | No new MAIN-owned ERP upsert engine or mutation helper appears in v0.4. | Boundary confusion will spread from docs into code. |
| Boundary enforcement | Preserve MAIN-owned local product binding | Keep Bridge-selected product cache/binding in MAIN so task state remains local and stable during rollback. | `service/erp_bridge_service.go`, `repo/mysql/product.go`, `service/task_erp_bridge_test.go` | Local `products` binding remains explicit and tested as MAIN-owned state. | Task state can become dependent on Bridge-only identifiers or behavior. |
| Compatibility handling | Publish a v0.4 compatibility register | Explicitly list what remains temporarily, what is compatibility-only, and what is retired now or after v0.4. | `docs/V0_4_MAIN_BRIDGE_CONVERGENCE_PLAN.md`, `docs/api/openapi.yaml`, `CURRENT_STATE.md` | The register distinguishes four sets: remain in MAIN and not compatibility-only (`/health`, `/ping`, `/v1/auth/*`, `/v1/tasks/*`, `/v1/products/sync/*`, `ERPSyncWorker`); compatibility-only in v0.4 (`/v1/products/search`, `/v1/products/{id}`, `POST /v1/integration/call-logs/{id}/advance`, same-host loopback Bridge URL assumption); retired in v0.4 (`cmd/api` as a production entrypoint, mixed cache/live ERP language); retire after v0.4 (actual route removal once callers no longer depend on compatibility paths). | Teams will keep extending temporary surfaces because nothing marks them as temporary. |
| Compatibility handling | Retire production use of `cmd/api` and the mixed-ERP responsibility model in v0.4 | Make the hard retirement decisions explicit, while leaving low-risk physical cleanup for later. | `deploy/lib.sh`, deploy docs, `CURRENT_STATE.md` | v0.4 closes the ambiguity even if some compatibility code still exists on disk. | The repo will say "canonical MAIN" while tooling still behaves otherwise. |
| Deployment/runtime/docs cleanup | Align package, deploy, verify, and cutover docs to one runtime model | Make packaging, runtime env defaults, smoke checks, and release assumptions describe the same thing. | `deploy/DEPLOYMENT_WORKFLOW.md`, `deploy/LOCAL_PACKAGE_DEPLOY.md`, `deploy/main.env.example`, `deploy/verify-runtime.sh`, `CURRENT_STATE.md` | One deploy story: MAIN `8080`, Bridge `8081`, loopback Bridge URL, side-by-side MAIN validation before cutover. | Cutover will fail on documentation drift rather than code defects. |
| Deployment/runtime/docs cleanup | Add a Bridge-coupled smoke and cutover checklist | Make pre-cutover validation operationally meaningful for the exact rollback-sensitive paths. | deploy docs, release notes docs, possibly helper script docs | A written checklist that covers `/health`, `/ping`, `/v1/erp/*`, and the `business-info` filing path before live cutover. | Future releases can claim "ready" without validating the actual ambiguous paths. |

## Sequencing

1. Lock the production entrypoint first.
Reason: every later validation artifact is untrustworthy if packaging can still resolve to `./cmd/api`.

2. Update route ownership language and compatibility labels second.
Dependency: step 1.
Reason: the release surface must be described correctly before tests and smoke procedures are finalized.

3. Harden the `business-info` mutation boundary and its regression tests third.
Dependency: step 2.
Reason: once the query surfaces are labeled, the write boundary can be enforced without re-opening scope.

4. Publish the compatibility register and retirement decisions fourth.
Dependency: steps 2 and 3.
Reason: compatibility handling must reflect the final route and mutation decisions, not preliminary wording.

5. Finish deploy/runtime/docs alignment and smoke/cutover instructions last.
Dependency: steps 1 through 4.
Reason: the operational docs should describe the converged state, not the pre-convergence state.

Before any future packaged release or cutover, steps 1, 3, and 5 must be complete. No v0.4 release candidate should be packaged, validated on `18080`, or cut over to `8080` while entrypoint resolution, Bridge write-boundary wording, and same-host smoke instructions still disagree.

## Acceptance Criteria

- `deploy/package-local.sh` and `deploy/deploy.sh --local-only` package MAIN from `./cmd/server` only; package metadata no longer resolves to `./cmd/api`, and the workflow fails fast instead of falling back.
- v0.4 documentation explicitly states that legacy 8080 sync behavior remains in MAIN for v0.4, `cmd/api` is retired from production use in v0.4 by demotion/deprecation rather than broad rewrite, and Bridge write boundaries remain unchanged in v0.4.
- `docs/api/openapi.yaml` and related route docs describe `/v1/erp/*` as the Bridge-backed live ERP query facade, `GET /v1/products/search` plus `GET /v1/products/{id}` as compatibility-only local-cache routes, and `GET /v1/products/sync/status` plus `POST /v1/products/sync/run` as MAIN-owned internal sync controls.
- Tests or explicit handler/service guards show that only `PATCH /v1/tasks/{id}/business-info` can trigger Bridge `POST /erp/products/upsert`; task create, procurement, warehouse, audit, and other product routes do not.
- The remaining compatibility surfaces are explicitly listed with owner and retirement posture: `GET /v1/products/search`, `GET /v1/products/{id}`, `POST /v1/integration/call-logs/{id}/advance`, and same-host loopback `ERP_BRIDGE_BASE_URL=http://127.0.0.1:8081`.
- Deployment and verification docs all assume the same runtime model: live MAIN on `8080`, live Bridge on `8081`, candidate MAIN on `18080`, and Bridge reachability checked through same-host loopback before cutover.
- The next managed or local package record after v0.4 work shows `entrypoint=./cmd/server` and no release workflow record shows `entrypoint=./cmd/api`.

## v0.4 Close-out Status

- completed in v0.4 so far:
  - canonical MAIN production entrypoint is locked to `./cmd/server`
  - `cmd/api` is demoted from production use without broad on-disk deletion
  - MAIN-versus-Bridge business/runtime ownership language is aligned
  - legacy compatibility surfaces are classified and bounded at repository level
  - sync/runtime terminology now explicitly keeps current MAIN-owned continuity in place for v0.4
  - post-v0.4 retirement sequencing is documented in `docs/V0_4_COMPATIBILITY_RETIREMENT_CHECKLIST.md`
- not yet retired or removed:
  - compatibility routes and remnants remain on disk where rollback-safe continuity still requires them
  - no broad route retirement, sync extraction, or topology redesign was performed in v0.4
- next likely implementation focus after v0.4:
  - execute narrow caller-inventory-driven retirement steps from the compatibility retirement checklist
  - keep Bridge-coupled smoke/cutover verification explicit before any removal of rollback-sensitive compatibility artifacts

## v0.4 Compatibility Register

The repository compatibility register for the remaining old 8080-style surface is now:

- `keep temporarily`
  - root operational probes: `/health`, `/ping`
  - legacy sync continuity that still remains MAIN-owned in v0.4: `GET /v1/products/sync/status`, `POST /v1/products/sync/run`, `ERPSyncWorker`
- `compatibility only`
  - local cached ERP reads: `GET /v1/products/search`, `GET /v1/products/{id}`
  - integration compatibility advance route: `POST /v1/integration/call-logs/{id}/advance`
  - rollback/runtime continuity assumption: `ERP_BRIDGE_BASE_URL=http://127.0.0.1:8081`
- `replaced by canonical MAIN`
  - `POST /v1/audit` replaced by task-centric audit routes under `/v1/tasks/{id}/audit/*`
- `retire after v0.4`
  - unrelated V6 public routes: `/v1/sku/*`, `/v1/agent/*`, `/v1/incidents`, `/v1/policies`

Post-v0.4 retirement sequencing is tracked in `docs/V0_4_COMPATIBILITY_RETIREMENT_CHECKLIST.md`.
The first real post-v0.4 retirement step is now complete for the wording-only mixed cache/live ERP responsibility remnant; active docs now treat that language as retired.

## Risks and Mitigations

- Risk: `cmd/api` remains on disk and drifts from `cmd/server`.
Mitigation: remove it from all production packaging and deploy paths in v0.4; if needed later, reduce it to a stub or delete it after convergence is proven.

- Risk: docs and OpenAPI drift away from router reality again.
Mitigation: update route wording, compatibility labels, and deploy docs in the same convergence change set; do not leave ownership updates split across later phases.

- Risk: Bridge runtime checks fail because host routing or ingress assumptions are wrong.
Mitigation: keep loopback `ERP_BRIDGE_BASE_URL` as the documented default and require side-by-side MAIN validation on `18080` before any live cutover.

- Risk: scope expands into general legacy cleanup or new ERP platform work.
Mitigation: reject any v0.4 task that does not directly improve entrypoint truthfulness, route ownership clarity, compatibility labeling, or same-host runtime correctness.

- Risk: mutation-boundary wording changes without matching regression coverage.
Mitigation: pair the `business-info` boundary work with explicit tests proving no other MAIN route triggers Bridge writes.

## Immediate Next Coding Turn

Prioritize mainline feature work plus integration/verification/release/deployment execution; use this plan as governance baseline only, and resume retirement work in review, close-out, or post-release governance windows.
