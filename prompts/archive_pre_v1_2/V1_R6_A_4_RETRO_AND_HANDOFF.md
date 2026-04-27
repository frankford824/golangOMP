# V1 R6.A.4 · V1 整体 Retro + P99 性能加压 + V1→V2 Model Handoff(三件套合并轮)

> **版本**:v1.2(架构师 2026-04-25 二次修补 · v1.1 在 `-p 1` 串行下又遇 R2 backfill smoke `60s deadline outlier timeout` ABORT · v1.2 架构师 inline patch `cmd/tools/migrate_v1_backfill/smoke_test.go:25` `60s → 300s` · 实证 350s 全栈 0 FAIL)
> **版本**:v1.1(架构师 2026-04-25 一次修补 · v1.0 因 §4.2 P1.3 漏写 `-p 1` 命中段污染伪退化 ABORT · v1.1 加入 `-p 1` + `set -o pipefail` + 段污染恢复脚本)
> **执行模式**:codex TUI · 原子 prompt 4(R6 系列收尾轮)
> **依赖**:R6.A.3 已签字闭环(`docs/iterations/V1_R6_A_3_REPORT.md §9` PASS · architect-cleared)
> **时长预期**:codex 跑 ~3-4h(只读 verify + 文档起草 + P99 实测)
> **核心约束**:**0 业务代码改动 · 0 OpenAPI 改动 · 0 migration · 0 4 份权威文档改动**;只起草新 retro/handoff 文档 + ROADMAP 签字行 + tmp/ 实测脚本

> **v1.0 ABORT 复盘 + v1.1 修补**(2026-04-25 架构师裁决):
>
> v1.0 codex 严格按 prompt §4.2 P1.3 跑 `go test -tags=integration -count=1 -timeout 30m ./...` 命中 ABORT 触发 #8(全栈 integration 4 包 FAIL):`migrate_v1_backfill` / `run-cleanup` / `service/search` / `service/task_lifecycle`。
>
> 架构师独立 verify(2026-04-25):清 [50000,80000) 段污染累积 + `-p 1` 串行跑全栈 integration → **295s · 0 FAIL**(失败 4 包全部 ok:`migrate_v1_backfill 35.162s / run-cleanup 171.260s / service/search 15.950s / service/task_lifecycle 8.498s`)。
>
> 根因:多个 integration 包(SA-A/SA-B/SA-C/SA-D/R5/R6.A.1/R6.A.3)共用 `[20000,80000)` 段子区间,默认 `go test ./...` 包级并行 GOMAXPROCS 个包同时跑,跨包同段并发 → fixture 互相污染 + DB 压力 noisy neighbor。这是 prompt v1.0 缺陷不是真退化。
>
> v1.1 修补:
> 1. §4.2 P1.3 命令加 `-p 1` 串行 + `set -o pipefail` 守 EXIT(`tee` 管道掩盖非零码问题已经被 codex 在 v1.0 报告 §7 #5 提出)
> 2. §9 step 5 同步加 `-p 1`
> 3. 新增 §0.4 段污染恢复脚本(verify 起步前必跑 · 清 [50000,80000) 累积污染)
> 4. ABORT 触发 #8 在 v1.0 已写明的"全栈 integration 任一包 FAIL"判定**仅在 `-p 1` 串行命令下生效** · 默认并行命令出 FAIL 不算硬退化(本节 v1.1 说明 + §4.2 P1.3 命令双重保险)
>
> v1.0 codex 已落盘的产物保留(`tmp/r6_a_4_*.log/.sql/.sh` + `docs/iterations/V1_RETRO_REPORT.md` ABORT 版),v1.1 codex 重跑时按本 prompt 全量覆盖。

> **v1.1 ABORT 复盘 + v1.2 修补**(2026-04-25 架构师裁决):
>
> v1.1 codex 严格按 prompt §4.2 P1.3 跑 `set -o pipefail && go test -tags=integration -p 1 -count=1 -timeout 45m ./...` · 段隔离全 0 / build/vet/unit/openapi/sha 全 PASS · 但**单包** `workflow/cmd/tools/migrate_v1_backfill.TestR2BackfillSmoke` 跑 60.17s timeout(`smoke_test.go:32: context deadline exceeded`)命中 ABORT #8。
>
> 根因:`cmd/tools/migrate_v1_backfill/smoke_test.go:25` 自带 `ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)` 测试级硬 deadline · v1.0 codex 报告 `Phase A duration=24.506s + Phase B duration=23.244s = 47.7s` + 4 SELECT count + tableCounts ~50s · **本身就贴 60s 边缘**。架构师上次 verify 实测 `migrate_v1_backfill 35.162s ok` 是 buffer 暖+SSH tunnel low latency outlier · v1.1 codex 跑 60.17s 是另一个 outlier · 这不是退化是 R2 测试稳定性 bug。
>
> v1.2 修补:
> 1. 架构师 inline patch `cmd/tools/migrate_v1_backfill/smoke_test.go:25` `60*time.Second` → `300*time.Second`(类比 R6.A.1 架构师 inline patch `defer db.Close()` vs `t.Cleanup` 顺序 bug · 改 `_test.go` 测试稳定性补丁不动业务代码 · 不在 §3.5 sha 锚定清单)
> 2. 架构师独立 verify 实测:patch 后 backfill 单包跑 **44.702s ok**(60s 实测会 timeout · 300s 给 5x buffer)+ 全栈 `-p 1` 串行 **350s · 0 FAIL · 全 ok**(`migrate_v1_backfill 44.112s` + `run-cleanup 217.641s` + `service/search 16.000s` + `service/task_lifecycle 8.254s` + 其余 ~30 包全 ok)
>
> v1.2 codex 重跑时 prompt 主体不变 · §3.5 sha 锚定不变(`smoke_test.go` 不在锚定) · 直接跑 P0 段恢复 → P1.3 `-p 1` 串行 应能复现 350s 0 FAIL 基线。

---

## §0 终止符与 ABORT

### §0.1 成功终止符

完成 §9 验证全 PASS 后,**最后一行**输出:

```
R6_A_4_DONE_PENDING_ARCHITECT_VERIFY
```

**禁止自签 PASS** · 等待架构师独立 verify。

### §0.2 ABORT 触发(命中任一立刻终止 + 报告 §X 写入根因)

| # | 触发 | 行为 |
| ---: | --- | --- |
| 1 | 主 §13 A1/A2/A4/A5/A7/A11 任一**程序化**验证 fail | ABORT · 不写终止符 · 报告标 V1 验收门 RED |
| 2 | `openapi-validate` 非 `0/0` | ABORT |
| 3 | `grep -c '501' docs/api/openapi.yaml` 大于 R6.A.3 闭环态 baseline(允许 schema 内 reference 但不允许新增 dangling 501) | ABORT |
| 4 | 任一业务代码文件 sha256 与 R6.A.3 闭环态 baseline 不一致(见 §3.5 锚定清单) | ABORT |
| 5 | `docs/api/openapi.yaml` sha256 ≠ `b3d7c3651ea2496a6e4ea1a948772c6a395d6b387bf6c4509e5c26477c75dd0f` | ABORT |
| 6 | 4 份权威文档(`docs/V1_MODULE_ARCHITECTURE.md` / `V1_INFORMATION_ARCHITECTURE.md` / `V1_CUSTOMIZATION_WORKFLOW.md` / `V1_ASSET_OWNERSHIP.md`)任一被 codex 写入 | ABORT |
| 7 | 段 `[70000,80000)` audit 任一表 BEFORE/AFTER 残留非 0 | ABORT |
| 8 | 全栈 integration 任一包 FAIL(`./service/...` + `./cmd/...` + `./transport/...`)**仅在 §4.2 P1.3 `-p 1` 串行命令下生效** | ABORT |
| 9 | 全栈 unit `go test ./...` 任一包 FAIL | ABORT |

> P99 `/v1/tasks/{id}/detail` ≥ 150ms **不**触发 ABORT — retro 标 §性能节 RED + handoff manifest 转入 V1.1 必修;但 codex 必须如实记录 P95/P99 实测数字。

### §0.3 v1.1 修补节(段污染恢复脚本)

R6.A.4 v1.0 ABORT 留下 [50000,80000) 段污染累积(R6.A.1 段 [50000,60000) + R6.A.3 段 [60000,70000) + R6.A.4 段 [70000,80000) 互相污染)。v1.1 codex 重跑前**必须先跑恢复脚本**:

```bash
# 1) 写恢复 SQL(已由架构师写好 · codex 直接复用)
# 文件:tmp/r6_a_4_recovery_clean.sql(架构师 v1.1 已落盘 · 不要重写)
# 文件:tmp/r6_a_4_recovery_run.sh(架构师 v1.1 已落盘 · 不要重写)

# 2) 跑恢复脚本(scp + ssh + mysql 清 [50000,80000) 段累积污染)
chmod +x tmp/r6_a_4_recovery_run.sh
tmp/r6_a_4_recovery_run.sh

# 3) 校验输出 9 表 0 行后,继续走 §4 P1.1 isolation
grep -E '^(tasks|task_modules|task_assets|task_drafts|notifications|org_move_requests|users)\s+0$' tmp/r6_a_4_recovery.log
# 必须每行尾 0 · 否则 ABORT
```

> 脚本由架构师 2026-04-25 落盘 · 实测结果:`tasks/task_modules/task_assets/task_drafts/notifications/org_move_requests/users` 全 0 · 证明 [50000,80000) 段重新清洁 · v1.1 codex 重跑全栈 integration `-p 1` 应能复现 295s 全 PASS 基线。

---

## §1 Scope

本轮单文件双产物 + 一签字行,**完全不动业务代码**:

| 产物 | 路径 | 性质 |
| --- | --- | --- |
| ① | `docs/iterations/V1_RETRO_REPORT.md` | V1 整体 retro 报告(R1~R6.A.3 闭环回顾 + 主 §13 A1~A12 验收实证 + 性能加压基线 + 已知遗留 + 教训固化) |
| ② | `docs/V1_TO_V2_MODEL_HANDOFF_MANIFEST.md` | V1→V2 模型/前端工程交接清单(类比 `docs/V0_9_MODEL_HANDOFF_MANIFEST.md` 风格)|
| ③ | `prompts/V1_ROADMAP.md` 编辑(§32 R6.A.4 行 + §变更记录 v25 起草 · **不自签 v25 PASS** · 留 `prompt v1.0 已起草 → 待架构师 verify` 状态) | ROADMAP 同步 |
| ④ | `tmp/r6_a_4_*.sh / .sql / .go`(实测脚本)| 段隔离 + P99 加压 + V0.9→V1 schema diff 工具 |

**不在范围**(本轮不做 · 转入后续 R6.A.5+ 或 V1.1):

- ❌ 业务代码改动(任何 `.go` 文件 · 关键锚 sha256 守卫见 §3.5)
- ❌ `docs/api/openapi.yaml` 改动(本轮锁定 R5 闭环态 sha)
- ❌ `db/migrations/*` 新增(retro 不创新表)
- ❌ 4 份权威文档(主/IA/定制/资产)改动(本轮只引用,不修订;若实证发现文档与代码不符,记入 retro §文档对齐性章 + handoff manifest 转入 V2 必修)
- ❌ §9.3 兼容路由真下线(retro 只列建议清单 · R7+ 实施)
- ❌ V1.1 任何 prompt 起草(handoff manifest 只列 V2 应继承项,不进入实装路径)

---

## §2 必读上下文(顺序读完再开工)

### §2.1 权威文档(本轮只读引用 · 0 写入)

1. `CLAUDE.md`(workspace authority order)
2. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md` + `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`(V0.9 baseline · handoff 起草模板)
3. `docs/V1_MODULE_ARCHITECTURE.md` v1.3(主权威 · §12 R 路线图 + §13 A1~A12 + §14 风险 + §15 U1~U6 + §17 决策清单 + §18 changelog)
4. `docs/V1_INFORMATION_ARCHITECTURE.md`(IA 权威)
5. `docs/V1_CUSTOMIZATION_WORKFLOW.md`(定制工作流权威)
6. `docs/V1_ASSET_OWNERSHIP.md`(资产权威)
7. `docs/V1_0_FRONTEND_INTEGRATION_GUIDE.md`(前端集成指南)

### §2.2 14 个签字报告(retro §timeline 节直接引用)

按时间顺序:

| 轮 | 报告 | 关键产物锚 |
| --- | --- | --- |
| R1 | `docs/iterations/V1_R1_REPORT.md` | OpenAPI 47 条新增 · 501 骨架 |
| R1.5 | `docs/iterations/V1_R1_5_DDL_ALIGNMENT.md` | 4 文档 v1.1 · 字段对齐真实 DDL |
| R1.6 | `docs/iterations/V1_R1_6_PROD_ALIGN.md` | priority 4 值 / asset_type 5 值 真生产对齐 |
| R1.7-A | `docs/iterations/V1_R1_7_OPENAPI_SA_A_PATCH.md` | SA-A 9 点补丁 |
| R1.7-B | `docs/iterations/V1_R1_7_B_OPENAPI_SA_B_PATCH.md` | SA-B 16 处 |
| R1.7-C | `docs/iterations/V1_R1_7_C_OPENAPI_SA_C_PATCH.md` | SA-C 11 路径 |
| R1.7-D | `docs/iterations/V1_R1_7_D_OPENAPI_SA_D_PATCH.md` | SA-D 5 处 + L1 报表固化 |
| R2 | `docs/iterations/V1_R2_REPORT.md` | 12 migration 落地 · backfill 3.899s |
| R3 | `docs/iterations/V1_R3_REPORT.md` | blueprint engine + CAS + aggregator |
| R3.5 | `docs/iterations/V1_R3_5_INTEGRATION_VERIFICATION.md` | jst_erp_r3_test · 100 线程 CAS 真验 |
| R4-SA-A | `docs/iterations/V1_R4_SA_A_REPORT.md` | 7 handler + 5 态机 + cleanup_job 骨架 |
| R4-SA-A Patch-A2 | `docs/iterations/V1_R4_SA_A_PATCH_A2_REPORT.md` | Gin wildcard 归一(`:asset_id`/`:notification_id`/`:request_id`)|
| R4-SA-A Patch-A3 | `docs/iterations/V1_R4_SA_A_PATCH_A3_REPORT.md` | DRIFT-RUNTIME-2 · `errors.Is(err, sql.ErrNoRows)` 修 500→404 |
| R4-SA-B | `docs/iterations/V1_R4_SA_B_REPORT.md` | 14 handler · me / org-move / activate / deactivate / delete |
| R4-SA-B.1 + B.2 | 同上(SA-B REPORT 末尾追加)| 11 dedicated tests + collation 热补丁 |
| R4-SA-C | `docs/iterations/V1_R4_SA_C_REPORT.md` | 11 handler + 通知 + WS + 草稿 + ERP by-code |
| R4-SA-C.1 | `prompts/V1_R4_FEATURES_SA_C_1_I1_I11_PATCH.md` 关联报告 | 10 dedicated tests · 段 [40000,50000) |
| R4-SA-D | `docs/iterations/V1_R4_SA_D_REPORT.md` | 4 handler · 全局搜索 + L1 报表 |
| R4-Retro | `docs/iterations/V1_R4_RETRO_REPORT.md` v1.1 | 36 触点 live smoke · p95=269ms |
| R5 | `docs/iterations/V1_R5_BATCH_SKU_REPORT.md` | Excel 二件套 · 0 业务写入 |
| R6.A.1 | `docs/iterations/V1_R6_A_1_REPORT.md` | run-cleanup CLI · AS-A5 70.4s/1000 |
| R6.A.2 | `docs/iterations/V1_R6_A_2_REPORT.md` | scheduler.Cron · cmd/server cron mount |
| R6.A.3 | `docs/iterations/V1_R6_A_3_REPORT.md` | task auto-archive · 1.90 ms/task |

### §2.3 ROADMAP

`prompts/V1_ROADMAP.md`(总表 · 230+ 行 · §32 表 + §变更记录)

### §2.4 V0.9 → V1 migration 12 张

`db/migrations/057_v1_0_org_master_convergence.sql`
`db/migrations/058_v1_0_org_team_department_scoped_uniqueness.sql`
`db/migrations/059_v1_0_task_modules.sql`
`db/migrations/060_v1_0_task_module_events.sql`
`db/migrations/061_v1_0_task_assets_source_module_key.sql`
`db/migrations/062_v1_0_reference_file_refs_flat.sql`
`db/migrations/063_v1_0_task_drafts.sql`
`db/migrations/064_v1_0_notifications.sql`
`db/migrations/065_v1_0_org_move_requests.sql`
`db/migrations/066_v1_0_task_assets_lifecycle.sql`
`db/migrations/067_v1_0_tasks_priority_constraint.sql`
`db/migrations/068_v1_0_task_customization_orders.sql`

---

## §3 Baseline facts(开工前必须先记录到 retro `§Baseline` 节)

### §3.1 测试库

- 主 DSN(测试库):`MYSQL_DSN='root:<TEST_DB_PASSWORD>@tcp(127.0.0.1:3306)/jst_erp_r3_test?parseTime=true&multiStatements=true&loc=Local'`(R3.5 起锁定 · `cmd/server` smoke 沿用)
- 段策略:本轮 retro 用 `[70000,80000)` 专属段,与 R6.A.1 [50000,60000) / R6.A.3 [60000,70000) / SA-A [20000,30000) / SA-B [30000,40000) / SA-C [40000,50000) / SA-D [50000,60000) 全互斥
- 数据隔离脚本:**先抄 `tmp/r6_a_3_isolation_run.sh` 改 70000~79999 段**生成 `tmp/r6_a_4_isolation_run.sh` + `r6_a_4_segment_audit.sql` + `r6_a_4_segment_clean.sql`
- 7 表 audit:`users / tasks / task_modules / task_module_events / task_assets / org_move_requests / notifications / task_drafts / permission_logs`(沿用 R6.A.3 7 表 audit + 加 users/permission_logs · 共 9 表 · 与 R5 audit_v3 模板对齐)

### §3.2 sha256 锚点(R6.A.3 闭环态 · 本轮 ABORT 守卫)

```bash
sha256sum docs/api/openapi.yaml                          # 必须 = b3d7c365…dd0f
sha256sum service/asset_lifecycle/cleanup_job.go         # R6.A.2 baseline · 不变
sha256sum service/task_draft/service.go                  # R6.A.2 baseline · 不变
sha256sum service/task_lifecycle/auto_archive_job.go     # R6.A.3 baseline · 不变
sha256sum repo/mysql/task_auto_archive_repo.go           # R6.A.3 baseline · 不变
sha256sum domain/task.go                                 # R1 起 0 改动 · 不能引入 closed_at/archived_at
sha256sum cmd/server/main.go                             # R6.A.3 baseline · 不变
```

> 在 retro `§Baseline` 节列出实测值并写入 `tmp/r6_a_4_baseline_sha.log`。任一 sha 在 verify 末段重测时漂移 → ABORT。

### §3.3 主 §13 A1~A12 验证矩阵(本轮硬验)

| 编号 | 项 | 实施类别 | 实证路径 |
| ---: | --- | --- | --- |
| A1 | `GET /v1/tasks` 任一登录用户可见所有未删除任务 | 程序化(integration) | 跑 `^TestRetro_A1_` 用 dept_admin/team_lead/designer/auditor 4 角色对同一非 deleted 任务 GET 返 200 + 任一含此 task_id |
| A2 | `GET /v1/tasks/{id}/detail` 任一登录用户返 200(除硬删除) | 程序化 | `^TestRetro_A2_` 4 角色对软删 task GET 200 · 对非存在 GET 404 · 对硬删 GET 404 |
| A3 | Module.scope.in_scope 与 §6 Layer 2 表 100% 一致 | 文档 traceability | retro 列出 `service/permission/authorize_module_action.go` · 引用 §6 Layer 2 + 单测 file path |
| A4 | module_state_mismatch / module_action_role_denied | 程序化 | `^TestRetro_A4_StateMismatch + _RoleDenied` |
| A5 | 接单 100 线程恰 1 成功 | 程序化(R3.5 已实证 · 引用 + 重跑) | `^TestClaim_100Concurrent` 引用 R3.5 + 本轮重跑确认仍 1/99 |
| A6 | 任务池可见性 | 文档 traceability | retro 列出 `service/task_pool/pool_query.go` · 引用 §4.1 池组分流逻辑 + R3.5 测试矩阵 |
| A7 | pool-reassign 部门管理员可执行 | 程序化 | `^TestRetro_A7_PoolReassign_DeptAdminAllowed_OthersDenied` |
| A8 | tasks.task_status 与 aggregator/derived 一致 | 文档 traceability + 程序化抽样 | retro 列 R2 backfill 落地数据 · 抽 200 task_id 跑 aggregator 比对 task_status 列 100% 一致 |
| A9 | 回滚脚本 staging 演练 | 文档 traceability | 引用 R2 报告 dry-run 通过段 · retro 不重跑(staging 不在本轮范围)|
| A10 | 详情页一屏 6 模块 | 程序化 | `^TestRetro_A10_DetailOneShot_6Modules` 单次 GET 返完整 6 模块 + 创建者可写 basic_info + 非接单者 design 只读 |
| A11 | grep audit_ 仅 deprecated | 程序化(grep) | `grep -r "TaskAction.*=\s*TaskAction\s*\"audit_"` 不出现在非 deprecated 文件中 |
| A12 | 事件流可独立消费 | 文档 traceability | 引用 R5 决策"v1 事件流 = 直查 task_module_events · 出口推迟 R7+(U1/U2)" · retro 标"v1 设计兼容 A12 · 实装等 R7+" |

> 任一程序化项 fail → ABORT(§0.2 #1)

### §3.4 性能加压基线(retro `§性能` 节)

| 项 | 来源 | 数值 | 状态 |
| --- | --- | --- | --- |
| `GET /v1/search` p95 | R4-SA-D 报告 | 96.77ms | PASS(< IA-A3 1s) |
| Claim 100 线程 CAS | R3.5 报告 | 1/99 0 死锁 | PASS |
| Excel `parse-excel` (R5) | R5 报告 | F1=200 7581 bytes | PASS |
| OSS-365 cleanup 1000 条 | R6.A.1 | elapsed=70393ms · 70.4 ms/asset | PASS |
| Task auto-archive AS-X 100 | R6.A.3 | elapsed=190ms · 1.90 ms/task | PASS |
| **`GET /v1/tasks/{id}/detail` p99** | **本轮新实测**(主 §12 R6 验收门 至今未实测)| **必须 < 150ms** | **本轮强验** |
| Live smoke F1~F6 latency | R5/R4-Retro | F1~F6 ≤ 4xx · 5xx=0 | 重跑确认 |

### §3.5 V0.9 → V1 实装清单(handoff manifest 来源)

| 维度 | V0.9 状态 | V1 落地 |
| --- | --- | --- |
| migration | `001~056` | `057~068` 12 张 + R2 backfill 5 phase |
| OpenAPI | V0.9 baseline | R1 +47 路径 + R1.7-A/B/C/D 4 补丁 + R5 2 路径 = ~`~115` paths(待 codex 实测确认) |
| 服务包 | V0.9 baseline | 新增 `service/asset_center` / `asset_lifecycle` / `asset_lifecycle/scheduler` / `task_lifecycle` / `task_draft` / `task_pool` / `task_cancel` / `org_move_request` / `notification` / `task_batch_excel` / `permission/authorize_module_action` / `blueprint` / `module/state_machine` / `report_l1` / `search` / `task_status_aggregator` / `erp_bridge` / `design_source`(共 ~18 包 · 待 codex 实测确认) |
| repo | V0.9 baseline | 新增 `task_auto_archive` / `task_module` / `task_module_event` / `task_asset_search` / `task_asset_lifecycle` / `task_draft` / `notification` / `org_move_request` / `reference_file_ref_flat` 等(待 codex grep `repo/*.go` 实测) |
| handler | V0.9 baseline | 新增 R1 47 + SA-A 7 + SA-B 14 + SA-C 11 + SA-D 4 + R5 2 + R6.A.1~A.3 0 = ~85 个 handler · 401/403/404/410 错误码体系完整 |
| cron job | V0.9 无 | R6.A.2 robfig/cron/v3 · 3 段 gate(`ENABLE_CRON_OSS_365` / `ENABLE_CRON_DRAFTS_7D` / `ENABLE_CRON_AUTO_ARCHIVE`)默认 off |
| 测试库 | V0.9 直接打生产 | R3.5 起 `jst_erp_r3_test` + DSN 守卫 + 段隔离 [20000,80000) |
| 段策略 | V0.9 无 | R3.5 起 `task_id/user_id ∈ [20000,80000) per round` · R6.A.4 用 [70000,80000) |
| 权威文档 | V0.9 无 | 4 份(MA v1.3 / IA / CW / AO)+ V1.0 前端集成指南 |
| WebSocket | V0.9 无 | SA-C in-memory hub · 单实例 · 30s ping/60s timeout |

---

## §4 实施清单

### §4.1 阶段 P0:Pre-flight

**P0.1** 抄段隔离脚本

```bash
cp tmp/r6_a_3_segment_audit.sql tmp/r6_a_4_segment_audit.sql
cp tmp/r6_a_3_segment_clean.sql tmp/r6_a_4_segment_clean.sql
cp tmp/r6_a_3_isolation_run.sh  tmp/r6_a_4_isolation_run.sh
sed -i 's/60000/70000/g; s/69999/79999/g' tmp/r6_a_4_segment_audit.sql tmp/r6_a_4_segment_clean.sql tmp/r6_a_4_isolation_run.sh
```

> Audit/clean 表清单**扩展为 9 表**:在 R6.A.3 的 7 表(tasks/task_modules/task_module_events/task_assets/task_drafts/notifications/permission_logs)基础上 **加 users + org_move_requests 两表**(本轮 retro 触及全 R 轮 · 必须覆盖 SA-B 段)

**P0.2** 跑 `bash tmp/r6_a_4_isolation_run.sh` · BEFORE/AFTER 9 表全 0(否则 ABORT § 0.2 #7)· 输出存 `tmp/r6_a_4_isolation_pre.log`

**P0.3** sha256 baseline 守卫

```bash
sha256sum docs/api/openapi.yaml service/asset_lifecycle/cleanup_job.go service/task_draft/service.go service/task_lifecycle/auto_archive_job.go repo/mysql/task_auto_archive_repo.go domain/task.go cmd/server/main.go > tmp/r6_a_4_baseline_sha.log
# 必须含 docs/api/openapi.yaml 行 = b3d7c3651ea2496a6e4ea1a948772c6a395d6b387bf6c4509e5c26477c75dd0f
```

### §4.2 阶段 P1:全栈 verify(只读)

> 全部用 WSL native go(`/home/wsfwk/go/bin/go`)+ 测试库 DSN · 与 R6.A.3 验证脚本同环境

**P1.1** build/vet

```bash
go build ./... 2>&1 | tee tmp/r6_a_4_build.log     # 0 error
go vet ./...   2>&1 | tee tmp/r6_a_4_vet.log       # 0 error
go build -tags=integration ./... 2>&1 | tee tmp/r6_a_4_build_integration.log
go build ./cmd/server  2>&1 | tee tmp/r6_a_4_build_server.log
```

**P1.2** 全栈 unit

```bash
go test -count=1 ./... 2>&1 | tee tmp/r6_a_4_unit.log
# 末行必须 ok 全绿 / 0 FAIL
```

**P1.3** 全栈 integration(SAAI/SABI/SACI/SADI/SAEI/SABI.1/SACI.1/R6A1/R6A2/R6A3)

> **重要**:必须用 `-p 1` 串行跑全栈 · 不能用默认并行(R6.A.4 v1.0 因为漏写 `-p 1` 命中段污染伪退化 ABORT · 架构师 v1.1 修补)。
> 多个 integration 包共用 `[20000,80000)` 测试段(SA-A 段 [20000,30000) / SA-B 段 [30000,40000) / SA-C 段 [40000,50000) / SA-D 段 [50000,60000) / R5 段 [50000,60000) / R6.A.1 段 [50000,60000) / R6.A.3 段 [60000,70000) / R6.A.4 段 [70000,80000))。
> 默认 `go test ./...` 包级并行(GOMAXPROCS 个包同时跑)· 跨包同段会互相污染 fixture · 出现:
> - `cmd/tools/run-cleanup` R6A3 测试 vs `service/task_lifecycle` R6A3 测试同 [60000,70000) 段并发 → `audit table tasks has 1 rows`、`scanned/archived = 21/0 want 8/0`
> - `migrate_v1_backfill` R2 smoke `task_id=50010` vs `cmd/tools/run-cleanup` R6A1 同 [50000,60000) 段并发 → `module not found after insert`
> - `service/search` SADI11 P95 测试与全栈 DB 压力 noisy neighbor → `context deadline exceeded`
>
> 同时 `go test ... | tee ...` 会被 shell pipeline 掩盖 `go test` 非零退出码 · 必须 `set -o pipefail` 守 EXIT。

```bash
export MYSQL_DSN='root:<TEST_DB_PASSWORD>@tcp(127.0.0.1:3306)/jst_erp_r3_test?parseTime=true&multiStatements=true&loc=Local'
export R35_MODE=1

set -o pipefail
go test -tags=integration -p 1 -count=1 -timeout 45m ./... 2>&1 | tee tmp/r6_a_4_integration.log
test ${PIPESTATUS[0]} -eq 0 || echo "ABORT integration FAIL"

# 必须 0 FAIL · 关键包:
#   service/asset_lifecycle              (SA-A I1~I8)
#   service/asset_lifecycle/scheduler    (R6.A.2)
#   service/task_lifecycle               (R6.A.3 + 既有)
#   service/task_draft                   (SA-C cleanup)
#   service/notification                 (SA-C 5 类)
#   service/org_move_request             (SA-B)
#   transport/handler                    (SAAI/SABI/SACI/SADI/SAEI/SABI.1/SACI.1)
#   cmd/tools/run-cleanup                (R6.A.1 + R6.A.3)
#   cmd/tools/migrate_v1_backfill        (R2 smoke)
#   service/search                       (SA-D + SADI11 P95)
```

> **架构师 v1.1 实证基线**(2026-04-25):清 [50000,80000) 段污染累积 + `-p 1` 串行跑 = **295s 全栈 0 FAIL**(`migrate_v1_backfill 35.162s ok` + `run-cleanup 171.260s ok` + `service/search 15.950s ok` + `service/task_lifecycle 8.498s ok` + 其余 ~30 包 全 ok)· 这是 codex 应能复现的硬基线。

**P1.4** OpenAPI 完整性

```bash
# openapi-validate 0/0
openapi-validate docs/api/openapi.yaml 2>&1 | tee tmp/r6_a_4_openapi_validate.log

# dangling 501 = 0(允许 schema 内 description 提及 501 但不允许 path 级 501 无 R6+ 接管)
grep -nE '^\s*"501":' docs/api/openapi.yaml > tmp/r6_a_4_501_paths.log || true
test $(wc -l < tmp/r6_a_4_501_paths.log) -eq 0   # 0 行 = 0 dangling

# sha256 锚定不变
sha256sum docs/api/openapi.yaml | grep -q 'b3d7c3651ea2496a6e4ea1a948772c6a395d6b387bf6c4509e5c26477c75dd0f' || { echo ABORT openapi sha drift; exit 1; }
```

### §4.3 阶段 P2:主 §13 A1~A12 验证

**P2.1** 写 `tmp/r6_a_4_a_matrix_test.go`(integration build tag)· 段 [70000,79999) · 程序化覆盖 A1/A2/A4/A5/A7/A10/A11(7 项)

骨架(完整代码 codex 自填):

```go
//go:build integration

package retro_test

import (
    "context"
    "database/sql"
    "net/http"
    "net/http/httptest"
    "sync"
    "sync/atomic"
    "testing"
    // ...
)

const (
    a1TaskID  = 70001
    a2TaskID  = 70002
    a2DeletedTaskID = 70003
    a4TaskID  = 70004
    a5TaskID  = 70005
    a7TaskID  = 70007
    a10TaskID = 70010
    // 用户 ID 段
    a1Users = 70010 // [70010,70019] 4 角色
)

func TestRetro_A1_AllAuthenticatedUsersSeeUndeletedTasks(t *testing.T) { /* seed → 4 token GET → 4 都返 200 + 含 a1TaskID */ }
func TestRetro_A2_TaskDetailReturns200(t *testing.T) { /* seed soft-deleted → GET detail 200 · seed hard-delete → 404 */ }
func TestRetro_A4_ModuleActionStateMismatchAndRoleDenied(t *testing.T) { /* basic_info pending → trigger audit_approve → module_state_mismatch · designer trigger pool-reassign → module_action_role_denied */ }
func TestRetro_A5_Claim100ConcurrentExactlyOneSucceeds(t *testing.T) { /* 100 goroutine claim 同一 pending_claim · 1 成功 99 returns module_claim_conflict */ }
func TestRetro_A7_PoolReassign_DeptAdminAllowed_OthersDenied(t *testing.T) { /* dept_admin 200 · designer/auditor/team_lead 403 module_action_role_denied */ }
func TestRetro_A10_TaskDetailOneShot6Modules(t *testing.T) { /* 单次 GET detail 返 6 modules · 创建者可写 basic_info · 非接单者 design 只读 */ }
func TestRetro_A11_NoNonDeprecatedAuditTaskAction(t *testing.T) {
    // grep "TaskAction.*=\s*TaskAction\s*\"audit_" 不出现在非 deprecated 文件
    // 用 filepath.Walk + bufio
    out, _ := exec.Command("grep", "-rEn", `TaskAction.*=\s*TaskAction\s*"audit_`, "service/", "transport/", "domain/").Output()
    // 每行必须包含 "deprecated"
}
```

**段隔离**:`t.Cleanup` 用 `truncate WHERE id BETWEEN 70000 AND 79999`(沿用 R6.A.3 模板 · 切勿用 `defer db.Close()` 顺序 bug)

**P2.2** 跑

```bash
go test -tags=integration -count=1 -run '^TestRetro_A' ./tmp/... 2>&1 | tee tmp/r6_a_4_a_matrix.log
# 7 用例全 PASS · 0 FAIL
```

**P2.3** 文档 traceability(A3/A6/A8/A9/A12 5 项 retro `§A 矩阵` 节直接列出)

| 编号 | retro 写法 |
| ---: | --- |
| A3 | "实装见 `service/permission/authorize_module_action.go` · 单测 `service/permission/authorize_module_action_test.go` · 与 §6 Layer 2 表 6 角色 × 6 模块 = 36 cell 100% 对齐(R3 报告 §A3 已硬验)" |
| A6 | "实装见 `service/task_pool/pool_query.go` · R3.5 6 条集成断言(SA-A I3 + R3.5-I4)实证常规池仅含 design_standard/audit_standard 组员 + 定制池仅含 customization 组员" |
| A8 | "R2 backfill 后 staging 200 条 task_id 抽样 task_status 列与 aggregator 计算结果 100% 一致(R2 报告 §6 smoke 第 3 断言)· retro `§A8` 节再抽 200 条重验:实测一致率 X%" |
| A9 | "R2 报告 §7 dry-run 已演练 · staging 演练在主 §13 R2 验收时完成 · 本轮不再实测(staging 环境本轮无访问权限)" |
| A12 | "R5 决策:v1 事件流出口为 'task_module_events 直查' · Kafka/本地 consumer 出口推迟 R7+(主 §12 R5 行 + §15 U1/U2)· retro 标 v1 设计兼容 A12 · 真消费器实装 V2" |

### §4.4 阶段 P3:cmd/server live smoke F1~F6

> 沿用 R5 retro 跑过的 F1~F6 模板(参 `docs/iterations/V1_R4_RETRO_REPORT.md` + `V1_R5_BATCH_SKU_REPORT.md` §9 smoke 节)

**P3.1** 启 cmd/server(后台 + 测试 DSN)

```bash
export MYSQL_DSN='root:<TEST_DB_PASSWORD>@tcp(127.0.0.1:3306)/jst_erp_r3_test?parseTime=true&multiStatements=true&loc=Local'
PORT=18086 setsid /home/wsfwk/go/bin/go run ./cmd/server > tmp/r6_a_4_smoke_server.log 2>&1 &
echo $! > tmp/r6_a_4_smoke.pid
sleep 5
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:18086/healthz   # 200
```

**P3.2** F1~F6(沿用 R5 模板 · 锚定每条预期)

| F# | 路径 | 预期 |
| ---: | --- | --- |
| F1 | `GET /v1/tasks/batch-create/template.xlsx?task_type=npd_basic` | 200 + Content-Type=`application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` + body > 5KB |
| F2 | `GET /v1/tasks/batch-create/template.xlsx?task_type=pt_basic` | 200 + > 5KB |
| F3 | `GET /v1/tasks/batch-create/template.xlsx?task_type=original_product_development` | 400 + body 含 `batch_not_supported_for_task_type` |
| F4 | `POST /v1/tasks/batch-create/parse-excel` (multipart with valid xlsx) | 200 + body 含 `"violations":[]` |
| F5 | `POST /v1/tasks/batch-create/parse-excel` (no file) | 400 + body 含 `file is required` |
| F6 | `GET /v1/tasks` (无 token) | 401 |

**P3.3** 关闭 cmd/server

```bash
kill -TERM $(cat tmp/r6_a_4_smoke.pid) || true
sleep 3
kill -KILL $(cat tmp/r6_a_4_smoke.pid) 2>/dev/null || true
```

### §4.5 阶段 P4:**P99 `/v1/tasks/{id}/detail` < 150ms 强验**(主 §12 R6 验收门)

> 至今未实测的硬门 · 本轮决定 R6.A.5 走"评估题"还是"实装题"

**P4.1** 数据准备

- 用测试库 `jst_erp_r3_test`
- 选 100 个非 deleted task_id(用 `SELECT id FROM tasks WHERE deleted_at IS NULL ORDER BY RAND() LIMIT 100`)
- 写 `tmp/r6_a_4_p99_task_ids.txt`

**P4.2** 启 cmd/server(用 §4.4 P3.1 流程 · 同 PORT 18086)+ seed 一个**有效 super_admin token**(用既有 `tmp/seed_super_admin.sh` 或新写 helper)

**P4.3** 跑 `tmp/r6_a_4_p99_runner.go`(codex 自写 · 100 task_id × 1 GET = 100 次 + warm-up 10 次)

骨架:

```go
//go:build ignore
package main

import (
    "fmt"
    "io"
    "net/http"
    "os"
    "sort"
    "time"
)

func main() {
    ids := readIDs("tmp/r6_a_4_p99_task_ids.txt")
    token := os.Getenv("SUPER_ADMIN_TOKEN")
    client := &http.Client{Timeout: 5 * time.Second}
    // warm-up 10
    for i := 0; i < 10 && i < len(ids); i++ { do(client, token, ids[i]) }
    durs := make([]time.Duration, 0, len(ids))
    for _, id := range ids {
        start := time.Now()
        if !do(client, token, id) { panic("non-200 on " + id) }
        durs = append(durs, time.Since(start))
    }
    sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
    p50 := durs[len(durs)/2]
    p95 := durs[len(durs)*95/100]
    p99 := durs[len(durs)*99/100]
    fmt.Printf("n=%d p50=%v p95=%v p99=%v max=%v\n", len(durs), p50, p95, p99, durs[len(durs)-1])
}

func do(c *http.Client, token, id string) bool {
    req, _ := http.NewRequest("GET", "http://127.0.0.1:18086/v1/tasks/"+id+"/detail", nil)
    req.Header.Set("Authorization", "Bearer "+token)
    resp, err := c.Do(req)
    if err != nil { return false }
    defer resp.Body.Close()
    io.Copy(io.Discard, resp.Body)
    return resp.StatusCode == http.StatusOK
}

func readIDs(path string) []string { /* ... */ }
```

**P4.4** 跑

```bash
SUPER_ADMIN_TOKEN=$(cat tmp/r6_a_4_super_admin_token.txt) /home/wsfwk/go/bin/go run tmp/r6_a_4_p99_runner.go 2>&1 | tee tmp/r6_a_4_p99.log
```

**P4.5** retro 报告 `§性能` 节填表 · 判定:

- p99 < 100ms → "GREEN · 主 §12 R6 P99 验收门通过 · derived_status 物化列与视图物化建议:不需要"
- 100ms ≤ p99 < 150ms → "AMBER · 通过门槛但 headroom 小 · 建议 V1.1 加 `(task_status, updated_at)` + `(actor_id)` 复合索引"
- p99 ≥ 150ms → "RED · 主 §12 R6 P99 门 fail · 必须 V1.1 实施视图物化 + derived_status 物化列 · handoff manifest 标 BLOCKER"

> ⚠️ p99 ≥ 150ms **不**触发 ABORT(retro 还要写) · 但 retro `§性能` 节必须显式标 RED + handoff `§必修` 节登记。

### §4.6 阶段 P5:V0.9 → V1 schema diff

**P5.1** 列 12 张 v1_0 migration 文件 + R1.5/R1.6 决策(`docs/iterations/V1_R1_5_DDL_ALIGNMENT.md` + `V1_R1_6_PROD_ALIGN.md`)· retro `§schema` 节 1 表归纳

| migration | 表 | 关键字段 | 来源轮 |
| --- | --- | --- | --- |
| 057 | org_master | dept/team/role 收敛 | R2 phase A |
| 058 | org_team | dept-scoped uniqueness | R2 phase A |
| 059 | task_modules | 6 module 行/任务 | R2 phase B |
| 060 | task_module_events | 事件流主表 | R2 phase B |
| 061 | task_assets.source_module_key | 5 值 enum 加列 | R2 phase B |
| 062 | reference_file_refs (flat) | 引用平表化 | R2 phase B |
| 063 | task_drafts | 草稿表 | R2 phase B |
| 064 | notifications | 5 类通知 | R2 phase B |
| 065 | org_move_requests | 跨部门调动 | R2 phase B |
| 066 | task_assets_lifecycle | 5 态机 | R2 phase B |
| 067 | tasks.priority | 4 值 CHECK + 复合索引 | R2 phase B + R1.6 |
| 068 | task_customization_orders | 定制订单 | R2 phase C |

**P5.2** 验证生产 + 测试库 schema 与上述 12 张 migration 一致(`SHOW CREATE TABLE` 抽查)

```bash
ssh -o ControlMaster=no -o ControlPath=none jst_ecs \
  '. /root/ecommerce_ai/shared/main.env && export MYSQL_PWD="$DB_PASS" && \
   mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" "$DB_NAME" -e "SHOW CREATE TABLE task_modules\G; SHOW CREATE TABLE task_module_events\G; SHOW CREATE TABLE task_assets\G; SHOW CREATE TABLE notifications\G; SHOW CREATE TABLE task_drafts\G; SHOW CREATE TABLE org_move_requests\G; SHOW CREATE TABLE task_customization_orders\G"' \
  > tmp/r6_a_4_prod_schema.log 2>&1
```

**P5.3** retro `§schema` 节列 schema diff 结论(应为 0 漂移 · R2 backfill 已对齐)

### §4.7 阶段 P6:4 控制字段 post-probe(沿用 R4-Retro 模板)

**P6.1** 跑只读 SQL(段 [70000,79999) 不含本轮新写 + 全表聚合)

```sql
-- task_assets 控制字段(SA-A v2.1)
SELECT 'is_archived_dirty' AS k, COUNT(*) FROM task_assets WHERE is_archived NOT IN (0,1);
SELECT 'cleaned_at_dirty' AS k, COUNT(*) FROM task_assets WHERE cleaned_at IS NOT NULL AND lifecycle_state != 'auto_cleaned';
SELECT 'deleted_at_dirty' AS k, COUNT(*) FROM task_assets WHERE deleted_at IS NOT NULL AND lifecycle_state != 'deleted';
-- notification_type 5 值闭环(SA-C)
SELECT 'notif_type_alien' AS k, COUNT(*) FROM notifications WHERE notification_type NOT IN ('task_assigned_to_me','task_rejected','claim_conflict','pool_reassigned','task_cancelled');
```

**P6.2** 4 列结果必须全 0 · 写入 `tmp/r6_a_4_control_fields_probe.log` · retro `§控制字段` 节嵌入

### §4.8 阶段 P7:retro 报告起草

**`docs/iterations/V1_RETRO_REPORT.md` 骨架**(codex 必填章节):

```markdown
# V1 Backend Retro Report · 2026-04-25

> 范围:R1 (2026-04-17) ~ R6.A.3 (2026-04-25) · 17 签字轮 · V0.9 → V1 后端重构整体回顾
> 性质:架构师独立 verify 闭环报告 + 主 §13 A1~A12 验收实证 + 性能加压基线 + 已知遗留 + V1.1/V2 路线建议

## §1 V1 全貌(2 段简述)
- 范围 / 不在范围(前端工程独立仓 · backend-only)
- 整体节奏:R1 契约冻结 → R1.5/R1.6 字段对齐 → R2 数据落地 → R3 引擎 → R4 P3 4 轮 → R5 Excel 二件套 → R6 性能与治理(A.1~A.3 闭环 · A.4 本报告)

## §2 17 轮 timeline 表
- 17 行 · 列 [date / round / file / verdict / scope / 关键产物 / 报告 link]

## §3 主 §13 A1~A12 验收实证
- 12 行表 · 列 [编号 / 项 / 实施类别(程序化/文档)/ 实证 / 结果]
- 程序化 7 项跑 P2.2 实测;文档 5 项 traceability
- 总判定:**ALL GREEN / X RED**

## §4 全栈 verify 矩阵
- §4.1 build/vet/build cmd/server 双 tag(P1.1)
- §4.2 全栈 unit(P1.2)
- §4.3 全栈 integration(P1.3 · 包级别 PASS 表)
- §4.4 OpenAPI 完整性(P1.4)

## §5 性能加压基线
- §5.1 §3.4 表完整版 + p99 detail 实测填表(P4.5 verdict)
- §5.2 derived_status 物化列评估(基于 P4 实测数字)
- §5.3 视图物化 / 索引优化建议(基于 P4 verdict + grep 慢查询 explain 抽样)

## §6 V0.9 → V1 schema diff
- 12 migration 表(P5.1)
- 生产 + 测试库 schema 一致性(P5.2)

## §7 cmd/server live smoke F1~F6
- P3.2 矩阵(实测填表)

## §8 控制字段 post-probe(SA-A 3 列 + SA-C 1 列)
- P6.2 矩阵 · 4 列全 0

## §9 Codex/架构师协作模型回顾
- §9.1 R3 起 codex exec autopilot 路径(R3 → R3.5 → SA-A → SA-A Patch-A2 → SA-A Patch-A3 → SA-B → SA-C → SA-C.1 → SA-D → R5)
- §9.2 R6 起 codex TUI 原子 prompt 路径(A.1 → A.2 → A.3 → A.4 本轮)
- §9.3 选型理由 · 教训(段污染 / 测试 t.Cleanup 顺序 / WSL native go vs Windows Device Guard 边界 / cron gate ENV 默认 off)

## §10 已知遗留 / 技术债清单
- §10.1 V1 已知遗留(主 §15 U1~U6 + R6.A.4 后续:索引优化 / 视图物化 / §9.3 兼容路由下线)
- §10.2 prompt 缺陷固化教训(R6.A.2 prompt §1.3 误称 scheduler 无 tests / R6.A.3 prompt 把 t.Cleanup 顺序 bug 列模板防护)
- §10.3 文档对齐性扫描(本轮 implicit 发现的 V1 文档与代码不符项 · 转 V2 必修)

## §11 V1.1 + V2 路线建议
- §11.1 V1.1(可选 · 性能门后续优化):索引优化 / 视图物化 / §9.3 兼容路由真下线
- §11.2 V2(下一代):U1~U6 实装(超时升级 / L2/L3 报表 / blueprint 外置 / actor_org_snapshot 维度 / SLA 自动升级)+ Kafka/本地 consumer 出口(主 §12 R7+ A12 实装)

## §12 数据隔离与测试库基线
- 段策略汇总([20000,80000) 6 段)
- 测试库 jst_erp_r3_test 基线 + 段污染防护历史

## §13 致谢(可选)+ V1 完成签字
- 架构师签字行 + 终止符候选

## §14 终止符
R6_A_4_DONE_PENDING_ARCHITECT_VERIFY
```

> 报告**总长 ≥ 200 行**(retro 内容厚 · 不能写薄了)

### §4.9 阶段 P8:V1 → V2 model handoff manifest 起草

**`docs/V1_TO_V2_MODEL_HANDOFF_MANIFEST.md` 骨架**(类比 `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`):

```markdown
# V1 → V2 Backend Model Handoff Manifest

> 用途:为接管 V1 后端的下一代模型/前端工程 / V2 重构架构师提供权威指引
> 范围:V1 整体重构闭环态(R6.A.3 PASS · 2026-04-25)的对外 contract 与必读文档清单
> 风格:对齐 `docs/V0_9_MODEL_HANDOFF_MANIFEST.md` 五段(authority / current state / reading order / non-authoritative / working rule)

## §1 Authority Order
1. `docs/V1_MODULE_ARCHITECTURE.md` v1.3(主权威)
2. `docs/V1_INFORMATION_ARCHITECTURE.md`(IA 权威)
3. `docs/V1_CUSTOMIZATION_WORKFLOW.md`(定制工作流权威)
4. `docs/V1_ASSET_OWNERSHIP.md`(资产权威)
5. `docs/api/openapi.yaml`(API contract · sha b3d7c365…dd0f)
6. `transport/http.go`(实际挂载真源)

## §2 Current Repo Baseline(R6.A.3 闭环态)
- main = V1.0 · 17 轮签字
- API surface(stable):
  - `/v1/auth/*`
  - `/v1/users*` + `/v1/me*`
  - `/v1/erp/products*` + `/v1/erp/products/by-code`
  - `/v1/tasks*` + `/v1/tasks/{id}/asset-center/*` + `/v1/tasks/batch-create/*`
  - `/v1/notifications*` + `/v1/task-drafts*` + `/v1/ws/v1`
  - `/v1/org-move-requests*` + `/v1/users/{id}/{activate|deactivate|delete}`
  - `/v1/search` + `/v1/reports/l1/*`
- API surface(deprecated · 待 R7+ 真下线):
  - 列出 §9.3 兼容路径清单(`/v1/tasks/{id}/audit_a_claim` 等)+ deprecated date
- cron gate(默认 off):
  - `ENABLE_CRON_OSS_365=1` · `0 4 * * *`
  - `ENABLE_CRON_DRAFTS_7D=1` · `0 3 * * *`
  - `ENABLE_CRON_AUTO_ARCHIVE=1` · `0 5 * * *`

## §3 Reading Order(下一代模型/前端工程师必读)
1. `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`(V0.9 baseline)
2. **本文**(V1 落地 baseline)
3. `docs/V1_MODULE_ARCHITECTURE.md` v1.3 §12 R 路线 + §13 A1~A12 + §17 决策清单
4. `docs/V1_INFORMATION_ARCHITECTURE.md` §3.5(Excel 二件套)+ §9.1(任务作废/关闭)
5. `docs/api/openapi.yaml`
6. `transport/http.go`
7. 最近 retro:`docs/iterations/V1_RETRO_REPORT.md`(本轮产物)
8. R6.A.1/A.2/A.3 报告(cron 启用与归档 job)

## §4 V1 → V2 应继承的核心契约
- §4.1 module 6 model(basic_info / customization / design / audit / warehouse + 派生 closed)
- §4.2 ModuleAction 三层授权(scope / role / state · 主 §6)
- §4.3 task lifecycle 状态集 · auto-archive 90 天 · OSS 365 天 · 草稿 7 天
- §4.4 段隔离规则([20000,80000) 段制度)
- §4.5 通知 5 类闭环(task_assigned_to_me / task_rejected / claim_conflict / pool_reassigned / task_cancelled)
- §4.6 报表 SuperAdmin-only · L1 实时 · v1 直查 task_module_events JOIN(SA-D 校正)
- §4.7 cron 默认 off · ENV gate · 不污染冷启动

## §5 V1 已知遗留 + V1.1/V2 路线
- §5.1 V1 主 §15 U1~U6 未决项映射:U1(超时升级)/ U2(L2/L3 报表)/ U3(blueprint 外置)/ U4(actor_org_snapshot 冻结点)/ U5(取消)/ U6(SLA 自动升级)
- §5.2 V1.1 可选优化(基于本轮 P99 实测):索引优化 / 视图物化 / §9.3 真下线
- §5.3 V2 应实装:Kafka/本地 consumer 出口(主 §12 R7+ A12)

## §6 Working Rule(下一代模型必须遵守)
- 任何 V1 已签字 contract(API/枚举/状态机)修改前 · 必须先发架构 ADR 走 R 流程(R7+/R8+ 路线)
- 测试库 + 段隔离规则不可绕过
- 文档冲突时:transport/http.go > openapi.yaml > 4 份权威文档 > V1 retro 报告

## §7 Non-Authoritative Materials(下一代必须区分)
- `CURRENT_STATE.md` / `MODEL_HANDOVER.md`:索引型历史入口 · 不当 spec
- `docs/archive/*` / `docs/iterations/*`(除最近 retro 外):证据/归档 · 不当 spec
- 早期 v0.9 specs / model_memory · 任何与 V1 OpenAPI 冲突的 v0.9 描述以本文档优先

## §8 V1 闭环态 sha 锚点
- openapi.yaml: b3d7c3651ea2496a6e4ea1a948772c6a395d6b387bf6c4509e5c26477c75dd0f
- 关键业务文件 sha(见本文档配套 `tmp/r6_a_4_baseline_sha.log`)
```

> handoff manifest 总长 ≥ 100 行 · 风格对齐 V0.9 同名文档 · 不写实装细节 · 只写权威指引

### §4.10 阶段 P9:ROADMAP 同步起草(不自签)

**P9.1** 编辑 `prompts/V1_ROADMAP.md`:

- 状态行更新:从 "v1 · ... + R6.A.3 任务自动归档 job 全部签字闭环 · R6.A.4 候选(...)待起草" → "... + R6.A.3 + R6.A.4 retro 起草中 · 待架构师 verify"
- §32 R6.A.4 行**新增**:status="prompt v1.0 已起草 · codex 已落 retro + handoff 双产物 · 待架构师独立 verify"
- §变更记录 v25 起草(**不自签 PASS** · 留架构师补 verdict 段)

**P9.2** 不要碰 §32 R6.A.1/A.2/A.3 行(已签字闭环态)

### §4.11 阶段 P10:终止符

写完所有产物 + verify 全 PASS · 输出最后一行:

```
R6_A_4_DONE_PENDING_ARCHITECT_VERIFY
```

---

## §5 白名单与禁止改动

### §5.1 允许新建

| 路径 | 用途 |
| --- | --- |
| `docs/iterations/V1_RETRO_REPORT.md` | retro 主报告 |
| `docs/V1_TO_V2_MODEL_HANDOFF_MANIFEST.md` | handoff manifest |
| `tmp/r6_a_4_*.sh` / `tmp/r6_a_4_*.sql` / `tmp/r6_a_4_*.go` / `tmp/r6_a_4_*.log` / `tmp/r6_a_4_*.txt` | 实测脚本 + 日志 |

### §5.2 允许编辑

| 路径 | 改动范围 |
| --- | --- |
| `prompts/V1_ROADMAP.md` | §32 R6.A.4 行 + 状态行 + §变更记录 v25 起草(不自签 PASS) |

### §5.3 禁止改动(任一改动触发 ABORT §0.2 #4/#5/#6)

- 任何 `*.go` 文件(`service/` / `repo/` / `transport/` / `domain/` / `cmd/server/main.go` / `cmd/tools/run-cleanup/main.go` 等)
- `docs/api/openapi.yaml`
- `db/migrations/*.sql`
- `docs/V1_MODULE_ARCHITECTURE.md`
- `docs/V1_INFORMATION_ARCHITECTURE.md`
- `docs/V1_CUSTOMIZATION_WORKFLOW.md`
- `docs/V1_ASSET_OWNERSHIP.md`
- `docs/V1_0_FRONTEND_INTEGRATION_GUIDE.md`
- 既有 14 个签字报告(`docs/iterations/V1_R*_REPORT.md` · 仅引用 · 不修订)

> 例外:`tmp/r6_a_4_a_matrix_test.go` 用 `// +build integration` 或 `//go:build integration` tag · 但**不放在 service/transport 包内** · 必须放 `tmp/` 子目录(避免污染主代码树)· 这是 retro 验证脚本不是业务代码

---

## §6 数据隔离

- 段:`task_id / user_id / asset_id / org_move_request_id ∈ [70000, 80000)`
- 9 表 audit + clean(`tmp/r6_a_4_isolation_run.sh`):
  - `users` / `tasks` / `task_modules` / `task_module_events` / `task_assets`
  - `task_drafts` / `notifications` / `org_move_requests` / `permission_logs`
- `t.Cleanup` 模板(沿用 R6.A.3 防 `defer db.Close()` 顺序 bug):

```go
t.Cleanup(func() {
    cleanupR6A4Segment(t, db)
    assertR6A4SegmentClean(t, db)
    _ = db.Close() // 必须在最后 · 不能 defer
})
```

- `permission_logs` 段 BEFORE 可能 ≠ 0(测试库历史 baseline drift · R6.A.3 已实证),AFTER 必须 = 0(本轮不 INSERT permission_logs)

---

## §7 验证(verify)11 步

**step 1**:`bash tmp/r6_a_4_isolation_run.sh`(P0.2)· BEFORE/AFTER 9 表全 0(允许 permission_logs BEFORE drift · AFTER 必 0)
**step 2**:`tmp/r6_a_4_baseline_sha.log` 与 §3.2 锚点一致(P0.3)
**step 3**:`go build ./... + vet + build -tags=integration + build cmd/server` 全 PASS(P1.1)
**step 4**:`go test -count=1 ./...` 全 PASS(P1.2)
**step 5**:`set -o pipefail && go test -tags=integration -p 1 -count=1 -timeout 45m ./...` 全 PASS(P1.3 · v1.1 必须 `-p 1` 串行)
**step 6**:`openapi-validate` 0/0 + 0 dangling 501 + sha 不变(P1.4)
**step 7**:`go test -tags=integration -count=1 -run '^TestRetro_A' ./tmp/...` 7/7 PASS(P2.2)
**step 8**:cmd/server live smoke F1~F6 全预期(P3.2)
**step 9**:`tmp/r6_a_4_p99_runner.go` 跑 100 次 detail · p50/p95/p99 入 retro `§性能` 节(P4.5)
**step 10**:控制字段 post-probe 4 列全 0(P6.2)
**step 11**:`bash tmp/r6_a_4_isolation_run.sh` 再跑(step-N)· 9 表 AFTER 全 0(retro `§段隔离` 节嵌入 BEFORE/AFTER 双 snapshot)

---

## §8 报告产出与 commit 节奏

- 写完 retro `docs/iterations/V1_RETRO_REPORT.md`(≥ 200 行)
- 写完 handoff `docs/V1_TO_V2_MODEL_HANDOFF_MANIFEST.md`(≥ 100 行)
- 编辑 `prompts/V1_ROADMAP.md` 加 §32 R6.A.4 行 + v25 起草段(留架构师补签 verdict)
- 不要 git commit(workspace 非 git 仓 · 直接落盘即可)
- 末尾输出 `R6_A_4_DONE_PENDING_ARCHITECT_VERIFY`

---

## §9 ABORT 检测清单(每步可重入检查)

| 步 | 守卫 | 触发 ABORT |
| ---: | --- | --- |
| 1 | 段 audit 9 表 BEFORE 漂移(除 permission_logs)| §0.2 #7 |
| 2 | sha 锚点漂移 | §0.2 #4 / #5 |
| 3 | build/vet 任一非 0 退出 | §0.2 #4 |
| 4 | 全栈 unit FAIL | §0.2 #9 |
| 5 | 全栈 integration FAIL | §0.2 #8 |
| 6 | openapi-validate ≠ 0/0 / 501 dangling > 0 / sha 漂移 | §0.2 #2 / #3 / #5 |
| 7 | A 矩阵 7 项任一 FAIL | §0.2 #1 |
| 8 | live smoke F1~F6 任一非预期 | report §smoke RED 但不 ABORT(R5 retro 已实证 PASS · 本轮重跑做基线验证) |
| 9 | p99 ≥ 150ms | **不 ABORT** · retro §性能 RED + handoff §必修 登记 |
| 10 | 控制字段任一非 0 | report §控制字段 RED · 但若仅是 SA-A 历史 baseline 累积(`is_archived=1` 来源 R4 真生产 backfill)则 retro 标 AMBER 不 ABORT |
| 11 | step-N audit 漂移(本轮新写)| §0.2 #7 |

---

## §10 工作时长预算 + Codex 自我节奏

| 阶段 | 预估 | 主要 | 可并行 |
| --- | ---: | --- | --- |
| P0 pre-flight | 5min | 段隔离 + sha baseline | — |
| P1 全栈 verify | 30~60min | 全 unit / integration / openapi | unit + integration 串行 · openapi 并行 |
| P2 A 矩阵 | 60~90min | 7 用例编写 + 跑 | A1~A7 各独立 · 可并写 |
| P3 live smoke | 15min | 启 server / F1~F6 / 关 server | — |
| P4 P99 加压 | 30min | seed token + runner | — |
| P5 schema diff | 20min | SHOW CREATE 抽样 | — |
| P6 控制字段 | 10min | 4 SQL | — |
| P7 retro 起草 | 60~90min | 14 节填表 + 嵌实测 | — |
| P8 handoff 起草 | 30~60min | 8 节 | — |
| P9 ROADMAP | 15min | §32 R6.A.4 行 + v25 段 | — |
| P10 终止符 | 1min | 输出最后一行 | — |
| **合计** | **~3-4h** | | |

> 任一阶段超预算 30min · 优先报告 + ABORT;不要为追求完美卡住 retro 出报告。

---

## §11 终止符规则(再次强调)

- 全部 verify PASS → 最后一行输出 `R6_A_4_DONE_PENDING_ARCHITECT_VERIFY` · 等架构师独立 verify
- 任一 ABORT 触发 → 最后输出 ABORT 原因 + 已完成阶段清单 · **不**输出 `_DONE_PENDING_ARCHITECT_VERIFY`
- 终止符前**不要自签 PASS**(架构师签字才闭环 · 类比 R6.A.1/A.2/A.3 模板)

---

> Prompt 起草:架构师 · 2026-04-25
> 待 codex TUI 开工 · 完工标志 = `R6_A_4_DONE_PENDING_ARCHITECT_VERIFY`
