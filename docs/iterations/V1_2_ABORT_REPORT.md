# V1.2 · ABORT · P2 OpenAPI dead schema GC

- date: 2026-04-27T04:35:06Z
- stage: P2
- trigger: Phase 2 verify conflict; continuing to the requested `components.schemas <= 190` target would require deleting live schemas and would trigger §1.3 (`Phase 2 删除 schema 时,任意一个待删 schema 在 OpenAPI 中实际出现 >= 2 次 $ref 引用`)
- evidence:
  - `sha256sum docs/api/openapi.yaml` = `0ff87aa90a53963a64350f92bf8bdce821dad3c24538bf70d61283b8dd97e5c3` at V1.2 step-0
  - `python3 tmp/v1_2_dead_schema_audit.py --openapi docs/api/openapi.yaml --json tmp/v1_2/dead_schema_audit_before.json`
  - audit result: `schema_count=313`, `reachable_from_paths=298`, `unreachable=15`
  - computed lower bound: `must_delete_live_to_reach_190=108`
  - live schemas with `$ref` count >= 2 include `APIReadiness`, `ActionPolicySummary`, `Actor`, `AdapterMode`, `AdapterRefSummary`, `Asset`, `AssetDownloadInfo`, `AssetOwnerType`, `AssetReasonRequest`, `AssetStorageAdapter`, `AssetStorageRef`, `AssetStorageRefType`, `AuditHandover`, `AuthResult`, `BoundaryStorageMode`
  - therefore the V1.2 prompt's stale expectation (`313 -> 150~190`) is incompatible with the actual V1.1-A2 OpenAPI reference graph unless live referenced schemas are deleted
- rollback action:
  - no OpenAPI schema/path deletion was applied to `docs/api/openapi.yaml`
  - no business Go file was modified
  - temporary evidence retained under `tmp/v1_2/`: `dead_schema_audit_before.json`, `dead_schema_apply.json`, `dead_schema_audit_after_tmp.json`, `openapi.gc.yaml`
  - Phase 1 governance artifacts are left in the working tree as handoff evidence: `docs/V1_BACKEND_SOURCE_OF_TRUTH.md`, `docs/iterations/V1_2_AUTHORITY_INVENTORY.md`, `docs/iterations/INDEX.md`, `prompts/INDEX.md`, plus archive moves and `CLAUDE.md` update
- next ask:
  - architect decision required: revise Phase 2 target to "delete only unreachable schemas" (`15` candidates, with handler/frontend reference exceptions called out), or authorize a separate schema inlining/contract-minification design that can reduce `components.schemas` without deleting live `$ref` targets

## Completed Before ABORT

- P0 baseline gate passed:
  - business SHA anchor set: `git status --short -- <10 files>` returned empty output
  - OpenAPI baseline sha matched `0ff87aa90a53963a64350f92bf8bdce821dad3c24538bf70d61283b8dd97e5c3`
  - `go vet ./...` PASS
  - `go build ./...` PASS
- P1 authority inventory completed:
  - `docs/iterations/V1_2_AUTHORITY_INVENTORY.md` created with 229 inventory rows
  - `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` created
  - `docs/iterations/INDEX.md` created
  - `prompts/INDEX.md` created
  - selected top-level legacy docs moved to `docs/archive/legacy_handoffs/`
  - selected old prompts moved to `prompts/archive_pre_v1_2/`
  - `CLAUDE.md` updated to V1 authority order

## Not Completed

- P2 OpenAPI GC was not applied to `docs/api/openapi.yaml`
- P3 route/path GC was not started
- P4 `tools/contract_audit/` was not created
- P5 CI guard was not created
- P6 frontend docs and final governance sync were not performed

