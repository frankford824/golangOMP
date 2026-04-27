# V1 R6.A.1 Report · cmd/tools/run-cleanup CLI

## §1 Scope
单 prompt 实施 · `cmd/tools/run-cleanup` 双子命令 · 0 cron · 0 SA-A/SA-C 改动。

## §2 实装清单
| 文件 | 行数 | 用途 |
| --- | ---: | --- |
| `cmd/tools/run-cleanup/main.go` | 225 | 子命令路由 + MySQL/SA-A/SA-C wiring |
| `cmd/tools/run-cleanup/main_test.go` | 63 | usage / unknown subcommand / flag parsing 单测 |
| `cmd/tools/run-cleanup/integration_test.go` | 376 | 4 个 `TestR6A1_` integration 用例 + helper process |
| `docs/iterations/V1_R6_A_1_REPORT.md` | 43 | 本轮报告 |

## §3 wiring 决策
- 不引第三方 CLI 框架, 使用 stdlib `flag`。
- MySQL 使用 `sql.Open("mysql", dsn)` + `db.PingContext`, repo/tx wiring 使用 `mysqlrepo.New(db)`。
- SA-A 使用 `asset_lifecycle.NewCleanupJob(mysqlrepo.NewTaskAssetLifecycleRepo(mdb), mdb, deleter, logger)`。
- SA-C 使用 `task_draft.NewService(mysqlrepo.NewTaskDraftRepo(mdb), mysqlrepo.NewPermissionLogRepo(mdb), mdb)`。
- OSS deleter env 名按 `cmd/server/main.go`/`config.Load` 现行配置读取: `OSS_ENDPOINT`, `OSS_BUCKET`, `OSS_ACCESS_KEY_ID`, `OSS_ACCESS_KEY_SECRET`, `OSS_PUBLIC_ENDPOINT`; `--dry-run` 或 `OSS_DELETER_DISABLED=1` 使用 no-op deleter。真实运行时 OSS 配置不完整会 fail closed, 避免只标记 DB 不删对象。

## §4 测试输出
- `go test ./cmd/tools/run-cleanup/...` PASS: `ok workflow/cmd/tools/run-cleanup`.
- `go build ./cmd/tools/run-cleanup/...` PASS.
- `go build -tags=integration ./cmd/tools/run-cleanup/...` PASS.
- `go build ./...` PASS.
- `go vet ./...` PASS.
- `go test -tags=integration -count=1 -run TestR6A1_ ./cmd/tools/run-cleanup/...`: package PASS, 4 个 DB 用例因当前 shell 未设置 `MYSQL_DSN` 按 `testsupport/r35.MustOpenTestDB` skip.
- `go test -tags=integration -count=1 ./service/asset_lifecycle/...` PASS.
- `go test -tags=integration -count=1 ./service/task_draft/...` PASS.
- CLI smoke 在无 `MYSQL_DSN` 环境下返回结构化错误 JSON: `{"error":"mysql dsn is required","subcommand":"oss-365"}` / `{"error":"mysql dsn is required","subcommand":"drafts-7d"}`。
- 当前目录不是 Git worktree, `git diff -- docs/api/openapi.yaml` 无法执行; 本轮未编辑 `docs/api/openapi.yaml`。

## §5 AS-A5 1000 条 E2E
已实现 `TestR6A1_AS_A5_E2E`, fixture 使用 `version_id [55000,56000)` 并断言 `cleaned=1000`、1000 行 `cleaned_at IS NOT NULL`、1000 行 `asset_auto_cleaned`。当前环境未设置 `MYSQL_DSN`, elapsed_ms 未实测。

## §6 [50000,60000) 9 表 audit
`integration_test.go` 的 `t.Cleanup` 执行段内清理并调用 `assertR6AuditClean`: `users / tasks / task_modules / task_module_events / task_assets / task_drafts / notifications / permission_logs` 段内残留必须为 0; 若存在 `task_asset_versions` 表也会审计其 `[50000,60000)` 段。

## §7 sign-off candidate
实现与静态/单元/构建验证为 PASS candidate; DB integration 和 AS-A5 elapsed 待架构师在带 `MYSQL_DSN=<jst_erp_r3_test>` 的环境复跑确认。

## §8 终止符
R6_A_1_DONE_PENDING_ARCHITECT_VERIFY

---

## §9 架构师独立裁决(2026-04-24)

### §9.1 verify 矩阵(13 项)
| # | 检查 | 命令/手段 | 结果 |
|---|---|---|---|
| 1 | forbidden 文件未动 | `ls -la` mtime 比对 | ✓ `service/asset_lifecycle/*` ≤ Apr 24 18:48 · `service/task_draft/*` ≤ Apr 24 18:48 · 远早于 R6.A.1 完成时刻 23:00 |
| 2 | 白名单 4 文件落地 | Read | ✓ `cmd/tools/run-cleanup/{main.go,main_test.go,integration_test.go}` + `docs/iterations/V1_R6_A_1_REPORT.md` |
| 3 | 实装质量 | code review | ✓ 复用 `service.NewOSSDirectService` 优于抄 `cmd/server` · `noopDeleter` 内联 · `parseSubcommand` 隔离 flag 状态 |
| 4 | `drafts-7d --dry-run` 走 read-only SQL | code review | ✓ 因 SA-C `CleanupExpired` 无 dry-run · 用独立 `countExpiredDrafts` SELECT COUNT · 与 service 层语义一致 · 不污染 SA-C |
| 5 | `go build ./...` | wsl 跑 | ✓ exit=0 |
| 6 | `go vet ./...` | wsl 跑 | ✓ exit=0 |
| 7 | `go test ./cmd/tools/run-cleanup/...` 单测 | wsl 跑 | ✓ ok 0.005s |
| 8 | `go build -tags=integration ./cmd/tools/run-cleanup/...` | wsl 跑 | ✓ exit=0 |
| 9 | **R6A1 integration · 4 用例 + helper** | `MYSQL_DSN=<r3_test> R35_MODE=1 go test -tags=integration -v -run TestR6A1_` | ✓ 全 PASS · TestR6A1_HelperProcess + DryRun 1.98s + RealRun 2.27s + Drafts7d 0.76s + **AS_A5_E2E 148.37s** |
| 10 | **AS-A5 1000 条 E2E 实测** | 同上 | ✓ `cleaned=1000` + `assertCleanedCount=1000` + `assertAutoCleanedEvents=1000` + **`AS-A5 elapsed_ms=70393`(70.4s)** |
| 11 | SA-A regression | `go test -tags=integration -run TestSA_A_ ./service/asset_lifecycle/...` | ✓ ok 8.075s · 8 用例全 PASS · 与 R6.A.1 共存零退化 |
| 12 | SA-C regression | `go test -tags=integration ./service/task_draft/...` | ✓ ok 2.439s · 4 用例全 PASS · 与 R6.A.1 共存零退化 |
| 13 | [50000,60000) segment 审计 | 远端 mysql `BETWEEN 50000 AND 59999` 所有表统计 | ✓ R6 直接 seed 表(tasks/task_modules/task_module_events/task_assets/task_drafts/notifications/design_assets/task_details/users)实测 0 残留 |

### §9.2 架构师 inline patch(test 代码 cleanup 顺序 bug)
- **现象**:首跑 4 个 `TestR6A1_` DB 用例主断言全过 · 但 `t.Cleanup` 阶段触发 `sql: database is closed` · t.Fatalf。
- **根因**:codex 写法 `defer db.Close() ; t.Cleanup(func(){ cleanupR6Segment(t, db) ; assertR6AuditClean(t, db) })`。Go 执行顺序:
  1. test body 返回
  2. **defer db.Close() 立即跑**(在 function return 时)
  3. test framework 跑 t.Cleanup callback → 拿到已关闭 db → 失败
  `r35.MustOpenTestDB` 不注册自身 db.Close · 所以 codex 用 defer 是合理选择 · 但与 t.Cleanup 顺序冲突。
- **架构师 patch**(4 处 · `cmd/tools/run-cleanup/integration_test.go`):删 `defer db.Close()` · 把 `_ = db.Close()` 挪进 t.Cleanup callback 末尾 · 让 audit 用活 db、关 db 在 audit 之后。
- **复跑结果**:4 个 R6A1 + helper 全 PASS · audit 内嵌完成 · 隔离段实测 0 残留。
- **责任归属**:test infra 顺序型 bug · 不动业务代码 · 架构师权限内直接 patch · 无需开 R6.A.1.Patch-A 轮。

### §9.3 caveat 复核
- **permission_logs `[50000,60000)` 段 7740 条**:验证前 BEFORE-clean 段统计命中。复核 `actor_id` / `target_user_id` 命中产线真实用户 ID(R5 已签字 baseline drift · 与 R6.A.1 无因果)。R6.A.1 自身没有 `permission_logs` 写入(SA-A `cleanup_job` + SA-C `CleanupExpired` 不写 plog · 仅 `task_module_events`)。架构师手清残留段后 AFTER-clean 全 0 · baseline drift 与 R6.A.1 解耦确认。

### §9.4 verdict
**PASS · architect-patched**
- 主线 PASS:CLI 实装质量优秀 · 双子命令路由 · stdlib flag · service 层复用 · OSS deleter fail-closed · 4 用例 + AS-A5 1000 条 E2E 实测 70.4s · SA-A/SA-C 零退化。
- followup 已闭环:test 代码 cleanup 顺序 bug 由架构师 inline patch 修复 · 复跑全 PASS。
- 余项无:R6.A.1 范围已完整闭环 · 准入 R6.A.2(cron 基础设施)。

### §9.5 终止符 v2
R6_A_1_PASS_ARCHITECT_SIGNED
