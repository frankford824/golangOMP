# V1 Backend Retro Report · 2026-04-25

> 范围:R1 (2026-04-17) ~ R6.A.3 (2026-04-25) · V0.9 → V1 后端重构整体回顾。
> 性质:Codex R6.A.4 v1.2 证据报告 · 主 §13 A1~A12 验收实证 · P99 性能加压基线 · V1.1/V2 路线建议。
> 裁决:等待架构师独立 verify；本文不自签 PASS。

## §0 Baseline

| 项 | 实测 |
| --- | --- |
| 测试库 | `jst_erp_r3_test` |
| DSN | `root:<TEST_DB_PASSWORD>@tcp(127.0.0.1:3306)/jst_erp_r3_test?parseTime=true&multiStatements=true&loc=Local` |
| 本轮段 | `[70000,80000)` |
| recovery | `tmp/r6_a_4_recovery_run.sh` 清 `[50000,80000)` 后 7 表全 0 |
| openapi sha | `b3d7c3651ea2496a6e4ea1a948772c6a395d6b387bf6c4509e5c26477c75dd0f` |
| detail P99 | RED · `334.721237ms` |
| 成功终止符候选 | `R6_A_4_DONE_PENDING_ARCHITECT_VERIFY` |

Baseline sha:

```text
b3d7c3651ea2496a6e4ea1a948772c6a395d6b387bf6c4509e5c26477c75dd0f  docs/api/openapi.yaml
5f4c9a10227e8321c4a87c8260b2bc0078adbb2dfb9fa0ebd2bd86601f46bae8  service/asset_lifecycle/cleanup_job.go
60103b15fa877a8d14b719dbd9f2aa82ee957271e8e8dea79a42106a8f346a1c  service/task_draft/service.go
32cd0201bf205bc2abfb6a9f489202de4bd099e188349184bd55a4ae1e22454b  service/task_lifecycle/auto_archive_job.go
f9d09d1fbc55734b00ff1f6c35cc1bccbf9db05298283eff6f255971262638c2  repo/mysql/task_auto_archive_repo.go
658a8cdf65c09335ab74176efb4057eff68440537e50ce0d9e550c57413e6e6b  domain/task.go
0bf70496a21c995d230efbcfaee4499257f1e3e46506e206a0ec6f51a73b6881  cmd/server/main.go
```

## §1 V1 全貌

V1 是 backend-only 的主链路重构。本轮不包含前端仓、不新增 migration、不改 OpenAPI、不改 4 份权威文档、不下线兼容路由；R6.A.4 只做全栈验证、retro、handoff 与 ROADMAP 待签字同步。

整体节奏为:R1 契约冻结 → R1.5/R1.6 字段对齐 → R2 数据落地 → R3 引擎 → R3.5 真 MySQL 验证 → R4 P3 四轮功能收口 → R4 Retro → R5 Excel 二件套 → R6.A.1/A.2/A.3 治理 job → R6.A.4 本报告。

V1 的可继承核心是 6 module 模型、ModuleAction 三层授权、任务池 CAS、task_status 聚合、资产生命周期、通知/草稿/WS、L1 报表、Excel batch 预览、3 个 cron gate 默认 off。

## §2 Timeline

| Date | Round | Report | Verdict | Scope | Key output |
| --- | --- | --- | --- | --- | --- |
| 2026-04-17 | R1 | `docs/iterations/V1_R1_REPORT.md` | signed | contract | OpenAPI +47 paths + 501 skeleton |
| 2026-04-17 | R1.5 | `docs/iterations/V1_R1_5_DDL_ALIGNMENT.md` | signed | docs | 4 authority docs v1.1 field alignment |
| 2026-04-17 | R1.6 | `docs/iterations/V1_R1_6_PROD_ALIGN.md` | signed | prod align | priority 4 values + asset_type 5 values |
| 2026-04-17 | R1.7-A | `docs/iterations/V1_R1_7_OPENAPI_SA_A_PATCH.md` | signed | OpenAPI | SA-A 9-point schema patch |
| 2026-04-17 | R1.7-B | `docs/iterations/V1_R1_7_B_OPENAPI_SA_B_PATCH.md` | signed | OpenAPI | SA-B 16 edits |
| 2026-04-24 | R1.7-C | `docs/iterations/V1_R1_7_C_OPENAPI_SA_C_PATCH.md` | signed | OpenAPI | SA-C 11 paths |
| 2026-04-24 | R1.7-D | `docs/iterations/V1_R1_7_D_OPENAPI_SA_D_PATCH.md` | signed | OpenAPI | SA-D search/report schemas |
| 2026-04-17 | R2 | `docs/iterations/V1_R2_REPORT.md` | signed | data | migrations 057~068 + backfill |
| 2026-04-17 | R3 | `docs/iterations/V1_R3_REPORT.md` | signed | engine | blueprint/CAS/aggregator |
| 2026-04-17 | R3.5 | `docs/iterations/V1_R3_5_INTEGRATION_VERIFICATION.md` | signed | integration | `jst_erp_r3_test` + 100-thread CAS |
| 2026-04-24 | R4-SA-A | `docs/iterations/V1_R4_SA_A_REPORT.md` | signed | assets | 7 handler + 5-state lifecycle |
| 2026-04-24 | R4-SA-A2 | `docs/iterations/V1_R4_SA_A_PATCH_A2_REPORT.md` | signed | patch | Gin wildcard normalization |
| 2026-04-24 | R4-SA-A3 | `docs/iterations/V1_R4_SA_A_PATCH_A3_REPORT.md` | signed | patch | `sql.ErrNoRows` 500→404 |
| 2026-04-24 | R4-SA-B | `docs/iterations/V1_R4_SA_B_REPORT.md` | signed | users/org | 14 handler + org move |
| 2026-04-24 | R4-SA-C | `docs/iterations/V1_R4_SA_C_REPORT.md` | signed | notification | drafts + notifications + WS |
| 2026-04-24 | R4-SA-D | `docs/iterations/V1_R4_SA_D_REPORT.md` | signed | search/report | global search + L1 reports |
| 2026-04-24 | R4-Retro | `docs/iterations/V1_R4_RETRO_REPORT.md` | signed | retro | 36 live smoke touchpoints |
| 2026-04-24 | R5 | `docs/iterations/V1_R5_BATCH_SKU_REPORT.md` | signed | Excel | template + parse-excel |
| 2026-04-24 | R6.A.1 | `docs/iterations/V1_R6_A_1_REPORT.md` | signed | CLI | run-cleanup `oss-365` / `drafts-7d` |
| 2026-04-24 | R6.A.2 | `docs/iterations/V1_R6_A_2_REPORT.md` | signed | cron | robfig/cron infra + gates |
| 2026-04-25 | R6.A.3 | `docs/iterations/V1_R6_A_3_REPORT.md` | signed | archive | task auto-archive 1.90ms/task |
| 2026-04-25 | R6.A.4 | this file | pending | retro | full verify + P99 RED + handoff |

## §3 主 §13 A1~A12 验收实证

| 编号 | 项 | 类别 | Evidence | Result |
| ---: | --- | --- | --- | --- |
| A1 | `GET /v1/tasks` 任一登录用户可见未删除任务 | programmatic | `tmp/r6_a_4_a_matrix.log` `TestRetro_A1_*` + full integration `transport/handler ok` | GREEN |
| A2 | `GET /v1/tasks/{id}/detail` 登录可读 | programmatic | `TestRetro_A2_*` + route `taskGroup.GET("/:id/detail")` | GREEN |
| A3 | module scope table 对齐 | traceability | `service/permission/scope.go` + `service/module/descriptor.go` + R3 report | GREEN |
| A4 | state mismatch / role denied | programmatic | `TestRetro_A4_*` + `domain/deny_code.go` | GREEN |
| A5 | 100 线程 claim 恰 1 成功 | programmatic | `TestRetro_A5_*` reran `TestClaimCAS_100Concurrent` | GREEN |
| A6 | 任务池可见性 | traceability | `service/task_pool/pool_query.go` + R3.5 pool assertions | GREEN |
| A7 | pool-reassign DeptAdmin allowed | programmatic | `TestRetro_A7_*` + `v1R1DepartmentAdminRoles()` route gate | GREEN |
| A8 | `tasks.task_status` 与 derived 一致 | traceability/sample | R2 smoke + full integration `service/task_aggregator ok`; test DB only has 98 tasks after recovery | GREEN |
| A9 | rollback staging dry-run | traceability | R2 report §7 dry-run | GREEN |
| A10 | 详情页一屏 6 module | programmatic | `TestRetro_A10_*` + blueprint `ModuleKeyBasicInfo` / `ModuleKeyWarehouse` | GREEN |
| A11 | `audit_` action only deprecated | programmatic | `TestRetro_A11_*` grep | GREEN |
| A12 | event stream consumable | traceability | R5 decision: v1 reads `task_module_events`; consumer outlet deferred R7+/V2 | GREEN-with-deferral |

总判定:11 GREEN + 1 GREEN-with-deferral。无 A-matrix ABORT。

## §4 全栈 Verify 矩阵

| Step | Command | Log | Result |
| --- | --- | --- | --- |
| recovery | `tmp/r6_a_4_recovery_run.sh` | `tmp/r6_a_4_recovery.log` | PASS |
| isolation pre | `bash tmp/r6_a_4_isolation_run.sh` | `tmp/r6_a_4_isolation_pre.log` | PASS |
| build | `/home/wsfwk/go/bin/go build ./...` | `tmp/r6_a_4_build.log` | PASS |
| vet | `/home/wsfwk/go/bin/go vet ./...` | `tmp/r6_a_4_vet.log` | PASS |
| build integration | `go build -tags=integration ./...` | `tmp/r6_a_4_build_integration.log` | PASS |
| build server | `go build ./cmd/server` | `tmp/r6_a_4_build_server.log` | PASS |
| unit | `go test -count=1 ./...` | `tmp/r6_a_4_unit.log` | PASS |
| integration | `go test -tags=integration -p 1 -count=1 -timeout 45m ./...` | `tmp/r6_a_4_integration.log` | PASS |
| OpenAPI | `go run ./cmd/tools/openapi-validate docs/api/openapi.yaml` | `tmp/r6_a_4_openapi_validate.log` | PASS `0 error 0 warning` |
| 501 | `grep -nE '^\s*"501":'` | `tmp/r6_a_4_501_paths.log` | PASS 0 lines |
| A matrix | `go test -tags=integration -run '^TestRetro_A' ./tmp/...` | `tmp/r6_a_4_a_matrix.log` | PASS |

Integration key package timings:

| Package | Time |
| --- | ---: |
| `workflow/cmd/tools/migrate_v1_backfill` | 34.810s |
| `workflow/cmd/tools/run-cleanup` | 171.424s |
| `workflow/service/search` | 15.997s |
| `workflow/service/task_lifecycle` | 8.184s |
| `workflow/transport/handler` | 7.012s |

## §5 性能加压基线

| Item | Source | Value | Status |
| --- | --- | ---: | --- |
| `GET /v1/search` p95 | R4-SA-D | 96.77ms | PASS |
| Claim 100 CAS | R3.5 + this round | 1/99 | PASS |
| Excel template | R5 + this round | F1 7581 bytes; F2 7557 bytes | PASS |
| OSS cleanup 1000 | R6.A.1 | 70393ms / 70.4ms asset | PASS |
| Task auto-archive AS-X 100 | R6.A.3 | 190ms / 1.90ms task | PASS |
| `/v1/tasks/{id}/detail` | this round | p50=252.537ms; p95=281.094ms; p99=334.721ms; max=334.721ms | RED |

P99 command:

```text
SUPER_ADMIN_TOKEN=$(cat tmp/r6_a_4_super_admin_token.txt) /home/wsfwk/go/bin/go run tmp/r6_a_4_p99_runner.go
n=100 p50=252.537446ms p95=281.093857ms p99=334.721237ms max=334.721237ms
```

P99 verdict:

- RED under main §12 R6 P99 gate (`p99 >= 150ms`).
- Not an ABORT under R6.A.4 prompt.
- Handoff manifest marks V1.1 performance work as mandatory: view materialization / derived status projection / index review.
- Test DB had 98 distinct tasks after recovery; runner used 100 GETs by cycling the sampled ids. This is recorded in `tmp/r6_a_4_select_task_ids.log`.

## §6 V0.9 → V1 Schema Diff

| Migration | Table / field | Key change | Source |
| --- | --- | --- | --- |
| 057 | `org_master` | dept/team/role convergence | R2 phase A |
| 058 | `org_team` | department-scoped uniqueness | R2 phase A |
| 059 | `task_modules` | six module rows per task | R2 phase B |
| 060 | `task_module_events` | module event stream | R2 phase B |
| 061 | `task_assets.source_module_key` | 5-value source module projection | R2 phase B |
| 062 | `reference_file_refs` | flat references | R2 phase B |
| 063 | `task_drafts` | draft table | R2 phase B |
| 064 | `notifications` | 5 notification types | R2 phase B |
| 065 | `org_move_requests` | cross-department move workflow | R2 phase B |
| 066 | `task_assets_lifecycle` | asset lifecycle | R2 phase B |
| 067 | `tasks.priority` | 4-value CHECK + index | R2 + R1.6 |
| 068 | `task_customization_orders` | customization order | R2 phase C |

Schema evidence:

- `tmp/r6_a_4_prod_schema.log` contains `SHOW CREATE TABLE` for 7 V1 tables.
- `task_assets` current schema has `is_archived`, `archived_at`, `archived_by`, `cleaned_at`, `deleted_at`; it does not have `lifecycle_state`.
- OpenAPI path count measured by current grep: 204 mounted `/v1` path entries in YAML.

## §7 cmd/server Live Smoke F1~F6

Server:

- `SERVER_PORT=18086`
- `tmp/r6_a_4_smoke_server.log`
- `tmp/r6_a_4_healthz.log` = `healthz=200`

| F | Request | Result | Status |
| ---: | --- | --- | --- |
| F1 | `GET /v1/tasks/batch-create/template.xlsx?task_type=new_product_development` | 200; 7581 bytes; XLSX content-type | PASS |
| F2 | `GET /v1/tasks/batch-create/template.xlsx?task_type=purchase_task` | 200; 7557 bytes | PASS |
| F3 | `GET ...?task_type=original_product_development` | 400; `batch_not_supported_for_task_type` | PASS |
| F4 | `POST /v1/tasks/batch-create/parse-excel` valid multipart | 200; `"violations":[]` | PASS |
| F5 | `POST /v1/tasks/batch-create/parse-excel` no file | 400; `file is required` | PASS |
| F6 | `GET /v1/tasks` no token | 401 | PASS |

Note:

- Prompt examples used `npd_basic` / `pt_basic`, but current OpenAPI enum is `new_product_development` / `purchase_task`. This report follows the live contract and R5 evidence.

## §8 控制字段 Post-Probe

`tmp/r6_a_4_control_fields_probe.log`:

```text
is_archived_dirty  0
cleaned_at_dirty   0
deleted_at_dirty   0
notif_type_alien   0
```

Probe note:

- Prompt SQL referenced `task_assets.lifecycle_state`, but current table does not have that column.
- Equivalent dirty checks used current control columns: invalid `is_archived`, `cleaned_at/deleted_at` non-null while not archived, and notification type alien values.
- Result remains all 0.

## §9 Codex / 架构师协作模型回顾

R3~R5 used Codex exec autopilot successfully for bounded implementation and report rounds. R6 switched to TUI atomic prompts with stronger white-listing and staged verification.

Lessons:

- Full-stack integration over shared DB segments must use `-p 1`.
- `tee` must be guarded by `set -o pipefail`.
- Test timeout stability is part of architecture quality; v1.2 fixed R2 backfill smoke `60s → 300s`.
- Segment recovery is required before multi-round verify because R5/R6.A.1/SA-D share `[50000,60000)`.
- `t.Cleanup` order matters; `db.Close` belongs at callback end.
- WSL native Go is the reliable baseline; Windows Device Guard can block test binaries.
- Cron gate ENV defaults off remain correct for cold start safety.

## §10 Known Debt

| Area | Debt | Target |
| --- | --- | --- |
| P99 detail | p99 334.721ms > 150ms | V1.1 mandatory |
| View/materialization | detail aggregate too heavy for R6 gate | V1.1 |
| Index review | likely task/detail/module/event joins need composite review | V1.1 |
| Compatibility routes | §9.3 old routes still mounted | R7+ |
| MED · OPEN V1.3-T1 onboarding sha 锚组过期 | `prompts/V1_NEXT_MODEL_ONBOARDING.md` §3.1 锚组停留在 V1.0/V1.1-A1 期间,V1.1-A2 / V1.2 / V1.2-C / V1.2-D-1 / V1.2-D-2 后未回写,导致每次新 codex 接手都触发 §9 漂移裁决。同时 CLAUDE.md L7 "Start model handoff with docs/V0_9_MODEL_HANDOFF_MANIFEST.md" 指向已归档文件 `docs/archive/legacy_handoffs/`。修复方式:V1.3 单独发起 onboarding/CLAUDE.md 回写轮,把锚组、authority 列表、handoff manifest 路径全部锁到 V1.2-D-2 head | `prompts/V1_NEXT_MODEL_ONBOARDING.md`, `CLAUDE.md`, `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` |
| U1 | timeout escalation | V2 |
| U2 | L2/L3 reports | V2 |
| U3 | blueprint externalization | V2 |
| U4 | actor_org_snapshot frozen dimensions | V2 |
| U5 | cancellation expansion | V2 |
| U6 | SLA auto escalation | V2 |
| Event consumer | Kafka/local consumer outlet deferred | V2 |

## §11 V1.1 + V2 路线建议

V1.1 should be treated as a performance and cleanup release:

- Materialize heavy detail projections or cache stable slices.
- Add or validate compound indexes around task detail, module, event, actor, and status joins.
- Re-run 100/500/1000 request P95/P99 with cold/warm separation.
- Convert §9.3 compatibility route removal into an ADR-backed R7 prompt.

V2 should inherit contracts, not implementation accidents:

- Preserve 6-module semantics and deny-code vocabulary.
- Preserve notification 5-value closure.
- Preserve cron default-off rule.
- Add durable event consumer outlet for A12.
- Introduce SLA automation only after actor_org_snapshot semantics are frozen.

## §12 数据隔离与测试库基线

Segment map:

| Segment | Round |
| --- | --- |
| `[20000,30000)` | SA-A |
| `[30000,40000)` | SA-B |
| `[40000,50000)` | SA-C |
| `[50000,60000)` | SA-D / R5 / R6.A.1 |
| `[60000,70000)` | R6.A.3 |
| `[70000,80000)` | R6.A.4 |

Final isolation:

```text
users 0
tasks 0
task_modules 0
task_module_events 0
task_assets 0
org_move_requests 0
notifications 0
task_drafts 0
permission_logs 0
```

## §13 V1 完成签字候选

Codex execution completed all R6.A.4 v1.2 required steps.

Not self-signed:

- P99 gate is RED and requires architect decision.
- Final verdict remains pending architect verify.

## §14 架构师独立 verify 裁决 · 2026-04-25

Verdict: **R6.A.4 PASS · architect-cleared · V1.1 P99 detail 必修**。

Independent baseline reproduced by architect on 2026-04-25(`tmp/arch_verify_r6_a_4.sh` + `tmp/r6_a_4_arch_verify.log`):

| 项 | 架构师独立实测 | Codex 报告 | 一致性 |
| --- | --- | --- | --- |
| OpenAPI sha256 | `b3d7c365…dd0f` | 同 | ✓ |
| Business 6 锚 sha256 | `5f4c…+60103…+32cd…+f9d09…+658a…+0bf70…` | 同 | ✓ |
| `smoke_test.go` 测试稳定性 patch | line 25 `300*time.Second` | 同(Apr 25 v1.2 patch) | ✓ |
| dangling 501 | 0 | 0 | ✓ |
| `/v1` path count(grep) | 203 | 204 | 差 1(grep 模式微差 · 不阻塞) |
| 4 份权威文档 mtime | Apr 24(R5 闭环态) | 0 改动 | ✓ |
| build / vet / cmd/server build | PASS | PASS | ✓ |
| openapi-validate | `0 error 0 warning` | 同 | ✓ |
| A-matrix `^TestRetro_A` | `ok workflow/tmp 1.100s` | PASS | ✓ |
| 全栈 integration `-p 1` 重跑 | 318s · 0 FAIL · 全 ok | 350s · 0 FAIL | ✓ 同量级 |
| 关键包计时(独立基线) | backfill 38.0s / run-cleanup 186.0s / search 17.4s / task_lifecycle 8.9s / handler 7.5s | 34.8 / 171.4 / 16.0 / 8.2 / 7.0 | ✓ outlier 内 |
| 段 [70000,80000) 9 表 BEFORE/AFTER | 全 0 | 全 0 | ✓ |
| 控制字段 4 项 probe | 全 0 | 全 0 | ✓ |
| P99 detail | p50=252.537ms / p95=281.094ms / p99=334.721ms | 同 | ✓ |

P99 裁决:

- P99 detail = 334.721ms · 主 §12 R6 验收门(< 150ms)RED · 单项不达标。
- 按 prompt §0.2:P99 ≥ 150ms 不 ABORT · retro RED · handoff `§必修` 登记 · 转 V1.1 mandatory。
- Codex v1.2 严格遵守该约定:retro §5 + §10 + handoff §5 + §6 全部登记 P99 RED → V1.1 mandatory · 无回避无掩盖。
- 因此 R6.A.4 工作签字 PASS;V1 主 §12 R6 P99 性能门保留 RED 单项 · V1 整体定级 **SUBSTANTIAL-COMPLETE**(功能/契约/数据/治理 100% · 性能门 1/N RED → V1.1 闭环)。

合规检查:

- 双产物结构齐 · retro 15 节 + handoff 10 节 · 均超出 prompt 最低门槛(retro ≥14 / handoff ≥8)。
- 0 业务代码 / 0 OpenAPI / 0 migration / 0 4 份权威文档改动 · sha 锚定 7/7 一致(`tmp/r6_a_4_arch_verify.log §1`)。
- 唯一非业务改动:`cmd/tools/migrate_v1_backfill/smoke_test.go:25` 60s→300s 测试稳定性 patch(架构师 v1.2 inline patch · 不在 §3.5 sha 锚定清单 · 已签字)。
- 段 [70000,80000) step-1 / step-N audit 全 0 · 无累积污染遗留。
- A-matrix 7 程序化 + 5 文档 traceability = 11 GREEN + 1 GREEN-with-deferral(A12 consumer outlet 推 V2)· 无 ABORT。

V1 后继轮约束(转入 V1.1):

- 必修 1 · `/v1/tasks/{id}/detail` P99 < 150ms(候选:detail aggregate 视图物化 · `task_status,updated_at` + `actor_id` 复合索引 · 冷暖 cache 分桶基线)。
- 必修 2 · CI 集成测试包级并行守卫(强制 `-p 1` for shared-DB integration packages · 防 R6.A.4 v1.0 段污染伪退化复刻)。
- 选修 · §9.3 兼容路由下线 ADR(R7+ prompt 拆轮)。

R6.A.4 工作 architect-cleared · V1 SUBSTANTIAL-COMPLETE · V1.1 P99 detail 必修 · 准入 V1.1 起草。

## §15 终止符

R6_A_4_DONE_ARCHITECT_VERIFIED · V1_SUBSTANTIAL_COMPLETE_PENDING_V1_1_P99


## §16 V1.1-A2 contract drift purge — CLOSED 2026-04-27

V1.1-A1 retro 中"0 OpenAPI 改动"在 v1.21 release 后被用户实测发现是失守的:detail 接口实际返回 5 段精简 schema,而 OpenAPI 仍声明老富 schema。本轮 V1.1-A2 全量回归 203 path,产出 drift inventory(P0 1 + P1 5 + P2 5 + defer 5),按 code-wins 原则修订 OpenAPI,重生成 frontend doc 16 份,完成签字:`V1_1_A2_CONTRACT_DRIFT_PURGED`。

详见:

- `docs/iterations/V1_1_A2_DRIFT_INVENTORY.md`
- `docs/iterations/V1_1_A2_FIX_PLAN.md`
- `docs/iterations/V1_1_A2_RETRO_REPORT.md`


## §17 V1.2 authority purge and contract guard — CLOSED 2026-04-27 · 经 V1.2-C 工具回炉补完

V1.2 主体交付通过架构师独立 verify(18 项矩阵中 P1 authority inventory / P2a unreachable schema GC / P2b deprecated path decision / P3 route triangulation 12 known-gap / P5 CI guard 框架 + code-only-changed 守门 / P6 V1 SoT + 16 份 frontend doc V1.2 marker + 治理 4 件套同步 全部接受)。

但 P4 `tools/contract_audit/` 工具核心 diff 引擎判定**未达成**:架构师独立查源码后发现 `main.go` L102-114 把 OpenAPI 抽出的 fields 同时赋给 `CodeFields` 和 `OpenAPIFields`、`Verdict` 写死 `"clean"`、`Summary.Drift` 始终为 0,L62-63 `_ = handlers / _ = domain` 把 handler/domain flag 直接丢弃,`StructJSONFields`/`DiffFields` 从未被 main 流程调用,`main_test.go` 仅覆盖孤立函数未覆盖 main 流程。drift seed 的 code-only-changed 路径有效,但字段不匹配路径无法被工具检出。

裁决终止符:`V1_2_PARTIAL_PASS_AUDIT_TOOL_REWORK_REQUIRED`。

✅ **V1.1-A2 Q-1(192 条 clean path 模板占位)** · 经 V1.2-C 工具回炉真三向 diff 落地后 · CLOSED 2026-04-27 · 详见 `docs/iterations/V1_2_C_RETRO_REPORT.md`。

V1.2 主体已产出的真实事实:
- schemas 313 → 298,unreachable 0
- 29 deprecated paths 全决断,全 mounted,加 `x-removed-at: v1.3`
- OpenAPI 14552 行
- 12 known-gap 显式落 GC report §4
- 业务 SHA 锚组 A 0 漂移
- frontend doc 16 份 V1.2 marker + INDEX V1 SoT 链接

详见:

- `docs/iterations/V1_2_OPENAPI_GC_REPORT.md`
- `docs/iterations/V1_2_CONTRACT_AUDIT_v1.json`(注:本 JSON 的 `summary.drift=0` 是工具 bug 的数学保证,不是真实 audit 结论)
- `docs/iterations/V1_2_RETRO_REPORT.md`(架构师追加修正段)
- `prompts/V1_2_C_AUDIT_TOOL_REWORK.md`(V1.2-C 子轮 prompt · 只修工具)
- `docs/iterations/V1_2_C_RETRO_REPORT.md`(V1.2-C retro · 18 项 verify 全 PASS)
- `docs/iterations/V1_2_CONTRACT_AUDIT_v2.json`(真三向 audit · 总 242 / 净 84 / drift 72 / unmapped 66 / known-gap 20)
- `docs/iterations/V1_2_CONTRACT_AUDIT_v2.md`

## §18 V1.2-D 候选(由 V1.2-C 工具暴露 · 排序待 V1.2-D 处理)

V1.2-C 工具落地后输出真实 drift = 72(只暴露不修复),架构师抽样后归类:

| 优先级 | 项 | 证据 |
|---|---|---|
| **CRITICAL · CLOSED V1.2-D-1 · architect-verified 2026-04-27** | `transport/handler/task_detail.go` GetByTaskID fallback 出口已切除;构造改为 `NewTaskDetailHandler(r3DetailSvc)`, `r3Svc == nil` 返回 internal error,成功路径唯一返回 5 段 `task_aggregator.Detail` | `docs/iterations/V1_2_D_1_REPORT.md` §10; commit `c3603db`; audit after: GET `/v1/tasks/:id/detail` verdict=clean, drift 72→71, clean 84→85; 架构师 13 项独立 verify 全 PASS,登记 cmd/api scope-creep 为 NET-POSITIVE。终止符 `V1_2_D_1_ARCHITECT_VERIFIED` |
| **HIGH · CLOSED V1.2-D-2 · architect-verified 2026-04-27** | V1.2-D ABORT 残留已由 V1.2-D-2 收口并经架构师 13 项独立 verify 全 PASS:final audit `total=233 / clean=179 / drift=0 / unmapped=0 / known_gap=54 / missing_in_openapi=0 / missing_in_code=0`;known_gap class unknown=0(P5 .md 手写表 · audit JSON 暂未携带 class 字段 · 工具无法自动 gate);frontend docs 16 份重生;7 业务 Go 锚 SHA 0 漂移;业务 Go 改动严格限定到 `tools/contract_audit/main.go` (+42 行)0 业务 handler/service/repo 改动;独立重跑 `--fail-on-drift true exit=0`。CONDITIONAL PASS,2 项灰色地带登记 V1.3 债:(Q-V1.2-D-2-1) known_gap class 字段写入 audit JSON · (Q-V1.2-D-2-2) 4 deprecated `/v1/assets/:id` 凭空消失需决策(OpenAPI 删除 vs transport 重挂载)。终止符 `V1_2_D_2_DRIFT_FULLY_TRIAGED_ARCHITECT_VERIFIED`。 | `docs/iterations/V1_2_D_2_ARCHITECT_VERIFY_REPORT.md`; `docs/iterations/V1_2_D_2_RETRO_REPORT.md`; `docs/iterations/V1_2_D_2_FINAL_AUDIT.{json,md}` |
| HIGH | `TaskReadModel` struct 字段被掏空 · `GET /v1/tasks/:id` 与 `/v1/tasks/:id/close` only_in_openapi 32 字段 | 同上 sample |
| MED | 类目家族 31 个 both_diff(categories / category-mappings / 等)| 同上 sample |
| MED | pagination wrap 字段统一(7 个 list 接口 `count/current_page/datas/page_size/pages` 是 only_in_code)| 同上 sample |
| LOW | unmapped_handler 35 条(29 不走 respondOK + 6 反推能力边界)以及 31 dynamic_payload(10 reserved + 10 gin.H + 11 其他)| `unmapped[]` 段
