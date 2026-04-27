# PHASE_AUTO_045

## Why This Phase Now
- Export center, integration center, and asset storage/upload now all expose adapter-like placeholder boundaries.
- If those three lines continue deepening independently, future real runner/storage/external execution work will duplicate terms and drift away from one coherent handoff/reference model.
- The latest PRD-aligned need is therefore cross-center boundary-language consolidation, not deeper single-center infrastructure work.

## Current Context
- Current CURRENT_STATE before this phase: Step 44 complete
- Current OpenAPI version before this phase: `0.40.0`
- Latest iteration before this phase: `docs/iterations/ITERATION_044.md`
- Current completed mainline:
  - export lifecycle / attempt / dispatch / claim-download / refresh
  - integration connector catalog / call logs / execution boundary
  - task asset upload requests / storage refs / placeholder storage adapter
- Current main gap:
  - adapter / execution / dispatch / storage / delivery terms are not yet unified across centers
  - `result_ref`, `storage_ref`, upload requests, dispatches, and executions do not yet share one minimal cross-center summary language

## Goals
- Consolidate cross-center adapter-boundary terminology across export, integration, and storage/upload
- Make the boundary split explicit for:
  - `adapter_mode`
  - `execution_mode`
  - `dispatch_mode`
  - `storage_mode`
  - `delivery_mode`
- Add shared minimal summaries:
  - `adapter_ref_summary`
  - `resource_ref_summary`
  - `handoff_ref_summary`
- Reuse those summaries in domain read models and OpenAPI without forcing table merges

## Allowed Scope
- Domain shared summary structs and derived read-model hydration
- Additive export / integration / storage-upload read-model fields
- OpenAPI / CURRENT_STATE / iteration / handover sync
- One new phase-plan document and one new iteration document

## Forbidden Scope
- Real runner / scheduler / async platform
- Real storage / NAS / object storage / signed download / real upload
- Real external ERP / HTTP / SDK execution
- Real approval workflow, finance system, ERP writeback, or table merge across centers
- Large DB refactors or cross-center persistence consolidation

## Expected File Changes
- Add shared cross-center boundary summary structs in domain
- Update export / integration / storage-upload derived read models
- Update focused service tests
- Update `docs/api/openapi.yaml`
- Update `CURRENT_STATE.md`, `MODEL_HANDOVER.md`, `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- Add `docs/iterations/ITERATION_045.md`

## Required API / DB Changes
- No new DB tables or migrations
- Add additive OpenAPI/read-model fields only:
  - export jobs: `dispatch_mode`, shared summary fields
  - integration call logs / executions: `adapter_mode`, `dispatch_mode`, shared summary fields
  - upload requests / asset storage refs: shared mode fields and shared summary fields

## Success Criteria
- Go code compiles and focused tests pass
- Export / integration / storage-upload read models use one consistent terminology layer without losing center-specific semantics
- OpenAPI explicitly states that Step 45 is consolidation only, not real infrastructure integration
- CURRENT_STATE, iteration memory, and handover docs all advance to Step 45 / `0.41.0`

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_045.md`
- `docs/api/openapi.yaml`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## Risks
- The new shared summary vocabulary may need extension once real adapters arrive beneath export/integration/storage boundaries
- Some concepts remain intentionally center-specific:
  - export `delivery_mode`
  - integration `execution_mode`
  - storage/upload binding semantics

## Completion Output Format
1. Changed files
2. DB / migration changes
3. API changes
4. Correction notes
5. Risks / known gaps
6. Suggested next step
