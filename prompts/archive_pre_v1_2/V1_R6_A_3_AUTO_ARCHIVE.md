# R6.A.3 · 任务自动归档 job(closed 满 90 天)· Codex Prompt

> 路线:`V1_ROADMAP §32 R6 + §171 修订版`(2026-04-24 删 Feature Flag 幻觉)· 本轮 = R6 第 3 个原子 prompt(R6.A.1 CLI ✓ · R6.A.2 cron infra ✓ · R6.A.3 = task auto-archive)
> 决策依据:`V1_MODULE_ARCHITECTURE §12 R6 行` "任务自动归档 job(closed 满 90 天)" + IA §3 "已归档 = closed + 90 天" + §7.3 "Q7.3/C4 自动:closed 满 90 天"
> 模式:**codex TUI 交互执行** · 完成后 stdout 末尾打印 `R6_A_3_DONE_PENDING_ARCHITECT_VERIFY` · 等架构师 verify · **不要自签 PASS**

---

## 0. 角色

你是 Codex 后端 senior engineer。本任务范围**极窄**:新建一个任务自动归档 service · 加 CLI 子命令 + cron gate · 让 SRE/cron 能把 90 天前 `Completed/Cancelled` 的 task 自动迁到 `Archived`。**零 cleanup_job 改动 · 零 R6.A.1 / R6.A.2 文件改动**。

---

## 1. 范围

### 1.1 可做(白名单)

**新建**:
- `service/task_lifecycle/auto_archive_job.go`(新包 · 类比 `service/asset_lifecycle/cleanup_job.go`)
- `service/task_lifecycle/auto_archive_job_test.go`(unit · fakeRepo)
- `service/task_lifecycle/auto_archive_integration_test.go`(integration · build tag `integration` · 段 `[60000,70000)`)
- `repo/task_auto_archive_repo.go`(interface)
- `repo/mysql/task_auto_archive_repo.go`(MySQL 实装)
- `cmd/tools/run-cleanup/auto_archive_integration_test.go`(R6A3 CLI E2E · build tag `integration`)
- 落报告 `docs/iterations/V1_R6_A_3_REPORT.md`

**编辑**(纯加 · 不动既有逻辑):
- `cmd/tools/run-cleanup/main.go` 加 `auto-archive` 子命令(不动 `oss-365` / `drafts-7d` 既有路由 / wiring)
- `cmd/tools/run-cleanup/main_test.go` 加 `TestFlagParsing_AutoArchive` 等单测
- `cmd/server/main.go` §7.1 cron 块**末尾**追加第 3 段(`ENABLE_CRON_AUTO_ARCHIVE=1` gate)· 不动 oss-365 / drafts-7d 既有块 / graceful shutdown 块

### 1.2 不可做(改一行 = ABORT)

- `service/asset_lifecycle/**`(SA-A v2.1 + R6.A.2 签字)
- `service/task_draft/**`(SA-C v1 签字)
- `service/asset_lifecycle/scheduler/**`(R6.A.2 签字)
- `repo/mysql/task.go`(既有 task repo · 不混淆 `UpdateStatus` 签名)
- `repo/mysql/task_asset_lifecycle_repo.go`(SA-A 签字)
- `repo/task_asset_lifecycle_repo.go` 接口
- `domain/**`(`TaskStatus` enum 已含 `Archived` · 不需改 · 不准加 `closed_at` / `archived_at` 字段)
- `db/migrations/**`(`tasks` 表 schema **不加列** · R6.A.3 范围之外)
- `docs/api/openapi.yaml`(本轮零 OpenAPI 改动 · auto-archive 是后台 job 无 HTTP 端点)
- `transport/**`
- 任何 R1~R5 / R6.A.1 / R6.A.2 已签字测试文件
- 引入 cobra/urfave-cli 等第三方 CLI 框架(stdlib `flag` 必须够)
- 引入新 cron 库(R6.A.2 已选 `robfig/cron/v3` · 复用)

---

## 1.5 Production baseline 锁定(防 schema 幻觉)

以下**架构师亲查的 2026-04-24 生产事实**:

### 1.5.1 `tasks` 表生产 schema(**关键**)

`SELECT COLUMN_NAME FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='tasks' AND COLUMN_NAME IN (...)` 实测结果:

| 列名 | 是否存在 |
| --- | --- |
| `task_status varchar NOT NULL` | ✓ |
| `created_at datetime NOT NULL` | ✓ |
| `updated_at datetime NOT NULL`(`ON UPDATE CURRENT_TIMESTAMP`) | ✓ |
| **`closed_at`** | ✗ 不存在 |
| **`archived_at`** | ✗ 不存在 |
| `closed_by` / `close_reason` | ✗ 不存在 |

> ⚠️ **重要**:`domain.Task` 结构体上的 `ClosedAt *time.Time db:"closed_at"`(`domain/entities.go L106`)是 v0.9→v1 设计幻觉 · 整个 service 层**从无人 SET 它** · 生产 tasks 表也**没有这个列**。R6.A.3 **必须用 `tasks.updated_at` 作 cutoff**(与 SA-A `cleanup_job` 完全一致 · 不要尝试用 `closed_at`/`archived_at`/`closed_by`)。

### 1.5.2 `domain.TaskStatus` 枚举(`domain/enums_v7.go`)

```go
TaskStatusCompleted TaskStatus = "Completed"
TaskStatusArchived  TaskStatus = "Archived"
TaskStatusCancelled TaskStatus = "Cancelled"
// 还有其他状态(Pending / InProgress / ...)· 但 R6.A.3 只关心这 3 个
```

### 1.5.3 生产 baseline(`docs/iterations/r6_a_3_pre_probe.log` 已落)

| 项 | 值 |
| --- | --- |
| `tasks_total` | 107 |
| `task_status='Archived'` | **0**(历史从未归档过) |
| `task_status='Completed'` | 2 |
| `task_status='Cancelled'` | 0 |
| 候选 90 天 Completed+Cancelled | **0**(2 条 Completed 都未满 90 天) |
| `[60000,70000)` 段 `tasks/task_modules/task_module_events` | 全 0 ✓ |

### 1.5.4 R6.A.2 cron 基础设施(签字 · 复用)

```go
// service/asset_lifecycle/scheduler/register.go (UNTOUCHED)
package scheduler

func New(parent context.Context, logger *log.Logger) *Cron
func (c *Cron) Add(name, spec string, job JobFunc) error
func (c *Cron) Start()
func (c *Cron) Stop(ctx context.Context) error
func (c *Cron) Entries() []EntryInfo
type JobFunc func(ctx context.Context) error
```

### 1.5.5 R6.A.1 CLI 框架(签字 · 扩展)

```go
// cmd/tools/run-cleanup/main.go (现有路由)
// 子命令:oss-365 / drafts-7d
// 你要加第 3 个子命令:auto-archive
// 复用 main.go 的 dispatch 模式 + JSON 报告 + flag 解析风格
```

### 1.5.6 关键架构裁决(架构师已签 · 不要回头讨论)

| 决策 | 值 | 理由 |
| --- | --- | --- |
| cutoff 字段 | **`tasks.updated_at`** | `closed_at` 列不存在 · 生产无任何代码 set 它;`updated_at` 与 `cleanup_job` 一致 |
| 触发态 | **`task_status IN ('Completed','Cancelled')`** | 权威 §7.3 / IA §3 "closed" 含 cancelled · 与 cleanup_job 同 status 集 |
| archive 副作用 | **零副作用**(仅 `UPDATE tasks SET task_status='Archived'`) | IA §2.1 throughput.archived≡completed 同集合 · 不需 module-level event;asset 状态由 `domain/asset_lifecycle_state.go L31` 自动推导 `Archived task → ClosedRetained asset` · 无需级联 |
| idempotent 语义 | UPDATE WHERE 包含 `task_status IN ('Completed','Cancelled')` 自然过滤已 Archived | 第二次跑相同 cutoff archived=0 |
| `archived_at` 列 | **不加**(R6.A.3 范围外) | 生产表无此列 · 不动 migration |
| 通知 / module event | **不写** | 任务级 archive 没专 event_type · 写会破坏 IA §2.1 throughput 假设 |

---

## 2. 必读上下文(按顺序)

1. `service/asset_lifecycle/cleanup_job.go` 全文(R6.A.3 `AutoArchiveJob` 的 struct/Options/Result/Run 风格直接照抄)
2. `repo/task_asset_lifecycle_repo.go`(R6.A.3 `TaskAutoArchiveRepo` interface 风格参照)
3. `repo/mysql/task_asset_lifecycle_repo.go::ListEligibleForCleanup`(SQL 风格参照)
4. `cmd/tools/run-cleanup/main.go` 全文(`auto-archive` 子命令的 flag/JSON/exit 风格 100% 参照 `oss-365` 段)
5. `cmd/tools/run-cleanup/integration_test.go`(段隔离 / `t.Cleanup` / `assertR6AuditClean` 风格;R6.A.3 测试要照搬这些 helper · 不要重写)
6. `service/asset_lifecycle/scheduler/register.go`(cron `Add/Start/Stop` API)
7. `cmd/server/main.go §7.1 cron 块(L383~L410)+ §9 graceful shutdown (L432~L438)`(R6.A.3 cron gate 加在第 2 段后 · §9 不动)
8. `prompts/V1_R6_A_1_RUN_CLEANUP_CLI.md` 全文(报告骨架 / verify 步骤模板)
9. `prompts/V1_R6_A_2_CRON_INFRA.md` 全文(cron 挂载块代码模板 / ENV gate 模式)
10. `domain/asset_lifecycle_state.go L31`(看 `Archived task → ClosedRetained asset` 自动推导 · 证 R6.A.3 不需级联)

---

## 3. 实施清单

### 3.1 `service/task_lifecycle/auto_archive_job.go`(新包)

**结构**(直接抄 `cleanup_job.go` 风格):

```go
package task_lifecycle

import (
    "context"
    "log"
    "time"

    "workflow/domain"
    "workflow/repo"
)

const AutoArchiveLogPrefix = "[TASK-AUTO-ARCHIVE]"

type AutoArchiveJob struct {
    archiveRepo repo.TaskAutoArchiveRepo
    txRunner    repo.TxRunner
    now         func() time.Time
    logger      *log.Logger
}

type AutoArchiveOptions struct {
    DryRun     bool
    Limit      int  // 默认 1000
    CutoffDays int  // 默认 90 · 测试可注入 0
}

type AutoArchiveResult struct {
    DryRun     bool
    Scanned    int      // ListEligibleForArchive 返回行数
    Archived   int      // 实际 UPDATE 影响行数
    Candidates []int64  // task_id 列表 · 用于测试断言与日志
    Cutoff     time.Time
}

func NewAutoArchiveJob(archiveRepo repo.TaskAutoArchiveRepo, txRunner repo.TxRunner, logger *log.Logger) *AutoArchiveJob
func (j *AutoArchiveJob) WithNow(now func() time.Time) *AutoArchiveJob
func (j *AutoArchiveJob) Run(ctx context.Context, opts AutoArchiveOptions) (*AutoArchiveResult, *domain.AppError)
```

**Run 实装要点**:

- `if opts.Limit <= 0 { opts.Limit = 1000 }`(与 cleanup_job 默认 100 不同 · auto-archive 默认 1000 因为是低频但批量大)
- `if opts.CutoffDays <= 0 { opts.CutoffDays = 90 }`
- `cutoff := j.now().UTC().AddDate(0, 0, -opts.CutoffDays)`
- `candidates, err := j.archiveRepo.ListEligibleForArchive(ctx, cutoff, opts.Limit)`(返 `[]int64` task_id)
- `result := &AutoArchiveResult{DryRun: opts.DryRun, Scanned: len(candidates), Candidates: candidates, Cutoff: cutoff}`
- `j.logger.Printf("%s dry_run=%t scanned=%d cutoff=%s limit=%d", AutoArchiveLogPrefix, opts.DryRun, len(candidates), cutoff.Format(time.RFC3339), opts.Limit)`
- 若 `opts.DryRun` 直返(scanned > 0 archived = 0)
- 否则 `txRunner.WithTx(ctx, func(tx) { archiveRepo.ArchiveTasks(ctx, tx, candidates) })`,把 affected 写到 `result.Archived`

### 3.2 `repo/task_auto_archive_repo.go`(新接口)

```go
package repo

import (
    "context"
    "time"
)

type TaskAutoArchiveRepo interface {
    // ListEligibleForArchive 返回 task_id 列表(任务满足 status IN ('Completed','Cancelled') AND updated_at < cutoff)
    // ORDER BY updated_at ASC, id ASC · LIMIT 由调用方传
    ListEligibleForArchive(ctx context.Context, cutoff time.Time, limit int) ([]int64, error)

    // ArchiveTasks 批量 UPDATE task_status='Archived'
    // WHERE id IN (taskIDs) AND task_status IN ('Completed','Cancelled')
    // 返回 affected rows · 用 tx 防止脏更新
    ArchiveTasks(ctx context.Context, tx Tx, taskIDs []int64) (int, error)
}
```

### 3.3 `repo/mysql/task_auto_archive_repo.go`(MySQL 实装)

```go
package mysql

import (
    "context"
    "database/sql"
    "fmt"
    "strings"
    "time"

    "workflow/domain"
    "workflow/repo"
)

type taskAutoArchiveRepo struct {
    db *Database
}

func NewTaskAutoArchiveRepo(db *Database) repo.TaskAutoArchiveRepo {
    return &taskAutoArchiveRepo{db: db}
}

func (r *taskAutoArchiveRepo) ListEligibleForArchive(ctx context.Context, cutoff time.Time, limit int) ([]int64, error) {
    if limit <= 0 || limit > 5000 {
        limit = 1000
    }
    rows, err := r.db.db.QueryContext(ctx, `
        SELECT id FROM tasks
         WHERE task_status IN (?, ?)
           AND updated_at < ?
         ORDER BY updated_at ASC, id ASC
         LIMIT ?`,
        string(domain.TaskStatusCompleted), string(domain.TaskStatusCancelled), cutoff, limit)
    if err != nil {
        return nil, fmt.Errorf("list eligible tasks for archive: %w", err)
    }
    defer rows.Close()
    var out []int64
    for rows.Next() {
        var id int64
        if err := rows.Scan(&id); err != nil {
            return nil, fmt.Errorf("scan task id: %w", err)
        }
        out = append(out, id)
    }
    return out, rows.Err()
}

func (r *taskAutoArchiveRepo) ArchiveTasks(ctx context.Context, tx repo.Tx, taskIDs []int64) (int, error) {
    if len(taskIDs) == 0 {
        return 0, nil
    }
    placeholders := strings.Repeat("?,", len(taskIDs))
    placeholders = placeholders[:len(placeholders)-1]
    args := make([]interface{}, 0, len(taskIDs)+3)
    args = append(args, string(domain.TaskStatusArchived))
    for _, id := range taskIDs {
        args = append(args, id)
    }
    args = append(args, string(domain.TaskStatusCompleted), string(domain.TaskStatusCancelled))

    sqlTx := Unwrap(tx)
    res, err := sqlTx.ExecContext(ctx,
        `UPDATE tasks SET task_status = ?
          WHERE id IN (`+placeholders+`)
            AND task_status IN (?, ?)`,
        args...)
    if err != nil {
        return 0, fmt.Errorf("archive tasks: %w", err)
    }
    n, err := res.RowsAffected()
    if err != nil {
        return 0, fmt.Errorf("archive tasks rows affected: %w", err)
    }
    return int(n), nil
}

var _ repo.TaskAutoArchiveRepo = (*taskAutoArchiveRepo)(nil)
var _ = sql.ErrNoRows // keep sql import for future use
```

> **注**:`Unwrap(tx)` / `Database` / `db.db` 等 helper 自查 `repo/mysql/` 包内现有约定(参 `task_asset_lifecycle_repo.go`)。如果命名不一致,以现有 repo 风格为准。

### 3.4 unit 测 `service/task_lifecycle/auto_archive_job_test.go`

- 用 fake `TaskAutoArchiveRepo` + fake `TxRunner`(参考 SA-A `service/asset_lifecycle/*_test.go` 怎么写 fake · 别 import mock 库)
- `TestAutoArchive_DryRun`:fake 返 5 个 task_id · DryRun=true · 验 `Scanned=5 Archived=0` · 验 fakeRepo `ArchiveTasks` 未被调用
- `TestAutoArchive_RealRun`:fake `ArchiveTasks` 返 (5, nil) · 验 `Scanned=5 Archived=5`
- `TestAutoArchive_Idempotent_NoCandidates`:fake `ListEligibleForArchive` 返 `[]int64{}` · 验 `Scanned=0 Archived=0` · `ArchiveTasks` 未被调用
- `TestAutoArchive_DefaultCutoffDays`:`Opts{}`(全空)· 验 cutoff = `now-90d`(用 `WithNow(func() time.Time { return fixed })` 注入)
- `TestAutoArchive_CutoffDaysOverride`:`Opts{CutoffDays:30}` · 验 cutoff = `now-30d`
- `TestAutoArchive_LimitDefault`:`Opts{}` 验调 fakeRepo 时 limit=1000

### 3.5 integration 测 `service/task_lifecycle/auto_archive_integration_test.go`

测试库 `jst_erp_r3_test` · 段 **`[60000, 70000)`** · 每 test `t.Cleanup` 兜底清理 + audit · 段外写入 = 测试 fail。

测试函数前缀 `TestR6A3_`:

- `TestR6A3_AutoArchive_DryRun`:
  - seed 5 个 task `id [60000,60005)` task_status='Completed' updated_at=NOW(6)-INTERVAL 91 DAY
  - seed 3 个 task `id [60005,60008)` task_status='Cancelled' updated_at=NOW(6)-INTERVAL 91 DAY
  - seed 4 个 task `id [60008,60012)` task_status='Completed' updated_at=NOW(6)-INTERVAL 30 DAY(未满)
  - 跑 `AutoArchiveJob.Run(DryRun=true, Limit=100)`
  - 验 `Scanned=8 Archived=0` · DB 未变(全 Completed/Cancelled)

- `TestR6A3_AutoArchive_RealRun`:
  - 同 seed
  - 跑 `Run(DryRun=false, Limit=100)`
  - 验 `Scanned=8 Archived=8`
  - DB:8 task `task_status='Archived'` · 4 task 仍 `Completed`(未满 90 天)

- `TestR6A3_AutoArchive_Idempotent`:
  - 同 seed → 跑 RealRun → 跑第 2 次 RealRun
  - 验第 2 次 `Scanned=0 Archived=0`(已 Archived 不再匹配)

- `TestR6A3_AutoArchive_Limit`:
  - seed 12 task 全满 90 天
  - 跑 `Run(Limit=5)`
  - 验 `Scanned=5 Archived=5`(LIMIT 5 起效)

> 段隔离 helper:照抄 `cmd/tools/run-cleanup/integration_test.go` 的 `cleanupR6Segment` / `assertR6AuditClean` 模式 · 但段范围改 `[60000, 70000)` · audit 表至少含:`tasks / task_modules / task_module_events / task_assets / task_drafts / notifications / permission_logs`(7 表 · 全 0)。
> **重要**:t.Cleanup 写法严格遵守 R6.A.1 教训 — `db.Close()` **不能用 `defer`**,必须放进 `t.Cleanup` callback **末尾**(否则 db 在 cleanup 前关闭)。

### 3.6 CLI 子命令 — 编辑 `cmd/tools/run-cleanup/main.go`(纯加段 · 不动 oss-365 / drafts-7d)

加第 3 个子命令 `auto-archive`,**完全照搬 `oss-365` 段的 flag/wiring 模式**:

flag(`flag.NewFlagSet("auto-archive", flag.ExitOnError)`):
- `--dry-run`(默认 false)
- `--limit int`(默认 1000)
- `--cutoff-days int`(默认 90)
- `--dsn string`(默认 `os.Getenv("MYSQL_DSN")`)
- `--reason string`(默认 `v1.r6.a.3.manual.auto-archive`)
- `--json`(默认 true)

启动 stderr:`[RUN-CLEANUP] subcommand=auto-archive dry_run=<bool> limit=<n> cutoff_days=<n> ts=<RFC3339>`

输出 JSON(stdout 最后行):
```json
{"subcommand":"auto-archive","dry_run":bool,"scanned":int,"archived":int,"cutoff":"RFC3339","elapsed_ms":int}
```

wiring:
- `db := sql.Open("mysql", dsn)` + `db.PingContext`
- `mdb := mysqlrepo.NewDatabase(db)`(自查现有 helper 名)
- `archiveRepo := mysqlrepo.NewTaskAutoArchiveRepo(mdb)`
- `txRunner := mysqlrepo.NewTxRunner(db)`(参 oss-365 段)
- `job := task_lifecycle.NewAutoArchiveJob(archiveRepo, txRunner, log.New(os.Stderr, "", log.LstdFlags))`
- `result, appErr := job.Run(ctx, task_lifecycle.AutoArchiveOptions{DryRun: dryRun, Limit: limit, CutoffDays: cutoffDays})`
- 输出 + exit 0/1

### 3.7 CLI integration 测 `cmd/tools/run-cleanup/auto_archive_integration_test.go`

- 复用 `cmd/tools/run-cleanup/integration_test.go` 的 `openR6DB` / `cleanupR6Segment` / `assertR6AuditClean` helper(只 import / 调用 · 不重写)
- `TestR6A3_AutoArchive_CLI_DryRun`:同 service integration seed · 跑 helper subprocess `auto-archive --dry-run --limit=100` · 验 stdout JSON `scanned=8 archived=0` + DB 未变
- `TestR6A3_AutoArchive_CLI_RealRun`:同 seed · 跑 `auto-archive --limit=100` · 验 `scanned=8 archived=8` + DB `task_status='Archived'` 8 行
- `TestR6A3_AS_X_E2E_100`:seed 100 task `[60100,60200)` 全满 90 天 · 跑 `auto-archive --limit=200` · 验 `archived=100` · 记录 `elapsed_ms` 到报告 §5(类比 R6.A.1 AS-A5 · R6.A.3 是 AS-X 演练验收)

> helper subprocess 模式:**完全照搬** `cmd/tools/run-cleanup/integration_test.go::TestR6A1_HelperProcess` 的 `os/exec` + `TEST_HELPER_MODE` env 模式 · 共用同一个 `TestR6A1_HelperProcess`(它已经能 dispatch 任何子命令)。**不重写 helper**。

### 3.8 cmd/server cron 挂载第 3 段 — 编辑 `cmd/server/main.go`(纯加段 · 不动既有块)

在 §7.1 cron 块的 **第 2 段(drafts-7d)之后 · `cronInst.Start()` 之前**追加:

```go
    if os.Getenv("ENABLE_CRON_AUTO_ARCHIVE") == "1" {
        archiveSpec := envOr("CRON_SCHEDULE_AUTO_ARCHIVE", "0 5 * * *")
        autoArchiveJob := task_lifecycle.NewAutoArchiveJob(taskAutoArchiveRepo, txRunner, log.New(os.Stderr, "[TASK-AUTO-ARCHIVE-CRON] ", log.LstdFlags))
        if err := cronInst.Add("auto-archive", archiveSpec, func(ctx context.Context) error {
            _, appErr := autoArchiveJob.Run(ctx, task_lifecycle.AutoArchiveOptions{Limit: 1000, CutoffDays: 90})
            if appErr != nil {
                return fmt.Errorf("%s: %s", appErr.Code, appErr.Message)
            }
            return nil
        }); err != nil {
            logger.Fatal("cron auto-archive add failed", zap.Error(err))
        }
        logger.Info("cron auto-archive enabled", zap.String("spec", archiveSpec))
    }
```

> 在 §7.1 块前面找现成的 `taskAutoArchiveRepo` 初始化点 — 如果不存在,你可以在该 cron 块**之前** 1~2 行加 `taskAutoArchiveRepo := mysqlrepo.NewTaskAutoArchiveRepo(mdb)`(就近 · 不影响其他 wiring)。`txRunner` 已存在(SA-A / R6.A.2 都用它)· 直接复用。
> **不动** §9 graceful shutdown · `cronInst.Stop()` 已在 R6.A.2 写好 · 自动覆盖新 entry。

---

## 4. allowed_files 白名单(改其他文件 = ABORT)

### 新建
- `service/task_lifecycle/auto_archive_job.go`
- `service/task_lifecycle/auto_archive_job_test.go`
- `service/task_lifecycle/auto_archive_integration_test.go`
- `repo/task_auto_archive_repo.go`
- `repo/mysql/task_auto_archive_repo.go`
- `cmd/tools/run-cleanup/auto_archive_integration_test.go`
- `docs/iterations/V1_R6_A_3_REPORT.md`

### 编辑(纯加 · 不改既有逻辑行)
- `cmd/tools/run-cleanup/main.go`(加 `auto-archive` 子命令路由 + flag set + wiring 函数)
- `cmd/tools/run-cleanup/main_test.go`(加 `TestFlagParsing_AutoArchive` / `TestUsage_AutoArchive_Help`)
- `cmd/server/main.go`(§7.1 加第 3 段 + 1~2 行 `taskAutoArchiveRepo` wiring · §9 不动)

### 严禁动一行
- `service/asset_lifecycle/**`
- `service/asset_lifecycle/scheduler/**`
- `service/task_draft/**`
- `repo/mysql/task.go` / `repo/mysql/task_asset_lifecycle_repo.go`
- `repo/task_asset_lifecycle_repo.go` 接口
- `domain/**`
- `db/migrations/**`
- `docs/api/openapi.yaml`
- `transport/**`
- 任何已签字测试(R1~R5 / R6.A.1 / R6.A.2)

---

## 5. 验证(13 步 · 照 R6.A.1 模板加强 step-1 段隔离)

执行后,在报告 §4 / §6 / §7 引用证据,全部必须 PASS:

> **🛡 step-1 强制段隔离**(吸收 R6.A.2 教训):**所有 integration 跑前**先做段 audit + clean。架构师已落 `tmp/r6_a_3_isolation_run.sh`(`tmp/r6_a_3_segment_audit.sql` + `tmp/r6_a_3_segment_clean.sql` 三件套)· 段范围 `[60000, 70000)` · audit 7 表(`tasks / task_modules / task_module_events / task_assets / task_drafts / notifications / permission_logs`)。
>
> **caveat**(R6.A.1 教训复用):`permission_logs` BEFORE 段命中可能 ≠ 0(测试库累积 baseline drift · 与 R6.A.3 解耦)。验证标准 = **AFTER clean 必须全 0** · 且 R6.A.3 自身只写 `tasks` 表(0 events / 0 notifications / 0 permission_logs) · 故 BEFORE 是参考 · AFTER 是硬门。

1. **段隔离 step-1**:`bash tmp/r6_a_3_isolation_run.sh` BEFORE 全 0(若非 0 → 报告中说明 + 清干净)
2. `go build ./service/task_lifecycle/...` exit 0
3. `go build ./cmd/tools/run-cleanup/...` exit 0
4. `go build -tags=integration ./service/task_lifecycle/...` exit 0
5. `go build -tags=integration ./cmd/tools/run-cleanup/...` exit 0
6. `go build ./...` exit 0(整仓不破 · 含 `cmd/server`)
7. `go vet ./...` exit 0
8. `go build ./cmd/server` exit 0(cron 挂载块编译通过)
9. `go test -count=1 ./service/task_lifecycle/...` 全绿(unit)
10. `MYSQL_DSN=<jst_erp_r3_test_dsn> go test -tags=integration -count=1 -run ^TestR6A3_ ./service/task_lifecycle/...` 全绿
11. `MYSQL_DSN=<jst_erp_r3_test_dsn> go test -tags=integration -count=1 -run ^TestR6A3_ ./cmd/tools/run-cleanup/...` 全绿
12. **R6.A.1 / R6.A.2 / SA-A / SA-C 回归**(零退化):
    - `go test -tags=integration -count=1 -run ^TestR6A1_ ./cmd/tools/run-cleanup/...` 全绿
    - `go test -count=1 ./service/asset_lifecycle/scheduler/...` 全绿
    - `go test -tags=integration -count=1 ./service/asset_lifecycle/...` 全绿
    - `go test -tags=integration -count=1 ./service/task_draft/...` 全绿
13. **段隔离 step-N**:跑完所有 integration 后,7 表 `[60000,70000)` 段全 0(t.Cleanup 必须真清干净)

> ⚠️ **如果 step-12 任何回归失败**:**先看是不是段污染 / R6.A.1 段 [50000,60000) 残留**(R6.A.2 教训 · `cleanup_job` 全表扫无段过滤 · 跨段会互扰)。**先跑 R6.A.1 段 audit + clean 再跑回归** · 不要直接报 ABORT。如果段干净仍 fail 再报 ABORT。
> ⚠️ **OpenAPI 0 改动验证**:`git diff -- docs/api/openapi.yaml` 必须空(本轮无 HTTP 端点)。

---

## 6. 数据隔离

- 测试库:`jst_erp_r3_test` · **严禁碰** `jst_erp` 生产
- task_id / id 段:**`[60000, 70000)`**(R5/R6.A.1 用 [50000,60000) · R6.A.3 沿用下一段 · 不开重叠段)
- AS-X 100 条演练在 task_id 子段 `[60100, 60200)`
- 每 test `t.Cleanup` 兜底清 + audit · 段外写入 → `t.Fatalf`
- **db.Close 严格放进 `t.Cleanup` callback 末尾**(R6.A.1 教训 · 不能 `defer db.Close()`)

---

## 7. ABORT 触发器(任一命中 → 写 `tmp/r6_a_3_ABORT.txt` + 立刻停)

- 改 `service/asset_lifecycle/**` / `service/task_draft/**` / `service/asset_lifecycle/scheduler/**` 任一行
- 改 `repo/mysql/task.go` / `repo/mysql/task_asset_lifecycle_repo.go` / `repo/task_asset_lifecycle_repo.go`
- 改 `domain/**` 任一行(包括添加 `closed_at` / `archived_at` 字段)
- 改 `db/migrations/**`(尝试加 `tasks.archived_at` 列)
- 改 `docs/api/openapi.yaml`
- 改 R6.A.1 / R6.A.2 既有测试文件
- 引入新 cron 库 / cobra-style CLI 库
- 写 `task_module_events` 任何 `task_archived_*` 类型(零副作用约束)
- 发 `notifications` 给 archive 任务的相关人(零副作用约束)
- archive 时级联写 task_modules / task_assets(架构师裁决:不级联 · `domain/asset_lifecycle_state.go` 自动推导)
- 测试代码段外写入(段外 = `id < 60000` 或 `id >= 70000`)
- 任一 `go test` 失败(且**段干净的前提下**仍 fail)
- step-12 R6.A.1 / R6.A.2 / SA-A / SA-C **回归失败**(且段已审计干净)

---

## 8. 输出协议

跑完后:
1. 落 `docs/iterations/V1_R6_A_3_REPORT.md`(§1~§8 完整 · §8 = `R6_A_3_DONE_PENDING_ARCHITECT_VERIFY`)
2. **stdout 最后一行打印**:`R6_A_3_DONE_PENDING_ARCHITECT_VERIFY`
3. **不要自己改** `prompts/V1_ROADMAP.md` / `prompts/V1_R6_A_3_AUTO_ARCHIVE.md`
4. **不要自己签字 PASS** · 等架构师 verify
5. 如遇 ABORT,写 `tmp/r6_a_3_ABORT.txt` 含原因 + stdout 打印 `R6_A_3_ABORT_<reason_code>` 后退出

---

## 附录 · 报告模板

```markdown
# V1 R6.A.3 Report · 任务自动归档 job(closed 满 90 天)

## §1 Scope
新建 `service/task_lifecycle` 包 + `TaskAutoArchiveRepo` + `cmd/tools/run-cleanup auto-archive` 子命令 + `cmd/server` cron 第 3 段;0 业务清理代码改动 · 0 OpenAPI 改动 · 0 migration · 仅 UPDATE `task_status='Archived'` 零副作用。

## §2 实装清单
| 文件 | 动作 | 行数 | 说明 |
| --- | --- | --- | --- |
| service/task_lifecycle/auto_archive_job.go | 新建 | ... | AutoArchiveJob struct + Run |
| service/task_lifecycle/auto_archive_job_test.go | 新建 | ... | unit fakeRepo |
| service/task_lifecycle/auto_archive_integration_test.go | 新建 | ... | 4 个 TestR6A3_ |
| repo/task_auto_archive_repo.go | 新建 | ... | interface |
| repo/mysql/task_auto_archive_repo.go | 新建 | ... | MySQL 实装 |
| cmd/tools/run-cleanup/auto_archive_integration_test.go | 新建 | ... | CLI E2E + AS-X |
| cmd/tools/run-cleanup/main.go | 编辑 | ... | 加 auto-archive 子命令 |
| cmd/tools/run-cleanup/main_test.go | 编辑 | ... | TestFlagParsing_AutoArchive |
| cmd/server/main.go | 编辑 | ... | §7.1 加第 3 段 cron gate |

## §3 wiring 决策
- cutoff = `tasks.updated_at`(closed_at/archived_at 列不存在)
- 触发态 = `Completed + Cancelled`
- 副作用 = 仅 UPDATE task_status · 0 events · 0 notifications
- idempotent 靠 WHERE 包含 `IN ('Completed','Cancelled')` 自然过滤已 Archived

## §4 测试输出
- go build / vet / unit / integration / 回归 全 PASS
- 段隔离 step-1 / step-N 全 0

## §5 AS-X 100 条 E2E
elapsed_ms = ... · 平均 ... ms/条

## §6 [60000,70000) 7 表 audit
- AFTER clean 全 0 · BEFORE 段如有 permission_logs > 0 注明为产线 baseline drift(actor_id 段命中 · R6.A.3 自身 0 写 permission_logs)
- tasks=0 / task_modules=0 / task_module_events=0 / task_assets=0 / task_drafts=0 / notifications=0 / permission_logs=0(AFTER)

## §7 sign-off candidate
PASS candidate

## §8 终止符
R6_A_3_DONE_PENDING_ARCHITECT_VERIFY
```
