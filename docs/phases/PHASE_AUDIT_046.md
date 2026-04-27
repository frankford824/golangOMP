# PHASE_AUDIT_046 - Post-Skeleton Prioritization Audit

## Why This Audit Now
- Step 45 finished cross-center adapter-boundary consolidation and stabilized shared boundary vocabulary across export, integration, and storage/upload.
- The repository now has several broad skeleton centers, but not all of them are equally ready for deeper runtime work.
- This round must re-rank Step 46 to Step 50 from actual repository truth and allow only one small, boundary-safe follow-up phase.

## Current Repository Truth

### Stable skeletons
- Task/workflow mainline is stable enough for continued reuse:
  - create/list/read/detail
  - assign
  - submit-design
  - business-info
  - procurement summary + advance
  - warehouse prepare/receive/reject/complete
  - close readiness and structured workflow reasons
- Audit / handover / outsource / warehouse loops exist with durable persistence and route coverage.
- Board / workbench / filter convergence is stable:
  - shared task filter contract
  - board candidate scan path
  - queue presets
  - saved workbench preferences
- Category center + category ERP mapping + mapped product search + task-side `product_selection` form one stable local original-product selection loop.
- Cost-rule center has a stable governance skeleton:
  - rule CRUD
  - preview / task-side prefill
  - governance state
  - history
  - override audit
  - approval / finance placeholder boundary summaries
- Export center is a stable placeholder platform skeleton:
  - job persistence
  - lifecycle
  - event timeline
  - claim/read/refresh handoff
  - explicit start boundary
  - attempts
  - dispatches
  - planning boundary summaries
- Integration center is a stable narrow placeholder seam:
  - static connector catalog
  - call-log persistence
  - execution persistence
  - list/detail visibility
  - compatibility advance facade over execution state
- Task asset storage/upload has a stable placeholder binding skeleton:
  - `upload_requests`
  - `asset_storage_refs`
  - task-asset write binding
  - shared adapter/resource/handoff summaries

### Placeholder skeletons
- Auth / org / visibility remain incomplete beyond current route-role checks:
  - debug-header actor only
  - no login/session
  - no org tree
  - no department/team visibility trimming
  - no task-level data scope engine
- Integration center is still not a real execution platform:
  - no outbound executor
  - no callback processor
  - no external auth/signature negotiation
  - no delivery guarantee / idempotency protocol beyond stored placeholder executions
- Export center is still not a real runner/storage platform:
  - no background runner or scheduler
  - no real file generation
  - no byte delivery
  - no NAS / object-storage integration
- Task asset upload/storage is still not a real file platform:
  - no upload session lifecycle beyond request creation + later binding
  - no signed URL
  - no object-storage or NAS adapter
  - no real byte-transfer confirmation
- Cost approval / finance remain placeholder-only:
  - no approval actor model
  - no real finance posting / accounting / settlement
  - no ERP writeback beneath the placeholder boundary
- KPI / finance / report platform layers are still mostly unstarted.

## Ranking Logic
- Prefer the smallest runtime hardening phase that deepens an existing skeleton without committing the repo to real infrastructure.
- Avoid phases that would implicitly choose async runner, storage, ERP, or finance-system topology.
- Prefer phases that convert an obvious placeholder seam into a clearer internal lifecycle loop with additive contracts only.

## Step 46 to Step 50 Candidate Phases

### 1. Step 46 Candidate
- Phase Name: Upload Request Lifecycle Hardening
- Goal:
  - turn `upload_requests` from create/get + passive bind only into a small placeholder lifecycle boundary
  - add explicit cancel/expire actions while keeping binding as the only asset-write attachment path
  - expose lifecycle readiness on the upload-request read model
- Why now:
  - smallest safe runtime deepening
  - closes an obvious storage/upload seam without entering real upload infrastructure

### 2. Step 47 Candidate
- Phase Name: Integration Execution Replay / Retry Hardening
- Goal:
  - make placeholder call-log/execution replay rules more explicit
  - harden retry/replay admission without introducing real external execution
- Why next:
  - integration already has executions and derived retry hints, so the next safe step is tighter lifecycle admission rather than real connector execution

### 3. Step 48 Candidate
- Phase Name: Export Dispatch / Attempt Admission Hardening
- Goal:
  - tighten placeholder dispatch/attempt redispatch and retry admission
  - keep export lifecycle, dispatch, and attempt seams explicit before any real runner/storage work
- Why later:
  - valuable, but much closer to future async-runner topology choices than upload/integration lifecycle hardening

### 4. Step 49 Candidate
- Phase Name: Auth / Org / Visibility Policy Skeleton Scaffolding
- Goal:
  - add explicit policy language for org / visibility scope without claiming real auth or data trimming
  - prepare later enforcement seams above existing route-role checks
- Why later:
  - product-policy-sensitive
  - requires clarity on org hierarchy and visibility ownership before code shape is worth locking in

### 5. Step 50 Candidate
- Phase Name: KPI / Finance / Report Platform Entry Boundary
- Goal:
  - define the first stable platform seam for KPI / report / finance consumers over current read models
  - keep real finance and reporting execution deferred
- Why last:
  - broad scope and highly likely to commit the repository to cross-domain reporting semantics too early

## Recommended Priority Order
1. Step 46: Upload Request Lifecycle Hardening
2. Step 47: Integration Execution Replay / Retry Hardening
3. Step 48: Export Dispatch / Attempt Admission Hardening
4. Step 49: Auth / Org / Visibility Policy Skeleton Scaffolding
5. Step 50: KPI / Finance / Report Platform Entry Boundary

## Auto-Advance vs Confirmation

### Safe to continue automatically
- Step 46: Upload Request Lifecycle Hardening
- Step 47: Integration Execution Replay / Retry Hardening

### Must stop for user confirmation before entering
- Step 48: Export Dispatch / Attempt Admission Hardening
  - reason: easily drifts into real runner / scheduler / storage topology choices
- Step 49: Auth / Org / Visibility Policy Skeleton Scaffolding
  - reason: requires product decisions around org structure, data scope, and ownership semantics
- Step 50: KPI / Finance / Report Platform Entry Boundary
  - reason: commits cross-domain reporting semantics and platform ownership too early without explicit confirmation

## High-Risk Infrastructure Zone
- Step 48 touches the edge of async-runner / dispatch / storage topology and must stay out of real infrastructure unless explicitly approved.
- Step 50 touches reporting / KPI / finance platform foundations and is likely to pull in durable cross-domain contracts.
- Any phase that tries to turn export/integration/storage/upload into real executor, scheduler, NAS, object-storage, signed-URL, approval, or finance systems is outside the safe post-skeleton zone for automatic advancement.

## Selected Execution Phase For This Round
- Execute only one new phase after this audit:
  - Step 46: Upload Request Lifecycle Hardening

## Selected Phase Boundaries

### Allowed scope
- `domain/asset_storage.go`
- `repo/interfaces.go`
- `repo/mysql/upload_request.go`
- `service/asset_upload_service.go`
- `transport/handler/asset_upload.go`
- `transport/http.go`
- focused tests
- required document sync

### Forbidden scope
- No real upload session bytes or multipart pipeline
- No NAS / object-storage / signed URL / CDN / file-service integration
- No async runner
- No export/integration real execution work
- No approval / finance / ERP runtime integration

## Selected Phase Success Criteria
- Upload requests gain an explicit internal placeholder lifecycle advance route.
- Upload-request reads expose clear lifecycle readiness instead of relying on implicit status interpretation only.
- Bound upload requests remain terminal and continue to be driven by task-asset writes.
- No second execution phase starts in this round.
