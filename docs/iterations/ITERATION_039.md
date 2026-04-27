# ITERATION_039 - Export Runner / Storage Boundary Planning Hardening

**Date**: 2026-03-10  
**Scope**: `docs/phases/PHASE_AUTO_039.md`

## 1. Goals
- Execute exactly one new phase after Step 38:
  - export runner / storage boundary planning hardening
- Keep existing export lifecycle, attempt, dispatch, `result_ref`, audit trace, and claim/read/refresh semantics additive and non-regressive.
- Avoid real runner, file generation, storage, signed URL, NAS, object storage, and download infrastructure while making the future replacement seams explicit.

## 2. Inputs
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_038.md`
- `docs/phases/PHASE_AUTO_039.md`
- latest PRD / V7 implementation spec
- `AGENT_PROTOCOL.md`
- `AUTO_PHASE_PROTOCOL.md`

## 3. Files Changed
- `docs/phases/PHASE_AUTO_039.md`
- `domain/export_center.go`
- `service/export_center_service.go`
- `service/export_center_service_test.go`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_039.md`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## 4. DB / Migration Changes
- No DB migration in this iteration.
- No new tables were introduced.
- Step 39 is read-model / schema hardening only:
  - export jobs already had sufficient persistence
  - the missing seam was explicit runner/storage/delivery planning expression

## 5. API Changes
- No new endpoints were added.
- Export-job list/detail schema was extended additively with planning-only boundary fields:
  - `adapter_mode`
  - `storage_mode`
  - `delivery_mode`
  - `execution_boundary`
  - `storage_boundary`
  - `delivery_boundary`
- OpenAPI version advanced from `0.34.0` to `0.35.0`.

## 6. Design Decisions
- Kept `export_job` as the business object and primary frontend/admin read model.
- Did not add new persistence because the current gap was not missing state storage; it was unclear boundary semantics.
- Chose additive planning fields over new internal-only endpoints so frontend and later backend work can read the same boundary contract directly from list/detail responses.
- Kept the existing layered split intact:
  - lifecycle = business-visible export job state
  - dispatch = adapter handoff state
  - attempt = one concrete execution try
  - `result_ref` = placeholder storage representation
  - claim/read/refresh = placeholder delivery handoff

## 7. Layering Clarification
- Start execution:
  - owned by `POST /v1/export-jobs/{id}/start`
  - backward-compatible `advance action=start` still reuses that same boundary
- Dispatch handoff:
  - owned by `export_job_dispatches`
  - answers whether a placeholder runner-adapter handoff was submitted/received/resolved
- Execution attempt:
  - owned by `export_job_attempts`
  - answers what happened in one concrete placeholder execution try
- Result generation:
  - currently still represented by export-job lifecycle advance into ready/failed/cancelled
  - this is placeholder result-state minting, not real file generation
- Storage:
  - currently represented by structured `result_ref` metadata only
  - no real file bytes, NAS path, signed URL, or object-storage object exists yet
- Delivery:
  - currently represented by claim/read/refresh handoff routes over `result_ref`
  - no real download service exists yet
- Future replacement path:
  - real runner attaches beneath execution boundary
  - real storage attaches behind `result_ref`
  - real delivery attaches behind claim/read/refresh handoff routes

## 8. Correction Notes
- Before this round, export center docs already said real runner/storage/download were deferred, but the code and OpenAPI did not expose one explicit planning contract for those seams on export-job list/detail.
- This iteration corrected that gap without changing persistence or lifecycle meaning.

## 9. Verification
- `gofmt -w domain/export_center.go service/export_center_service.go service/export_center_service_test.go`
- `go test ./service -run ExportCenter`
- `go test ./...`

## 10. Risks / Known Gaps
- No real runner exists yet.
- No real file generation exists yet.
- No real storage integration exists yet.
- No real download service exists yet.
- `result_ref` still represents placeholder metadata only.
- Result generation is still described through lifecycle transition rather than a dedicated generated-artifact table or runner callback contract.

## 11. Next Batch Recommended Roadmap
1. Step 40: cost-rule governance / versioning / override hardening
2. Keep export real runner / storage / delivery infrastructure deferred until these planning boundaries stay stable
