# PHASE_AUTO_039 - Export Runner / Storage Boundary Planning Hardening

## Why This Phase Now
- Step 38 completed integration-center execution boundary hardening and left export center as the most obvious remaining platform seam.
- Export center already has lifecycle, event trace, start, dispatch, attempt, and placeholder download handoff, but the future replacement layers for runner, storage, and delivery are still too implicit.
- Before any real runner, file generation, NAS, object storage, or download service is introduced, the current placeholder stack needs a stable planning contract that keeps lifecycle, attempt, dispatch, storage, and delivery fully separated.

## Current Context
- Current state: `V7 Migration Step 38 complete`
- Current OpenAPI version before this phase: `0.34.0`
- Latest iteration: `docs/iterations/ITERATION_038.md`
- Stable export-center skeletons already present:
  - export job persistence
  - lifecycle status and audit trace
  - placeholder result handoff through `result_ref`
  - claim / read / refresh handoff APIs
  - explicit start boundary
  - dispatch and attempt persistence plus summaries
- Current main gap:
  - export runner / storage / delivery boundary planning hardening

## Goals
- Add a stable planning-only read model that makes execution, storage, and delivery boundaries explicit on export-job list/detail responses.
- Clarify which current layer owns:
  - start execution
  - dispatch handoff
  - one concrete attempt
  - placeholder result generation
  - placeholder storage representation
  - placeholder download delivery
- Keep all semantics placeholder-only and avoid any real infrastructure hookup.

## Allowed Scope
- additive export-center domain/read-model hardening only
- additive service-layer boundary hydration and invalid-state detail hardening
- additive export-center tests
- additive OpenAPI/state/iteration/handover synchronization
- create `docs/phases/PHASE_AUTO_039.md`

## Forbidden Scope
- real async runner or scheduler platform
- real file generation
- real file-byte download
- signed URL delivery
- NAS / object-storage integration
- new BI / KPI / finance export domains
- broad integration-center or cost-rule expansion

## Expected File Changes
- update export-center domain/read-model structures
- update export-center service helper details and tests
- add `docs/phases/PHASE_AUTO_039.md`
- add `docs/iterations/ITERATION_039.md`
- update `docs/api/openapi.yaml`
- update `CURRENT_STATE.md`
- update `MODEL_HANDOVER.md`
- update `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## Required API / DB Changes
- DB:
  - no DB migration in this phase
  - no new tables required; this is read-model/schema hardening only
- API:
  - no new endpoints required
  - extend export-job list/detail schema additively with planning-only fields:
    - `adapter_mode`
    - `storage_mode`
    - `delivery_mode`
    - `execution_boundary`
    - `storage_boundary`
    - `delivery_boundary`

## Success Criteria
- export-job list/detail responses clearly state which current layer owns start, dispatch, attempt, result generation, storage representation, and delivery handoff
- placeholder boundary fields are additive and do not regress lifecycle / dispatch / attempt / result_ref / claim-read-refresh contracts
- OpenAPI and docs clearly say these new fields are planning/hardening skeletons, not proof of real runner/storage/download infrastructure
- tests pass without adding real infrastructure

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_039.md`
- `docs/api/openapi.yaml`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## Risks
- new boundary fields must not be mistaken for real infrastructure readiness
- boundary wording must stay consistent with existing dispatch/attempt/result_ref semantics so the layering is clearer rather than more confusing

## Completion Output Format
1. changed files
2. DB / migration changes
3. API changes
4. correction notes
5. risks / remaining gaps
6. next recommended single phase
