# PHASE_AUTO_048

## Why This Phase Now
- Step 47 completed integration replay/retry admission hardening and left export dispatch/attempt admission as the clearest bounded next gap.
- Export center already has lifecycle, event trace, start boundary, dispatch/attempt persistence, and planning-only runner/storage/delivery boundaries, but admission semantics are still mostly implicit booleans.
- The smallest safe next step is to harden export admission rules and reasons without entering real scheduler/runner/storage/download infrastructure.

## Current Context
- Current CURRENT_STATE before this phase: Step 47 complete
- Current OpenAPI version before this phase: `0.43.0`
- Latest iteration before this phase: `docs/iterations/ITERATION_047.md`
- Current main remaining safe gap: export dispatch / attempt admission hardening

## Goals
- Make dispatch/start/attempt/redispatch admission rules explicit and machine-readable.
- Expose additive admission reason fields on export-job read models and error details.
- Keep backward-compatible auto-placeholder dispatch behavior for `/start`, but make its admission semantics explicit.

## Allowed Scope
- Additive export-center domain/read-model admission fields
- Additive export-center service admission validation and error-detail hardening
- Additive export dispatch/attempt derived admission summaries
- Focused export-center tests
- Required OpenAPI/state/iteration/handover synchronization

## Forbidden Scope
- Real async runner or scheduler platform
- Real file generation
- Real file-byte download
- Signed URL delivery
- NAS / object storage integration
- BI / KPI / finance export expansion
- Real ERP integration expansion

## Expected File Changes
- Update export-center domain admission derivation and read-model fields
- Update export-center service admission checks/details
- Update export dispatch/attempt hydration summaries
- Update focused export-center tests
- Add `docs/phases/PHASE_AUTO_048.md`
- Add `docs/iterations/ITERATION_048.md`
- Update `docs/api/openapi.yaml`
- Update `CURRENT_STATE.md`
- Update `MODEL_HANDOVER.md`
- Update `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## Required API / DB Changes
- DB:
  - no new tables or migrations in this phase
- API:
  - no new endpoints
  - additive export-job read-model fields for admission reasoning and latest decision summary
  - additive export dispatch/attempt list admission-hint fields
  - clearer internal/admin placeholder semantics on dispatch/start/attempt routes

## Success Criteria
- Export dispatch/start/attempt/redispatch admission is consistently represented as `allowed + reason` semantics.
- `/start` compatibility auto-placeholder dispatch behavior remains intact and explicitly documented.
- Export-job list/detail expose additive admission reason fields and latest admission decision summary without regressing existing lifecycle/dispatch/attempt/download contracts.
- OpenAPI and docs clearly describe this as admission hardening skeleton only, not a real scheduler/runner/storage platform.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_048.md`
- `docs/api/openapi.yaml`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## Risks
- Reason-code naming may need refinement when real scheduler/runner policy is introduced.
- Richer future policy inputs (priority, quota, tenancy) may require extending current admission summaries.

## Completion Output Format
1. Changed files
2. DB / migration changes
3. API changes
4. Correction notes
5. Risks / known gaps
6. Suggested next step
