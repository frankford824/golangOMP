# PHASE_AUDIT_036 - Phase Audit / Step 36 Reordering

## Why This Audit Now
- The repository just finished Step 35 and has accumulated several stable skeleton centers plus several still-placeholder platform seams.
- The highest remaining risk is no longer missing CRUD surface area; it is boundary drift between documented role intent, actual route behavior, and future platform work.
- This round must stop the prior three-phase auto-run pattern and re-rank Step 36 to Step 40 from current repository truth.

## Current Repository Truth

### Stable skeletons
- Task/workflow mainline is stable enough for continued reuse:
  - task create/list/read/detail
  - assignment
  - business-info
  - procurement summary + advance
  - warehouse prepare/receive/reject/complete
  - close readiness and structured workflow reasons
- Audit / handover / outsource / warehouse read-write loops exist with durable persistence and tests.
- Board/workbench/filter convergence is stable:
  - shared task filter contract
  - board candidate scan path
  - preset queues
  - saved workbench preferences
- Category center + category ERP mapping + mapped product search + task-side `product_selection` read model form one stable local original-product selection loop.
- Export center is a stable placeholder platform skeleton:
  - job persistence
  - lifecycle
  - download handoff
  - events
  - attempts
  - dispatches
  - read-model summaries
- Integration center has a stable narrow placeholder seam:
  - static connector catalog
  - call-log persistence
  - lifecycle advancement
  - list/detail visibility

### Placeholder skeletons
- Auth/RBAC is still placeholder-biased:
  - request actor comes only from debug headers
  - route metadata declares `required_roles`
  - no request is actually rejected by role today
  - org tree / department / team / visibility scope does not exist
- Asset storage/upload boundary is still placeholder-only:
  - `POST /v1/tasks/{id}/assets/mock-upload` is the only task-asset write route
  - `whole_hash` is passthrough metadata only
  - no upload session, storage adapter, or NAS/object boundary exists for V7 task assets
- ERP sync and integration execution remain stub-only:
  - stub file provider
  - no outbound executor
  - no callback processor
  - no retry/idempotency boundary beyond placeholder call logs
- Cost-rule center is still a business-usable skeleton, not a governed pricing platform:
  - no rule version graph
  - no override governance/audit layer
  - limited formula support
- Export center is still not a real runner/storage system:
  - no generated artifact bytes
  - no storage adapter
  - no scheduler/worker platform

### Missing or not-yet-started platform layers
- No org/account/login/session module.
- No task visibility policy engine or owner/group/department visibility trimming.
- No KPI center, finance layer, or deeper reporting/export domain.

## Highest-Impact Gap
- The single gap class that most affects PRD-total implementation is auth/org/visibility/role boundary drift.
- Reason:
  - almost every V7 route already carries documented role intent
  - frontend-ready and internal-placeholder APIs are increasingly numerous
  - continuing platform work without actual route enforcement will compound contract drift
  - org/visibility can remain future work, but route-level role enforcement is overdue now

## Step 36 to Step 40 Candidate Phases

### 1. Step 36 Candidate
- Phase Name: Placeholder Auth / RBAC Enforcement Hardening
- Goal:
  - turn existing V7 `required_roles` metadata into actual route-level role checks
  - keep auth source limited to debug headers
  - do not introduce org tree or data-visibility engine yet
- Why now:
  - highest leverage cross-cutting hardening
  - smallest bounded step that reduces future drift immediately

### 2. Step 37 Candidate
- Phase Name: Task Asset Storage / Upload Adapter Boundary
- Goal:
  - replace task-asset `mock-upload` as the only write seam with a real placeholder upload/storage boundary
  - separate metadata persistence from storage-adapter handoff
- Why next:
  - task asset handling is the largest still-open PRD backbone gap after role enforcement

### 3. Step 38 Candidate
- Phase Name: Integration Center Execution Boundary Hardening
- Goal:
  - harden integration call logs around replay/idempotency/request envelope semantics
  - keep real external execution deferred
- Why next:
  - current integration center has persistence but still lacks a stronger execution seam

### 4. Step 39 Candidate
- Phase Name: Export Runner / Storage Boundary Planning Hardening
- Goal:
  - make export job runner/storage seams explicit enough for later worker/storage implementation
  - still avoid real storage delivery in this step
- Why later:
  - export placeholder skeleton is already richer than task-asset/integration auth seams

### 5. Step 40 Candidate
- Phase Name: Cost Rule Governance / Versioning / Override Hardening
- Goal:
  - move cost rules from usable skeleton toward governed business infrastructure
  - add versioning and stronger override governance semantics
- Why later:
  - important, but business-policy-sensitive and should follow boundary hardening phases first

## Recommended Priority Order
1. Step 36: Placeholder Auth / RBAC Enforcement Hardening
2. Step 37: Task Asset Storage / Upload Adapter Boundary
3. Step 38: Integration Center Execution Boundary Hardening
4. Step 39: Export Runner / Storage Boundary Planning Hardening
5. Step 40: Cost Rule Governance / Versioning / Override Hardening

## Auto-Advance vs Confirmation

### Continue automatic advancement
- Step 36: Placeholder Auth / RBAC Enforcement Hardening
- Step 37: Task Asset Storage / Upload Adapter Boundary
- Step 38: Integration Center Execution Boundary Hardening

### Must stop for user confirmation before entering
- Step 39: Export Runner / Storage Boundary Planning Hardening
  - reason: likely commits the repo to runner/storage topology choices
- Step 40: Cost Rule Governance / Versioning / Override Hardening
  - reason: likely commits business pricing governance semantics and override policy
- KPI / finance / deeper export/report platform layers
  - reason: broad product-scope choice, reporting semantics, and data-ownership questions are still open

## Selected Execution Phase For This Round
- Execute only one new phase after this audit:
  - Step 36: Placeholder Auth / RBAC Enforcement Hardening

## Allowed Scope For Step 36
- `domain/rbac_placeholder.go`
- `transport/auth_placeholder.go`
- route-level auth/RBAC tests
- `docs/api/openapi.yaml`
- state / iteration / handover docs required for repository truth

## Forbidden Scope For Step 36
- No org tree or account system
- No department/team membership persistence
- No task-level visibility filtering
- No export/integration/storage runner work
- No KPI/finance/report module work

## Step 36 Success Criteria
- V7 routes with `withAccessMeta(...)` reject unauthorized actors using current debug-header role input.
- OpenAPI and state docs no longer describe role metadata as advisory-only.
- Internal placeholder and ready-for-frontend classification stays unchanged.
- No second phase is started in this round.
