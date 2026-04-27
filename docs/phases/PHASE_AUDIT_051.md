# PHASE_AUDIT_051 - Post-Step-50 Prioritization Audit

## Why This Audit Now
- Step 50 completed KPI / finance / report entry-boundary scaffolding.
- The repository now has several mature skeleton centers, but the post-Step-50 route should be re-ranked from code truth rather than continuing a stale roadmap.
- This round must stop broad auto-continuation and select only one small, boundary-safe phase.

## Current Repository Truth

### Stable skeletons
- Task/workflow mainline is stable and reusable:
  - create/list/read/detail
  - assign / submit-design
  - business-info / procurement / warehouse / close
  - structured workflow, readiness, and blocking-reason contracts
- Audit / handover / outsource / warehouse loops are durable and route-complete.
- Board / workbench / filter convergence is stable:
  - shared task-filter contract
  - board candidate scan path
  - queue presets
  - saved workbench preferences
- Category center + category ERP mapping + mapped local product search + task `product_selection` form a stable original-product selection loop.
- Cost-rule center is a stable governance skeleton:
  - rule CRUD
  - preview / task prefill
  - governance history
  - override audit
  - approval / finance placeholder boundary
  - consolidated governance read model
- Export center is a stable placeholder platform:
  - job lifecycle
  - event timeline
  - dispatch / attempt seams
  - admission reasons
  - planning-only execution / storage / delivery boundaries
- Integration center is a stable placeholder execution seam:
  - connector catalog
  - call logs
  - execution records
  - retry / replay actions and summaries
- Task asset storage/upload has a stable narrow placeholder seam:
  - upload request create / get / advance
  - task-asset binding to storage refs
  - shared adapter / resource / handoff language
- Cross-center adapter boundary language is stable across export / integration / upload.
- Auth / org / visibility and KPI / finance / report both have stable scaffolding layers.

### Placeholder skeletons
- Auth / org / visibility still stops at route-role checks over debug actor headers:
  - no login/session
  - no org tree
  - no row-level visibility trimming
  - no field-level enforcement
- Integration center is still not a real external execution platform:
  - no outbound executor
  - no callback processor
  - no external auth/signature negotiation
  - no delivery guarantee / idempotency protocol
- Export center is still not a real runner/storage platform:
  - no real runner
  - no file generation
  - no byte delivery
  - no NAS / object storage
- Task asset upload/storage is still not a real file platform:
  - no upload session allocator
  - no signed URL
  - no object storage / NAS adapter
  - no real byte confirmation
- Cost approval / finance remain placeholder-only:
  - no approval chain
  - no finance posting
  - no ERP writeback
- KPI / finance / report remain boundary-only:
  - no KPI runtime
  - no finance runtime
  - no report generation runtime

## Step 51 to Step 55 Candidate Phases

### 1. Step 51 Candidate
- Phase Name: Upload Request Management Query Hardening
- Goal:
  - add internal paginated list/filter visibility for upload requests
  - make upload/storage placeholder management inspectable without touching real upload infrastructure
- Why now:
  - smallest safe runtime deepening left after Step 50
  - closes an obvious management gap in the current upload/storage seam

### 2. Step 52 Candidate
- Phase Name: Integration Execution Boundary Hardening
- Goal:
  - deepen execution-side troubleshooting / callback-prep / action visibility without introducing real outbound execution
- Why next:
  - integration already has call-log and execution layering, but the next step is closer to real external-system topology choices

### 3. Step 53 Candidate
- Phase Name: Export Runner / Storage Boundary Hardening
- Goal:
  - deepen export planning boundary around runner/storage/result handling without introducing a real async runner or storage platform
- Why later:
  - useful, but much closer to future scheduler / storage architecture decisions than Step 51

### 4. Step 54 Candidate
- Phase Name: Policy Runtime Narrowing
- Goal:
  - move from pure policy summaries to very limited runtime visibility/action narrowing over existing placeholder auth
- Why later:
  - requires product decisions on org ownership, visibility scope, and acceptable placeholder enforcement semantics

### 5. Step 55 Candidate
- Phase Name: Cost Approval / Finance / Report Runtime Entry Alignment
- Goal:
  - deepen finance/report-facing runtime seams above current boundary summaries without entering real approval, finance, or BI systems
- Why last:
  - broad cross-domain semantics and highest risk of overcommitting future finance/report architecture

## Recommended Priority
1. Step 51: Upload Request Management Query Hardening
2. Step 52: Integration Execution Boundary Hardening
3. Step 53: Export Runner / Storage Boundary Hardening
4. Step 54: Policy Runtime Narrowing
5. Step 55: Cost Approval / Finance / Report Runtime Entry Alignment

## Auto-Advance vs Confirmation

### Safe to continue automatically
- Step 51 only
  - reason: schema-preserving, boundary-safe, and clearly inside the current upload/storage placeholder seam

### Must stop for user confirmation before entering
- Step 52
  - reason: easily drifts into real executor / callback / signature / delivery semantics
- Step 53
  - reason: touches async-runner and storage topology decisions
- Step 54
  - reason: changes product-visible auth / org / visibility behavior
- Step 55
  - reason: commits finance/report semantics too early

## High-Risk Infrastructure Zone
- Any phase that introduces real upload bytes, signed URLs, NAS, object storage, async runners, export file generation, ERP callbacks, approval chains, or finance posting is outside the safe automatic zone.
- Step 52 and Step 53 are the nearest edges of that zone.
- Step 55 is high-risk because it can silently commit the repository to long-lived finance/report contracts.

## Selected Execution Phase For This Round
- Execute exactly one bounded phase after this audit:
  - Step 51: Upload Request Management Query Hardening

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
- no real upload bytes or multipart pipeline
- no signed URLs
- no NAS / object storage
- no file-delivery system
- no export/integration runner work
- no auth/org runtime redesign
- no approval / finance runtime integration

## Selected Phase Success Criteria
- Internal placeholder upload requests can be listed with pagination.
- List filters cover owner boundary, task-asset type, and lifecycle status.
- Existing create/get/advance/bind semantics stay unchanged.
- No second execution phase starts in this round.
