# V1.4 Governance Phase 1 Report

Date: 2026-04-29
Scope: `prompts/V1_4_GOVERNANCE.md` §A, A1 through A7.

## A1 Known Gap Reasons

A1 found 65 `known_gap` path rows and no K3 drift promotions. The audit
tool now emits `paths[].reason` and stores the current known-gap allowlist in
code by `method + path`. Current reasons are split into legitimate reserved or
binary responses and tool dereference limits for inline routes, delegated
handlers, and dynamic documented payloads. Any future known gap that is not
explicitly registered is promoted to drift instead of silently expanding the
grey zone.

## A2 SoT Index

A2 rewrote `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` as an authority index instead
of a partial state report. The file now points to the four V1 substance
authority files and keeps route existence, field contracts, frontend docs, and
contract audit locations separate.

## A3 Hard Boundaries SoT List

A3 classified all current `docs/V1_*.md` files. The SoT set is the V1 backend
index plus `V1_MODULE_ARCHITECTURE.md`, `V1_INFORMATION_ARCHITECTURE.md`,
`V1_ASSET_OWNERSHIP.md`, and `V1_CUSTOMIZATION_WORKFLOW.md`.
`V1_TO_V2_MODEL_HANDOFF_MANIFEST.md` is handoff evidence, not current V1
contract authority. `AGENTS.md` Hard Boundaries now lists the real SoT set and
removes the stale `V1_ASSET_GOVERNANCE.md` name.

## A4 V0.9 Archive

A4 moved `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md` to
`docs/archive/legacy_specs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`. `AGENTS.md` and
the V1 backend index now refer to the archived path when historical background
is needed.

## A5 Ambiguous Docs Consolidation

A5 read `API_DOC_SYNC_RULES.md`, `docs/RELEASE_NOTES.md`, and
`docs/ENGINEERING_RULES.md`. Current, non-stale rules were folded into
`AGENTS.md`: API contract-change triggers, referenced OpenAPI schemas,
smallest-useful-change guidance, compatibility-surface discipline, and no
release/deploy without explicit instruction. The three ambiguous files were
deleted.

## A6 Measured Changed Files

A6 added the measured changed-files response rule to `AGENTS.md` and
`prompts/CODEX_SESSION_BOOTSTRAP.md`. Future end-of-turn reports must derive
Changed files from `git status --short --untracked-files=all` and
`git diff --stat HEAD`, and must explicitly label inherited dirty files.

## A7 Audit Side Effect

A7 removed the `tools/contract_audit` default output paths under
`docs/iterations/`. The tool now writes files only when `--output` or
`--markdown` is explicitly provided; otherwise it prints JSON to stdout. A
regression test runs the tool without `--markdown` and asserts
`docs/iterations/` is unchanged.

## Final Audit Summary

- `total_paths`: 234
- `clean`: 169
- `drift`: 0
- `unmapped`: 0
- `known_gap`: 65
- `known_gap_with_reason`: 65
- `known_gap_no_reason`: 0
- `missing_in_openapi`: 0
- `missing_in_code`: 0

## P1 Oversized Files

- `service/identity_service.go`: 109935 bytes
- `service/task_service.go`: 103265 bytes
- `service/export_center_service.go`: 82942 bytes
- `repo/mysql/task.go`: 68388 bytes
- `transport/http.go`: 69605 bytes, already split once; continue monitoring
  per §B if any sub-area grows beyond the threshold.

## Boundary Attestation

No `db/migrations/**` files were changed. No business code was changed. No
OpenAPI contract substance was changed. V1 SoT substance files were not edited;
only the V1 backend index and AGENTS governance text were updated under the
explicit V1.4 phase 1 authorization.
