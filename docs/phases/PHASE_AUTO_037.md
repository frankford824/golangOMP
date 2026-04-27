# PHASE_AUTO_037 - Task Asset Storage / Upload Adapter Boundary Hardening

## Why This Phase Now
- Step 36 finished route-level placeholder auth enforcement and explicitly left task asset upload/storage as the next highest-impact PRD gap.
- Current `task_assets` still mix business asset timeline semantics with legacy-style `file_path` / `whole_hash` fields.
- The repository already has placeholder export `result_ref` semantics; task assets now need an equivalent storage/upload boundary so later NAS/object-storage work lands behind one explicit seam instead of inside business tables.

## Current Context
- Current state: `V7 Migration Step 36 complete`
- Current OpenAPI version: `0.32.1`
- Latest iteration: `docs/iterations/ITERATION_036.md`
- Stable skeletons already present:
  - task/workflow mainline
  - audit/handover/outsource/warehouse loop
  - board/workbench/filter convergence
  - category center + mapped product search + `product_selection`
  - export center placeholder platform
  - integration call-log skeleton
- Current main gap:
  - task asset storage / upload adapter boundary

## Goals
- Add a placeholder upload-request boundary for future file ingress.
- Add a reusable placeholder asset-storage reference model with adapter/type/key/metadata semantics.
- Keep existing `task_assets` timeline contract stable while making the migration direction explicit through additive fields.
- Clarify the model relationship between task-asset storage refs and export `result_ref` without forcing a premature merge.

## Allowed Scope
- `task_assets` additive schema evolution only
- new placeholder tables for upload requests and asset storage refs
- new domain / repo / service / handler code for upload-request boundary
- task-asset service / read-model updates to expose storage refs
- OpenAPI / state / iteration / handover synchronization

## Forbidden Scope
- real NAS / object storage integration
- real file upload transport
- large-file upload, chunking, resume, signed URL, strict whole-hash verification
- CDN / download service / byte delivery
- export real runner/storage implementation
- integration real executor / ERP deep integration
- cost-rule governance/versioning work

## Expected File Changes
- add one migration for upload-request and asset-storage-ref persistence plus additive task-asset linkage
- add storage/upload boundary domain models
- add repo and service implementations for:
  - `POST /v1/assets/upload-requests`
  - `GET /v1/assets/upload-requests/{id}`
- update task-asset persistence / service / handlers to bind task assets onto storage refs
- update OpenAPI, `CURRENT_STATE.md`, `ITERATION_037.md`, and handover notes if needed

## Required API / DB Changes
- DB:
  - add `upload_requests`
  - add `asset_storage_refs`
  - add additive linkage fields on `task_assets`
- API:
  - add internal placeholder upload-request create/get routes
  - extend task asset response/read models with storage-ref metadata
  - extend submit-design and mock-upload payloads with optional upload-request linkage and file metadata fields

## Success Criteria
- task assets can expose a stable placeholder `storage_ref` object without removing legacy fields
- upload intent can be represented independently through internal placeholder upload requests
- task-asset write flow can consume an upload request or auto-create a placeholder storage ref
- code, OpenAPI, `CURRENT_STATE`, and `ITERATION_037` are synchronized
- tests pass without introducing real storage or upload infrastructure

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_037.md`
- `docs/api/openapi.yaml`
- `MODEL_HANDOVER.md` if new boundary rules need handoff emphasis
- V7 appendix docs only if current readiness/internal markers drift

## Risks
- additive schema/read-model changes must not regress existing frontend task-asset timeline consumers
- placeholder upload requests must be clearly documented as non-upload endpoints to avoid being mistaken for a real file service
- export `result_ref` and task asset `storage_ref` should align semantically without accidental over-coupling in this phase

## Completion Output Format
1. changed files
2. DB / migration changes
3. API changes
4. correction notes
5. risks / remaining gaps
6. next recommended single phase
