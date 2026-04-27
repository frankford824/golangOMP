# V1 R6.A.3 Report · 任务自动归档 job(closed 满 90 天)

## §1 Scope
新建 `service/task_lifecycle` 包 + `TaskAutoArchiveRepo` + `cmd/tools/run-cleanup auto-archive` 子命令 + `cmd/server` cron 第 3 段。0 业务清理代码改动 · 0 OpenAPI 改动 · 0 migration · 仅 `UPDATE tasks SET task_status='Archived'` 零副作用。

## §2 实装清单
| 文件 | 动作 | 行数 | 说明 |
| --- | --- | ---: | --- |
| `service/task_lifecycle/auto_archive_job.go` | 新建 | 82 | `AutoArchiveJob` + `Run` |
| `service/task_lifecycle/auto_archive_job_test.go` | 新建 | 124 | fake repo / fake tx unit tests |
| `service/task_lifecycle/auto_archive_integration_test.go` | 新建 | 226 | 4 个 `TestR6A3_` service integration |
| `repo/task_auto_archive_repo.go` | 新建 | 14 | `TaskAutoArchiveRepo` interface |
| `repo/mysql/task_auto_archive_repo.go` | 新建 | 76 | MySQL `updated_at` cutoff + batch archive |
| `cmd/tools/run-cleanup/auto_archive_integration_test.go` | 新建 | 220 | CLI E2E + AS-X 100 |
| `cmd/tools/run-cleanup/main.go` | 编辑 | 269 | 加 `auto-archive` 子命令 |
| `cmd/tools/run-cleanup/main_test.go` | 编辑 | 86 | 加 auto-archive flag/help 单测 |
| `cmd/server/main.go` | 编辑 | 555 | §7.1 加 auto-archive cron gate |

## §3 wiring 决策
- cutoff = `tasks.updated_at`; 不引用不存在的 `closed_at` / `archived_at`。
- 触发态 = `Completed + Cancelled`。
- 副作用 = 仅更新 `tasks.task_status='Archived'`; 0 events, 0 notifications, 0 task_modules/task_assets 级联。
- idempotent 靠 `WHERE task_status IN ('Completed','Cancelled')` 自然过滤已 Archived。
- CLI 复用 R6.A.1 `run-cleanup` dispatch / JSON 输出 / helper subprocess 模式。
- cron 复用 R6.A.2 `scheduler.Cron`, 默认 disabled: `ENABLE_CRON_AUTO_ARCHIVE=1`, spec 默认 `0 5 * * *`。

## §4 测试输出
- step-1 `bash tmp/r6_a_3_isolation_run.sh`: BEFORE/AFTER 七表全 0。
- `go build ./service/task_lifecycle/...`: PASS.
- `go build ./cmd/tools/run-cleanup/...`: PASS.
- `go build -tags=integration ./service/task_lifecycle/...`: PASS.
- `go build -tags=integration ./cmd/tools/run-cleanup/...`: PASS.
- `go build ./...`: PASS.
- `go vet ./...`: PASS.
- `go build ./cmd/server`: PASS.
- `go test -count=1 ./service/task_lifecycle/...`: PASS.
- `go test -count=1 ./cmd/tools/run-cleanup/...`: PASS.
- `MYSQL_DSN=<jst_erp_r3_test> R35_MODE=1 go test -tags=integration -count=1 -run '^TestR6A3_' ./service/task_lifecycle/...`: PASS, `ok workflow/service/task_lifecycle 8.287s`.
- `MYSQL_DSN=<jst_erp_r3_test> R35_MODE=1 go test -tags=integration -count=1 -run '^TestR6A3_' ./cmd/tools/run-cleanup/...`: PASS, `ok workflow/cmd/tools/run-cleanup 18.496s`.
- R6.A.1 regression: PASS, `ok workflow/cmd/tools/run-cleanup 150.947s`.
- R6.A.2 scheduler regression: PASS, `ok workflow/service/asset_lifecycle/scheduler 0.160s`.
- SA-C regression: PASS, `ok workflow/service/task_draft 5.945s`.
- SA-A regression: PASS via WSL native Go after Windows Device Guard blocked `asset_lifecycle.test.exe`, `ok workflow/service/asset_lifecycle 7.959s` and scheduler `3.705s`.
- step-N `bash tmp/r6_a_3_isolation_run.sh`: BEFORE/AFTER 七表全 0.
- `git diff -- docs/api/openapi.yaml`: workspace is not a Git worktree, so git diff cannot run; this round did not edit `docs/api/openapi.yaml`.

## §5 AS-X 100 条 E2E
`TestR6A3_AS_X_E2E_100` PASS. `elapsed_ms=190`, average `1.90 ms/task`.

## §6 [60000,70000) 7 表 audit
step-1 AFTER clean:
`tasks=0 / task_modules=0 / task_module_events=0 / task_assets=0 / task_drafts=0 / notifications=0 / permission_logs=0`.

step-N AFTER clean:
`tasks=0 / task_modules=0 / task_module_events=0 / task_assets=0 / task_drafts=0 / notifications=0 / permission_logs=0`.

## §7 sign-off candidate
PASS candidate for architect verification. No self-sign; waiting for independent verify.

## §8 终止符
R6_A_3_DONE_PENDING_ARCHITECT_VERIFY

## §9 架构师独立裁决(architect-cleared · 2026-04-25)

### §9.1 verification matrix · 13/13 PASS

| # | 项 | 命令 / 工件 | 结果 |
| ---: | --- | --- | --- |
| 1 | 文件清单合规(白名单) | `service/task_lifecycle/{auto_archive_job,auto_archive_job_test,auto_archive_integration_test}.go` + `repo/task_auto_archive_repo.go` + `repo/mysql/task_auto_archive_repo.go` + `cmd/tools/run-cleanup/{auto_archive_integration_test,main,main_test}.go` + `cmd/server/main.go` §7.1 第 3 段 | PASS · 8 文件全在白名单 · 742 LOC |
| 2 | step-1 段 [60000,70000) 7 表 audit + clean | `tmp/r6_a_3_isolation_run.sh` | PASS · BEFORE/AFTER 七表全 0 |
| 3 | `go build ./...` | WSL native go | PASS |
| 4 | `go vet ./...` | WSL native go | PASS |
| 5 | `go build ./cmd/server` | WSL native go | PASS |
| 6 | `go build -tags=integration ./service/task_lifecycle/... ./cmd/tools/run-cleanup/...` | WSL native go | PASS |
| 7 | unit `./service/task_lifecycle/...`(6 用例) | dry-run/real-run/no-candidates/default cutoff(90d)/cutoff override(30d)/default limit(1000) | PASS · 0.003s |
| 8 | unit `./cmd/tools/run-cleanup/...`(含 R6.A.1 + R6.A.3 flag/help) | WSL native go | PASS · 0.005s |
| 9 | R6.A.3 service integration `^TestR6A3_` | `MYSQL_DSN=jst_erp_r3_test R35_MODE=1 go test -tags=integration -count=1` | PASS · 8.153s |
| 10 | R6.A.3 CLI integration + AS-X 100 `^TestR6A3_` | 同上 | PASS · 18.372s · AS-X 100 elapsed=190ms · 1.90 ms/task |
| 11 | R6.A.1 regression `^TestR6A1_`(全 run-cleanup integration) | 同上 | PASS · 153.993s · 0 退化 |
| 12 | R6.A.2 scheduler regression | 同上 · `./service/asset_lifecycle/scheduler/...` | PASS · 3.705s |
| 13 | SA-A + SA-C regression | `./service/asset_lifecycle/...` + `./service/task_draft/...` | PASS · 8.023s + 2.488s |
| 14 | step-N 段 audit 全 0 + OpenAPI 0-touch + 业务 0-touch | sha256(`docs/api/openapi.yaml`) = `b3d7c365…dd0f` 与 R5 闭环态一致 · `service/asset_lifecycle/cleanup_job.go` / `service/task_draft/service.go` sha 未变 · `domain/task.go` 无 `closed_at`/`archived_at` 引用 | PASS |

> 注:Codex 报告 §4 提及 "Windows Device Guard blocked `asset_lifecycle.test.exe`"。架构师全程使用 WSL native Go(`/home/wsfwk/go/bin/go` + `MYSQL_DSN=root@127.0.0.1:3306` 经 SSH tunnel),与 Codex 同环境,SA-A 直接 PASS,确认 Device Guard 仅是 Windows 边界副作用,不影响代码正确性。

### §9.2 关键合规检查

| 风险点 | 检查 | 结果 |
| --- | --- | --- |
| `tasks.closed_at` / `archived_at` 未存在 | `grep closed_at\|archived_at\|ClosedAt\|ArchivedAt domain/task.go` | 0 行 — codex 严格遵守 prompt §1.4,只用 `updated_at` |
| OpenAPI 改动 | sha256 与 R5 baseline 一致 | 0 改动 |
| `cleanup_job.go` / `task_draft/service.go` 改动 | sha256 与 R6.A.2 baseline 一致 | 0 改动 |
| 副作用扩散(events/notifications) | repo `ArchiveTasks` 仅 `UPDATE tasks SET task_status=?` · 无 `task_module_events` / `notifications` insert | 0 副作用 |
| idempotent 语义 | repo `UPDATE … WHERE id IN (?) AND task_status IN ('Completed','Cancelled')` · 已 Archived 自然过滤 | 单测 `TestAutoArchive_Idempotent_NoCandidates` + integration `TestR6A3_AutoArchive_Idempotent` 双覆盖 |
| 段隔离 | 用 `cmd/tools/run-cleanup/auto_archive_integration_test.go` t.Cleanup 每用例自清 + `tmp/r6_a_3_isolation_run.sh` step-1/step-N 7 表 audit | 全 0 |
| 默认禁用 | `cmd/server/main.go:411 if os.Getenv("ENABLE_CRON_AUTO_ARCHIVE") == "1"` | gate 默认 off |

### §9.3 性能基线
- **AS-X 100 条 E2E**:elapsed=190ms,平均 1.90 ms/task,远优于 R6.A.1 OSS-365 的 70.4ms/task。
- 原因:auto-archive 仅一次 batch UPDATE,无外部对象删除/HTTP 调用。
- 1000 条上限默认 + 索引候选(`task_status, updated_at`)单次 cron 跑预计远 < P95(R6 §32.3 上限 5 分钟)。

### §9.4 codex 自报"段污染担忧"复核
Codex `§4` 没像 R6.A.2 一样误报回归。架构师独立跑全套(R6.A.1/R6.A.2/SA-A/SA-C)在同一段池上零退化,确认 R6.A.2 引入的 `scheduler.Cron` API 与 R6.A.3 `cmd/server` cron 第 3 段 wiring 兼容,**无 prompt 缺陷,无补丁需要**。

### §9.5 verdict
**PASS · architect-cleared**。R6.A.3 闭环。下一步移交 R6.A.4(根据 ROADMAP §32 第 4 项,候选:索引优化或 derived_status 物化评估)。
