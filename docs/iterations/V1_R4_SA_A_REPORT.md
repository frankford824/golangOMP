# V1 R4-SA-A Report

## Scope

Implemented the seven `x-owner-round: R4-SA-A` asset-management routes without changing OpenAPI or migrations:

| Route | Handler | Deny path | Status matrix |
| --- | --- | --- | --- |
| `GET /v1/assets/search` | `SearchGlobalAssets` | session/access middleware | `200`, `400`, `401`, `500` |
| `GET /v1/assets/{id}` | `GetGlobalAsset` | session/access middleware | `200`, `400`, `401`, `404`, `500` |
| `GET /v1/assets/{id}/download` | `DownloadGlobalAsset` | session/access middleware | `200`, `400`, `401`, `404`, `410`, `500` |
| `GET /v1/assets/{asset_id}/versions/{version_id}/download` | `DownloadGlobalAssetVersion` | session/access middleware | `200`, `400`, `401`, `404`, `410`, `500` |
| `POST /v1/assets/{asset_id}/archive` | `ArchiveGlobalAsset` | exact `SuperAdmin`; non-SuperAdmin returns `module_action_role_denied` | `202`, `400`, `401`, `403`, `404`, `409`, `500` |
| `POST /v1/assets/{asset_id}/restore` | `RestoreGlobalAsset` | exact `SuperAdmin`; non-SuperAdmin returns `module_action_role_denied` | `202`, `400`, `401`, `403`, `404`, `409`, `500` |
| `DELETE /v1/assets/{id}` | `DeleteGlobalAsset` | exact `SuperAdmin`; non-SuperAdmin returns `module_action_role_denied` | `204`, `400`, `401`, `403`, `404`, `409`, `500` |

New code paths:

- `domain/asset_lifecycle_state.go`, `domain/asset_search_query.go`
- `repo/task_asset_search_repo.go`, `repo/task_asset_lifecycle_repo.go`
- `repo/mysql/task_asset_search_repo.go`, `repo/mysql/task_asset_lifecycle_repo.go`
- `service/asset_center/*`
- `service/asset_lifecycle/*`, `service/asset_lifecycle/scheduler/*`

## 5-State Machine

`DeriveLifecycleState` is DB-field-only:

- `deleted_at != NULL` -> `deleted`
- `cleaned_at != NULL` -> `auto_cleaned`
- `is_archived = 1` -> `archived`
- task status in `Completed/Cancelled/Archived` -> `closed_retained`
- otherwise `active`

Unit result: `go test ./domain -count=1` passed. Table coverage includes deleted precedence, cleaned precedence, archived, each terminal task state, and active.

## 8 Integration Assertions

Integration command:

```bash
MYSQL_DSN='root:***@tcp(127.0.0.1:33306)/jst_erp_r3_test?parseTime=true&multiStatements=true' R35_MODE=1 /home/wsfwk/go/bin/go test ./... -tags=integration -count=1
```

Result: passed. SA-A focused verbose run also passed:

- `SA-A-I1`: search `is_archived=false` returned only `active/closed_retained`.
- `SA-A-I2`: search `is_archived=all` found self-created `archived` asset.
- `SA-A-I3`: non-SuperAdmin archive returned `module_action_role_denied`; SuperAdmin archive wrote `asset_archived_by_admin`.
- `SA-A-I4`: restore cleared `is_archived/archived_at` and wrote `asset_unarchived_by_admin`.
- `SA-A-I5`: delete set `deleted_at`, nulled `storage_key`, wrote `asset_deleted_by_admin`.
- `SA-A-I6`: `auto_cleaned` download returned 410 code path; `deleted` returned 404.
- `SA-A-I7`: cleaned version returned 410 while `task_module_events.payload.asset_versions_snapshot` remained readable.
- `SA-A-I8`: cleanup dry-run scanned candidates without writes; real run cleaned and emitted event; rerun cleaned 0.

## Cleanup Job

Implemented `CleanupJob.Run(ctx, CleanupOptions{DryRun, Limit})` with 365-day cutoff based on terminal task `updated_at`, idempotent `cleaned_at IS NULL/deleted_at IS NULL` filtering, `[ASSET-CLEANUP]` log prefix, OSS delete hook, DB update, and `asset_auto_cleaned` event payload including original storage key.

SA-A-I8 inserted one representative terminal task at `task_id=20009`, `updated_at=NOW()-400 DAY`. Results:

- dry-run: `Scanned > 0`, `Cleaned = 0`, storage still present.
- real run: `Cleaned > 0`, `cleaned_at IS NOT NULL`, `storage_key IS NULL`, event count 1.
- idempotent rerun: `Cleaned = 0`.

## Scheduler Posture

`config.AssetCleanup.Enabled` defaults to `false` via `ASSET_CLEANUP_ENABLED=false`.

Scheduler code only exposes `service/asset_lifecycle/scheduler.Register`; it does not register runtime cron. Grep evidence: no `AddFunc` or `cron.Schedule` references target the cleanup job.

## Test DB Touch

Integration used `testsupport/r35.MustOpenTestDB(t)` and DSN database `jst_erp_r3_test` through an SSH tunnel to local port `33306`; guard requires `_r3_test`.

SA-A test data used `task_id >= 20000`. Post-run cleanup evidence:

```text
tasks_ge_20000         0
task_assets_ge_20000   0
design_assets_ge_20000 0
```

## Production Probe Diff

Probe log: `docs/iterations/r4_sa_a_probe.log`.

Read-only lifecycle safety evidence:

```text
task_assets_is_archived 0 287
task_assets_lifecycle_dirty 0 0
```

This proves SA-A did not archive, clean, or delete production assets. However, row-count diff against R2 post-run baseline is not ±0:

| Table | R2 post | R4-SA-A probe | Diff |
| --- | ---: | ---: | ---: |
| `tasks` | 98 | 104 | +6 |
| `task_assets` | 265 | 287 | +22 |
| `asset_storage_refs` | 451 | 491 | +40 |

Per prompt failure condition, production row-count drift is a sign-off blocker even though lifecycle columns remain zero-written.

## OpenAPI Conformance

```bash
/home/wsfwk/go/bin/go run ./cmd/tools/openapi-validate docs/api/openapi.yaml
# openapi validate: 0 error 0 warning
```

## Validation Commands

```bash
/home/wsfwk/go/bin/go build ./...
# passed

/home/wsfwk/go/bin/go test ./... -count=1
# passed

MYSQL_DSN='root:***@tcp(127.0.0.1:33306)/jst_erp_r3_test?parseTime=true&multiStatements=true' R35_MODE=1 /home/wsfwk/go/bin/go test ./... -tags=integration -count=1
# passed

/home/wsfwk/go/bin/go run ./cmd/tools/openapi-validate docs/api/openapi.yaml
# openapi validate: 0 error 0 warning

bash /tmp/sa_a_probe.sh > docs/iterations/r4_sa_a_probe.log
# completed; row-count drift noted above
```

## Known Non-Goals

- Cleanup cron trigger remains disabled for R6.
- `/v1/assets/{id}/preview` remains untouched.
- Audit snapshot write-side remains owned by R3 action executor; SA-A only proves snapshots remain readable after cleanup.

---

## Architect Adjudication(主对话追加 · 2026-04-17)

> 签字结论:**SA-A 签字通过**。Codex 报告里提到的 "row-count drift sign-off blocker" 由架构师裁决不成立,原因如下:

**Drift 来源证据链**:

1. `task_assets_is_archived (0, 287)` · 287 条全部 `is_archived=0`,0 条非零 → SA-A archive handler 在生产零命中。
2. `task_assets_lifecycle_dirty 0` · `cleaned_at/deleted_at/archived_at` 非 NULL 条目数 = 0 → SA-A delete/cleanup/archive 在生产零写入。
3. `tasks +6 / task_assets +22 / asset_storage_refs +40` 增长符合生产 live traffic 常态模式:
   - `asset_type` 分布 `delivery 138→161 / source 47→47 / design_thumb 35→35 / preview 35→35 / reference 9→9` → 仅 `delivery` 有 + 23 的增长,与生产当下批量发货物料上传线匹配;
   - 与 R3.5 post-run 到 R4 开始之间的业务窗口一致。
4. SA-A 代码路径审计:7 个 handler + cleanup_job 无任一 `INSERT INTO task_assets / INSERT INTO asset_storage_refs / INSERT INTO tasks` 能力,不具备写入 + 22 / + 40 / + 6 行的机制。

**根因 · Prompt §9 规则粗放**:

v1 prompt §9 写 "生产 probe 显示 R2 目标表行数变化 → abort" 未区分 "SA-A 改生产" vs "生产自行增长",所以 Codex 对 ± 0 的字面要求正确 abort 是尽职表现。规则修订后放进 prompt v2.1,为 SA-B/C/D 定义 "控制字段零写入" 而非 "表行数 ± 0"。

**结论**:SA-A 7 handler + 5 态机 + 清理 job 骨架 + 8 集成断言 + 0 生产写入,全部通过。`docs/iterations/V1_R4_SA_A_REPORT.md` 升 **v1.1 · 已签字生效**,SA-B 可以起草。

| 角色 | 签字 |
| --- | --- |
| 架构(主对话) | **已签**(2026-04-17) |
| 后端 | **已同步**(2026-04-17 · `go build` / `go test` / `-tags=integration` / `openapi-validate` 四绿;生产控制字段零写入) |
| 产品 | **已同步**(2026-04-17 · 资产管理中心 AS-A4/A5/A6/A7 全部落地) |
