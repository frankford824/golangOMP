## V1.2 · Authority 归位 + OpenAPI 死内容清算 + 字段级 Audit 工具 + CI 硬门 · 治本一次性收口

> 起草人:架构师(Claude Opus 4.7)
> 起草时间:2026-04-26
> 性质:**codex TUI / codex exec autopilot 单 prompt** · 6 阶段串行 · 强 ABORT 守卫
> 前置签字:`V1_1_A2_DONE_ARCHITECT_VERIFIED`(2026-04-26 PT)
> 终止符(成功):`V1_2_AUTHORITY_PURGED_AND_GUARD_LIVE`
> **状态**:v1 在 P2 ABORT(体检算法错误,详见 `docs/iterations/V1_2_ABORT_REPORT.md`)· P0/P1 已完成产物保留 · **续接执行用 [`prompts/V1_2_RESUME_FROM_P2.md`](V1_2_RESUME_FROM_P2.md)**(基于实测数字 schemas=313 / reachable=298 / unreachable=15 / deprecated paths=29 修正)
> 范围:`docs/**` 治理归位 + `docs/api/openapi.yaml` 死内容清算 + `tools/contract_audit/` 工具落盘 + `.cursor/hooks/` + `scripts/contract-guard.*` CI 硬门 + `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` 新建 + `CLAUDE.md` 升级
> 不做:**不改任何业务 Go 实现**(`transport/handler/`、`service/`、`repo/`、`domain/`、`cmd/server/main.go` 全部冻结) · **不重部署** · **不动数据库**

---

## §0 你是谁 · 你接的是什么

你是**接手 V1 治理收口轮**的 codex。你的目标不是"再修一次漂移",而是**让 V1.3 之后字段级漂移不再可能进入 main**。

### §0.0 上一轮(V1.1-A2)交付了什么 · 还差什么

V1.1-A2 已交付:
- OpenAPI 6 条 P0/P1 schema 修订(detail / product-info / cost-info / business-info / assign / warehouse-prepare)
- 4 个新 schema(`TaskAggregateDetailV2` / `TaskAggregateModule` / `TaskModuleEvent` / `ReferenceFileRefFlat`)
- 16 份 frontend doc 重生 + 修订历史
- 治理 4 件套同步(ROADMAP v33/v34/v35 / handoff manifest / onboarding / frontend handoff)
- 终止符 `V1_1_A2_DONE_ARCHITECT_VERIFIED`,带 2 条 known-debt

**架构师签字时记入的已知质量缺陷**:
- **Q-1** · `docs/iterations/V1_1_A2_DRIFT_INVENTORY.md` §6 中 192 条 clean path 是模板占位,违反"逐 path 真做字段 diff"硬约束(取样而非穷举)
- **Q-2** · `service/identity_service.go` 线上 v1.21 二进制 sha(`224e96fe`)与 git HEAD sha(`00ec340a`)不同步,需 v1.22 rebuild 对齐

**这一轮要收的更深的债**(用户元提问):
1. **真相文件混乱** · `docs/` 顶层 25 份 .md + `docs/archive/` 27 份 + `docs/iterations/` 164 份 + `prompts/` 28 份,每轮新模型来都要拼图
2. **OpenAPI 14757 行有大量死内容** · 313 个 schema 中 12 个零引用 + 150 个仅自定义未被 ref(共 162 个 = 52% 死)· 30 条 `deprecated:true` 永不过期 · transport `.GET/.POST/...` 239 mount 与 OpenAPI 203 path 数对不上
3. **V1.0+ 没有等价于 V0_9_BACKEND_SOURCE_OF_TRUTH.md 的当前真相文档** · 真相被分散到 4+ 份运营文件里
4. **没有自动化字段级 audit 工具** · 这就是 V1.1-A2 模板占位的直接原因
5. **没有 CI 硬门** · 改 handler 不强制改 OpenAPI,漂移可以悄悄进 main

### §0.1 你这一轮要做的 6 件事

1. **Phase 1 · Authority Inventory** · 把 25+164+27+28 份治理文件分桶(权威/事件流/归档/已删除),把 `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` 新建为 V1 真相单点
2. **Phase 2 · OpenAPI 死 schema 清算** · 用 yaml parser + 引用计数,删除所有零引用 schema,移除"自定义未被 ref"的孤儿 schema
3. **Phase 3 · OpenAPI 死/缺/错 path 清算** · 用 Go AST 反扫 `transport/http.go` 真实 mount 集合,与 OpenAPI 203 path 三向对账(mounted-and-documented / mounted-not-documented / documented-not-mounted)
4. **Phase 4 · 字段级 contract_audit 工具落盘** · 写 Go 工具 `tools/contract_audit/main.go` · 输入 transport AST + domain json tag + OpenAPI · 输出每个 path 一行真实字段集 diff,替代 V1.1-A2 inventory §6 的 192 条模板占位
5. **Phase 5 · CI 硬门** · `.cursor/hooks/contract-guard.json` + `scripts/contract-guard.ps1`/`.sh` · 任何改 `transport/handler/*.go` 或 `service/**.go` 的 commit 必须同时改 OpenAPI · 反之亦然
6. **Phase 6 · CLAUDE.md 升级 + frontend doc 重生 + V1 SoT 文档发布**

### §0.2 你**不**做的 6 件事

1. **不**改任何业务 Go 文件(`transport/handler/*.go` / `service/**.go` / `repo/**.go` / `domain/*.go` / `cmd/server/main.go`)
2. **不**重部署 v1.21(部署对齐 v1.22 留给 V1.2-B 轮)
3. **不**动数据库(任何环境)
4. **不**写新 prompt 到 `prompts/`(避免再造半权威 · 本轮 prompt 是最后一份过渡 prompt)
5. **不**删 `docs/archive/` 物理文件(只压缩归档 · 保留可逆性)
6. **不**删 `docs/iterations/` 物理文件(只压缩归档)

### §0.3 必读先决条件

接手前必须读完(按顺序):

- `CLAUDE.md`(authority 优先级当前定义)
- `prompts/V1_1_A2_CONTRACT_DRIFT_PURGE.md`(理解上一轮做了什么 · 含 §0.1.7 环境漂移说明)
- `docs/iterations/V1_1_A2_RETRO_REPORT.md`(上一轮交付证据)
- `docs/iterations/V1_1_A2_DRIFT_INVENTORY.md`(尤其 §6 详尽附录 · 看 192 条模板占位长什么样)
- `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`(authority · 看模板,V1 SoT 要照这个写)
- `docs/api/openapi.yaml`(14757 行 · 313 schemas · 本轮主要清理对象)
- `transport/http.go`(676 行 · authority 第一档 · 本轮 AST 解析对象)
- `docs/V1_TO_V2_MODEL_HANDOFF_MANIFEST.md`(V1.1-A2 后状态)

---

## §0.4 当前 baseline 锚(任何 stage 启动前 step-0 必校)

### §0.4.1 业务 SHA 锚组 A(10 文件 · 本轮一行不改 · 任何文件出现 modified/staged → ABORT)

```text
5f4c9a10227e8321c4a87c8260b2bc0078adbb2dfb9fa0ebd2bd86601f46bae8  service/asset_lifecycle/cleanup_job.go
60103b15fa877a8d14b719dbd9f2aa82ee957271e8e8dea79a42106a8f346a1c  service/task_draft/service.go
32cd0201bf205bc2abfb6a9f489202de4bd099e188349184bd55a4ae1e22454b  service/task_lifecycle/auto_archive_job.go
f9d09d1fbc55734b00ff1f6c35cc1bccbf9db05298283eff6f255971262638c2  repo/mysql/task_auto_archive_repo.go
658a8cdf65c09335ab74176efb4057eff68440537e50ce0d9e550c57413e6e6b  domain/task.go
0bf70496a21c995d230efbcfaee4499257f1e3e46506e206a0ec6f51a73b6881  cmd/server/main.go
6e10c7e6d3f8096538015385fd317e94715a24568122159154538be17e347c7e  service/task_aggregator/detail_aggregator.go
c6518daef3db588525c6cada3f366118c21483643c9241e81b1e6a13a81b70ba  repo/mysql/task_detail_bundle.go
00ec340a81738a75a88d3b0d32d834b49879bea7df6ac1baa0eb1932d1d47644  service/identity_service.go
d8c135221fc8c6745b6863521230a0a39ba43cc6420c713cc836e474fc1e8a6a  repo/mysql/identity_actor_bundle.go
```

step-0 校验命令(PowerShell):

```powershell
git status --short -- service/asset_lifecycle/cleanup_job.go service/task_draft/service.go service/task_lifecycle/auto_archive_job.go repo/mysql/task_auto_archive_repo.go domain/task.go cmd/server/main.go service/task_aggregator/detail_aggregator.go repo/mysql/task_detail_bundle.go service/identity_service.go repo/mysql/identity_actor_bundle.go
```

期望:**空输出**。任意文件出现非空 → ABORT。

### §0.4.2 OpenAPI baseline(进入 Phase 2 前必校 · `0ff87aa9...` 是 V1.1-A2 交付 sha)

```powershell
$expected = '0ff87aa90a53963a64350f92bf8bdce821dad3c24538bf70d61283b8dd97e5c3'
$actual = (Get-FileHash -Algorithm SHA256 docs\api\openapi.yaml).Hash.ToLower()
if ($actual -ne $expected) { throw "OpenAPI baseline drift: $actual vs $expected" }
```

### §0.4.3 当前体检数字(Phase 2/3 交付门槛由这些数字定)

| 项 | V1.1-A2 后值 | V1.2 期望值 | 阈值 |
|---|---:|---:|---|
| OpenAPI 总行数 | 14757 | 7000~10500 | 必须 < 12000 |
| `components.schemas` 总数 | 313 | 150~190 | 零引用 = 0 · 单自引用 ≤ 5 |
| 零引用 schema | 12 | **0** | 硬门 |
| 仅自定义未被 ref 的 schema | 150 | ≤ 5 | 硬门 |
| OpenAPI `/v1/*` path 数 | 203 | 与 transport AST mount 集合 ±0 | 硬门(差额 = 0) |
| `deprecated: true` 路径 | 30 | ≤ 30 | 每条必须有 `x-removed-at: <release>` |
| `docs/` 顶层 .md 数 | 25 | ≤ 8 | 多余的转 archive 或合并 |
| `docs/iterations/` 文件数 | 164 | 物理文件不删 · 必须有 INDEX | 必须有 `docs/iterations/INDEX.md` |
| `prompts/` 文件数 | 28 | ≤ 12(active)+ 余转 archive | 必须有 `prompts/INDEX.md` |

---

## §1 ABORT triggers(8 条硬门 · 命中即写 ABORT 报告并停止)

任意一条命中 → 立即写 `docs/iterations/V1_2_ABORT_REPORT.md` · 不进 §2~§6 · 不输出终止符。

1. §0.4.1 业务 SHA 锚组 A 任意一个文件出现 modified/staged
2. §0.4.2 OpenAPI baseline sha 与 `0ff87aa9...` 不一致
3. Phase 2 删除 schema 时,任意一个待删 schema 在 OpenAPI 中实际出现 ≥ 2 次 `$ref` 引用
4. Phase 3 删除 path 时,任意一个待删 path 在 transport AST 中实际有 mount
5. Phase 4 contract_audit 工具运行时 Go build/test 失败
6. Phase 5 CI hook 在已知 drift case(用 `tests/contract_drift_seed/` 注入)上未能阻塞
7. `go vet ./...` 或 `go build ./...` 在任一阶段 PASS → FAIL 反向回归
8. 本轮新增/修改的文件**意外**改到了 §0.2 列出的"不做"集合(业务 Go 文件)

ABORT 报告模板(必填字段):

```markdown
# V1.2 · ABORT · <stage>

- date: <ISO>
- stage: <P1|P2|P3|P4|P5|P6>
- trigger: <§1 中第几条>
- evidence: <文件路径 · 行号 · 命令 · 输出>
- rollback action: <已执行的 rollback 命令清单>
- next ask: <用户决策项>
```

---

## §2 输出物清单(Phase 6 完成后必须全部存在)

```
新增:
  tools/contract_audit/main.go                         (Go 工具源码)
  tools/contract_audit/main_test.go                    (单元测试)
  tools/contract_audit/README.md                       (使用说明)
  scripts/contract-guard.ps1                           (Windows 守门脚本)
  scripts/contract-guard.sh                            (Linux 守门脚本)
  .cursor/hooks/contract-guard.json                    (Cursor agent hook)
  docs/V1_BACKEND_SOURCE_OF_TRUTH.md                   (V1 真相单点)
  docs/iterations/INDEX.md                             (164 份文件索引 + 时间线)
  prompts/INDEX.md                                     (28 份 prompt 分类)
  docs/iterations/V1_2_AUTHORITY_INVENTORY.md          (Phase 1 产物)
  docs/iterations/V1_2_OPENAPI_GC_REPORT.md            (Phase 2+3 产物)
  docs/iterations/V1_2_CONTRACT_AUDIT_v1.json          (Phase 4 工具首跑产物)
  docs/iterations/V1_2_RETRO_REPORT.md                 (本轮 retro)
修改(治理类 · 允许):
  CLAUDE.md                                            (Authority 升级)
  docs/api/openapi.yaml                                (死内容清算后)
  docs/frontend/INDEX.md                               (修订历史 + V1 SoT 链接)
  docs/frontend/V1_API_*.md                            (16 份 · 重生)
  docs/V1_TO_V2_MODEL_HANDOFF_MANIFEST.md              (V1.2 状态同步)
  prompts/V1_NEXT_MODEL_ONBOARDING.md                  (V1.2 状态同步)
  prompts/V1_1_FRONTEND_INTEGRATION_HANDOFF.md         (通告 OpenAPI 已清算)
  prompts/V1_ROADMAP.md                                (v36/v37/v38/v39/v40/v41 追加)
  docs/iterations/V1_RETRO_REPORT.md                   (Q-1 known-debt 关闭)
归档移动(物理文件不删 · 用 git mv):
  docs/archive/legacy_specs/**                         (保持现状 · 仅纳入 INDEX)
  docs/archive/iterations_pre_v1_1/**                  (新建 · 把 V1.1-A2 之前的迭代报告移入)
禁止动:
  transport/handler/*.go    service/**.go    repo/**.go    domain/*.go
  cmd/server/main.go        go.mod / go.sum
```

---

## §3 Phase 1 · Authority Inventory + V1 SoT 文档新建

**目的**:把 25 + 164 + 27 + 28 份治理文件分桶,确立单一真相源。

### §3.1 步骤

1. 用 Get-ChildItem 列出 `docs\*.md`、`docs\archive\` 全部 + `docs\iterations\` 全部 + `prompts\` 全部
2. 对每个文件按下表分桶:

| 桶 | 判定规则 | 处理动作 |
|---|---|---|
| **A · authority(硬权威)** | CLAUDE.md 钦定的 4 份 | 保留在 `docs/` 顶层 · 任何修改需 prompt 显式声明 |
| **B · authority(V1 真相文档)** | 本轮新建 `V1_BACKEND_SOURCE_OF_TRUTH.md` | 新建 |
| **C · contract(契约)** | `docs/api/openapi.yaml` + `docs/frontend/*.md` | 保留 |
| **D · operational(运营/通告)** | `RELEASE_NOTES.md` / `V1_TO_V2_MODEL_HANDOFF_MANIFEST.md` / `V1_NEXT_MODEL_ONBOARDING.md` / `V1_1_FRONTEND_INTEGRATION_HANDOFF.md` / `V1_ROADMAP.md` | 保留在 `docs/` 或 `prompts/` · 但**禁止描述当前契约字段**,只允许指针式链接到 OpenAPI |
| **E · event-stream(事件流)** | `docs/iterations/*.md`(164 份) | 保留物理文件 · 必须新增 `docs/iterations/INDEX.md` 给出时间线和 abstract |
| **F · archive(已归档)** | `docs/archive/*` | 保留 · INDEX 列入 |
| **G · candidate-archive(候选归档)** | `docs/` 顶层中描述已废契约的(如 `V0_9_MODEL_HANDOFF_MANIFEST.md`、`V7_FRONTEND_INTEGRATION_ORDER.md`、`V1_0_FRONTEND_INTEGRATION_GUIDE.md` 这类)| `git mv` 到 `docs/archive/legacy_handoffs/` · INDEX 标注 |
| **H · merge-or-delete(合并或淘汰)** | 内容已被其他文档覆盖的(如 `MODEL_HANDOVER.md` 等) | 优先合并 · 实在没价值就 `git mv` 到 `docs/archive/_legacy_index/` |

3. 输出 `docs/iterations/V1_2_AUTHORITY_INVENTORY.md`,字段:`path | bucket | size | last_modified | abstract(<=80 字) | action(keep/archive/merge/index-only)`

4. 创建 `docs/V1_BACKEND_SOURCE_OF_TRUTH.md`(template 见 §3.2),作为 V1 真相单点 · 内容**不重复 OpenAPI 字段**,只承载:
   - V1 当前 release 状态(含 v1.21 / v1.22 占位)
   - 路由家族总览(指向 `transport/http.go` 行号)
   - SoT 优先级(Tier-0/1/2/3)
   - 已交付里程碑指针(链接到 retro 报告 · 不复述内容)
   - 已知遗留指针(链接到 known-debt entries · 不复述内容)

5. 同步 `CLAUDE.md` 的 AUTHORITY 块,把 `V0_9_BACKEND_SOURCE_OF_TRUTH.md` 升级为"历史依据",新增 `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` 为"当前依据"。

### §3.2 `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` 模板

```markdown
# V1 Backend · Source of Truth(当前依据 · 替代 V0.9 SoT 的 V1 主线)

> Tier-2 真相文档 · 不重复字段定义 · 字段查 OpenAPI
> Last verified: <ISO>
> Authority head: V1.<minor>.<patch> · git commit <short-sha>

## §0 SoT 优先级
1. `transport/http.go`        — Tier-0 · 路由是否 mount = API 是否存在
2. `docs/api/openapi.yaml`    — Tier-1 · 字段契约
3. 本文档                      — Tier-2 · 路由家族 / 治理状态 / 里程碑指针
4. 其他                        — Tier-3 · 事件流 / 归档 / prompts(指针,不描述契约)

## §1 当前 release 与部署
- production: v<x.y>
- artifact sha256: <...>
- detail P99: warm <…> / cold <…>

## §2 路由家族总览(只列 family · 字段查 OpenAPI)
| family | path prefix | mount 行号 | OpenAPI tag | frontend doc |
| Auth | /v1/auth | transport/http.go:<line> | Auth | docs/frontend/V1_API_AUTH.md |
| ... | | | | |

## §3 已交付里程碑(指针)
- R6.A.4 · `docs/iterations/V1_RETRO_REPORT.md`
- V1.1-A1 · `docs/iterations/V1_1_A1_DETAIL_P99_REPORT.md`
- v1.21 release · `docs/iterations/V1_RELEASE_v1_21_REPORT.md`
- V1.1-A2 · `docs/iterations/V1_1_A2_RETRO_REPORT.md`
- V1.2 · `docs/iterations/V1_2_RETRO_REPORT.md`

## §4 已知遗留(指针)
- V1.2-debt · v1.22 rebuild(identity_service.go 二进制对齐)
- 其他: 见 `docs/iterations/V1_2_RETRO_REPORT.md` §<n>

## §5 反向规则(必读)
- 任何"V1 当前契约字段"问题 → 看 `docs/api/openapi.yaml` · 不要看 prompts/iterations/archive
- 任何"V1 当前 mount 路由是否存在"问题 → 看 `transport/http.go` · 不要看 OpenAPI
- 本文档与 `transport/http.go` 或 OpenAPI 冲突 → **以本文档为错** · 立即修
```

### §3.3 ABORT 条件

- 任何 archive `git mv` 操作改到了非 .md 文件
- `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` 中出现具体字段名定义(违反"指针不描述"原则)→ 重写至合规

### §3.4 verify

- `docs/iterations/V1_2_AUTHORITY_INVENTORY.md` 行数 ≥ 200(覆盖 25+164+27+28 ≈ 244 文件)
- `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` 字段 fenced code block 内不出现 `string`/`int`/`object`/`array` 任何 OpenAPI 字段类型(grep 验证)
- `docs/` 顶层 .md ≤ 12 份(其余移入 archive 或并入 SoT 文档)

---

## §4 Phase 2 · OpenAPI 死 schema 清算

**目的**:把 162 个候选死 schema 严谨复核 → 确认死 → 删除 · 保留 git diff 作为可逆证据。

### §4.1 步骤

1. 写临时 Python 脚本 `tmp/v1_2_dead_schema_audit.py`(脚本本身可不归档,但产物必须归档),用 `yaml.safe_load` 解析 OpenAPI · 对每个 schema:
   - 计算 `#/components/schemas/<name>` 在原文 yaml 中的出现次数
   - **次数 == 0** → 候选死 schema bucket A
   - **次数 == 1** → 候选孤儿 schema bucket B(只是它自己的定义,没人 ref)
   - **次数 ≥ 2** → 活 schema bucket C
2. 输出审计报告 `docs/iterations/V1_2_OPENAPI_GC_REPORT.md` §1 dead-schema-audit · 三栏列表 + 总计
3. 对 bucket A · B 逐条核查:
   - 在 `docs/frontend/*.md` 中 grep schema 名,如出现 → 标注 in-frontend-doc · **不删**(改进:把 frontend doc 的引用替换为活 schema 或 inline 字段)
   - 在 `transport/handler/*.go` 注释或 swagger annotation 中 grep,如出现 → 标注 in-handler-comment · **不删**
   - 在 `prompts/` 中 grep · 出现 → 标注 in-prompt · **可删**(prompt 不是 SoT)
4. 真正可删的 schema 列表 → bucket A_clean · B_clean
5. 删除 bucket A_clean · B_clean 中的 schema 定义块(yaml 编辑 · 保持缩进)
6. 删除后跑 `python -c "import yaml;yaml.safe_load(open('docs/api/openapi.yaml'))"` 必须 OK
7. 重跑步骤 1 审计 · 死 schema 数应 = 0(或仅剩"frontend doc 还在用"的有意保留项 · 此时记录为 V1.3 待清理)
8. 在 `docs/iterations/V1_2_OPENAPI_GC_REPORT.md` §2 deleted-schemas 中列出所有删除条目 + 删除前 grep 引用次数 + git diff 行数

### §4.2 ABORT 条件

- 删除后 yaml parse 失败
- 删除后某 schema 的 `$ref` 出现次数 > 0(说明误删)
- bucket A · B 中有任一 schema 在 frontend doc 出现但未先做替换

### §4.3 verify

- `components.schemas` 数量从 313 降至 ≤ 190
- 零引用 schema = 0
- 仅自定义未被 ref 的 schema ≤ 5(且每条在 GC 报告中有 keep 理由)
- yaml 可 parse · `paths` 数仍 = 206(3 非 v1 + 203 v1)

---

## §5 Phase 3 · OpenAPI 死/缺/错 path 清算

**目的**:transport AST 反扫得到真实 mount 集合 · 与 OpenAPI 203 path 三向对账 · 三种 case 各自处理。

### §5.1 步骤

1. 写临时 Go 脚本 `tmp/v1_2_route_audit/main.go`(本轮工具的精简版),只做路由提取:
   - 用 `go/parser` 解析 `transport/http.go`
   - 抓所有 `r.Group("/v1", ...)` 与 `g.GET/POST/PUT/PATCH/DELETE/HEAD("...", ...)` 调用,拼接出真实 mount 路径集合 `mountedSet`
   - 输出 `tmp/v1_2/mounted_paths.txt` (一行一条 method+path)
2. 用 yaml parser 提取 OpenAPI paths 集合 `documentedSet`
3. 三向对账:
   - **case A · mountedSet ∩ documentedSet** = 正常,逐条进入 Phase 4 字段 audit
   - **case B · documentedSet \ mountedSet** = OpenAPI 有但 code 没 mount → 候选删除(可能是历史 path)
   - **case C · mountedSet \ documentedSet** = code 有但 OpenAPI 没记 → 必须补
4. case B 处理:
   - 对每条逐一在 `docs/frontend/*.md` 与 `transport/handler/*.go` grep
   - 如纯 OpenAPI 历史遗留 → 删除 OpenAPI 中的 path 块
   - 如 code 中有但被 `if FeatureFlag` 包住的 conditional mount → 标 `deprecated: true` + `x-removed-at: v1.3`
5. case C 处理:
   - 罕见但严重 · 必须补 OpenAPI(用临时占位 `summary: TODO V1.2-add` + `responses: {200: {description: TODO}}`)
   - 同时记入 V1.2 已知遗留(转 V1.2-B)
6. 输出 `docs/iterations/V1_2_OPENAPI_GC_REPORT.md` §3 path-audit · 三 bucket 详尽清单
7. 完成后,documentedSet 与 mountedSet 应该精确相等(差额 = 0)· 否则记入 ABORT 或 V1.2-B 已知遗留

### §5.2 ABORT 条件

- case B 中删除某 path 后,`docs/frontend/*.md` 仍在 reference 该 path(说明 frontend doc 也要同改)
- case C 不为空但未补全 → 转 V1.2-B 必须显式列出

### §5.3 verify

- `documentedSet ⊕ mountedSet` 对称差集 = 0,或在 GC 报告 §4 known-gap 中显式记入
- `transport/http.go` 一行不改(SHA 锚组 A 仍 0 漂移)
- yaml 可 parse · `paths` 数 = 3 + |mountedSet|

---

## §6 Phase 4 · 字段级 contract_audit 工具落盘

**目的**:替代 V1.1-A2 inventory §6 的 192 条模板占位。让"逐 path 字段集 diff"变成 1 条命令。

### §6.1 工具架构

```
tools/contract_audit/
├── main.go                       (CLI 入口 · 读 flag · 调度)
├── http_parser.go                (Go AST 解析 transport/http.go · 抽 mount 表)
├── handler_resolver.go           (Go AST 解析 transport/handler/*.go · 反射 respondOK/respondJSON 第二参数类型)
├── domain_struct_walker.go       (Go AST 走 domain/repo struct → 抽 json tag 字段集)
├── openapi_loader.go             (yaml parser · 抽 path 200 response data 字段集)
├── differ.go                     (字段集 diff: only-in-code / only-in-openapi / type-mismatch)
├── reporter.go                   (输出 JSON + Markdown)
├── main_test.go                  (3 个 case 单元测试)
└── README.md
```

### §6.2 关键约束

- 工具**不**用启发式或正则解析 Go,**必须**用 `go/parser` + `go/ast`(否则 ABORT)
- 工具**不**用启发式解析 yaml,**必须**用 `gopkg.in/yaml.v3`(项目已用)
- 工具输出 stable JSON(字段顺序固定 · 用 `encoding/json` + 排序),便于 `git diff`
- 工具运行时间 < 30s(否则 CI hook 不可用)
- 工具单元测试覆盖:
  - case 1 · drift seed:故意把 `domain.Task` 加一个 `XYZ` json tag,工具应识别为 only-in-code
  - case 2 · clean case:取一条 V1.1-A2 已对齐的 path,工具应输出空 diff
  - case 3 · openapi-extra:故意在 OpenAPI 加一个不存在的 `phantom_field`,工具应识别为 only-in-openapi
- 测试用 testdata 而不是改实际源码(testdata 放 `tools/contract_audit/testdata/`)

### §6.3 CLI

```
contract_audit \
  --transport transport/http.go \
  --handlers transport/handler \
  --domain domain \
  --openapi docs/api/openapi.yaml \
  --output docs/iterations/V1_2_CONTRACT_AUDIT_v1.json \
  --markdown docs/iterations/V1_2_CONTRACT_AUDIT_v1.md \
  --fail-on-drift true
```

`--fail-on-drift true` 时:发现任意 drift exit code != 0(供 CI 用)

### §6.4 输出 JSON schema

```json
{
  "version": "v1.2",
  "generated_at": "<ISO>",
  "openapi_sha256": "<...>",
  "transport_sha256": "<...>",
  "summary": {
    "total_paths": 203,
    "clean": 199,
    "drift": 4,
    "missing_in_openapi": 0,
    "missing_in_code": 0
  },
  "paths": [
    {
      "method": "GET",
      "path": "/v1/tasks/{id}/detail",
      "handler": "transport/handler/task_detail.go:GetByTaskID",
      "response_type": "service/task_aggregator.Detail",
      "code_fields": ["task","task_detail","modules","events","reference_file_refs"],
      "openapi_fields": ["task","task_detail","modules","events","reference_file_refs"],
      "only_in_code": [],
      "only_in_openapi": [],
      "type_mismatch": [],
      "verdict": "clean"
    }
  ]
}
```

### §6.5 ABORT 条件

- `go test ./tools/contract_audit/...` 失败
- `go build ./tools/contract_audit/...` 失败
- 工具首跑发现 drift 数 > 0 但 V1.1-A2 retro 未记录 → 必须立即写 `docs/iterations/V1_2_CONTRACT_AUDIT_v1.md` §unexpected-drift,转入 V1.2-B 决策

### §6.6 verify

- `go test ./tools/contract_audit/... -v -count=1` PASS
- `go build ./tools/contract_audit/...` PASS
- 工具运行时间(`Measure-Command { ./contract_audit.exe ... }`)< 30s
- 工具首跑 `summary.drift == 0`(本轮目标 · 否则记入 V1.2-B)

---

## §7 Phase 5 · CI 硬门(Cursor agent hook + 守门脚本)

**目的**:让"改 handler 不同步改 OpenAPI"在 commit 阶段就被阻断。

### §7.1 守门脚本 `scripts/contract-guard.ps1`(Windows)+ `scripts/contract-guard.sh`(Linux)

行为:

1. 读取 `git diff --cached --name-only`
2. 计算两个 set:
   - `code_changed` = changed files matching `^transport/handler/.*\.go$` 或 `^service/.*\.go$` 或 `^domain/.*\.go$`
   - `contract_changed` = changed files matching `^docs/api/openapi\.yaml$`
3. 决策矩阵:
   - 两个都空 → exit 0
   - 两个都非空 → 跑 `contract_audit --fail-on-drift true` · 通过 → exit 0
   - `code_changed` 非空 + `contract_changed` 空 → exit 1 + 输出 reminder:必须同 commit 改 OpenAPI · 或加 `[contract-skip-justified]` commit message tag(需架构师 review)
   - `contract_changed` 非空 + `code_changed` 空 → 允许(纯文档修订),但跑 `contract_audit` 验证不破坏现有 invariant
4. 跑 `contract_audit` 时必须 timeout 60s(否则 commit hook 会卡住)

### §7.2 Cursor agent hook `.cursor/hooks/contract-guard.json`

```json
{
  "version": 1,
  "hooks": {
    "before_shell_exec": {
      "rules": [
        {
          "match_command_regex": "^git\\s+commit",
          "action": "run",
          "command": "scripts/contract-guard.ps1",
          "block_on_failure": true,
          "timeout_ms": 60000
        }
      ]
    }
  }
}
```

(Linux 用 `scripts/contract-guard.sh`,Windows 用 `scripts/contract-guard.ps1`)

### §7.3 ABORT 条件

- 守门脚本不能在 1 分钟内完成
- 在 drift seed test 中,守门脚本未能阻断 commit
- hook 文件 JSON schema 无效

### §7.4 verify

- 在 `tests/contract_drift_seed/` 注入故意 drift(如改 `domain/task.go` 加一个 OpenAPI 没有的 json tag),用 `git add` + `git commit` 试,**应被阻断**
- 用 `[contract-skip-justified]` tag 应允许通过
- 还原 drift 后再 commit,应通过

(注:`tests/contract_drift_seed/` 不能真改 `domain/task.go`,而是用 `tools/contract_audit/testdata/` 模拟)

---

## §8 Phase 6 · CLAUDE.md 升级 + frontend doc 重生 + V1 SoT 发布

### §8.1 CLAUDE.md 升级

把 `## Current Repo Baseline` 段改写,把 `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md` 降级为"历史依据",把 `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` 升为"当前依据"。Reading order 调整:

```
1. docs/V1_BACKEND_SOURCE_OF_TRUTH.md           (Tier-2 当前真相 · 入口)
2. docs/api/openapi.yaml                         (Tier-1 字段契约)
3. transport/http.go                             (Tier-0 路由)
4. docs/iterations/V1_2_RETRO_REPORT.md          (V1.2 状态)
5. docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md          (历史依据 · 仅作背景)
```

### §8.2 frontend doc 重生

跑 `contract_audit --markdown docs/iterations/V1_2_CONTRACT_AUDIT_v1.md` 后,基于 Phase 2/3 清算后的 OpenAPI 重新生成 `docs/frontend/*.md` 16 份。重点:

- 引用任何 schema 必须先在 OpenAPI 中存在(Phase 2 删除的死 schema 不允许被 frontend doc 引用)
- 修订历史追加 V1.2 entry
- INDEX.md 顶部加链接到 `docs/V1_BACKEND_SOURCE_OF_TRUTH.md`

### §8.3 治理 4 件套同步

- `docs/V1_TO_V2_MODEL_HANDOFF_MANIFEST.md` · 当前状态 = `V1_2_AUTHORITY_PURGED_AND_GUARD_LIVE`
- `prompts/V1_NEXT_MODEL_ONBOARDING.md` · 必读列表把 V1 SoT 文档加在第一位
- `prompts/V1_1_FRONTEND_INTEGRATION_HANDOFF.md` · 通告 OpenAPI 已清算 · 死 schema 已删
- `prompts/V1_ROADMAP.md` · 追加 v36(authority inventory)/ v37(dead schema gc)/ v38(path 三向对账)/ v39(contract_audit 工具)/ v40(CI 硬门)/ v41(V1 SoT + frontend doc 重生)

### §8.4 retro 报告

`docs/iterations/V1_2_RETRO_REPORT.md` 落盘,字段:

```markdown
# V1.2 · Retro Report
- 日期 · 终止符 · 6 phase 状态
- 18 项 verify 矩阵结果表
- Phase 2 删除 schema 数 + 行数变化
- Phase 3 path 三向对账 bucket 数
- Phase 4 contract_audit 首跑结果(drift = ?)
- Phase 5 CI 硬门 drift seed 测试结果
- 已关闭 known-debt(V1.1-A2 Q-1)
- 转 V1.2-B 已知遗留(若有 · case C path / 工具首跑非 0 drift / v1.22 rebuild 等)
```

---

## §9 架构师 verify 矩阵(18 项 · 全 PASS 才允许输出终止符)

| # | 检查项 | 命令/证据 | 期望 |
|---|---|---|---|
| 1 | 业务 SHA 锚组 A 0 漂移 | `git status --short -- <10 files>` | 空输出 |
| 2 | OpenAPI baseline pre-V1.2 sha | `Get-FileHash docs/api/openapi.yaml`(在 §0.4.2 时点)| `0ff87aa9...` |
| 3 | OpenAPI yaml 可 parse | `python -c "import yaml;yaml.safe_load(...)"` | 无异常 |
| 4 | 零引用 schema = 0 | dead schema audit 重跑 | 0 |
| 5 | 仅自定义未被 ref schema ≤ 5 | dead schema audit | ≤ 5 + GC 报告每条有 keep 理由 |
| 6 | OpenAPI components.schemas ≤ 190 | python `len(d['components']['schemas'])` | ≤ 190 |
| 7 | OpenAPI 总行数 < 12000 | `(Get-Content docs/api/openapi.yaml).Count` | < 12000 |
| 8 | path documentedSet ⊕ mountedSet = 0 | route audit JSON | 0(或转 V1.2-B 已知遗留) |
| 9 | `go vet ./...` PASS | exit 0 | PASS |
| 10 | `go build ./...` PASS | exit 0 | PASS |
| 11 | `go test ./tools/contract_audit/...` PASS | exit 0 | PASS |
| 12 | contract_audit 首跑 drift = 0 | JSON `summary.drift` | 0(或转 V1.2-B) |
| 13 | contract_audit 运行时间 < 30s | `Measure-Command` | < 30s |
| 14 | CI hook drift-seed 阻断测试 | 模拟 drift commit · 应被阻断 | 阻断 PASS |
| 15 | docs/ 顶层 .md ≤ 12 | `(Get-ChildItem docs/*.md).Count` | ≤ 12 |
| 16 | docs/V1_BACKEND_SOURCE_OF_TRUTH.md 不含字段类型 | grep `string\|integer\|object\|array` 在 fenced code 内 | 0 命中 |
| 17 | frontend doc 16 份重生 + INDEX 含 V1 SoT 链接 | `Select-String V1_BACKEND_SOURCE_OF_TRUTH` | ≥ 1 命中 |
| 18 | retro 报告 + 治理 4 件套同步落盘 | 文件存在性 + sha | 8 文件全在 |

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

**架构师** verify 通过后另行签字 `V1_2_DONE_ARCHITECT_VERIFIED`。

### §10.2 已知遗留(必填)

转 V1.2-B 或 V1.3 的已知遗留(若有):

- **V1.2-B-1** · v1.22 rebuild + 部署对齐(继承 V1.1-A2 Q-2)
- **V1.2-B-2** · case C 中 mount 但未 documented 的 path(若 Phase 3 留下)
- **V1.2-B-3** · contract_audit 首跑 drift > 0 的字段(若 Phase 4 留下)
- **V1.3** · 任何因 frontend doc 仍 reference 而未删的 schema(Phase 2 留下)

### §10.3 ABORT handoff

任何 Phase 触发 §1 ABORT trigger → 写 `docs/iterations/V1_2_ABORT_REPORT.md` · 不输出终止符 · 不进后续 phase · 不签字。

---

## §11 操作顺序(强制串行 · 不允许并行)

```
P0 · step-0 baseline 校验(SHA 锚组 A + OpenAPI sha + go vet/build)
   ↓
P1 · Authority Inventory + V1 SoT 新建 + CLAUDE.md 升级(草稿)
   ↓ (verify §3.4)
P2 · OpenAPI 死 schema GC
   ↓ (verify §4.3 · yaml parse + 零引用 = 0)
P3 · OpenAPI 死/缺/错 path 三向对账
   ↓ (verify §5.3 · 对称差集 = 0 或转 V1.2-B)
P4 · contract_audit 工具落盘 + 首跑
   ↓ (verify §6.6 · drift = 0)
P5 · CI 硬门(scripts + .cursor/hooks)+ drift seed 测试
   ↓ (verify §7.4 · 阻断 PASS)
P6 · frontend doc 重生 + 治理 4 件套同步 + retro 落盘
   ↓ (verify 18 项)
输出终止符 V1_2_AUTHORITY_PURGED_AND_GUARD_LIVE
```

任何 Phase verify 不通过 → 立即 ABORT · 不进下一 Phase。

---

## §12 给 codex 的最后提醒

- 你是治本一轮,不是再修一次。**修流程 ≫ 修文档**。
- 你删除任何 schema/path 之前,必须有 yaml-parser/AST 证据。**不许启发式**。
- 你不写新 prompt 到 `prompts/`(本 prompt 是最后一份过渡 prompt · V1.3 之后用 `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` + `docs/iterations/V1_X_RETRO_REPORT.md` 替代)。
- 你交付 contract_audit 工具时,**测试比工具本身更重要**。drift seed 测试不通过 → 整轮 ABORT。
- 你完成后**不自签 PASS**,只签 `V1_2_AUTHORITY_PURGED_AND_GUARD_LIVE` 等待架构师 verify。

---

签发:架构师(Claude Opus 4.7)
日期:2026-04-26 PT
前置签字:`V1_1_A2_DONE_ARCHITECT_VERIFIED`
