# V1 R6.A.2 · Cron 基础设施 + 启用两条清理 job

> **执行模式**:Codex TUI · 单原子 prompt · 架构师 · 已验过 baseline。
> **前置**:R6.A.1 (`cmd/tools/run-cleanup` CLI 双子命令)签字生效 · `cleanup_job.go` / `task_draft/service.go` 业务代码已存在且**禁止再动**。
> **目标**:把 R4-SA-A v2.1 留的 `service/asset_lifecycle/scheduler/` dummy 包重写为基于 `robfig/cron/v3` 的通用 cron infra · `cmd/server` 加 cron 挂载 + 优雅关停 · 通过 ENV gate 默认 disabled · ENABLE 后调度既有 cleanup_job.Run 与 service.CleanupExpired。
> **不做**:不动业务清理逻辑 · 不动 OpenAPI · 不新建 migration · 不引非 cron 的新依赖。

---

## §1 上下文与 baseline(必读 · 防幻觉锚)

### §1.1 R6.A.1 签字成果(已落盘 · 不要重做)
- `cmd/tools/run-cleanup/{main.go, main_test.go, integration_test.go}` — 手动 CLI 双子命令 `oss-365` / `drafts-7d` · stdlib `flag` · 复用 SA-A `cleanup_job` + SA-C `CleanupExpired`。**R6.A.2 禁止动这 3 个文件**。
- `docs/iterations/V1_R6_A_1_REPORT.md` §9 已签 PASS · 架构师 13 项 verify 全绿 · `AS-A5 elapsed_ms=70393` 1000 条端到端 70.4s。

### §1.2 既有业务 API(R6.A.2 仅消费 · 禁止改)
- `service/asset_lifecycle/cleanup_job.go`:`CleanupJob.Run(ctx, CleanupOptions{DryRun bool, Limit int}) (*CleanupResult, *domain.AppError)` · 当前 cutoff = `now() - 365 day`(写死)· 写 `task_assets.cleaned_at + storage_key=NULL` + `task_module_events asset_auto_cleaned`。
- `service/task_draft/service.go`:`Service.CleanupExpired(ctx) (int, error)` · 当前 cutoff = `now()` 比 `expires_at`(草稿 7 天 expires_at 已在 SA-C 写入时设置 · service 层无 cutoff 参数)· 删 `task_drafts.expires_at < now`。
- 这两个函数签名是 R6.A.2 的输入契约 · **禁止修改**(改了 R6.A.1 整套 CLI + integration 都会破)。

### §1.3 现状 scheduler 包(必读 · 23 行 dummy)
- 文件:`service/asset_lifecycle/scheduler/register.go`
- 内容(全文):
  ```go
  package scheduler

  import "context"

  type Config struct {
      Enabled bool
  }

  type JobFunc func(context.Context) error

  func DefaultConfig() Config {
      return Config{Enabled: false}
  }

  func Register(ctx context.Context, cfg Config, job JobFunc) error {
      if !cfg.Enabled {
          return nil
      }
      if job == nil {
          return nil
      }
      return job(ctx)
  }
  ```
- **现状判断**:`go test ./service/asset_lifecycle/scheduler/...` 为空(R6.A.1 跑过 `[no tests to run]`)· 无 caller 引用此包(grep 全仓 `asset_lifecycle/scheduler` import 0 匹配 · `cmd/server/main.go` 也未挂)。**结论**:scheduler 包是 R4-SA-A 留的占位 · 重写无 backward-compat 风险。
- **位置选择**:为不破 R4-SA-A v2.1 报告里"scheduler shell 在 asset_lifecycle/scheduler/" 措辞 · R6.A.2 **保留路径** `service/asset_lifecycle/scheduler/` · 但 package 内容重写为**通用 Cron infra**(允许 asset_lifecycle 与 task_draft 共用)。命名洁癖延 R6.D 处理 · 现轮加 TODO 注释即可。

### §1.4 cmd/server 当前结构(必读)
`cmd/server/main.go` 关键行(以下行号据 R6.A.1 闭环时实测 · codex TUI 应当再 Read 一次确认):
- L24:`assetlifecycle "workflow/service/asset_lifecycle"`
- L35:`taskdraftsvc "workflow/service/task_draft"`
- L287:`globalAssetLifecycleSvc := assetlifecycle.NewService(taskAssetSearchRepo, taskAssetLifecycleRepo, mdb, ossDirectSvc)`(SA-A `Service` · **不是** `CleanupJob`)
- L308:`taskDraftSvc := taskdraftsvc.NewService(taskDraftRepo, permissionLogRepo, mdb)`
- L373:`router := transport.NewRouter(...)` HTTP 路由组装
- L375-L379:**§7 Background workers** 已有 worker 启动模式 · `workerCtx, cancelWorkers := context.WithCancel(...)`;`workers.NewGroup(...).Start(workerCtx)`;**R6.A.2 cron 挂载点位**位于此块之后、§8 HTTP server 之前。
- L388-L393:`go srv.ListenAndServe()` HTTP 启动。
- L395-L408:**§9 Graceful shutdown** 模式:`signal.Notify(quit, SIGINT, SIGTERM)` → `cancelWorkers()` → `srv.Shutdown(30s timeout)`。**R6.A.2 cron stop 应在 `cancelWorkers()` 与 `srv.Shutdown` 之间**(顺序:停接受新 job → 等当前 cron job tick 完 → 关 HTTP)。

### §1.5 SA-A `CleanupJob` 构造依赖(必读)
`cmd/server` 当前没有 `assetlifecycle.NewCleanupJob` 的实例化点(只有 `NewService`)· R6.A.1 CLI 在 `cmd/tools/run-cleanup/main.go` 中按以下方式构建:
```go
mdb := mysqlrepo.New(db)
lifecycleRepo := mysqlrepo.NewTaskAssetLifecycleRepo(mdb)
deleter := /* OSSDirectService 或 noopDeleter */
job := assetlifecycle.NewCleanupJob(lifecycleRepo, mdb, deleter, log.New(os.Stderr, "", log.LstdFlags))
```
R6.A.2 在 cmd/server 中复用同样的 wiring · 但 **deleter 直接用 ossDirectSvc**(已在 main.go 内 instantiated · 即 line 287 的第 4 参) · `lifecycleRepo` 与 `mdb` 也在 main.go 已有(taskAssetLifecycleRepo + mdb)。R6.A.2 **不要**重复 sql.Open · 复用现有 db。

### §1.6 SA-C `CleanupExpired` 调用方式(必读)
`taskDraftSvc.CleanupExpired(ctx) (int, error)` · 直接调用即可 · `taskDraftSvc` 在 main.go L308 已构造。

---

## §2 范围与产出

### §2.1 在范围(本轮交付)
1. 引入 `github.com/robfig/cron/v3` 到 `go.mod` + `go.sum`(`go get github.com/robfig/cron/v3@latest`)。
2. **重写** `service/asset_lifecycle/scheduler/register.go` 为通用 cron infra(API 见 §3.1)· 旧 `Config{Enabled bool}` / `Register(...)` 全删(无人引用)。
3. **新增** `service/asset_lifecycle/scheduler/scheduler_test.go` · unit 测覆盖 New/Add/Start/Stop/cron expression 解析 + 假 job 触发(用 `time.AfterFunc` 模拟 fast tick · 不依赖真 cron 调度时延)。
4. **修改** `cmd/server/main.go`:在 §7 Background workers 块之后、§8 HTTP server 之前,加 §7.1 Cron 块 · 解析 4 个 ENV(`ENABLE_CRON_OSS_365`、`CRON_SCHEDULE_OSS_365`、`ENABLE_CRON_DRAFTS_7D`、`CRON_SCHEDULE_DRAFTS_7D`)· 默认 disabled · ENABLE 时挂对应 job · graceful shutdown 顺序补上 cron.Stop。
5. **新增** integration test(在 `service/asset_lifecycle/scheduler/cron_integration_test.go`)· 验证:cron expression `@every 1s` 触发 fake job ≥ 2 次内于 3s · `Stop()` 后 30ms 再不触发。**不**调用真 `CleanupJob.Run`(避免 OSS 副作用)· 用 mock job。
6. 报告 `docs/iterations/V1_R6_A_2_REPORT.md`(模板见 §6)。

### §2.2 不在范围(R6.A.2 严禁)
- ❌ 修改 `service/asset_lifecycle/cleanup_job.go` 或 `service/asset_lifecycle/service.go`
- ❌ 修改 `service/task_draft/service.go` 或 SA-C 任何文件
- ❌ 修改 `cmd/tools/run-cleanup/*`(R6.A.1 已签)
- ❌ 修改 `docs/api/openapi.yaml`(本轮零路由)
- ❌ 新增 `db/migrations/*`(本轮零 schema 改动)
- ❌ 引 `gocron` / 自写 cron parser / 用 `time.Ticker` 替代 cron 表达式
- ❌ 真跑 OSS 删除或真删 task_drafts 作 integration(用 fake job)
- ❌ 在 `service/task_draft/` 下新建 scheduler 子包(通用 scheduler 复用 asset_lifecycle/scheduler/)
- ❌ 修改 §7 Background workers 块或 worker 类的接口
- ❌ 把 cron 挂在 init() / TestMain · 必须挂在 main() 受 graceful shutdown 控制

---

## §3 设计与实装契约

### §3.1 scheduler 包 API(强制)
**package** `scheduler`(路径 `service/asset_lifecycle/scheduler` 不变 · 加包级注释说明此包是**通用 cron** · 命名将于 R6.D 重命名)

```go
package scheduler

import (
    "context"
    "fmt"
    "log"
    "sync"

    "github.com/robfig/cron/v3"
)

// JobFunc is the unit a Cron entry executes. It receives a context that is
// cancelled when the cron is stopped or the parent context is cancelled.
type JobFunc func(ctx context.Context) error

// Cron wraps robfig/cron/v3 with structured logging and a cancellable context.
//
// TODO(R6.D): rename package from service/asset_lifecycle/scheduler to a
// neutral location once SA-A v2.1 docs are revised.
type Cron struct {
    inner  *cron.Cron
    parent context.Context
    cancel context.CancelFunc
    logger *log.Logger
    mu     sync.Mutex
    jobs   []entry
}

type entry struct {
    name string
    spec string
    id   cron.EntryID
}

// New creates a stopped Cron. Caller must call Add(...) then Start().
func New(parent context.Context, logger *log.Logger) *Cron { ... }

// Add registers a named job with the given cron spec. Returns error if the
// spec fails to parse. Safe to call before Start. After Start, additional
// Add calls schedule immediately.
func (c *Cron) Add(name, spec string, job JobFunc) error { ... }

// Start begins ticking. Idempotent.
func (c *Cron) Start() { ... }

// Stop signals all running jobs to finish and waits up to ctx timeout.
// Returns the cron's done context to inspect timing.
func (c *Cron) Stop(ctx context.Context) error { ... }

// Entries returns a snapshot of currently registered job names + specs.
// Used by tests and observability hooks.
func (c *Cron) Entries() []EntryInfo { ... }

type EntryInfo struct {
    Name string
    Spec string
}
```

实装要求:
- `New` 内部 `cron.New(cron.WithSeconds())`?**不要** · 用 5 字段标准表达式(分时日月周)· 与 cron table 等价 · 简化 ENV 配置体验。但 integration test 需要 sub-second 触发 → 用 `@every 1s` 描述符(`robfig/cron/v3` 默认支持 `@every` 不需要 `WithSeconds()`)。
- `Add` 在内部 `cron.AddFunc(spec, wrapped)` · `wrapped` 把 `c.parent` ctx 传给 job · 并在 job 返回 error 时 `logger.Printf("[CRON] job=%s err=%v", name, err)` 不 panic。
- `Start` 调 `c.inner.Start()` · 第二次调 noop。
- `Stop` 调 `c.inner.Stop()` 拿到 `done <-chan struct{}` · 等 ctx done 或 done close · 取较先者 · `c.cancel()` 关 parent。返 nil 或 ctx.Err。
- `JobFunc` 不能 panic(wrapped 用 `defer recover()`)· panic 时 log error 不 propagate。
- 包级 `log.Default()` fallback:若 `New(parent, nil)` · 用 `log.Default()`。

### §3.2 cmd/server cron 挂载块(强制位置 · §7 后)

```go
    // ── 7.1 Cron(R6.A.2) ─────────────────────────────────────────────────────
    cronInst := scheduler.New(workerCtx, log.New(os.Stderr, "", log.LstdFlags))
    if os.Getenv("ENABLE_CRON_OSS_365") == "1" {
        ossSpec := envOr("CRON_SCHEDULE_OSS_365", "0 3 * * *")
        cleanupJob := assetlifecycle.NewCleanupJob(taskAssetLifecycleRepo, mdb, ossDirectSvc, log.New(os.Stderr, "[ASSET-CLEANUP-CRON] ", log.LstdFlags))
        if err := cronInst.Add("oss-365", ossSpec, func(ctx context.Context) error {
            _, appErr := cleanupJob.Run(ctx, assetlifecycle.CleanupOptions{Limit: 1000})
            if appErr != nil {
                return fmt.Errorf("%s: %s", appErr.Code, appErr.Message)
            }
            return nil
        }); err != nil {
            logger.Fatal("cron oss-365 add failed", zap.Error(err))
        }
        logger.Info("cron oss-365 enabled", zap.String("spec", ossSpec))
    }
    if os.Getenv("ENABLE_CRON_DRAFTS_7D") == "1" {
        draftSpec := envOr("CRON_SCHEDULE_DRAFTS_7D", "0 4 * * *")
        if err := cronInst.Add("drafts-7d", draftSpec, func(ctx context.Context) error {
            _, err := taskDraftSvc.CleanupExpired(ctx)
            return err
        }); err != nil {
            logger.Fatal("cron drafts-7d add failed", zap.Error(err))
        }
        logger.Info("cron drafts-7d enabled", zap.String("spec", draftSpec))
    }
    cronInst.Start()
    logger.Info("cron started", zap.Int("entries", len(cronInst.Entries())))
```

graceful shutdown(§9)修改:
```go
    cancelWorkers() // stop background workers first
    cronStopCtx, cronCancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cronCancel()
    if err := cronInst.Stop(cronStopCtx); err != nil {
        logger.Warn("cron stop timeout/err", zap.Error(err))
    }
    // existing srv.Shutdown(...)
```

`envOr` helper:若 cmd/server 已有 `envOr`(grep 之确认)直接复用 · 否则在 main.go 末尾追加:
```go
func envOr(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}
```

`taskAssetLifecycleRepo` 与 `mdb` 必须复用 main.go 早期已有的局部变量 · **不**重复 `mysqlrepo.NewTaskAssetLifecycleRepo` / `mysqlrepo.New` 调用。如果当前 main.go 没把这两个变量保存在外层作用域 · 提升其作用域(read 一遍 main.go 确认)。

### §3.3 unit test(必须项)
`service/asset_lifecycle/scheduler/scheduler_test.go`(无 build tag · 跟随 default `go test` 跑):
- `TestCron_New_DefaultLogger`:`New(ctx, nil)` 不 panic。
- `TestCron_Add_BadSpec`:`Add("bad", "not a cron", noop)` 返非 nil error · 不 panic。
- `TestCron_AddStart_Stop_NoEntry`:无 entry 直接 Start + Stop · 30ms 内返回 nil。
- `TestCron_Entries`:Add 2 个 entry · `Entries()` 返长度 2 · 含 name/spec。
- **不依赖真 cron tick**(避免 1 秒 sleep)· 用 `Cron.Entries()` 作快照断言。

### §3.4 integration test(必须项)
`service/asset_lifecycle/scheduler/cron_integration_test.go` 加 `//go:build integration` · 不依赖 DB(纯 cron 逻辑测试):
- `TestCronTick_FiresFakeJob`:用 `@every 1s` 注册 fake job(`atomic.Int32` 计数)· `Start()` · `time.Sleep(2500*time.Millisecond)`(留 1 个 tick 余量) · 计数 ≥ 2(因 robfig `@every` 立即首跑 + 1s 后第 2 跑)。`Stop(2s ctx)` · sleep 1.2s · 计数不再增加。

### §3.5 报告骨架(`docs/iterations/V1_R6_A_2_REPORT.md`)

```markdown
# V1 R6.A.2 Report · cron 基础设施 + 两条清理 job 启用

## §1 Scope
通用 cron infra · cmd/server gate 启用 · 0 业务代码改动 · 0 OpenAPI 改动。

## §2 实装清单
| 文件 | 动作 | 行数 | 说明 |
| ... |

## §3 wiring 决策
- robfig/cron/v3 选型理由
- 包路径保留 service/asset_lifecycle/scheduler 不破 SA-A v2.1 契约 · 加 TODO(R6.D) 重命名
- ENV gate 默认 disabled
- graceful shutdown 顺序:cancelWorkers → cron.Stop → srv.Shutdown

## §4 测试输出
- `go test ./service/asset_lifecycle/scheduler/...` PASS
- `go test -tags=integration ./service/asset_lifecycle/scheduler/...` PASS
- `go build ./...` PASS
- `go vet ./...` PASS
- SA-A regression `go test -tags=integration -run TestSA_A_ ./service/asset_lifecycle/...` PASS
- SA-C regression `go test -tags=integration ./service/task_draft/...` PASS
- R6.A.1 regression `go test -tags=integration -run TestR6A1_ ./cmd/tools/run-cleanup/...` PASS(本轮零退化)
- cmd/server build:`go build ./cmd/server` PASS
- cmd/server smoke(默认 ENABLE_CRON_*=空):启动 → log "cron started entries=0" → SIGTERM → log "cron stop" → 退出 exit=0

## §5 ENV 配置
| 变量 | 默认 | 含义 |
| --- | --- | --- |
| ENABLE_CRON_OSS_365 | 空 | "1" 启用 OSS 365 天清理 |
| CRON_SCHEDULE_OSS_365 | "0 3 * * *" | 每日 03:00 UTC |
| ENABLE_CRON_DRAFTS_7D | 空 | "1" 启用草稿 7 天清理 |
| CRON_SCHEDULE_DRAFTS_7D | "0 4 * * *" | 每日 04:00 UTC |

## §6 sign-off candidate
本轮基础设施层 PASS candidate · staging 实测真 cron tick 触发 cleanup_job.Run 与 CleanupExpired 留 R6.A.3(staging 演练 + AS-A5 验收)。

## §7 终止符
R6_A_2_DONE_PENDING_ARCHITECT_VERIFY
```

---

## §4 文件白名单(允许 codex 创建/编辑 · 其他一律 ABORT)

| 文件 | 动作 |
| --- | --- |
| `go.mod` | 编辑(`go get github.com/robfig/cron/v3` 自动)|
| `go.sum` | 编辑(`go mod tidy` 自动)|
| `service/asset_lifecycle/scheduler/register.go` | **重写**(旧 23 行 dummy 全删 · 新通用 cron · 见 §3.1)|
| `service/asset_lifecycle/scheduler/scheduler_test.go` | **新建** unit test(§3.3)|
| `service/asset_lifecycle/scheduler/cron_integration_test.go` | **新建** integration test(§3.4)|
| `cmd/server/main.go` | 编辑(§3.2 cron 块 + graceful shutdown 修改 + envOr helper · 不动 §1~§7 现有逻辑)|
| `docs/iterations/V1_R6_A_2_REPORT.md` | **新建** 报告(§3.5)|

任何对其他文件的写入 = **ABORT**。

---

## §5 验证清单(codex 必须按序跑 · 每步出 PASS/FAIL · 全 PASS 才能写终止符)

1. `go get github.com/robfig/cron/v3` 完成 · `go.mod` 出现 `require github.com/robfig/cron/v3` 行。
2. `go build ./...` exit=0(全仓 build 通过)。
3. `go vet ./...` exit=0。
4. `go test -count=1 ./service/asset_lifecycle/scheduler/...` PASS · 4 个 unit test 全过(§3.3)。
5. `go build -tags=integration ./service/asset_lifecycle/scheduler/...` exit=0。
6. `go test -tags=integration -count=1 ./service/asset_lifecycle/scheduler/...` PASS · `TestCronTick_FiresFakeJob` 真触发 ≥ 2 次。
7. **SA-A regression**:`MYSQL_DSN=<jst_erp_r3_test> R35_MODE=1 go test -tags=integration -count=1 -run "TestSA_A_" ./service/asset_lifecycle/...` PASS · 8 用例全过 · 与 R6.A.1 一致。
8. **SA-C regression**:`MYSQL_DSN=<...> R35_MODE=1 go test -tags=integration -count=1 ./service/task_draft/...` PASS · 4 用例全过。
9. **R6.A.1 regression**:`MYSQL_DSN=<...> R35_MODE=1 go test -tags=integration -count=1 -run TestR6A1_ ./cmd/tools/run-cleanup/...` PASS · 5 用例全过 · 1000 条 E2E 仍 ≤ 90s。
10. `go build ./cmd/server` PASS。
11. **cmd/server smoke**(默认 ENV 空):
    - 启 cmd/server(`OSS_DELETER_DISABLED=1 ENABLE_CRON_OSS_365= ENABLE_CRON_DRAFTS_7D= go run ./cmd/server`)
    - 等 5s · 检查 stderr 含 `"cron started"` 且 `entries=0`(因 ENV 空 · 0 个 job 挂)
    - 健康检查:`curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8080/healthz` == `200`
    - SIGTERM(`kill <pid>`)· 等 3s · 进程 exit=0 · 日志含 `"cron stop"` 与 `"server stopped gracefully"`
12. **gate-on smoke**(可选证明 cron 真挂):`ENABLE_CRON_OSS_365=1 CRON_SCHEDULE_OSS_365="@every 30s" OSS_DELETER_DISABLED=1 go run ./cmd/server` · 等 1s · 日志含 `"cron oss-365 enabled spec=@every 30s"` 与 `entries=1` · SIGTERM · cron.Stop。**不等真 tick**(30s 太长)· 仅证明 wire 通。
13. 报告 `docs/iterations/V1_R6_A_2_REPORT.md` 写完 · §6 写入 R6_A_2_DONE_PENDING_ARCHITECT_VERIFY 终止符。

---

## §6 ABORT 触发器(任何一条命中立即停 · 写报告说明)

1. ✋ 任何对 `service/asset_lifecycle/cleanup_job.go` / `service/asset_lifecycle/service.go` / `service/task_draft/service.go` 的写入。
2. ✋ 任何对 `cmd/tools/run-cleanup/*` 的写入。
3. ✋ 任何对 `docs/api/openapi.yaml` 的写入。
4. ✋ 任何 `db/migrations/*` 新文件。
5. ✋ `go test -count=1 ./service/asset_lifecycle/scheduler/...` 单测 FAIL。
6. ✋ SA-A / SA-C / R6.A.1 任一回归 FAIL(本轮零退化硬门)。
7. ✋ `go build ./cmd/server` FAIL。
8. ✋ cmd/server 启动后 healthz != 200 或 stderr 出现 `panic` / `fatal`。
9. ✋ 试图引入 `gocron` 或非 `robfig/cron/v3` 的 cron 库。
10. ✋ 试图在 `task_draft/` 下新建 scheduler 子包(违反 §1.3 单包共用决策)。
11. ✋ 试图把 cron 实例挂在 `init()` 或 `TestMain` 之外的非 `main()` 位置。
12. ✋ `cronInst.Stop` 被放在 `srv.Shutdown` 之后(顺序错误)。

---

## §7 codex TUI 操作 SOP(给我用 · 你只跑 prompt)

我把这份 prompt 路径粘给 codex TUI:
```
/path/to/yongboWorkflow/go/prompts/V1_R6_A_2_CRON_INFRA.md
```
要求 codex:
1. 严格按 §4 白名单 · 任何越线 ABORT;
2. 严格按 §5 验证清单逐项跑 · 每步 PASS/FAIL 写报告 §4;
3. 严格按 §6 ABORT 触发器自检;
4. 完成后写 `docs/iterations/V1_R6_A_2_REPORT.md` + 终止符 `R6_A_2_DONE_PENDING_ARCHITECT_VERIFY`;
5. **不要**清空 [50000,60000) 段(本轮 integration 不动 DB · 无段污染风险);
6. **不要**碰 SA-B / SA-D / R3 / R5 任何文件。

完成后我做架构师独立 verify(预计 2 分钟):
- 读 `register.go` / `scheduler_test.go` / `cron_integration_test.go` / `main.go` diff
- 重跑 §5 关键步骤(unit + integration + regression + smoke)
- 改判 V1_R6_A_2_REPORT.md PASS · 推 ROADMAP 进 R6.A.3
