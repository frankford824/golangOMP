# V1 R6.A.2 Report · cron 基础设施 + 两条清理 job 启用

> 架构师终判:**PASS · architect-cleared**(详见 §8)。Codex 自跑命中 ABORT 系测试库段污染 / 测试间累积残留所致 · 不是 R6.A.2 实装引入的真退化。架构师独立复跑(干净段)全 PASS。

## §1 Scope
通用 cron infra · cmd/server ENV gate 启用 · 0 业务清理代码改动 · 0 OpenAPI 改动。

本报告原状态为 ABORTED · 经架构师独立复跑 + 段隔离审计 · 改判 PASS。

## §2 实装清单
| 文件 | 动作 | 行数 | 说明 |
| --- | --- | ---: | --- |
| `go.mod` | 编辑 | 62 | 新增 `github.com/robfig/cron/v3 v3.0.1` |
| `go.sum` | 编辑 | 160 | 新增 robfig/cron 校验和 |
| `service/asset_lifecycle/scheduler/register.go` | 重写 | 149 | robfig cron wrapper + cancellable context + panic/error logging |
| `service/asset_lifecycle/scheduler/scheduler_test.go` | 新建 | 55 | New/Add/Start/Stop/Entries unit tests |
| `service/asset_lifecycle/scheduler/cron_integration_test.go` | 新建 | 36 | `@every 1s` fake job tick integration |
| `cmd/server/main.go` | 编辑 | 539 | §7.1 Cron gate + graceful shutdown `cancelWorkers -> cron.Stop -> srv.Shutdown` |
| `docs/iterations/V1_R6_A_2_REPORT.md` | 新建 | 43 | 本报告 |

## §3 wiring 决策
- 选 `robfig/cron/v3`:支持标准 5 字段 cron spec 和 `@every`, 无自写 parser。
- 包路径保留 `service/asset_lifecycle/scheduler`, 并加 `TODO(R6.D)` 说明后续迁到中性路径。
- 当前 checkout 实际存在旧 `register_test.go`, 与 prompt baseline “无 tests” 不一致。为不改白名单外文件, `register.go` 保留 `Config/DefaultConfig/Register` 兼容 shim; server 新路径只用 `New/Add/Start/Stop`。
- ENV gate 默认 disabled:`ENABLE_CRON_OSS_365` / `ENABLE_CRON_DRAFTS_7D` 只有值为 `"1"` 才挂 job。
- `cmd/server` 复用既有 `mdb`, `taskAssetLifecycleRepo`, `ossDirectSvc`, `taskDraftSvc`; 不重复 `sql.Open`。
- graceful shutdown 顺序:`cancelWorkers()` -> `cronInst.Stop(15s)` -> `srv.Shutdown(30s)`。

## §4 测试输出
- `go get github.com/robfig/cron/v3@latest`:PASS, resolved `v3.0.1`.
- `go mod tidy`:PASS, `go.mod` direct require present.
- `go build ./...`:PASS.
- `go vet ./...`:PASS.
- `go test -count=1 ./service/asset_lifecycle/scheduler/...`:PASS.
- `go build -tags=integration ./service/asset_lifecycle/scheduler/...`:PASS.
- `go test -tags=integration -count=1 ./service/asset_lifecycle/scheduler/...`:PASS, fake job tick test passed.
- `go build ./cmd/server`:PASS.
- SA-C regression with `MYSQL_DSN=jst_erp_r3_test` + `R35_MODE=1`:PASS, `ok workflow/service/task_draft 4.441s`.
- SA-A regression with `MYSQL_DSN=jst_erp_r3_test` + `R35_MODE=1`:FAIL. Failures: `TestSA_A_I3_ArchiveRoleAndEvent` got `NOT_FOUND`; `TestSA_A_I6_DownloadAutoCleanedGoneDeletedNotFound` got `NOT_FOUND` want gone; `TestSA_A_I8_CleanupJobDryRunRealRunIdempotent` cleaned again `10`, want `0`.
- R6.A.1 regression with `MYSQL_DSN=jst_erp_r3_test` + `R35_MODE=1`:FAIL. Failures: `TestR6A1_Drafts7d` audit left `task_modules=2`; `TestR6A1_AS_A5_E2E` cleaned `981`, want `1000`.
- cmd/server smoke default gate:BLOCKED/FAIL in this local environment before cron start when using default DSN, MySQL rejected `root:password`; not rerun after regression ABORT.

## §5 ENV 配置
| 变量 | 默认 | 含义 |
| --- | --- | --- |
| `ENABLE_CRON_OSS_365` | 空 | `"1"` 启用 OSS 365 天清理 |
| `CRON_SCHEDULE_OSS_365` | `"0 3 * * *"` | 每日 03:00 |
| `ENABLE_CRON_DRAFTS_7D` | 空 | `"1"` 启用草稿过期清理 |
| `CRON_SCHEDULE_DRAFTS_7D` | `"0 4 * * *"` | 每日 04:00 |

## §6 sign-off candidate
原:不是 PASS candidate(基础设施通过 · regression 失败)。
改判:**PASS candidate**(架构师独立复跑全过 · 详见 §8)。

## §7 终止符
原:ABORTED: `R6_A_2_DONE_PENDING_ARCHITECT_VERIFY` 未签发。
改判:`R6_A_2_DONE_ARCHITECT_CLEARED`。

## §8 架构师独立裁决(architect-cleared)

### §8.1 复核动作

| # | 检查项 | 命令 / 证据 | 结果 |
| --- | --- | --- | --- |
| 1 | 段隔离审计 + 清(干净段开跑) | `tmp/r6_a_1_isolation_run.sh` · `[50000,60000)` 9 表 BEFORE/AFTER 全 0 | PASS |
| 2 | scheduler unit | `go test -count=1 -v ./service/asset_lifecycle/scheduler/...` | PASS · 6/6 · `0.004s` |
| 3 | scheduler integration | `go test -count=1 -tags=integration ./service/asset_lifecycle/scheduler/...` · `TestCronTick_FiresFakeJob 3.70s` | PASS |
| 4 | 全栈构建 | `go build ./...` | PASS |
| 5 | 静态检查 | `go vet ./...` | PASS |
| 6 | server 构建 | `go build ./cmd/server` | PASS |
| 7 | R6.A.1 regression(全套) | `go test -count=1 -tags=integration -v -run ^TestR6A1 ./cmd/tools/run-cleanup/...` | PASS · 5/5 · `OSS365_DryRun 1.88s · OSS365_RealRun 2.21s · Drafts7d 0.74s · AS_A5_E2E 146.35s(seed1000+cleanup elapsed_ms=69781)`,合计 `151.190s` |
| 8 | SA-A regression | `go test -count=1 -tags=integration ./service/asset_lifecycle` | PASS · `8.021s` · I3/I6/I8 全过 |
| 9 | SA-C regression | `go test -count=1 -tags=integration ./service/task_draft` | PASS · `2.451s` |
| 10 | cmd/server cron 块审计 | 默认 ENV 空 · 不挂任何 job · `cronInst.Start()` 后 entries=0 · 安全 | PASS |
| 11 | 白名单遵守 | 业务清理代码 `service/asset_lifecycle/cleanup_job.go` / `service/task_draft/service.go` 未动 · 仅动 `scheduler/register.go`(及测试)+ `cmd/server/main.go` §7.1 + §9 + go.mod/go.sum | PASS |

### §8.2 codex ABORT 根因 (false-positive)

| codex 报失败项 | 真因 | 本次复跑结果 |
| --- | --- | --- |
| `TestSA_A_I3/I6/I8` | SA-A 段 `taskID=20002~20009` · `cleanup_job.Run` 扫**全表无段过滤** · codex 跑前未做段隔离 audit · 前次 SA-A run 未清干净的残留 + 累积测试 fixture 污染了 SA-A 用例 | PASS · 8.021s |
| `TestR6A1_Drafts7d` 段审计 `task_modules=2` 残留 | 同上 · codex 顺序跑 SA-A → R6A1 时 SA-A fail 未清完段 · 累积留入 R6A1 audit | PASS · 0.74s · 段审计干净 |
| `TestR6A1_AS_A5_E2E cleaned 981 / 1000` | repo `ListEligibleForCleanup` `ORDER BY t.updated_at ASC` · LIMIT=1000 · 当时 baseline 有 ~19 条更早(非段内)的历史候选占用 limit slot · R6A1 自身 1000 条只清到 981 | PASS · cleaned=1000 · elapsed_ms=69781 · 干净段下候选只剩 R6A1 自己 seed |

### §8.3 prompt 缺陷追溯(架构师认账)

- prompt §1.3 写"baseline:`scheduler/` 仅 `register.go` 23 行 · 无 tests" — 与现实(已存在 `register_test.go`)不一致。codex 选择保留 `Config/DefaultConfig/Register` 兼容 shim 而不删除是合理 conservative 决定 · 报告 §3 也明确披露,**不计违白名单**。架构师认账,本应在 prompt 里把 `register_test.go` 列为 baseline 已存在并明确"shim 保留 OK"。
- prompt §6 ABORT 条款只要求"不能引入 regression",未明确要求"先做段隔离 audit 再跑 regression"。codex 顺序跑导致段累积污染 → 误判 regression。本次架构师 verify 已用 `tmp/r6_a_1_isolation_run.sh` 在每次跑前清段 · 全 PASS。后续 prompt 模板将明确要求 verify 流程 step-1 = 段隔离 audit + clean。

### §8.4 终判

R6.A.2(cron 基础设施 + ENV gate)**PASS · architect-cleared**。
- 实装:`scheduler/register.go`(149 行 · robfig/cron/v3 wrapper · 含 panic 捕获 · cancellable context · 兼容 shim)+ `cmd/server/main.go` §7.1 cron 挂载块(默认 disabled · ENV `"1"` 启用)+ §9 graceful shutdown(`cancelWorkers → cron.Stop(15s) → srv.Shutdown(30s)`)
- 测试:scheduler 6 unit + 1 integration · 全 PASS
- 回归:R6.A.1 5/5 · SA-A 8 用例 · SA-C 全过 · 段干净
- 默认安全:`ENABLE_CRON_OSS_365` / `ENABLE_CRON_DRAFTS_7D` 不显式设 `"1"` 时 entries=0 · cron 不会清任何 row(生产部署默认无副作用)

终止符改签:`R6_A_2_DONE_ARCHITECT_CLEARED`。

签字:架构师 · 2026-04-24。
