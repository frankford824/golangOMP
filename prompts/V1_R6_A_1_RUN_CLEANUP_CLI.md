# R6.A.1 · cmd/tools/run-cleanup CLI 双子命令(SRE 手动触发)· Codex Prompt

> 路线:`V1_ROADMAP §32 R6 + §171 修订版`(2026-04-24 删 Feature Flag 幻觉)· 本轮 = R6 第 1 个原子 prompt(共 13~22 个)
> 决策依据:Q1=A3(cron 全写默认 disabled · `cmd/run-cleanup` 手动触发 · UAT 后再决定开 cron)→ R6.A 拆为 R6.A.1(本轮 · 仅 CLI · 0 cron)+ R6.A.2(cron infra · 默认 disabled)
> 模式:**codex TUI 交互执行** · 完成后 stdout 末尾打印 `R6_A_1_DONE_PENDING_ARCHITECT_VERIFY` · 等架构师 verify · 不要自签 PASS

---

## 0. 角色

你是 Codex 后端 senior engineer。本任务范围**极窄**:为 SA-A v2.1 + SA-C v1 已写好的两个清理函数,加一个 `cmd/tools/run-cleanup` 命令行工具,让 SRE 能手动触发清理。**零 cron · 零 SA-A/SA-C 代码改动**。

## 1. 范围

**可做**:
- 新建 `cmd/tools/run-cleanup/main.go`
- 新建 `cmd/tools/run-cleanup/main_test.go`
- 新建 `cmd/tools/run-cleanup/integration_test.go`(build tag `integration`)
- 落报告 `docs/iterations/V1_R6_A_1_REPORT.md`
- 仅在确需新依赖时改 `go.mod`/`go.sum`(stdlib `flag` 应够 · 禁止引 cobra/urfave-cli 等第三方 CLI 框架)

**不可做(改一行 = ABORT)**:
- `service/asset_lifecycle/**`(SA-A v2.1 签字 · 含 `cleanup_job.go` / `scheduler/register.go`)
- `service/task_draft/**`(SA-C v1 签字 · 含 `service.go::CleanupExpired`)
- `cmd/server/main.go` 或 `cmd/api/main.go`(R6.A.2 才接 server)
- `repo/mysql/**` / `domain/**` / `transport/**`
- `docs/api/openapi.yaml`(本轮零 OpenAPI 改动)
- 任何 R1~R5 已签字测试文件(只能新建本目录下的测试)

## 1.5 Production baseline 锁定(防 schema 幻觉)

以下函数签名/事实是**架构师亲查的当前签字状态(2026-04-24)**,你必须复用而非重写:

```go
// service/asset_lifecycle/cleanup_job.go (UNTOUCHED)
func NewCleanupJob(lifecycleRepo repo.TaskAssetLifecycleRepo, txRunner repo.TxRunner, deleter ObjectDeleter, logger *log.Logger) *CleanupJob
func (j *CleanupJob) Run(ctx context.Context, opts CleanupOptions) (*CleanupResult, *domain.AppError)
type CleanupOptions struct { DryRun bool; Limit int }
type CleanupResult  struct { DryRun bool; Scanned int; Cleaned int; Candidates []*repo.TaskAssetCleanupCandidate }
const CleanupLogPrefix = "[ASSET-CLEANUP]"
// cutoff 内置 now().UTC().AddDate(0,0,-365) · 不要重新实现 365 day 计算

// service/task_draft/service.go:166 (UNTOUCHED)
func (s *Service) CleanupExpired(ctx context.Context) (int, error)  // 返回删除条数
```

DB 列名(R2 已落 · 生产 jst_erp 实测):
- `task_assets`/`task_asset_versions`/`task_drafts` 三表 schema 完整
- `task_drafts.expires_at DATETIME NOT NULL`(`s.drafts.DeleteExpired` 走该列)
- `task_module_events.event_type` 是字符串(SA-A 写入 `asset_auto_cleaned`)

`ObjectDeleter` 接口:在 `service/asset_lifecycle/` 包内,自己读对位实装(`service.go` 或 `delete_service.go`)。

OSS 客户端初始化:**复用** `cmd/server/main.go` 现有的 OSS env 读法(env 名 + sdk new client 模式),但**不要 import cmd/server 包**,把代码直接抄到本 main.go(允许 ~30 行重复 · 防止 cmd 间相互 import)。

## 2. 必读上下文(按顺序)

1. `service/asset_lifecycle/cleanup_job.go` 全文
2. `service/asset_lifecycle/sa_a_integration_test.go`(看 SA-A 怎么 wire CleanupJob · 你复用同 wiring 模式)
3. `service/asset_lifecycle/service.go`(找 ObjectDeleter 接口定义)
4. `service/task_draft/service.go` L40~170
5. `service/task_draft/sa_c_test_helper_test.go`(SA-C 测试 wiring 模式)
6. `cmd/tools/migrate_v1_forward/main.go`(现存 cmd/tools 工具的 main 模式参考)
7. `cmd/server/main.go`(OSS / MySQL DSN 初始化代码 · 仅复制不 import)
8. `prompts/V1_R5_BATCH_SKU.md` §1.5 + §7(R5 prompt 范本 · prompt 风格参照)

## 3. 实施清单(6 步)

### 3.1 main.go 子命令路由

- 第一个位置参数 = 子命令名(`oss-365` / `drafts-7d`),其他值打 usage + `os.Exit(2)`
- 公共 flag(`flag.NewFlagSet` 每个子命令独立):
  - `--dry-run`(默认 false)
  - `--limit int`(默认 100 · 仅 oss-365 用)
  - `--dsn string`(默认 `os.Getenv("MYSQL_DSN")`)
  - `--reason string`(默认 `v1.r6.a.1.manual.cleanup` · 当前未写入,留给 R6.A.2 cron job 用,但参数先占位)
  - `--json`(默认 true · 输出结构化 JSON)
- 启动行 stderr 打印:`[RUN-CLEANUP] subcommand=<name> dry_run=<bool> limit=<n> ts=<RFC3339>`
- stdout 最后行 = JSON 报告(单行,便于 jq):
  - `oss-365` 成功:`{"subcommand":"oss-365","dry_run":bool,"scanned":int,"cleaned":int,"elapsed_ms":int}`
  - `drafts-7d` 成功:`{"subcommand":"drafts-7d","dry_run":bool,"deleted":int,"elapsed_ms":int}`
  - 失败:`{"subcommand":"...","error":"..."}` + `os.Exit(1)`

### 3.2 oss-365 子命令(wire SA-A CleanupJob)

- 从 `--dsn` 建 `*sql.DB`(`sql.Open("mysql", dsn)` + `db.PingContext`)
- 建 `lifecycleRepo := mysqlrepo.NewTaskAssetLifecycleRepo(db)` + `txRunner := mysqlrepo.NewTxRunner(db)`(自行确认 constructor 名 · 见 SA-A integration test)
- 建 ObjectDeleter:
  - 若 `--dry-run` 或 ENV `OSS_DELETER_DISABLED=1`:wire no-op deleter(实现 `ObjectDeleter` 接口 · `Enabled()` 返 false)
  - 否则按 cmd/server 模式从 ENV(OSS_ENDPOINT / OSS_BUCKET / OSS_AK / OSS_SK 等 · 自查 cmd/server 真实 env 名)初始化真 deleter
- `NewCleanupJob(lifecycleRepo, txRunner, deleter, log.New(os.Stderr, "", log.LstdFlags))`
- `result, appErr := job.Run(ctx, asset_lifecycle.CleanupOptions{DryRun: dryRun, Limit: limit})`
- 输出 JSON 报告 + exit 0/1

### 3.3 drafts-7d 子命令(wire SA-C Service)

- 同步建 DSN
- 建 task_draft repo + Service(自查 SA-C 现有 constructor 签名)
- `n, err := srv.CleanupExpired(ctx)`
- 输出 JSON 报告 + exit 0/1

### 3.4 单测 main_test.go(纯 stdlib 测试 · 不连 DB)

- `TestUsage_NoArgs` · `TestUsage_UnknownSubcommand` → exit code 2
- `TestFlagParsing_OSS365` · `TestFlagParsing_Drafts7d` → 验 `--limit 50 --dry-run` 正确解析
- 用 `os/exec` 调本程序或重构成可测函数(参 `cmd/tools/migrate_v1_forward/main.go` 怎么做)

### 3.5 integration test integration_test.go(build tag `integration`)

测试库 `jst_erp_r3_test` · 用户/数据 ID 段 **`[50000, 60000)`** · 每 test 必须 `t.Cleanup` 兜底清理 · 段外写入 = 测试 fail。

四个测试,函数名前缀 `TestR6A1_`:

- `TestR6A1_OSS365_DryRun`:
  - 造 5 个 task_asset_versions(`task_assets.task_id` 在 [50000,55000) · 关联任务 `tasks.closed_at < NOW()-INTERVAL 366 DAY`)+ 5 个未过期(`closed_at < NOW()-INTERVAL 100 DAY`)
  - 调本程序 oss-365 子命令 `--dry-run --limit=100`
  - 验 stdout JSON `scanned=5 cleaned=0`
  - 验 DB:`auto_cleaned_at` 仍 NULL · `task_module_events` 无新事件

- `TestR6A1_OSS365_RealRun`:
  - 同上造 5+5 · 用 no-op deleter(ENV `OSS_DELETER_DISABLED=1`)
  - 调 oss-365 `--limit=100`(无 --dry-run)
  - 验 JSON `scanned=5 cleaned=5`
  - 验 DB:5 行 `auto_cleaned_at IS NOT NULL` + 5 行 `task_module_events.event_type='asset_auto_cleaned'`

- `TestR6A1_Drafts7d`:
  - 造 3 条 `task_drafts.expires_at < NOW()` + 3 条 `expires_at > NOW()`(owner_user_id 在 [50000,51000))
  - 调 drafts-7d
  - 验 JSON `deleted=3` · DB 仅留 3 条未过期

- `TestR6A1_AS_A5_E2E`:
  - 造 1000 条过期 versions(version_id 段 [55000, 56000))
  - 调 oss-365 `--limit=1000`
  - 验 JSON `cleaned=1000` + DB `auto_cleaned_at IS NOT NULL` 全 1000 + `task_module_events` 新增 1000 行 `asset_auto_cleaned`
  - 记录 elapsed_ms 到报告 §5(AS-A5 验收)

### 3.6 报告

`docs/iterations/V1_R6_A_1_REPORT.md`(参 `V1_R5_BATCH_SKU_REPORT.md` 节奏):
- §1 Scope · §2 实装清单(列文件)· §3 wiring 决策 · §4 单测 + integration 输出 · §5 AS-A5 1000 条 elapsed · §6 测试库 [50000,60000) 隔离 audit · §7 sign-off candidate · §8 终止符

## 4. allowed_files 白名单(改其他文件 = ABORT)

- 新建:`cmd/tools/run-cleanup/main.go`
- 新建:`cmd/tools/run-cleanup/main_test.go`
- 新建:`cmd/tools/run-cleanup/integration_test.go`
- 新建:`docs/iterations/V1_R6_A_1_REPORT.md`
- 仅在确需依赖时改:`go.mod` / `go.sum`

## 5. 验证(11 步)

执行后,在报告 §4 / §6 / §7 引用证据,全部必须 PASS:

1. `go build ./cmd/tools/run-cleanup/...` exit 0
2. `go build -tags=integration ./cmd/tools/run-cleanup/...` exit 0
3. `go build ./...` exit 0(整仓不破)
4. `go vet ./...` exit 0
5. `go test ./cmd/tools/run-cleanup/...` 全绿(单测)
6. `MYSQL_DSN=<jst_erp_r3_test> go test -tags=integration -count=1 -run TestR6A1_ ./cmd/tools/run-cleanup/...` 全绿
7. SA-A 旧 integration 抽样:`go test -tags=integration -count=1 ./service/asset_lifecycle/...` 全绿(回归)
8. SA-C 旧 integration 抽样:`go test -tags=integration -count=1 ./service/task_draft/...` 全绿(回归)
9. `docs/api/openapi.yaml` 0 改动(`git diff` 验)
10. CLI smoke:`./run-cleanup oss-365 --dry-run` 输出 JSON 可被 `jq .scanned` 解析 · `./run-cleanup drafts-7d --dry-run` 同
11. `[50000, 60000)` 段隔离 audit:测试跑完后 9 表(`users / tasks / task_modules / task_module_events / task_assets / task_asset_versions / task_drafts / notifications / permission_logs`)按 id 范围 [50000,60000) 残留全 0(t.Cleanup 必须真清干净)

## 6. 数据隔离

- 测试库:`jst_erp_r3_test` · 严禁碰 `jst_erp` 生产
- 用户/任务/资产 ID 段:`[50000, 60000)`(R5 已用过 · R6 沿用 · 不开新段)
- AS-A5 1000 条演练在 `version_id [55000, 56000)` 子段
- 每 test `t.Cleanup` 兜底 + 段外写入 → 立刻 `t.Fatalf`

## 7. ABORT 触发器(任一命中 → 写 `tmp/r6_a_1_ABORT.txt` + 立刻停止)

- 改 `service/asset_lifecycle/**` 任何一行
- 改 `service/task_draft/**` 任何一行
- 改 `cmd/server/main.go` 或 `cmd/api/main.go`
- 改 `repo/mysql/**`
- 引入 `robfig/cron` / `gocron` / 同等 cron 库(那是 R6.A.2)
- 引入 cobra/urfave-cli/spf13 等第三方 CLI 框架(stdlib flag 必须够)
- `cmd/tools/run-cleanup/**` 之外创建任何 Go 包
- 任一 `go test` 失败
- `openapi.yaml` 任何改动
- 测试代码段外([50000,60000) 之外)写入

## 8. 输出协议

跑完后:
1. 落 `docs/iterations/V1_R6_A_1_REPORT.md`(§1~§8 完整 · §8 = `R6_A_1_DONE_PENDING_ARCHITECT_VERIFY`)
2. **stdout 最后一行打印**:`R6_A_1_DONE_PENDING_ARCHITECT_VERIFY`
3. **不要自己改** `prompts/V1_ROADMAP.md` / `prompts/V1_R6_A_1_RUN_CLEANUP_CLI.md`
4. **不要自己签字 PASS** · 等架构师 verify
5. 如遇 ABORT 触发,写 `tmp/r6_a_1_ABORT.txt` 含原因 + stdout 打印 `R6_A_1_ABORT_<reason_code>` 后退出

---

## 附录 · 报告模板

```markdown
# V1 R6.A.1 Report · cmd/tools/run-cleanup CLI

## §1 Scope
单 prompt 实施 · cmd/tools/run-cleanup 双子命令 · 0 cron · 0 SA-A/SA-C 改动

## §2 实装清单
| 文件 | 行数 | 用途 |
| --- | --- | --- |
| cmd/tools/run-cleanup/main.go | ... | 子命令路由 + wiring |
| cmd/tools/run-cleanup/main_test.go | ... | 单测 |
| cmd/tools/run-cleanup/integration_test.go | ... | 4 个 TestR6A1_ |

## §3 wiring 决策
- 不引第三方 CLI 框架(stdlib flag 够)
- OSS 客户端代码段从 cmd/server 抄(允许 ~30 行重复 · 不 import)
- ObjectDeleter no-op 实现内联在 main.go

## §4 测试输出
go build / go test / integration test / OpenAPI diff 全 0

## §5 AS-A5 1000 条 E2E
elapsed_ms = ... · 平均 ... ms/条

## §6 [50000,60000) 9 表 audit
users=0 / tasks=0 / ... / permission_logs=0

## §7 sign-off candidate
PASS candidate

## §8 终止符
R6_A_1_DONE_PENDING_ARCHITECT_VERIFY
```
