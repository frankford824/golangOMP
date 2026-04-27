# Archive Rules

Files in this directory are historical evidence only.

Do not treat them as current API, field, route-classification, or handoff authority unless the same statement is restated in:

1. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
2. `docs/api/openapi.yaml`
3. `transport/http.go`

For model handoff, start with `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`, not with archive files.

## Archive Layout

- `docs/archive/CURRENT_STATE_2026-04-09_ARCHIVE.md`
  - full historical current-state body; replaced by root `CURRENT_STATE.md` index
- `docs/archive/MODEL_HANDOVER_2026-04-09_ARCHIVE.md`
  - full historical handover body; replaced by root `MODEL_HANDOVER.md` index
- `docs/archive/legacy_specs/*`
  - historical formal-looking specs and PRD files removed from the repository root because they can be mistaken for the current contract
- `docs/archive/model_memory/*`
  - historical model memory files kept only for forensic context
- `docs/archive/*_ARCHIVE.md`
  - purified snapshots of older guides and indexes

## Root Files Demoted Or Moved On 2026-04-09

- moved to `docs/archive/legacy_specs/`
  - `设计流转自动化管理系统_V7.0_重构版_技术实施规格_2026-04-09_ARCHIVE.md`
  - `设计流转自动化管理系统_PRD_V2.0_重构版_2026-04-09_ARCHIVE.md`
- moved to `docs/archive/model_memory/`
  - `MODEL_v0.4_memory_2026-04-09_ARCHIVE.md`

These files may still contain useful background, but they are not valid sources for new integration or new implementation decisions.

## Root Files Demoted Or Moved On 2026-04-13

- moved to `docs/archive/`
  - `V7_MODEL_HANDOVER_APPENDIX.md`
  - `ARCHITECTURE_FORENSIC_AUDIT_REPORT.md`
  - `DOCUMENT_CORRECTION_AUDIT_ITERATION_083.md`
  - `NEXT_PHASE_ROADMAP.md`
  - `V0_4_MAIN_BRIDGE_CONVERGENCE_PLAN.md`
  - `V0_4_COMPATIBILITY_RETIREMENT_CHECKLIST.md`
  - `MAIN_BRIDGE_RESPONSIBILITY_MATRIX.md`
  - `ERP_REAL_LINK_VERIFICATION.md`
  - `LIVE_TRUTH_SOURCE_VERIFICATION.md`
  - `V7_API_READY.md`
  - `V0_4_RELEASE_SUMMARY.json`
- moved to `docs/archive/obsolete_alignment/`
  - `FRONTEND_ALIGNMENT_v0.5.md`

## New Subfolder: obsolete_alignment

- `docs/archive/obsolete_alignment/*` stores deprecated frontend alignment notes that were historically useful but are unsafe as current integration specs.
- Allowed use:
  - historical comparison
  - migration forensics
- Not allowed use:
  - new frontend rollout baseline
  - current API contract reference
  - route/field authority
