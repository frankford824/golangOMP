## V1.2 · RESUME · 从 P2 接续 · 基于 path-closure 可达性的真实数字

> 起草人:架构师(Claude Opus 4.7)
> 起草时间:2026-04-26 PT
> 性质:**codex TUI / codex exec autopilot 单 prompt** · 5 阶段串行(P2a → P2b → P3 → P4 → P5 → P6)· 强 ABORT 守卫
> 前置签字:`V1_1_A2_DONE_ARCHITECT_VERIFIED`(2026-04-26 PT)
> 接续来源:`prompts/V1_2_AUTHORITY_AND_OPENAPI_PURGE.md`(v1 · P2 ABORT)
> ABORT 报告:`docs/iterations/V1_2_ABORT_REPORT.md`
> 终止符(成功):`V1_2_AUTHORITY_PURGED_AND_GUARD_LIVE`
> **不重做** v1 已完成的 P0 baseline + P1 authority inventory(产物已落盘,本轮直接复用)

---

## §0 体检算法修正声明(必读 · 解释为什么 v1 ABORT)

### §0.0.1 v1 prompt 错误

`prompts/V1_2_AUTHORITY_AND_OPENAPI_PURGE.md` v1 §0.4.3 的体检数字基于**简单文本引用计数**:把"yaml 文本中 `#/components/schemas/<name>` 出现次数 == 1"误判为"死 schema",得到"150 个仅自定义未被 ref"。

**算法错误**:OpenAPI schema 之间是**引用图**(schema A 内部可以 `$ref` schema B)。一个 schema 即便文本中只出现 1 次,也可能是被其他 schema 引用 → 通过 path → ... → schema 闭包传递可达 → 实际是活的。

**正确算法**:从 paths 子树收集所有 root `$ref` 作为 seed → 在 schemas 内做不动点闭包 → 闭包内 = reachable(活) · 闭包外 = unreachable(死)。

### §0.0.2 实测真实数字(架构师独立复算)

```text
schemas total              = 313
seed schemas (paths 直接 ref) = 167
reachable closure          = 298
unreachable                = 15   ← 真死 schema
deprecated paths           = 29   ← 候选连锁删除入口
```

**15 个 unreachable schema**(本轮 P2a 直接删除目标):

```text
APIReadiness               ← 注:已确认全图未引用,作为元数据保留可选
AuditRecord
AvailableAction
BatchCreatePreviewItem
BatchCreateViolation
DerivedStatus
ExportJobAdapterMode
ExportJobStorageMode
RouteAccessPlaceholder
TaskCostInfo
TaskCostQuotePreviewResponse
TaskModule
TaskModuleProjection
TaskModuleScope
TaskPriority
TaskProductInfo
```

(注:架构师独立 audit 实际算出 15 条 · 列表上面是 16 条因为 APIReadiness 在某些 audit 出现 · codex 以 `tmp/v1_2/dead_schema_audit_before.json` 为准,以 path-closure 算法的实际不可达集为最终待删集 · 任何不在 audit JSON `unreachable[]` 中的 schema 不可删)

**29 条 deprecated paths**(本轮 P2b 决断目标):

```text
GET    /v1/products/search
GET    /v1/products/{id}
POST   /v1/task-create/asset-center/upload-sessions
GET    /v1/task-create/asset-center/upload-sessions/{session_id}
POST   /v1/task-create/asset-center/upload-sessions/{session_id}/complete
POST   /v1/task-create/asset-center/upload-sessions/{session_id}/abort
GET    /v1/tasks/{id}/assets/timeline
GET    /v1/tasks/{id}/assets/{asset_id}/versions
GET    /v1/tasks/{id}/assets/{asset_id}/download
GET    /v1/tasks/{id}/assets/{asset_id}/versions/{version_id}/download
POST   /v1/tasks/{id}/assets/upload-sessions
GET    /v1/tasks/{id}/assets/upload-sessions/{session_id}
POST   /v1/tasks/{id}/assets/upload-sessions/{session_id}/complete
POST   /v1/tasks/{id}/assets/upload-sessions/{session_id}/abort
POST   /v1/tasks/{id}/assets/upload
GET    /v1/tasks/{id}/asset-center/assets
GET    /v1/tasks/{id}/asset-center/assets/{asset_id}/versions
GET    /v1/tasks/{id}/asset-center/assets/{asset_id}/download
GET    /v1/tasks/{id}/asset-center/assets/{asset_id}/versions/{version_id}/download
POST   /v1/tasks/{id}/asset-center/upload-sessions
POST   /v1/tasks/{id}/asset-center/upload-sessions/small
POST   /v1/tasks/{id}/asset-center/upload-sessions/multipart
GET    /v1/tasks/{id}/asset-center/upload-sessions/{session_id}
POST   /v1/tasks/{id}/asset-center/upload-sessions/{session_id}/complete
POST   /v1/tasks/{id}/asset-center/upload-sessions/{session_id}/cancel
POST   /v1/tasks/{id}/asset-center/upload-sessions/{session_id}/abort
POST   /v1/tasks/{id}/outsource
GET    /v1/outsource-orders
POST   /v1/audit
```

### §0.0.3 修正后的 verify 数字目标(取代 v1 §0.4.3)

| 项 | V1.1-A2 后实测 | V1.2 期望值 | 阈值 |
|---|---:|---:|---|
| OpenAPI 总行数 | 14757 | 13500~14500 | 必须 < 14600(放宽 · 不强制砍 40%) |
| `components.schemas` 总数 | 313 | **= 313 - len(unreachable_audit)**(基于审计 JSON 实测 · 不强求绝对数) | 闭包外 schema = 0 |
| `components.schemas` unreachable(closure 算法) | 15 | **0** | 硬门 |
| `paths` 数 | 206(3 非 v1 + 203 v1) | 与 transport AST mount 集合差额 = 0 或转 V1.2-B 已知遗留 | 硬门 |
| `deprecated: true` 路径 | 29 | ≤ 29 · 每条必须有 `x-removed-at: <release>` 或显式 keep 理由 | 硬门 |
| `docs/` 顶层 .md | P1 已处理 | ≤ 12 | P1 已 PASS · 复核 |
| 业务 SHA 锚组 A 漂移 | 0 | 0 | 硬门 |
| OpenAPI 仍可 yaml.safe_load | OK | OK | 硬门 |

---

## §0.1 V1.2-v1 已完成产物(本轮直接复用 · 不重做)

`docs/iterations/V1_2_ABORT_REPORT.md` "Completed Before ABORT" 段落记录的产物:

- `docs/iterations/V1_2_AUTHORITY_INVENTORY.md`(229 inventory rows)
- `docs/V1_BACKEND_SOURCE_OF_TRUTH.md`(V1 真相单点)
- `docs/iterations/INDEX.md`
- `prompts/INDEX.md`
- `docs/archive/legacy_handoffs/*`(legacy docs 移入)
- `prompts/archive_pre_v1_2/*`(老 prompts 移入)
- `CLAUDE.md` 已更新到 V1 authority 顺序
- `tmp/v1_2/dead_schema_audit_before.json`(P2 算法证据 · 保留)

**本轮不动这些产物的内容**(只允许在 P6 retro 落盘时追加最终状态)。

---

## §0.2 业务 Go 一行不许改(继承 v1 §0.4.1)

业务 SHA 锚组 A 10 文件本轮仍**冻结**,任何 stage 启动前 step-0 必校(命令同 v1):

```powershell
git status --short -- service/asset_lifecycle/cleanup_job.go service/task_draft/service.go service/task_lifecycle/auto_archive_job.go repo/mysql/task_auto_archive_repo.go domain/task.go cmd/server/main.go service/task_aggregator/detail_aggregator.go repo/mysql/task_detail_bundle.go service/identity_service.go repo/mysql/identity_actor_bundle.go
```

期望:**空输出**。任意一文件出现 modified/staged → ABORT。

---

## §1 ABORT triggers(8 条 · 修正 v1 错误版)

任意一条命中 → 立即写 `docs/iterations/V1_2_RESUME_ABORT_REPORT.md` · 不进后续 phase · 不输出终止符。

1. §0.2 业务 SHA 锚组 A 任意文件出现 modified/staged
2. OpenAPI baseline sha 与 `0ff87aa90a53963a64350f92bf8bdce821dad3c24538bf70d61283b8dd97e5c3` 不一致(P2a step-0)
3. **P2a 删除 schema 时,任意一个待删 schema 不在 `tmp/v1_2/dead_schema_audit_before.json` 的 `unreachable[]` 列表中**(取代 v1 错误的"$ref ≥ 2 次"判据)
4. **P2a 删除后重跑闭包可达性,unreachable 数量仍 > 0**(说明 P2a 没删干净 · 必须先排查再决定下一步)
5. P2b 删除 path 时,该 path 在 transport AST 中实际有 mount(用 `tools/route_audit/` 验证)
6. P3 三向对账 documentedSet ⊕ mountedSet > 0 且未在 GC 报告 §known-gap 显式记入
7. `go vet ./...` 或 `go build ./...` 在任一阶段从 PASS → FAIL
8. P5 守门脚本在 drift seed 测试中未能阻断 commit

ABORT 报告模板(同 v1 §1)。

---

## §2 Phase 2a · 安全删除 15 个 unreachable schema

### §2a.1 步骤

1. **step-0 校验**(SHA + OpenAPI sha + go vet/build PASS)
2. 重跑 `tmp/v1_2_dead_schema_audit.py`(或 codex v1 P2 已用脚本),用 path-closure 算法,确认当前 unreachable 集合
3. 与 `tmp/v1_2/dead_schema_audit_before.json` 中 `unreachable[]` 比对 · 必须完全一致(否则证明 OpenAPI 在 P2 ABORT 后被外部修改 → ABORT)
4. 对每个 unreachable schema · 在以下三个地方 grep:
   - `docs/frontend/*.md`
   - `transport/handler/*.go`(含 swagger annotation 注释)
   - `docs/V1_BACKEND_SOURCE_OF_TRUTH.md`
   分类:
   - **A · 全 0 命中** → 安全删除
   - **B · 仅 frontend doc 有命中** → 先把 frontend doc 替换成新引用或 inline 字段,再删
   - **C · handler 注释/SoT 有命中** → **不删**,记入"V1.3 待清理"
5. 应用 A bucket 删除(yaml 编辑 · 完整删除 schema 块及其下所有字段 · 保留缩进)
6. yaml.safe_load 重 parse · OK
7. 重跑闭包可达性 · 确认 unreachable = 0
8. 跑 `go vet ./...` + `go build ./...` PASS
9. 输出 `docs/iterations/V1_2_OPENAPI_GC_REPORT.md` §1 unreachable-schemas-deleted:
   - 每条:name | grep evidence | bucket(A/B/C) | action(deleted/inlined-then-deleted/keep-for-v1.3)
10. 记 `tmp/v1_2/dead_schema_audit_after.json`

### §2a.2 ABORT(本 phase 专属)

- A 删除后 yaml parse 失败
- A 删除后任意被删 schema 名字仍在 OpenAPI 中作为 `$ref` 出现
- C bucket 太大(> 10 条)→ 必须停下来与架构师确认是否要 inline frontend doc 的引用

### §2a.3 verify

- `len(unreachable) == 0` · path-closure 算法
- `components.schemas` 总数 = 313 - len(deleted_in_A) - len(deleted_in_B)
- yaml 可 parse · paths 数仍 = 206
- `go vet ./...` + `go build ./...` PASS

---

## §3 Phase 2b · 29 条 deprecated path 决断

### §3b.1 关键原则

**不为了减行数而强删 path**。每条 deprecated path 必须按下表决断,处理动作必须有证据:

| 状态 | 判据 | 动作 |
|---|---|---|
| **D1 · still mounted, has alternative** | transport AST 中有 mount + 有等价新 path 且新 path 已被 frontend 用 | 加 `x-removed-at: v1.3`(给前端 1 个 release 迁移期) · 不删 |
| **D2 · still mounted, no alternative** | transport AST 中有 mount + 没等价新 path | 移除 `deprecated: true`(实际是活的 · 标错了) · 记入 V1.2 已知遗留:V1.3 须重新评估去留 |
| **D3 · not mounted** | transport AST 中无 mount | 删除 path 整块 + 该 path 专用 schema 自动变 unreachable · 第二次跑可达性 GC 删 schema |
| **D4 · mounted but FeatureFlag-gated** | mount 在 `if FeatureFlag` 包裹内 | 加 `x-removed-at: <decision-pending>` + 记入 V1.2-B 决策清单 |

### §3b.2 步骤

1. 写临时工具 `tmp/v1_2_route_audit/main.go`(Go AST 解析 transport/http.go · 抽 mount 表 · 输出 `tmp/v1_2/mounted_paths.txt`,一行一条 `METHOD /v1/...`)
2. 对 §0.0.2 列出的 29 条 deprecated path,逐条按 §3b.1 决断
3. 对 D3 · 删除 OpenAPI path 块 + grep 该 path 在 frontend doc 是否仍出现 · 若出现,frontend doc 需在 P6 同步删
4. D3 删除完后,**重跑闭包可达性**,把因此变 unreachable 的 schema 加入第二轮 P2a 删除(限定:这些 schema 同样要走 §2a.1 step 4 grep)
5. 输出 `docs/iterations/V1_2_OPENAPI_GC_REPORT.md` §2 deprecated-paths-decision:
   - 每条:method | path | bucket(D1/D2/D3/D4) | mount evidence | alternative path | action
6. 输出 §3 cascade-deleted-schemas:第二轮可达性 GC 删除的 schema 清单

### §3b.3 ABORT

- D3 删除某 path 但其在 transport AST 中确实 mounted → ABORT(决断错误 · 改成 D1)
- D1 加 `x-removed-at` 但选错版本号(必须 ≥ v1.3,不能 ≤ 当前)→ ABORT 改正

### §3b.4 verify

- 29 条全部决断完毕(`docs/iterations/V1_2_OPENAPI_GC_REPORT.md` §2 表行数 = 29)
- D1+D2+D4 中每条 OpenAPI 仍可 parse · path 仍存在
- D3 删除后,该 path 集合在 transport AST mount 表中确认全部不存在
- 重跑闭包可达性 · unreachable 仍 = 0(级联删除已完成)

---

## §4 Phase 3 · OpenAPI 死/缺/错 path 三向对账

(沿用 v1 §5 不变 · 仅修正 §5.3 verify 表达)

### §4.1 步骤(同 v1 §5.1)

mounted set vs documented set 三向对账。

### §4.2 ABORT(同 v1 §5.2)

### §4.3 verify(修正版)

- `documentedSet ⊕ mountedSet == 0` 或差额条目在 `docs/iterations/V1_2_OPENAPI_GC_REPORT.md` §4 known-gap 中显式列入(每条带 evidence)
- transport/http.go SHA 锚不变(0 漂移)
- yaml 可 parse · paths 数 = 3 + |mountedSet|

---

## §5 Phase 4 · 字段级 contract_audit 工具落盘

(完全沿用 v1 §6 · 工具架构 + 测试要求不变)

### §5.1 工具目录(同 v1 §6.1)

### §5.2 关键约束(同 v1 §6.2)

### §5.3 CLI(同 v1 §6.3)

### §5.4 输出 JSON schema(同 v1 §6.4)

### §5.5 ABORT(同 v1 §6.5)

### §5.6 verify(修正:首跑 drift 数)

- `summary.drift == 0` · **若 > 0 必须列入 V1.2-B 已知遗留,不允许 silent pass**
- 工具运行时间 < 30s

---

## §6 Phase 5 · CI 硬门

(完全沿用 v1 §7 · drift seed 测试要求不变)

### §6.1 ~ §6.4 内容(同 v1 §7.1~§7.4)

---

## §7 Phase 6 · 治理 4 件套同步 + frontend doc 重生 + retro

### §7.1 frontend doc 重生

基于 P2a + P2b 清算后的 OpenAPI 重新生成 16 份 `docs/frontend/V1_API_*.md` + INDEX。

**关键约束**:
- 任何引用过被删 schema 的 doc 必须改为引用活 schema 或 inline 字段
- INDEX.md 修订历史追加 V1.2 entry(注明:基于 path-closure 真死集 · unreachable 已清零)
- INDEX.md 顶部 SoT 链接保留指向 `docs/V1_BACKEND_SOURCE_OF_TRUTH.md`(P1 已落)

### §7.2 治理 4 件套(同 v1 §8.3 · 数字按本轮实测改写)

- `docs/V1_TO_V2_MODEL_HANDOFF_MANIFEST.md` · 当前状态 = `V1_2_AUTHORITY_PURGED_AND_GUARD_LIVE`
- `prompts/V1_NEXT_MODEL_ONBOARDING.md` · V1.2 状态 + V1 SoT 入口
- `prompts/V1_1_FRONTEND_INTEGRATION_HANDOFF.md` · 通告 OpenAPI 已清算(unreachable 0 · deprecated 已决断)
- `prompts/V1_ROADMAP.md` · 追加 v36~v41(分别对应 P1 已完成 · P2a · P2b · P3 · P4 · P5+P6)

### §7.3 retro 报告

`docs/iterations/V1_2_RETRO_REPORT.md` 落盘(同 v1 §8.4 · 模板字段不变),**额外字段**:

- §x · v1 ABORT 教训:体检算法错误 · path-closure vs 文本计数差异 · 列入 V1.3 工程规则:任何 OpenAPI schema 量化判定必须用闭包算法
- §y · V1.1-A2 Q-1(192 模板占位 known-debt)是否随 contract_audit 工具上线一并关闭(若 P4 工具首跑 drift = 0,可以关闭)
- §z · v1.21 binary vs 本地 git HEAD identity_service.go sha 漂移仍未解决,转 V1.2-B v1.22 rebuild 子轮

### §7.4 verify(修正版)

- 16 份 frontend doc 全部含 V1.2 修订标记
- frontend doc grep 全部被删 schema 名 · 命中数 = 0
- INDEX.md 顶部 SoT 链接存在
- 治理 4 件套 + retro 全部落盘
- `git status` 业务 Go 仍 0 改动

---

## §8 架构师 verify 矩阵(18 项 · 修正 v1 数字目标)

| # | 检查项 | 期望(修正后) |
|---|---|---|
| 1 | 业务 SHA 锚组 A 0 漂移 | 空输出 |
| 2 | OpenAPI 起始 baseline sha 校对(P2a step-0) | `0ff87aa9...` |
| 3 | OpenAPI yaml 可 parse(每个 stage 后) | 无异常 |
| 4 | **path-closure unreachable schema = 0**(P2a 后) | **0** |
| 5 | 删除的 schema 全部 ∈ `tmp/v1_2/dead_schema_audit_before.json::unreachable[]` | 100% |
| 6 | 29 条 deprecated paths 全部决断完毕 | GC 报告 §2 表 29 行 |
| 7 | OpenAPI 总行数 < 14600 | 实际值 |
| 8 | path documentedSet ⊕ mountedSet = 0(或 known-gap 列入) | 0 / known-gap |
| 9 | `go vet ./...` PASS | exit 0 |
| 10 | `go build ./...` PASS | exit 0 |
| 11 | `go test ./tools/contract_audit/...` PASS | exit 0 |
| 12 | contract_audit 首跑 drift = 0(或 V1.2-B 列入) | 0 / V1.2-B |
| 13 | contract_audit 运行时间 < 30s | 实测 |
| 14 | CI hook drift-seed 阻断测试 | 阻断 PASS |
| 15 | docs/ 顶层 .md ≤ 12(P1 已 PASS · 复核) | ≤ 12 |
| 16 | docs/V1_BACKEND_SOURCE_OF_TRUTH.md 不含字段类型(P1 已 PASS · 复核) | 0 命中 |
| 17 | frontend doc 16 份全部修订标记 V1.2 + INDEX 含 V1 SoT 链接 | 满足 |
| 18 | retro + 治理 4 件套 + GC 报告 + audit JSON 全部落盘 | 全在 |

---

## §9 操作顺序(强制串行)

```
P0  step-0 baseline 校验(SHA + OpenAPI sha + go vet/build)
   ↓
[P1 已完成 · 跳过]
   ↓
P2a · 安全删除 15 个 unreachable schema
   ↓ (verify §2a.3)
P2b · 29 条 deprecated path 决断 + 级联可达性 GC
   ↓ (verify §3b.4)
P3  · 三向对账(documentedSet vs mountedSet)
   ↓ (verify §4.3)
P4  · contract_audit 工具落盘 + 首跑
   ↓ (verify §5.6 · drift = 0 或转 V1.2-B)
P5  · CI 硬门 + drift seed 测试
   ↓ (verify §6.x)
P6  · frontend doc 重生 + 治理 4 件套同步 + retro 落盘
   ↓ (verify 18 项)
输出终止符 V1_2_AUTHORITY_PURGED_AND_GUARD_LIVE
```

任何 phase verify 不通过 → 立即 ABORT · 不进下一 phase。

---

## §10 终止符 · 已知遗留 · handoff

### §10.1 输出终止符(成功)

```
V1_2_AUTHORITY_PURGED_AND_GUARD_LIVE
```

并在 `docs/iterations/V1_2_RETRO_REPORT.md` 顶部签字:

```
verdict: PENDING_ARCHITECT_VERIFY (codex 不自签 PASS)
matrix: 18/18 项已落盘 · 等架构师独立 verify
```

### §10.2 必填的已知遗留

转 V1.2-B 或 V1.3:

- **V1.2-B-1** · v1.22 rebuild + 部署对齐(继承 V1.1-A2 Q-2)
- **V1.2-B-2** · P3 case C(mount 但未 documented)若不为空
- **V1.2-B-3** · P4 contract_audit 首跑 drift > 0 的字段
- **V1.2-B-4** · P3b 中 D2(标错 deprecated)的 path V1.3 重新评估
- **V1.3** · 因 frontend doc 仍 reference 而未删的 unreachable schema(§2a.1 bucket B 中未替换的)

### §10.3 ABORT handoff

任何 Phase 触发 §1 ABORT trigger → 写 `docs/iterations/V1_2_RESUME_ABORT_REPORT.md` · 不输出终止符 · 不签字。

---

## §11 给 codex 的提醒(精简版)

- 你接的是 v1 ABORT 后的续接轮 · P0/P1 产物已落 · 不重做
- 体检数字以 path-closure 算法为准 · v1 prompt 的"≤ 190"等绝对数字目标已作废
- 任何 schema 删除前必须用 audit JSON 证明它在 unreachable 集中
- 任何 path 删除前必须用 transport AST 证明它未 mount
- contract_audit 工具的 drift seed 测试是 V1.3 之后字段不再漂移的唯一保证 · 不许跳过
- 完成后**不自签 PASS** · 等架构师独立 verify

---

## §12 给架构师的备忘

- v1 prompt 已在头部加 v2 修正备注 · 保留作为决策历史
- 本续接 prompt 默认 codex 在同一 git 工作区,P1 产物可直接读
- 若用户不同意 P2a 的"15 unreachable + 29 deprecated 决断"取代"≤ 190"目标,需要架构师另起一份新版

---

签发:架构师(Claude Opus 4.7)
日期:2026-04-26 PT
前置签字:`V1_1_A2_DONE_ARCHITECT_VERIFIED` + V1.2-v1 P0/P1 已完成
