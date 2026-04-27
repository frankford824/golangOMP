# V1 → V2 Backend Model Handoff Manifest

> 用途:为接管 V1 后端的下一代模型、前端工程师、V2 重构架构师提供权威指引。
> 范围:V1.0 backend closed state through R6.A.4, V1.1-A1 detail P99 remediation, Release v1.21 production evidence, and V1.1-A2 contract drift purge dated 2026-04-27.
> 风格:对齐 `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`: authority / current state / reading order / non-authoritative / working rule.

## §1 Authority Order

1. `docs/V1_MODULE_ARCHITECTURE.md` v1.3.
2. `docs/V1_INFORMATION_ARCHITECTURE.md`.
3. `docs/V1_CUSTOMIZATION_WORKFLOW.md`.
4. `docs/V1_ASSET_OWNERSHIP.md`.
5. `docs/api/openapi.yaml` with post V1.2 path-closure GC sha `80730ec3d272e4124ab95244feb0c1daf499d4c0a032f47b70179cdd4189488f`.
6. `transport/http.go`, the actual mounted route source.

Conflict rule:

1. `transport/http.go` decides what is mounted.
2. `docs/api/openapi.yaml` decides current HTTP contract.
3. The 4 V1 authority docs decide architecture semantics.
4. `docs/iterations/V1_RETRO_REPORT.md` is evidence, not primary spec.

## §2 Current Repo Baseline

- Current state: `V1_2_AUTHORITY_PURGED_AND_GUARD_LIVE · 待架构师 verify`. Main backend line remains Release v1.21 on production `jst_erp`.
- R1~R6.A.4 are architect-cleared.
- V1.1-A1 remediated `/v1/tasks/{id}/detail` P99 and is architect-verified.
- Release v1.21 deployed V1 backend to `jst_ecs:/root/ecommerce_ai/releases/v1.21`.
- OpenAPI path grep count after V1.1-A2: 203 `/v1` entries; contract schemas are post drift-purge and aligned to current git HEAD implementation.
- Service directory count in this round: 21 directories.
- `transport/http.go` route/group grep count in this round: 140 route-related lines.

Stable API surface:

- `/v1/auth/*`
- `/v1/users*`
- `/v1/me*`
- `/v1/erp/products*`
- `/v1/erp/products/by-code`
- `/v1/tasks*`
- `/v1/tasks/{id}/asset-center/*`
- `/v1/tasks/batch-create/*`
- `/v1/me/notifications*`
- `/v1/task-drafts*`
- `/ws/v1` WebSocket path in current OpenAPI; frontend docs keep the `/v1/ws/v1` prompt mismatch visible.
- `/v1/org-move-requests*`
- `/v1/users/{id}/activate`
- `/v1/users/{id}/deactivate`
- `/v1/users/{id}/delete`
- `/v1/search`
- `/v1/reports/l1/*`

Deprecated compatibility surface:

- `/v1/tasks/{id}/audit_a_claim`
- `/v1/tasks/{id}/audit_b_claim`
- `/v1/tasks/{id}/asset-center/assets`
- `/v1/tasks/reference-upload`
- Compatibility paths marked by `withCompatibilityRoute` and `withDeprecatedRoute` in `transport/http.go`.
- Do not remove without ADR / R7+ prompt.

Cron gates:

- `ENABLE_CRON_OSS_365=1` uses `0 4 * * *`.
- `ENABLE_CRON_DRAFTS_7D=1` uses `0 3 * * *`.
- `ENABLE_CRON_AUTO_ARCHIVE=1` uses `0 5 * * *`.
- All are default off.
- Cron must not add startup side effects when ENV is absent.

## §3 Reading Order

1. `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`.
2. This document.
3. `docs/V1_MODULE_ARCHITECTURE.md` v1.3, especially §12, §13, §15, §17.
4. `docs/V1_INFORMATION_ARCHITECTURE.md`, especially Excel and task detail sections.
5. `docs/V1_CUSTOMIZATION_WORKFLOW.md`.
6. `docs/V1_ASSET_OWNERSHIP.md`.
7. `docs/api/openapi.yaml`.
8. `transport/http.go`.
9. `docs/iterations/V1_RETRO_REPORT.md`.
10. `docs/iterations/V1_1_A1_DETAIL_P99_REPORT.md`.
11. `docs/iterations/V1_R6_A_1_REPORT.md`.
12. `docs/iterations/V1_R6_A_2_REPORT.md`.
13. `docs/iterations/V1_R6_A_3_REPORT.md`.

## §4 Core Contracts To Inherit

Module model:

- `basic_info`
- `customization`
- `design`
- `audit`
- `warehouse`
- derived `closed` / completed task state.

Authorization model:

- Layer 1: logged-in role route gate.
- Layer 2: module scope.
- Layer 3: module action role and state gate.
- Deny codes include `module_state_mismatch` and `module_action_role_denied`.

Lifecycle model:

- OSS cleanup: 365 days.
- Draft cleanup: 7 days.
- Task auto-archive: Completed/Cancelled after 90 days.
- Auto-archive has no events or notifications by design.

Notification model:

- `task_assigned_to_me`
- `task_rejected`
- `claim_conflict`
- `pool_reassigned`
- `task_cancelled`

Report model:

- L1 reports are SuperAdmin-only.
- V1 report implementation reads MySQL directly.
- SA-D corrected dwell logic via `task_module_events JOIN task_modules JOIN tasks`.

Excel model:

- Template route supports `new_product_development` and `purchase_task`.
- `original_product_development` is rejected for batch with `batch_not_supported_for_task_type`.
- Parse route is preview-only and writes no business rows.

## §5 Verification Baseline

R6.A.4 v1.2 evidence:

- build PASS.
- vet PASS.
- unit `go test ./...` PASS.
- integration `go test -tags=integration -p 1 ./...` PASS.
- OpenAPI validate `0 error 0 warning`.
- dangling path-level `501` count 0.
- A matrix `TestRetro_A*` PASS.
- live smoke F1~F6 PASS.
- control probe 4/4 zero.
- final `[70000,80000)` isolation AFTER zero.

V1.1-A1 evidence:

- build PASS.
- vet PASS.
- unit `go test ./...` PASS.
- integration `go test -tags=integration -p 1 ./...` PASS.
- OpenAPI validate `0 error 0 warning`.
- A matrix `TestRetro_A*` PASS.
- final `[80000,90000)` isolation AFTER zero.

Performance baseline:

- `/v1/search` p95 96.77ms from R4-SA-D.
- task auto-archive 1.90ms/task from R6.A.3.
- `/v1/tasks/{id}/detail` R6.A.4 p99 334.721ms before remediation.
- `/v1/tasks/{id}/detail` V1.1-A1 cold p99 47.525ms, warm p99 47.513ms, final warm n=500 p99 47.126ms.
- Detail P99 is GREEN after V1.1-A1.

## §6 Known Debt And Mandatory Follow-Up

V1.1 completed:

- Detail P99 remediation via detail bundle multi-result fast path, session actor bundle fast path, async best-effort success `route_access` logging, and old-path fallback when multi-result execution is unavailable.
- View/materialization was evaluated and rejected for V1.1-A1 to avoid write amplification.
- Re-run P99 after optimization: cold p99 47.525ms, warm p99 47.513ms.

V1.1 contract debt:

- V1.1-A2 已收口 OpenAPI/frontend contract drift;详见 `docs/iterations/V1_1_A2_RETRO_REPORT.md`.

V1.1 remaining mandatory:

- CI script / workflow guard that always uses `-p 1` for shared-DB integration packages.
- Test stability sweep for timeout buffer and `t.Cleanup` DB close order.
- V1.2 rebuild/deploy v1.22 to align production binary with git HEAD after the documented `identity_service.go` micro-drift baseline decision.

V1.1 optional:

- §9.3 compatibility route removal.
- Additional route-level latency smoke.

## §6.1 Release v1.21 · 2026-04-25

- Current online version: **v1.21**; replaces v1.20 old V0.9 backend.
- Deploy directory: `jst_ecs:/root/ecommerce_ai/releases/v1.21`.
- Artifact sha256: `977da0e4561a6baf841f89fca1c2cd0cb1c14b93bb97d981ee72632488a513bc`.
- Production DB: `jst_erp`.
- Production detail P99: warm `32.933ms` (gate < 80ms), cold `32.995ms` (gate < 150ms).
- Production pre-release backup: `/root/ecommerce_ai/backups/20260425T102852Z_pre_v1_21_jst_erp.sql.gz`.
- Test DB `jst_erp_r3_test` was backed up then dropped; backup: `/root/ecommerce_ai/backups/20260425T103533Z_pre_drop_jst_erp_r3_test.sql.gz`.
- Frontend integration docs: `docs/frontend/INDEX.md` plus 15 companion docs; 203 `/v1` paths covered, with `/ws/v1` WebSocket noted separately.

V2 candidates:

- U1 timeout escalation.
- U2 L2/L3 reports.
- U3 external blueprint store.
- U4 actor_org_snapshot freeze model.
- U5 cancellation expansion.
- U6 SLA automation.
- A12 durable consumer outlet.

## §6.2 V1.1-A2 Contract Drift Purge · 2026-04-27

- Status: `V1_1_A2_CONTRACT_DRIFT_PURGED · 待架构师 verify`.
- OpenAPI sha after purge: `0ff87aa90a53963a64350f92bf8bdce821dad3c24538bf70d61283b8dd97e5c3`.
- Inventory coverage: 203/203 `/v1` paths.
- P0/P1 fixed: 6 path-level schema drift items.
- `GET /v1/tasks/{id}/detail` now documents V1.1-A1 5-section fast path: `task`, `task_detail`, `modules`, `events`, `reference_file_refs`.
- Frontend docs under `docs/frontend/` were updated with V1.1-A2 revision marker; `V1_API_TASKS.md` contains the canonical detail handoff section.

## §7 Data Isolation Rule

Never run integration suites against production.

`jst_erp_r3_test` was dropped after Release v1.21. Recreate it with `scripts/r35/setup_test_db.sh` before running DB-backed integration suites again.

Segment ownership:

- SA-A: `[20000,30000)`.
- SA-B: `[30000,40000)`.
- SA-C: `[40000,50000)`.
- SA-D/R5/R6.A.1: `[50000,60000)`.
- R6.A.3: `[60000,70000)`.
- R6.A.4: `[70000,80000)`.
- V1.1-A1: `[80000,90000)`.
- Next backend verification round should use `[90000,100000)` or explicitly clean and reserve `[80000,90000)` before reuse.

Full-stack integration must be serial:

```bash
set -o pipefail
go test -tags=integration -p 1 -count=1 -timeout 45m ./...
```

## §8 Non-Authoritative Materials

Do not treat these as current spec:

- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/archive/*`
- old model-memory files
- early V0.9 specs that conflict with V1 OpenAPI
- `docs/iterations/*` except as evidence

`docs/iterations/V1_RETRO_REPORT.md` is useful evidence for R6.A.4 but not a replacement for the authority order above.

## §9 SHA Anchors

OpenAPI:

```text
b3d7c3651ea2496a6e4ea1a948772c6a395d6b387bf6c4509e5c26477c75dd0f  docs/api/openapi.yaml
```

Business anchors:

```text
5f4c9a10227e8321c4a87c8260b2bc0078adbb2dfb9fa0ebd2bd86601f46bae8  service/asset_lifecycle/cleanup_job.go
60103b15fa877a8d14b719dbd9f2aa82ee957271e8e8dea79a42106a8f346a1c  service/task_draft/service.go
32cd0201bf205bc2abfb6a9f489202de4bd099e188349184bd55a4ae1e22454b  service/task_lifecycle/auto_archive_job.go
f9d09d1fbc55734b00ff1f6c35cc1bccbf9db05298283eff6f255971262638c2  repo/mysql/task_auto_archive_repo.go
658a8cdf65c09335ab74176efb4057eff68440537e50ce0d9e550c57413e6e6b  domain/task.go
0bf70496a21c995d230efbcfaee4499257f1e3e46506e206a0ec6f51a73b6881  cmd/server/main.go
```

## §10 Working Rule

Any V1 contract change must start with ADR or R7+ prompt.

Do not silently reinterpret enums.

Do not add cron side effects without ENV gate.

Do not use default package parallelism for shared test DB integration.

Do not use `defer db.Close()` when prompt requires cleanup assertions in `t.Cleanup`; close DB at the callback end.

Do not regress detail P99; frontend rollout may use `/v1/tasks/{id}/detail` as the single task-detail screen fetch after V1.1-A1.


## §V1.2 Authority Purge

- Current status: `V1_2_AUTHORITY_PURGED_AND_GUARD_LIVE · 待架构师 verify`.
- V1 SoT: `docs/V1_BACKEND_SOURCE_OF_TRUTH.md`.
- OpenAPI unreachable schema closure: 0. Deprecated paths: 29 decided with `x-removed-at: v1.3`.
- Contract guard: `tools/contract_audit/` + `scripts/contract-guard.*` + `.cursor/hooks/contract-guard.json`.
