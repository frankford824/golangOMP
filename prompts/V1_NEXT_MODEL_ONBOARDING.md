# V1 后端 · 下一模型接手包(V1.1 → V1.2 / V2 通用入口)

> Last updated: 2026-04-25
> 性质:**接手 prompt 起步包**(给下一个接手 V1 后端的 Codex / Claude / GPT 模型用)
> 当前状态:**Release v1.21 production deployed · test DB dropped · frontend docs ready · 2026-04-25**
> 终止符:`V1_RELEASE_v1_21_DONE_FRONTEND_DOCS_READY`
> 本文件不是 API 契约 · 不覆盖权威文档 · 仅作下一轮起步入口。

---

## §0 你是谁 · 你接的是什么

你接手的是一个 **backend-only Go 工程**(模块名 `workflow` · 仓位 `c:\Users\wsfwk\Downloads\yongboWorkflow\go`),已完成 V0.9 → V1.0 主链路重构、V1.1-A1 detail P99 收口与 Release v1.21 生产部署,目前处于前端联调起步状态:

- **闭环签字**:R1 → R6.A.4 + V1.1-A1 + Release v1.21 全部签字(详见 `docs/iterations/V1_RELEASE_v1_21_REPORT.md`)
- **P99 状态**:`/v1/tasks/{id}/detail` R6.A.4 `p99=334.721ms` RED 已由 V1.1-A1 收口;生产 v1.21 复测 warm `32.933ms` / cold `32.995ms`
- **前端文档**:`docs/frontend/INDEX.md` + 15 份 companion docs 已落盘,覆盖 203 个 `/v1` path
- **接手任务候选**:见 §6 候选下一轮(用户/起草模型选其一开工)

接手前你**必须**完整读完本文件 + 下面的"必读五件套"。**不要跳读**。

---

## §1 必读五件套(按顺序 · 不可跳)

| 序 | 文件 | 性质 | 你需要从中拿走什么 |
|---|---|---|---|
| 1 | `docs/V0_9_MODEL_HANDOFF_MANIFEST.md` | V0.9 时代入口(历史背景) | 知道 V0.9 baseline 与兼容性 surface 怎么来 |
| 2 | `docs/V1_TO_V2_MODEL_HANDOFF_MANIFEST.md` | **V1 当前权威 handoff**(2026-04-25 Release v1.21 更新) | Authority order / Stable surface / Cron gates / Verification baseline / Known debt / 段映射 / sha 锚 |
| 3 | `docs/iterations/V1_RETRO_REPORT.md`(§14 架构师裁决节)| V1 整体 retro + 架构师独立 verify 证据 | A1~A12 验收实证 / 全栈 verify 矩阵 / P99 RED 详情 / 已知债务 / V1.1+V2 路线建议 |
| 4 | `docs/iterations/V1_1_A1_DETAIL_P99_REPORT.md` | V1.1-A1 性能收口报告 | detail P99 实测 / 实现决策 / 验证矩阵 / 架构师裁决 |
| 5 | `prompts/V1_ROADMAP.md`(§32 总览表 + §变更记录 v1~v32) | V1 全过程编年 | 每轮 prompt 文件路径 / 每轮 abort 史与修补 / 教训固化 |

读完后你应能独立回答:
- V1 stable API 有哪些?compatibility 路由有哪些?哪些不能动?
- 4 份 V1 权威文档分别管什么(主 / IA / 资产 / 定制)?
- 段隔离规则(`[20000~80000)` 当前各段 ownership)?
- 全栈 integration 为什么必须 `-p 1`?
- 哪 7 个业务文件的 sha 是 baseline 锚?
- V1.1-A1 为什么选择 multi-result bundle + auth fast path,而不是 MySQL 物化表 / 进程 cache?

如果以上 6 题任一答不上,**回去重读 §1 五件套**,不要急着开工。

---

## §2 Authority Order(铁律 · 不可改)

冲突时:

1. `transport/http.go` 决定**实际挂载了什么**。
2. `docs/api/openapi.yaml` 决定**当前 HTTP 契约**(sha `b3d7c365…dd0f`)。
3. 4 份权威文档决定**架构语义**:
   - `docs/V1_MODULE_ARCHITECTURE.md` v1.3
   - `docs/V1_INFORMATION_ARCHITECTURE.md`
   - `docs/V1_CUSTOMIZATION_WORKFLOW.md`
   - `docs/V1_ASSET_OWNERSHIP.md`
4. `docs/iterations/V1_RETRO_REPORT.md` 是**证据**,不是契约。
5. `CURRENT_STATE.md` / `MODEL_HANDOVER.md` / `docs/archive/*` / `docs/iterations/*`(retro 除外)/ `docs/iterations/legacy_specs/*` / 旧 model-memory 文件 **都不是当前 spec**(参 `CLAUDE.md` 工作规则)。

---

## §3 当前状态硬锚(2026-04-25)

### §3.1 SHA Baseline(7 个业务文件 + OpenAPI · 任何接手轮 step-1 必校)

```text
b3d7c3651ea2496a6e4ea1a948772c6a395d6b387bf6c4509e5c26477c75dd0f  docs/api/openapi.yaml
5f4c9a10227e8321c4a87c8260b2bc0078adbb2dfb9fa0ebd2bd86601f46bae8  service/asset_lifecycle/cleanup_job.go
60103b15fa877a8d14b719dbd9f2aa82ee957271e8e8dea79a42106a8f346a1c  service/task_draft/service.go
32cd0201bf205bc2abfb6a9f489202de4bd099e188349184bd55a4ae1e22454b  service/task_lifecycle/auto_archive_job.go
f9d09d1fbc55734b00ff1f6c35cc1bccbf9db05298283eff6f255971262638c2  repo/mysql/task_auto_archive_repo.go
658a8cdf65c09335ab74176efb4057eff68440537e50ce0d9e550c57413e6e6b  domain/task.go
0bf70496a21c995d230efbcfaee4499257f1e3e46506e206a0ec6f51a73b6881  cmd/server/main.go
```

如果你是 V1.2 / 后续性能或联调修补轮:任何业务文件 sha 变化都必须在 prompt §3 明示新 baseline,并在 retro 里更新锚。
如果你是 V1.1-A2 CI 守卫 / V1.1-A3 测试稳定性轮 / §9.3 路由下线轮:这 7 文件 sha 应**完全不动**(测试稳定性轮只动 `_test.go`,§9.3 下线轮只动 `transport/http.go` + openapi.yaml)。

### §3.2 段隔离 Map(历史测试库 `jst_erp_r3_test`)

> 注:测试库 `jst_erp_r3_test` 已于 2026-04-25 在 Release v1.21 后 DROP(详见 ROADMAP v31)。本表保留作为历史段映射记录;若未来需重建测试库,跑 `scripts/r35/setup_test_db.sh` 即可恢复。

| Segment | Owner | 状态 |
|---|---|---|
| `[20000,30000)` | SA-A | 可被 V1.1 复用(SA-A 闭环) |
| `[30000,40000)` | SA-B / SA-B.1 | 可被 V1.1 复用 |
| `[40000,50000)` | SA-C / SA-C.1 | 可被 V1.1 复用 |
| `[50000,60000)` | SA-D / R5 / R6.A.1 | **多轮共用 · 高污染风险** |
| `[60000,70000)` | R6.A.3 | 可被 V1.1 复用 |
| `[70000,80000)` | R6.A.4 | 可被 V1.1 复用 |
| `[80000,90000)` | V1.1-A1 | 已用 · 复用前必须 clean |
| `[90000,100000)` | **下一轮建议使用** | 推荐 |

**新轮请用 `[90000,100000)` 起步**,prompt §1 必须明示:
- step-1 段 audit + clean(BEFORE/AFTER 全 0)
- step-N 段 audit(全 0)
- 复用现有 `tmp/r6_a_4_recovery_run.sh` 模板适配新段

### §3.3 防呆条款(任何接手轮硬门)

| # | 防呆 | 触发 |
|---|---|---|
| 1 | 全栈 integration 必须 `-p 1` | 默认并行 → 跨包同段污染 → 伪退化 ABORT(参 v25 教训) |
| 2 | `tee` 必须 `set -o pipefail` | 否则非零退出码被 `tee` 掩盖(参 v25 教训) |
| 3 | `r35.MustOpenTestDB` + `t.Cleanup` 用 db 时 · `db.Close()` 必须放进 `t.Cleanup` callback 末尾 | 不能 `defer db.Close()`(参 v22 R6.A.1 教训) |
| 4 | 测试代码内嵌的 `context.WithTimeout` 必须留 ≥3x buffer | R2 `smoke_test.go` 60s deadline 在 R5/R6 累积 fixture 后边缘化(参 v26 R6.A.4 v1.1 ABORT 教训) |
| 5 | 4 份权威文档 `mtime` 必须保持 R5 闭环态(2026-04-24)| 任何 V1.1 内的改动都需先升 v1.x 起 ADR 节(R7+ 拆轮) |
| 6 | OpenAPI 改动必须 `openapi-validate 0 error 0 warning` | dangling 501 行级必须 0 |
| 7 | cron 必须默认 OFF · 通过 ENV gate 启 | `ENABLE_CRON_OSS_365` / `ENABLE_CRON_DRAFTS_7D` / `ENABLE_CRON_AUTO_ARCHIVE` |
| 8 | 跨 v0.9/v1 表 JOIN 必须显式 `COLLATE utf8mb4_0900_ai_ci` | 参 v15 SA-B.2 collation mix bug 教训 |

---

## §4 工程基础设施清单

### §4.1 测试 / 跑命令(WSL native Go · 不要用 Windows Go · Device Guard 拦截过 SA-A)

```bash
export PATH=/usr/bin:/usr/local/bin:/home/wsfwk/go/bin:$PATH
export MYSQL_DSN='root:<TEST_DB_PASSWORD>@tcp(127.0.0.1:3306)/jst_erp_r3_test?parseTime=true&multiStatements=true&loc=Local'
export R35_MODE=1

# build / vet / unit
/home/wsfwk/go/bin/go build ./...
/home/wsfwk/go/bin/go vet ./...
/home/wsfwk/go/bin/go test -count=1 ./...

# 全栈 integration · -p 1 是硬门
set -o pipefail && /home/wsfwk/go/bin/go test -tags=integration -p 1 -count=1 -timeout 60m ./... 2>&1 | tee tmp/<round>_integration.log

# OpenAPI 校验
/home/wsfwk/go/bin/go run ./cmd/tools/openapi-validate docs/api/openapi.yaml

# 段恢复脚本(模板 · 改段范围用)
tmp/r6_a_4_recovery_run.sh   # 清 [50000,80000) · 复制改段范围
```

### §4.2 SSH 隧道(连生产 / 测试 MySQL 必经)

```bash
# 隧道脚本
tmp/start_tunnel.sh
# host: jst_ecs · 隧道映射 127.0.0.1:3306 → 远端
```

### §4.3 cmd/server live smoke 模板

```bash
SERVER_PORT=18086 /home/wsfwk/go/bin/go run ./cmd/server > tmp/<round>_server.log 2>&1 &
sleep 3
curl -sS http://127.0.0.1:18086/healthz   # 应 200
```

### §4.4 工具脚本目录(R6.A.4 已落盘 · 可作模板)

| 脚本 | 用途 |
|---|---|
| `tmp/arch_verify_r6_a_4.sh` | 架构师独立 verify · 12 项静态 + 4 项重跑全绿 |
| `tmp/r6_a_4_recovery_clean.sql` + `.._run.sh` | 段污染恢复模板(改段范围复用) |
| `tmp/r6_a_4_isolation_run.sh` | step-1/step-N 段 audit 模板 |
| `tmp/r6_a_4_p99_runner.go` | R6.A.4 P99 加压 100 次实测 runner(旧 baseline) |
| `tmp/r6_a_4_seed_super_admin.go` | super admin token 种子(测试库) |
| `tmp/v1_1_a1_p99_runner.go` | V1.1-A1 detail P99 runner(支持 `WARMUP`/`N`) |
| `tmp/v1_1_a1_seed_super_admin.go` | V1.1-A1 super admin token 种子 |
| `tmp/v1_1_a1_isolation_run.sh` | `[80000,90000)` 段 audit/clean 模板 |

---

## §5 已知债务(从 V1 retro §10 + handoff §6 复制)

### §5.1 V1.1 completed

1. **`/v1/tasks/{id}/detail` P99 < 150ms**:V1.1-A1 已完成,cold `47.525ms`,warm `47.513ms`,final warm n=500 `47.126ms`
   - 实施:detail multi-result bundle + session actor bundle + 成功 `route_access` async best-effort + 旧路径 fallback
   - 未采用:MySQL 物化表 / 触发器 / 进程 cache,避免写入放大和一致性风险

### §5.2 V1.1 remaining mandatory(稳定性收口)

1. **CI 集成测试包级并行守卫**:强制 `-p 1` for shared-DB integration packages(防 R6.A.4 v1.0 段污染伪退化复刻)
2. **测试代码稳定性统一**:R2 `smoke_test.go` 60→300s patch + R6.A.1 `defer db.Close()` 顺序 patch 已修 · V1.1 应做"测试代码稳定性轮"统一 buffer + 顺序矫正 + DB 隔离强化

### §5.3 R7+ 选修

- **§9.3 兼容路由下线**:`/v1/tasks/{id}/audit_a_claim` 等(`withCompatibilityRoute` / `withDeprecatedRoute` 标注)· R1~R6 已过 6 轮 · 主 §9.3 "保留 3 个迭代周期"早过期 · 需要 ADR 拆轮

### §5.4 V2 候选(继承契约 · 不继承实现 accident)

- U1 timeout escalation
- U2 L2/L3 reports
- U3 blueprint 外置存储
- U4 actor_org_snapshot 冻结模型
- U5 cancellation 扩展
- U6 SLA 自动化
- A12 durable event consumer outlet

---

## §6 候选下一轮(请挑选其一开工 · prompt 起草指引)

> ✅ 已完成:Release v1.21(2026-04-25)+ 前端 API 文档(`docs/frontend/`)。前端联调首轮门已开。下一轮候选见下表。

### §6.A V1.1-A1 · `/v1/tasks/{id}/detail` P99 改造(**已完成 · architect-verified**)

| 项 | 起草指引 |
|---|---|
| 文件名 | `prompts/V1_1_A1_DETAIL_P99.md` |
| 报告 | `docs/iterations/V1_1_A1_DETAIL_P99_REPORT.md` |
| 实施 | detail multi-result bundle + session actor bundle + 成功 `route_access` async best-effort |
| 结果 | cold p99 `47.525ms` + warm p99 `47.513ms` + final warm n=500 p99 `47.126ms` |
| 段 | `[80000,90000)` |
| 终止符 | `V1_1_A1_DONE_ARCHITECT_VERIFIED` |

### §6.B V1.1-A2 · CI 集成测试包级并行守卫(**下一后端推荐**)

| 项 | 起草指引 |
|---|---|
| 文件名 | `prompts/V1_1_A2_CI_GUARD.md` |
| 范围 | 0 业务代码 · 0 OpenAPI · 仅加 `Makefile` / `scripts/test-integration.sh` / `.github/workflows/*` 强制 `-p 1` for `-tags=integration` |
| 验收 | 故意尝试默认并行应 fail-fast · `-p 1` 串行应 PASS |
| 工作时长 | 1~2h |

### §6.C V1.1-A3 · 测试代码稳定性轮(timeout buffer 统一 + `t.Cleanup` 顺序矫正)

| 项 | 起草指引 |
|---|---|
| 文件名 | `prompts/V1_1_A3_TEST_STABILITY.md` |
| 范围 | 仅 `_test.go` · 业务代码 0 改动 · 全仓 grep 出所有 `context.WithTimeout(.., < 120*time.Second)` 评估 buffer · 全仓 grep 出所有 `defer db.Close()` 在 `r35.MustOpenTestDB` 后的潜在顺序 bug |
| 验收 | 全栈 integration `-p 1` 0 FAIL · 5 次连跑 0 outlier |
| 工作时长 | 3~4h |

### §6.D Frontend Integration · 前端联调规划入口(**可并行启动**)

| 项 | 起草指引 |
|---|---|
| 文件名 | `prompts/V1_1_FRONTEND_INTEGRATION_HANDOFF.md` |
| 范围 | 前端只接 canonical MAIN routes · 不接 compatibility/deprecated routes |
| 后端状态 | detail P99 GREEN · OpenAPI 0/0 · full integration `-p 1` PASS |
| 首屏建议 | `GET /v1/tasks` + `GET /v1/tasks/{id}/detail` |

### §6.E R7 · §9.3 兼容路由下线(写在 V1.1 之后或并行)

| 项 | 起草指引 |
|---|---|
| 前置 | ADR 文件 `docs/iterations/ADR_009_3_DEPRECATION.md` · 列每条兼容路由的 successor + 通知期 + 切换断言 |
| 文件名 | `prompts/V1_R7_COMPAT_REMOVAL.md` |
| 范围 | `transport/http.go` 删 `withCompatibilityRoute` / `withDeprecatedRoute` 入口 + openapi.yaml 删兼容 path · 0 业务代码 |
| 验收 | A1~A12 zero regression(canonical 路径仍工作)· 兼容路径 404 |

### §6.F V2 启动(继承契约 · 重写实现)

V2 是大动作:**不要直接挑这个轮起步**。建议先做完 V1.1-A2/A3 把 V1 工程稳定性闭环,同时让前端按 `prompts/V1_1_FRONTEND_INTEGRATION_HANDOFF.md` 开始联调;之后再做 V2 ADR 起草轮(`docs/iterations/V2_ADR_KICKOFF.md`)收敛 6 个 V2 候选(U1~U6 + A12)。

---

## §7 接手第一动作 SOP(任何候选都跑这套)

```text
step-0  · 读 §1 必读五件套(权威 + handoff + retro + A1 report + ROADMAP)
step-1  · 校 sha 锚:`sha256sum docs/api/openapi.yaml service/asset_lifecycle/cleanup_job.go ...`
         · 应与 §3.1 完全一致(否则 baseline 已漂移 · 找用户确认)
step-2  · 隧道 + 测试库连通性:`tmp/start_tunnel.sh` + `mysql -h 127.0.0.1 ...`
step-3  · build / vet / unit · 应当全 PASS(否则 baseline 不干净 · ABORT)
step-4  · 段恢复脚本跑一遍清新轮段(建议 `[90000,100000)` 范围)· BEFORE/AFTER 全 0
step-5  · 全栈 integration `-p 1 -timeout 60m` · 应当 318s 左右 0 FAIL
         · 这是接手 baseline · 不是新工作 · 只是建立"我接手时一切正常"的硬证据
step-6  · 起草 prompt 文件 · 落 `prompts/V1_1_<round>_*.md`
         · 必须包含:§1 baseline / §2 范围 / §3 sha 锚 / §4 验收门 / §5 ABORT 触发
step-7  · 用户审 prompt · 签字后开工
step-8  · 工作完成 · 写 `docs/iterations/V1_1_<round>_REPORT.md`
step-9  · 架构师独立 verify · 落 §架构师裁决节
step-10 · 用户签字 · 更新 ROADMAP 变更记录(下一个版本号 v33+)
```

---

## §8 工程纪律红线(违反即 ABORT)

1. **不要默认并行跑 integration**(必须 `-p 1`)
2. **不要把 `tee` 用在没 `set -o pipefail` 的管道里**
3. **不要在 `r35.MustOpenTestDB` + `t.Cleanup` 里 `defer db.Close()`**
4. **不要静默改 4 份权威文档** · 任何架构语义改动必须先升 v1.x ADR 节
5. **不要静默改 7 个 sha 锚业务文件** · 任何改动必须在 prompt §3 明示并更新 baseline
6. **不要用 Windows Go 跑测试** · WSL native Go(`/home/wsfwk/go/bin/go`)是唯一可信 baseline
7. **不要在 retro / handoff 里掩盖 RED 项** · P99 RED 转 V1.1 必修是合规处置 · 不是失败
8. **不要在 V2 起步轮里先重构 6 module 模型 / 5 类 NotificationType / 三层授权** · 这 3 个是 V2 必须继承的契约不变量

---

## §9 联系点 / 找用户裁决的边界

接手模型遇到以下情况**必须停下来找用户裁决**(不要自作主张):

- sha 锚漂移(7 文件任一 hash 不对)
- 4 份权威文档需要改(任一)
- OpenAPI 路径需要新增 / 删除 / 改 method
- 段池规则需要变更(超出 §3.2 ownership)
- 全栈 integration `-p 1` 0 FAIL 跑不通(baseline 已坏)
- 用户的 prompt / 输入与 retro / handoff / ROADMAP 三方任一冲突

不在以上清单的实施细节(具体 SQL / 索引 / cache 策略 / handler 内部重构):**自主决策即可**,但需在 prompt 起草时明示候选与权衡 · 在 retro 时记录决策 trace。

---

## §10 终止符约定

接手轮**必须**输出明确的终止符:

- 工作完成 + 自跑 verify 全绿 + 等待架构师独立 verify:`<ROUND>_DONE_PENDING_ARCHITECT_VERIFY`
- 架构师 verify 通过签字:`<ROUND>_DONE_ARCHITECT_VERIFIED`
- ABORT 中止(任意硬门 fail):`<ROUND>_ABORT · reason=<short>`

不要自签 PASS · 不要省略终止符 · 不要写"基本完成"等模糊措辞。

---

## §11 当前未解决问题清单(给接手模型作 starting context)

- ✅ V1.1-A1 P99 已完成:未走 MySQL 物化 / 触发器 / cache,采用 multi-result bundle + auth fast path
- ❓ V1.1-A2 与 A3 顺序(CI `-p 1` 守卫优先 / 测试稳定性统一优先?)→ 用户决策
- ❓ 前端联调阶段谁持有联调计划和环境 smoke → 用户 / 前端工程师 / 接手模型决策
- ❓ §9.3 兼容路由下线时间窗(R7 跟 V1.1 并行还是串行?)→ 用户决策
- ❓ V2 是否复用现有 `domain/*` 还是重写 → V2 ADR 轮决策

---

## §12 接手模型的开工咒语

```
我已读完 prompts/V1_NEXT_MODEL_ONBOARDING.md。
我已读完 docs/V1_TO_V2_MODEL_HANDOFF_MANIFEST.md。
我已读完 docs/iterations/V1_RETRO_REPORT.md §14 架构师独立 verify 裁决节。
我已读完 docs/iterations/V1_1_A1_DETAIL_P99_REPORT.md。
我已读完 prompts/V1_ROADMAP.md §32 总览表 + §变更记录 v25~v29。

我接手的轮:<V1.1-A2 / V1.1-A3 / Frontend-Integration / R7 / V2-ADR>
我接手的段:[90000, 100000)
我已校 §3.1 sha 锚:7/7 一致。
我承诺遵守 §8 红线 8 条 + §3.3 防呆 8 条。

现在开始 step-1。
```

---

> 末:本文件不是冻结契约 · 接手轮如发现入口指引过时(例:V1.1-A1 完成后 P99 锚需要更新),**应该**在轮签字时同步修订本文件 · 防止下一代接手模型读到陈旧入口。
