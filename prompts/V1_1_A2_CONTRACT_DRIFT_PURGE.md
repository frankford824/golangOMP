## V1.1-A2 · 全量契约漂移清算(Contract Drift Purge)· 一次性收口

> 起草人:架构师(Claude Opus 4.7)
> 起草时间:2026-04-26
> 性质:**codex TUI / codex exec autopilot 单 prompt** · 5 阶段串行 · 强 ABORT 守卫
> 前置签字:`V1_RELEASE_v1_21_DONE_FRONTEND_DOCS_READY`
> 终止符(成功):`V1_1_A2_CONTRACT_DRIFT_PURGED`
> 用户决策:**走方案 A(契约跟实现走) · 全量回归 · 一次性彻底收口**
> 范围:`docs/api/openapi.yaml` + `docs/frontend/*.md` + 治理文档三件套
> 不做:**不改 Go 实现 / 不重部署 / 不动数据库**(本轮纯契约 + 文档治理)

---

## §0 你是谁 · 你接的是什么

你是**接手 V1.1-A1 contract-debt 收口轮**的 codex。

**事情怎么来的**

用户在 v1.21 上线后实测 `GET /v1/tasks/{id}/detail`,返回结构是 `data.{task, task_detail, modules, events, reference_file_refs}` 5 段精简,而 OpenAPI(行 10199-10226)仍 `$ref: '#/components/schemas/TaskDetail'`,该 schema(行 9759 起)声明 30+ 富字段(`procurement_summary` / `product_selection` / `matched_rule_governance` / `governance_audit_summary` / `design_assets` / `asset_versions` / `sku_items` / `cost_override_summary` / `boundary_summary` 等)。

源头:V1.1-A1 在 `transport/handler/task_detail.go:31-42` 接通了 `task_aggregator.DetailService` 这条 fast path · `cmd/server/main.go:321+:350` 默认 wire,生产 v1.21 永远走 fast path · 实际响应结构定义在 `service/task_aggregator/detail_aggregator.go:19-25` 的 `Detail` struct,只有 5 个字段。V1.1-A1 retro 写"0 OpenAPI 改动"字面成立但**实质失守**:响应 schema 字段集从 30+→5 是结构性变更,本应同步 OpenAPI。

**你这一轮要做的**

1. **全量扫描 203 个 `/v1` 路径**,逐条做 handler-vs-OpenAPI 字段级 diff,产出 drift inventory
2. **修订 OpenAPI**(契约跟实现走),让 `docs/api/openapi.yaml` 100% 对齐 v1.21 生产实际响应
3. **重生成 `docs/frontend/*.md`**(基于修订后的 OpenAPI),保证前端联调文档与生产一致
4. **关闭 V1.1-A1 retro 已记的 known debt**,把本轮列为 V1.1-A2 已交付
5. **同步 ROADMAP / handoff manifest / onboarding / frontend integration handoff** 四份治理文档
6. **不**改任何 Go 文件(实现已在 v1.21 生产稳定运行 · 你是修文档,不是修代码)

**必读先决条件**

接手前必须读完(如果某文件已读过,你需要重读最新版本,因为可能在 v1.21 release 中刷过):

- `CLAUDE.md`(authority 优先级:transport/http.go > openapi.yaml > V0_9_BACKEND_SOURCE_OF_TRUTH > 任何其他)
- `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`(authority)
- `docs/api/openapi.yaml`(15147 行 · 203 path)
- `docs/V1_TO_V2_MODEL_HANDOFF_MANIFEST.md`(v1.21 release 后状态)
- `docs/iterations/V1_RETRO_REPORT.md`(R6.A.4 retro · 含 V1.1-A1 收口)
- `docs/iterations/V1_1_A1_DETAIL_P99_REPORT.md`(V1.1-A1 P99 报告)
- `docs/iterations/V1_RELEASE_v1_21_REPORT.md`(v1.21 release 报告)
- `prompts/V1_1_FRONTEND_INTEGRATION_HANDOFF.md`(前端联调入口)
- `prompts/V1_NEXT_MODEL_ONBOARDING.md`(接手包)
- `prompts/V1_ROADMAP.md`(§32 总览表 v1~v32 changelog)
- `transport/http.go`(实际挂载的 v1 路由,authority 第一档)
- `service/task_aggregator/detail_aggregator.go`(种子证据 · `Detail` struct)
- `service/identity_service.go`(V1.1-A1 fast path · 可能也漂移)
- `repo/mysql/task_detail_bundle.go` + `repo/mysql/identity_actor_bundle.go`
- `transport/handler/*.go`(全部 handler · §1 扫描必查)
- `domain/*.go`(struct json tag 是字段命名权威)
- `docs/frontend/INDEX.md` + `docs/frontend/V1_API_*.md`(全 16 份 · 输出对象)

---

## §0.1 当前 baseline 锚(任何 stage 启动前 step-0 必校)

### §0.1.1 v1.21 release 现状(不动)

| 项 | 值 |
|---|---|
| 当前线上版本 | **v1.21** |
| 部署时间 | 2026-04-25T10:28:52Z |
| Artifact sha256 | `977da0e4561a6baf841f89fca1c2cd0cb1c14b93bb97d981ee72632488a513bc` |
| 生产数据库 | `jst_erp` |
| 生产 detail P99 | warm 32.933ms · cold 32.995ms |
| 测试库 | `jst_erp_r3_test` 已 DROP |
| Go 代码 | **本轮一行不改** |

### §0.1.2 SHA 锚分两组

**组 A · 业务实现 sha(11 文件 · 任何 stage 都不可改 · 改了立即 ABORT)**

```text
5f4c9a10227e8321c4a87c8260b2bc0078adbb2dfb9fa0ebd2bd86601f46bae8  service/asset_lifecycle/cleanup_job.go
60103b15fa877a8d14b719dbd9f2aa82ee957271e8e8dea79a42106a8f346a1c  service/task_draft/service.go
32cd0201bf205bc2abfb6a9f489202de4bd099e188349184bd55a4ae1e22454b  service/task_lifecycle/auto_archive_job.go
f9d09d1fbc55734b00ff1f6c35cc1bccbf9db05298283eff6f255971262638c2  repo/mysql/task_auto_archive_repo.go
658a8cdf65c09335ab74176efb4057eff68440537e50ce0d9e550c57413e6e6b  domain/task.go
0bf70496a21c995d230efbcfaee4499257f1e3e46506e206a0ec6f51a73b6881  cmd/server/main.go
```

V1.1-A1 4 个新改动文件 sha(本轮新 baseline · git commit `207f9a1` 入库内容):

```text
6e10c7e6d3f8096538015385fd317e94715a24568122159154538be17e347c7e  service/task_aggregator/detail_aggregator.go
c6518daef3db588525c6cada3f366118c21483643c9241e81b1e6a13a81b70ba  repo/mysql/task_detail_bundle.go
00ec340a81738a75a88d3b0d32d834b49879bea7df6ac1baa0eb1932d1d47644  service/identity_service.go
d8c135221fc8c6745b6863521230a0a39ba43cc6420c713cc836e474fc1e8a6a  repo/mysql/identity_actor_bundle.go
```

注意:`identity_service.go` 当前 sha `00ec340a...` 与 `docs/iterations/V1_1_A1_DETAIL_P99_REPORT.md` 行 39 记录的 `224e96fe...` 不同 · 这是已知漂移 · 详见 §0.1.7。**本轮以 git commit `207f9a1` 入库内容为权威**,V1.1-A1 P99 报告 sha 仅作历史参考,不再用于 step-0 校验。

`transport/http.go` **不在** V1.1-A1 改动集合(V1.1-A1 改动为上述 4 个文件 · 不含 http.go)· **不进 SHA 锚组 A**。

step-0 必校(10 个文件 · 6 业务 + 4 V1.1-A1):

```powershell
# PowerShell · 期望 CLEAN
git status --short -- service/asset_lifecycle/cleanup_job.go service/task_draft/service.go service/task_lifecycle/auto_archive_job.go repo/mysql/task_auto_archive_repo.go domain/task.go cmd/server/main.go service/task_aggregator/detail_aggregator.go repo/mysql/task_detail_bundle.go service/identity_service.go repo/mysql/identity_actor_bundle.go
```

任意文件出现非空(modified / staged) → ABORT。

**回退到 sha256 计算**(防 git 工具不可用):用 `Get-FileHash -Algorithm SHA256 <path>` (PowerShell) 或 `sha256sum <path>` (WSL),把 10 文件实际 sha 写入 `tmp/v1_1_a2/baseline_sha.log`,逐条对照本节给出的 10 条 sha,任意不一致 → ABORT。

**组 B · 契约/文档 sha(本轮**允许**变更 · 但变更必须可解释)**

```text
docs/api/openapi.yaml         (本轮主要修订对象)
docs/frontend/INDEX.md
docs/frontend/V1_API_*.md     (全 16 份)
docs/V1_TO_V2_MODEL_HANDOFF_MANIFEST.md
prompts/V1_NEXT_MODEL_ONBOARDING.md
prompts/V1_1_FRONTEND_INTEGRATION_HANDOFF.md
prompts/V1_ROADMAP.md
docs/iterations/V1_RETRO_REPORT.md
```

step-0 还要把组 B 的 sha 也写入 baseline,**stage 末尾再算一次,变更范围必须 ⊂ inventory 列出的 path 集合**。

### §0.1.3 OpenAPI 路径数与挂载数

```bash
grep -c '^  /v1' docs/api/openapi.yaml      # 期望 203 · stage 末尾再校验,数字不可变(本轮**不增不删 path · 只改 schema**)
grep -c '^paths:' docs/api/openapi.yaml     # 期望 1
grep -c '"/v1/' transport/http.go           # 与 v1.21 release 报告记录的值一致
```

**203 path 数本轮硬不可变**:本轮不允许新增 path、不允许删除 path、不允许重命名 path。只允许改 schema、改 description、改 example、改 components.schemas 内容。任何 path 数变化 → ABORT。

### §0.1.4 OpenAPI validator

本轮 openapi 改完必须 `0 error 0 warning`。validator 命令(参考 v1.21 release 报告 §6):

```bash
# 用项目内已有的 openapi-validate 工具(参考 deploy/lib.sh / v1.21 release report)
make openapi-validate
# 或:
swagger-cli validate docs/api/openapi.yaml
```

**任何 error/warning → ABORT**(允许预先告警的 schema 警告除外,需在 retro 中明确说明)。

### §0.1.5 dangling 501

```bash
grep -c 'HTTP 501' docs/api/openapi.yaml   # 期望 0
```

任何 501 出现 → ABORT。

### §0.1.6 工具与环境

- 操作系统:Windows + WSL Ubuntu(WSL 内 sha256sum / grep / sed / jq 可用 · PowerShell 内用 `Get-FileHash`)
- Go 工具链:1.24+ · 仅用于 `go vet` / `go build` 兜底,不改代码
- 不需要数据库(本轮纯文档)
- 不需要 SSH 到 jst_ecs(本轮不部署)
- 用 ripgrep 替代 grep:`rg`(Windows 上 Cursor 默认安装,WSL 需 `apt install ripgrep`)
- **git 工具链**:本轮项目已是 git 仓库 · `core.autocrlf=false` 已锁(2026-04-26)· 远端 `origin = github.com/frankford824/golangOMP` · 当前 HEAD = `207f9a1`(`chore: initialize repository at v1.21 production baseline`)· 仅 1 个 commit · 无可 diff 的历史。每个 stage 末尾必跑 `git status` 把改动范围对照 §0.1.2 / §0.2 限制。

### §0.1.7 环境漂移说明(v1.21 release → git baseline)

**关键事实**:`docs/iterations/V1_1_A1_DETAIL_P99_REPORT.md` 与 v1.21 release 落盘的 `tmp/v1_21_baseline_sha.log` 中记录的 `service/identity_service.go` sha 是 `224e96fe180a27349131710848229c73c686cfe5aa1704339573859398fa8f89`,**当前 git HEAD 入库内容 sha 是** `00ec340a81738a75a88d3b0d32d834b49879bea7df6ac1baa0eb1932d1d47644`。

时间线复盘:

| 时间(UTC-7 / PT) | 事件 | identity_service.go sha |
|---|---|---|
| 2026-04-25 02:22~02:43 | V1.1-A1 4 文件最后保存(LF 工作区) | `224e96fe` |
| 2026-04-25 10:28 UTC | **v1.21 部署到生产**(基于 `224e96fe` 编译) | `224e96fe` |
| 2026-04-25 | `tmp/v1_21_baseline_sha.log` 落盘 | `224e96fe` |
| 2026-04-26 19:34 PT | mtime 显示 `identity_service.go` 被某次未受控操作改写 | `00ec340a` |
| 2026-04-26 19:34~20:33 PT | 用户 `git init` + `git add .` + `git commit` + `git push GitHub`,固化 `207f9a1` | `00ec340a`(入库) |
| 2026-04-26 20:47 PT | V1.1-A2 V1 prompt 首跑 → 撞 SHA gate ABORT | `00ec340a` |
| 2026-04-26 ~21:00 PT | 架构师裁决 · 接受 `207f9a1` 为新 baseline · prompt v2 重启 | `00ec340a` |

**架构师裁决**:由于(1)git 仓库只有 1 个 commit · 无 v1.21 release 时刻历史可 diff;(2)`jst_ecs:/root/ecommerce_ai/releases/v1.21/` 是否含源码未做尽职调查;(3)漂移文件的 V1.1-A1 fast path 关键结构特征仍在(`reader interface ResolveActorBundle` 在行 231 · fast path 调用在行 1448),**本轮接受 `207f9a1` 为新 baseline**,后续验证 sha 全部以 git HEAD 为权威。`224e96fe` 仅作历史参考,不再用于校验。

**已知遗留 debt(转 V1.2)**:

1. 线上 v1.21 二进制(基于 `224e96fe` 编译)与本地 git HEAD `207f9a1`(基于 `00ec340a`)的 `identity_service.go` 内容不完全一致 — 这是**线上 ≠ 本地代码**的短期事实
2. 19:34 那次改动**未做内容尽职调查**(sha 不同但结构推断为格式化级或单点微调)
3. V1.2 必修项:重新构建 v1.22 → 部署生产 · 让线上二进制重新对齐本地 git HEAD · 同步刷新 baseline_sha.log

**本轮 V1.1-A2 不为此 debt 部署 / 不改任何 .go 文件**。inventory 阶段(P1)对 identity 相关接口(`/v1/auth/me` / session 校验 / actor 解析等)字段级 diff 必须详尽 · 单独标注 "post-v1.21 micro-drift potential" · 任意字段集疑点立刻列入 V1.2 修复清单(本轮不修)。

---

## §0.2 硬约束(违反任意一条立即 ABORT)

1. **不改任何 Go 文件**:本轮范围严格在 `docs/api/openapi.yaml` + `docs/frontend/*.md` + 4 份治理文档内。每个 stage 末尾跑 `git status --short -- '*.go'` 必须返回**空输出**,任意非空 → ABORT。
2. **203 path 不增不减**:不能为了"补漂移"新增路径,也不能为了"清理"删路径。结构性增删走 V1.2 立项,不在本轮。
3. **Stage 顺序不可乱**:必须 P1 全量 inventory 完成后才进 P2 裁决,P2 完成才进 P3 修 OpenAPI,P3 完成才进 P4 重生成 frontend doc。中途乱序 → ABORT。
4. **每个 schema 修订必须有 inventory 行号背书**:不能"顺手优化"无 inventory 记录的 schema,即使你看到明显错误。可疑项追加到 inventory,不直接动手。
5. **不破坏 v1.21 RBAC / 状态码语义**:`x-rbac-placeholder` / `responses` / `parameters` 三类元数据本轮不改(因为代码没改)。仅改 `responses.<code>.content.application/json.schema` 与对应的 `components.schemas.*`。如果发现 RBAC/状态码本身就漂移,**记入 inventory 待 V1.2 处理,不在本轮修**。
6. **不重写、不删 frontend doc 文件名集合**:`docs/frontend/` 下 16 份 markdown 文件名保持不变。内容可全量重生成,但目录结构不变。
7. **不改 `/ws/v1` WebSocket 文档路径**(已澄清是 OpenAPI 实际路径,不是 prompt 错列的 `/v1/ws/v1`)。
8. **整轮一次性失败时**:不允许产出"半成品 inventory + 半成品 OpenAPI 修订"。要么完整提交,要么 ABORT 报告 + 原状回滚 OpenAPI/frontend。
9. **architect 第一档优先级是 `transport/http.go`**:任何疑似漂移项,先看 http.go 实际挂的是哪个 handler、handler 实际返回哪个 struct,struct 的 json tag 是命名权威,**OpenAPI 必须跟 struct 走,不是反过来**。
10. **本 prompt 内任何"参考""示意""可能"等模糊指令,你都要返回到 inventory 中确认事实,再决定动手**。

---

## §1 阶段 P1 · 全量 drift inventory(只读扫描)

**目标**:对 203 个 `/v1` 路径逐条做 handler-vs-OpenAPI 字段级 diff,产出权威 inventory 报告。

### §1.1 准备

- `mkdir -p tmp/v1_1_a2`
- `mkdir -p docs/iterations`
- 将本 prompt 的 §0.1.2 SHA 锚组 A + 组 B baseline 写入 `tmp/v1_1_a2/baseline_sha.log`

### §1.2 路径清单提取

step-1.2.1 从 OpenAPI 提取 203 path 清单:

```bash
grep -E '^  /v1[a-zA-Z0-9_/{}-]*:$' docs/api/openapi.yaml \
  | sed -E 's/^  (.+):$/\1/' \
  | sort -u > tmp/v1_1_a2/openapi_paths.txt
wc -l tmp/v1_1_a2/openapi_paths.txt   # 期望 203
```

step-1.2.2 从 `transport/http.go` 提取实际挂载路径:

```bash
grep -oE '"(/v1/[^"]*)"' transport/http.go \
  | sed 's/"//g' | sort -u > tmp/v1_1_a2/transport_paths.txt
```

step-1.2.3 path 双向 diff:

```bash
diff <(sort -u tmp/v1_1_a2/openapi_paths.txt) <(sort -u tmp/v1_1_a2/transport_paths.txt) \
  > tmp/v1_1_a2/path_diff.log
```

任何 OpenAPI 有但 transport 无的 path(死契约),或 transport 有但 OpenAPI 无的 path(暗路径)→ 记入 inventory `class=path-mismatch`,**但本轮不修**(违反 §0.2 #2)· V1.2 立项。

### §1.3 逐 path 字段级 diff(核心工作量)

对 `tmp/v1_1_a2/openapi_paths.txt` 中**每一条**路径,执行 §1.3.1~§1.3.5。

**强制要求**:你必须真的逐条做,不允许偷懒只抽样。inventory 的覆盖率 < 100% 即 ABORT。

#### §1.3.1 定位 handler

```text
对路径 P,在 transport/http.go 中 grep "P" 找到 .GET/.POST/.PUT/.DELETE 行,
读出右侧绑定的 handler method,例如:
  taskDetailH.GetByTaskID  → handler/task_detail.go
然后到 transport/handler/<file>.go 阅读 handler method 的 respondOK / c.JSON 调用,
拿到实际返回的 Go struct 或 map。
```

#### §1.3.2 反推实际响应 JSON shape

- 如果 handler 调用 `respondOK(c, X)`,X 的类型决定 shape;follow 到 service/struct 定义,记录字段名(json tag) + 字段类型 + omitempty 标志。
- 如果 X 是 `*service.SomethingResponse`,follow 到 service 内部 struct 定义。
- 如果是 `map[string]any`,记录 map 的所有 key + value 类型推断。
- **`respondOK` 默认包一层 `{"data": ...}`**:这是 v1.21 顶层 envelope,你要在 inventory 中明确"`data` 包裹层是否存在"。
- 处理特殊响应:分页(`{data, pagination}`)、错误(`{error, code, message}`)、空体(`204 No Content`)等,逐项标记。

#### §1.3.3 提取 OpenAPI 中该 path 当前声明

- 找到 `paths.<P>.<method>.responses.200.content.application/json.schema`
- 如果是 `$ref`,跟到 `components.schemas.<Name>`,继续 follow $ref 直到全展开,记录字段集
- 同样标记 `data` envelope 是否存在 / 字段 nullable / required 等

#### §1.3.4 字段级 diff 分类

对每个字段,二选一:

- **存在性 diff**:OpenAPI 有但 struct 无 → `class=phantom-field`(契约虚字段)· OpenAPI 无但 struct 有 → `class=missing-field`(契约缺字段)
- **类型 diff**:同名字段,OpenAPI 类型 ≠ struct 类型 → `class=type-mismatch`
- **命名 diff**:json tag 写法与 OpenAPI 字段名不一致(snake_case vs camelCase 等)→ `class=naming-drift`
- **nullability diff**:OpenAPI 标 required 但 struct 是 omitempty → `class=nullability-drift`
- **wrapper diff**:`data` envelope 存在性不一致 → `class=envelope-drift`
- **status-code diff**:实际可能返回的状态码 OpenAPI 没列(例如 401/403/409)→ `class=status-code-gap`(本轮**只标不修**,违反 §0.2 #5)

#### §1.3.5 写一行 inventory

每条 path 至少一行 inventory(无漂移也写一行 `class=clean`)。漂移多的接口可以多行。格式:

```text
| path | method | handler | response_struct | class | field | openapi_says | code_says | severity | fix_plan |
```

**severity 三档**:
- `P0` — 字段集级别 ≥ 50% 不匹配(detail 这种)· 或 envelope 错位 · 或核心字段类型错
- `P1` — 字段集 ≤ 50% 漂移 · 或 nullability / naming 漂移
- `P2` — 仅 description / example / 状态码补充类(本轮可能不动手)

**fix_plan 五档**:
- `rewrite-schema` — 整个 schema 重写
- `add-fields` — OpenAPI 加缺失字段
- `remove-fields` — OpenAPI 删 phantom 字段
- `flip-required` — 调 required/nullable
- `defer-v12` — 本轮不修(P2 / 跨契约风险)

### §1.4 落盘 inventory

**必须落盘**:`docs/iterations/V1_1_A2_DRIFT_INVENTORY.md`

最小结构:

```markdown
# V1.1-A2 · OpenAPI/code drift inventory

> 生成时间:<UTC>
> 范围:203 个 /v1 path · v1.21 生产实际响应 vs OpenAPI 当前声明
> 用途:V1.1-A2 修复源数据 · 不改 Go 实现 · 仅改 OpenAPI/frontend doc

## §1 概览
- 总 path 数:203
- clean(无漂移):<n>
- P0:<n>
- P1:<n>
- P2:<n>
- defer-v12:<n>

## §2 P0 漂移清单(本轮必修)
| path | method | handler | class | severity | fix_plan | 简述 |
|---|---|---|---|---|---|---|
| /v1/tasks/{id}/detail | GET | TaskDetailHandler.GetByTaskID | rewrite-schema | P0 | rewrite-schema | OpenAPI 声明 30+ 富字段 · 实际 fast path 返回 5 段 |
| ... | ... | ... | ... | ... | ... | ... |

## §3 P1 漂移清单(本轮必修)
...

## §4 P2 漂移清单(本轮酌情修)
...

## §5 defer-v12 清单(本轮不修 · 留 V1.2)
...

## §6 字段级原始 diff(详尽附录)
按 path 顺序列出每条 inventory 行
```

### §1.5 P1 终止条件

- inventory 覆盖 203 / 203 path
- 每行有完整 7 列(path/method/handler/response_struct/class/field/severity)
- P0 + P1 至少包含 detail 接口(种子证据)
- **inventory 用 git add 暂存但不 commit**(等到 P5)

落盘后输出阶段性中间符:`V1_1_A2_P1_INVENTORY_READY`

---

## §2 阶段 P2 · 分类裁决与修订计划

**目标**:基于 inventory 决定每条 P0/P1 的修订动作,产出 `V1_1_A2_FIX_PLAN.md`。

### §2.1 默认裁决:code wins

按 `CLAUDE.md` 工作规则,**实现是第一档权威**(transport/http.go > openapi.yaml)。

默认:OpenAPI 跟 struct 改,struct 不动。

### §2.2 例外裁决(必须列原因)

如果某条漂移你认为应当**改 struct 不改 OpenAPI**(例如 OpenAPI 才是真正约定俗成的命名,struct 是后期错误),记入"例外清单",但**本轮不修 struct**(违反 §0.2 #1),仅在 inventory 标 `defer-v12` + 详细原因。

### §2.3 schema 命名策略

新增的 components.schemas 命名规则:

- 与现有命名冲突时:用 `*V2` 后缀(例如 `TaskDetailV2`)
- 嵌套对象:用层级名(例如 `TaskDetailV2.Module` → `TaskAggregateModule`)
- 动作枚举:就近 inline,不抽 schema
- 命名权威是 **struct json tag**,不是 OpenAPI 历史命名

### §2.4 落盘 fix plan

**必须落盘**:`docs/iterations/V1_1_A2_FIX_PLAN.md`

包含:

- 每条 P0/P1 漂移的具体修订动作(改哪个 schema · 改哪几行 · 新增哪个 schema)
- 受影响 components.schemas 列表
- 受影响 frontend doc family 列表(`V1_API_TASKS.md` / `V1_API_AUTH.md` 等)
- 修订顺序(避免循环依赖)
- 预估 OpenAPI 行数变化(±N 行)

输出:`V1_1_A2_P2_PLAN_READY`

---

## §3 阶段 P3 · OpenAPI 修订(主要工作量)

**目标**:按 fix plan 修订 `docs/api/openapi.yaml`,让契约 100% 对齐 v1.21 实现。

### §3.1 修订准则

1. 一次只动一个 schema · 改完立即 `swagger-cli validate` 一次,确保还能 parse
2. 每改一个 schema · 在 fix plan 对应行打 `[done]`
3. 每个 schema 改动**必须**在 fix plan 中有对应条目(违反 §0.2 #4)
4. **保留所有 `x-` 扩展**(`x-api-readiness` / `x-rbac-placeholder` / `x-owner-round` 等),原样保留
5. **保留所有 `description`** —— 除非描述本身漂移(例如 description 写"返回 procurement_summary"而 v1.21 fast path 没返回),那要改描述
6. 修订过程中**不允许**:
   - 重排 paths 顺序(diff 友好)
   - 重命名 path
   - 改 method
   - 改 parameters(本轮不动)
   - 改 RBAC 元数据
   - 改 status code(P2 / defer-v12)

### §3.2 detail 接口示例(必修)

`/v1/tasks/{id}/detail` 是 P0 标杆。期望最终 OpenAPI:

```yaml
/v1/tasks/{id}/detail:
  get:
    summary: Get task aggregate detail (V1.1-A1 fast-path 5-section)
    description: |
      Returns 5-section task aggregate produced by `task_aggregator.DetailService`
      fast path. Top-level `data` envelope contains: `task` (主表)
      / `task_detail` (详情 omitempty) / `modules[]` / `events[]` (最多 50)
      / `reference_file_refs[]`. 富字段如 `procurement_summary`、
      `product_selection` 完整快照、`matched_rule_governance`、`design_assets`、
      `asset_versions`、`sku_items`、`governance_audit_summary` 等不在此接口返回 —
      请改用专用接口(/v1/tasks/{id}/procurement-summary、
      /v1/tasks/{id}/asset-center/*、/v1/tasks/{id}/cost-overrides 等)。
    tags: [TaskDetail, ReadyForFrontend]
    x-api-readiness: ready_for_frontend
    x-owner-round: V1.1-A1
    x-rbac-placeholder: <保留>
    parameters:
      - in: path
        name: id
        required: true
        schema: { type: integer }
    responses:
      '200':
        description: 5-section aggregate detail
        content:
          application/json:
            schema:
              type: object
              properties:
                data:
                  $ref: '#/components/schemas/TaskAggregateDetailV2'
      '404':
        description: Task not found
```

新增 `components.schemas.TaskAggregateDetailV2`,含 5 个 properties · 字段集严格对齐 `service/task_aggregator/detail_aggregator.go:19-25` 的 `Detail` struct + `domain.Task` / `domain.TaskDetail` / `domain.TaskModule` + `task_aggregator.ModuleDetail` 的额外字段(`visibility` / `allowed_actions` / `projection`)+ `domain.TaskModuleEvent` + `domain.ReferenceFileRefFlat` 的全部 json tag。

**`task_detail` 中的 omitempty 字段**(用户已实测列出):`category_id` / `width` / `height` / `area` / `quantity` / `cost_price` / `estimated_cost` / `cost_rule_id` / `matched_rule_version` / `prefill_at` / `override_at` / `last_filing_attempt_at` / `last_filed_at` / `filed_at` / `product_id` / `requester_id` / `designer_id` / `current_handler_id` / `operator_group_id` / `last_customization_operator_id` / `warehouse_reject_reason` / `warehouse_reject_category` / `primary_sku_code` / `deadline_at` / `missing_fields` / `missing_fields_summary_cn` / `base_sale_price` / `product_selection`。在 OpenAPI 中标记为非 required,nullable: true。

字段顺序遵循 struct 顺序(diff 友好)。

### §3.3 同名 schema 处置

- 老 `TaskDetail` schema(行 9759 起)如果只被 `/v1/tasks/{id}/detail` 引用 → 整段删除
- 如果还被其他 path 引用(P1 扫描时记录引用图)→ 保留老 schema,新接口指向新 schema `TaskAggregateDetailV2`
- 删除老 schema 时,顺带删除其只被它引用的子 schema(例如 `TaskDetailMatchedRuleGovernance` 等)
- **删除前必须 inventory 行有"orphan-schema-cleanup"标记**

### §3.4 P3 末尾校验

```bash
make openapi-validate    # 0 error 0 warning
grep -c '^  /v1' docs/api/openapi.yaml   # 期望 203
grep -c 'HTTP 501' docs/api/openapi.yaml # 期望 0
go vet ./...             # 期望 PASS(防止意外动到 go 文件)
git status               # 期望只有 docs/api/openapi.yaml + 治理文档变更,*.go 0 改动
```

任何一条不通过 → 立即停手,产出 ABORT 报告。

输出:`V1_1_A2_P3_OPENAPI_FIXED`

---

## §4 阶段 P4 · 前端文档全量重生成

**目标**:基于修订后的 OpenAPI 重生成 `docs/frontend/*.md`,16 份(或修订后实际数量)markdown 全量更新。

### §4.1 重生成规则

- 复用 v1.21 release 时使用的生成方法(参考 `V1_RELEASE_v1_21_REPORT.md` § frontend doc 章节)
- 文件名 16 份保持不变(违反 §0.2 #6 即 ABORT)
- 每份 family doc 顶部追加修订记号:

```markdown
> Revision: V1.1-A2 contract drift purge (2026-04-2x)
> Source: docs/api/openapi.yaml (post V1.1-A2)
> 与 v1.21 生产实际响应对齐
```

### §4.2 INDEX.md 更新

`docs/frontend/INDEX.md` 顶部追加:

```markdown
## 修订历史

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.21 release | 2026-04-25 | 首版 16 份 family 联调文档 |
| V1.1-A2 contract drift purge | 2026-04-2x | 全量对齐 v1.21 实际响应 schema · detail 接口由 30+ 富字段更正为 5 段精简 · 共 <N> 处 schema 修订 |
```

### §4.3 CHEATSHEET 更新

`docs/frontend/V1_API_CHEATSHEET.md` 顶部加红色提示:`⚠️ V1.1-A2 已校准,如有第三方文档与此处不一致,以本表为准`(如果用户禁用 emoji,改纯文本"重要")。

### §4.4 detail 接口典范段落

`docs/frontend/V1_API_TASKS.md` 中 `GET /v1/tasks/{id}/detail` 段落必须包含:

- 5 段顶层结构 + 字段表(用户已实测列出的版本)
- 一个真实 curl 示例 + 一段真实 JSON 响应裁剪样本
- 富字段(procurement_summary / product_selection 完整快照 / matched_rule_governance / design_assets / asset_versions / sku_items 等)的"请改用以下专用接口"对照表

### §4.5 P4 末尾校验

- frontend doc 全量字段集与 OpenAPI 对齐(spot check 抽 5 条 path 手工 diff)
- 16 文件名清单不变
- 每份 markdown 顶部都有修订记号

输出:`V1_1_A2_P4_FRONTEND_DOCS_REGENERATED`

---

## §5 阶段 P5 · 治理文档同步与签字

**目标**:更新 4 份治理文档,关闭 V1.1-A1 retro 中"contract debt"条目。

### §5.1 ROADMAP 追加

`prompts/V1_ROADMAP.md` §32 总览表追加 v33 / v34 / v35:

```markdown
| v33 | 2026-04-2x | OpenAPI contract drift purge | docs/api/openapi.yaml | <N> 个 schema 修订,detail 接口由 TaskDetail 富 schema 改为 TaskAggregateDetailV2 5 段精简 schema |
| v34 | 2026-04-2x | Frontend doc V1.1-A2 重生成 | docs/frontend/*.md | 全 16 份 family doc 对齐新 schema |
| v35 | 2026-04-2x | V1.1-A2 retro 关闭 | docs/iterations/V1_RETRO_REPORT.md | known debt "contract drift" 条目 → CLOSED |
```

### §5.2 V1_RETRO_REPORT 关闭 known debt

`docs/iterations/V1_RETRO_REPORT.md` 末尾追加 §X:

```markdown
## §X V1.1-A2 contract drift purge — CLOSED 2026-04-2x

V1.1-A1 retro 中"0 OpenAPI 改动"在 v1.21 release 后被用户实测发现是失守的:
detail 接口实际返回 5 段精简 schema,而 OpenAPI 仍声明老富 schema。本轮 V1.1-A2
全量回归 203 path,产出 drift inventory(<P0 N> + <P1 N> + <P2 N> + <defer N>),
按 code-wins 原则修订 OpenAPI,重生成 frontend doc 16 份,完成签字:
V1_1_A2_CONTRACT_DRIFT_PURGED。

详见:
- docs/iterations/V1_1_A2_DRIFT_INVENTORY.md
- docs/iterations/V1_1_A2_FIX_PLAN.md
- docs/iterations/V1_1_A2_RETRO_REPORT.md
```

### §5.3 落盘 V1.1-A2 retro 报告

`docs/iterations/V1_1_A2_RETRO_REPORT.md` 包含:

- §1 启动证据(v1.21 实测 detail 漂移 · 用户上报)
- §2 全量扫描结论(P0/P1/P2/defer 数量)
- §3 修订摘要(OpenAPI 行数变化 · schema 增/删/改 数量 · frontend doc 重生成统计)
- §4 验证矩阵 11 项结果(见 §6)
- §5 SHA 锚 baseline / final 对比表
- §6 已知遗留(defer-v12 列表 · 给 V1.2 立项参考)
- §7 待架构师独立 verify
- 不自签 PASS

### §5.4 handoff manifest 同步

`docs/V1_TO_V2_MODEL_HANDOFF_MANIFEST.md`:

- 头部"当前状态"由 `V1_RELEASE_v1_21_DONE_FRONTEND_DOCS_READY` 更新为 `V1_1_A2_CONTRACT_DRIFT_PURGED · 待架构师 verify`
- "已知契约 debt" 段落改写为"V1.1-A2 已收口 · 详见 V1_1_A2_RETRO_REPORT.md"

### §5.5 onboarding 同步

`prompts/V1_NEXT_MODEL_ONBOARDING.md`:

- "最近 release"段落追加 V1.1-A2 行
- "契约真实性"段落新增提示:OpenAPI 是 v1.21 + V1.1-A2 后的状态,与 v1.21 生产实际响应一致

### §5.6 frontend handoff 同步

`prompts/V1_1_FRONTEND_INTEGRATION_HANDOFF.md`:

- 顶部加"V1.1-A2 契约修订"通告:之前发布的 v1.21 frontend doc 已替换 · 前端实现如已开始按老 doc 编码 · 请按新 doc 校对(列出受影响最大的 5 条 P0 path)

### §5.7 P5 末尾

- `git status` 检查:仅 docs/* 与 prompts/* 改动,*.go 0 改动
- 计算 baseline_sha 组 A 的 final sha,与 §0.1.2 完全一致
- 所有 markdown md-lint(如配置)通过
- 输出最终签字:`V1_1_A2_CONTRACT_DRIFT_PURGED`(待 architect verify)

---

## §6 架构师 verify 矩阵(11 项 · codex 不可自签 · 必须 architect 跑)

| # | 检查项 | 期望 | 数据源 |
|---|---|---|---|
| 1 | 业务 sha 锚组 A(11 文件) | 与 §0.1.2 完全一致 · 0 漂移 | tmp/v1_1_a2/baseline_sha.log |
| 2 | OpenAPI path 数 | 203 | grep 计数 |
| 3 | OpenAPI validator | 0 error 0 warning | make openapi-validate |
| 4 | dangling 501 | 0 | grep -c |
| 5 | inventory 覆盖率 | 203/203 path | inventory 行数 |
| 6 | P0 / P1 全部 [done] | 100% | fix plan 状态 |
| 7 | go vet ./... | PASS | go vet |
| 8 | go build ./... | PASS | go build |
| 9 | frontend doc 文件数 | 16(或 inventory 中明确解释的数字) | ls docs/frontend |
| 10 | git status `*.go` 改动 | 0 文件 | git status |
| 11 | 抽样 5 条 P0 path 手工 diff | OpenAPI 与 handler struct 字段集 100% 一致 | 人工抽样 |

11 项全过 → architect 签字 PASS V1.1-A2。

---

## §7 ABORT 触发器(任意一条立即停手 + 写 ABORT 报告)

1. SHA 锚组 A 任一文件改动(违反 §0.2 #1)
2. 203 path 数变化(违反 §0.2 #2)
3. inventory 覆盖率 < 100%
4. OpenAPI validator 出现 error(warning 需 retro 中说明)
5. P3 中途任意 step 让 OpenAPI 无法 parse(swagger-cli 失败)
6. P4 frontend doc 文件名清单变化(违反 §0.2 #6)
7. P5 治理文档同步过程中发现 V1_ROADMAP / handoff 与现状已经偏离(说明前面 release 没归档,先回去补)
8. 整轮工作量明显超出 4 份治理文档 + 16 份 frontend doc + 1 份 OpenAPI 范围(说明 §1 inventory 漏项,需要回去补)
9. 任意 stage 跳过(不允许并行 P1+P3,必须串行)
10. inventory 中出现"无法定位 handler"的 path(说明 transport/http.go 与 OpenAPI 已经断裂,先解决路由本身,本轮废)

ABORT 报告位置:`docs/iterations/V1_1_A2_ABORT_REPORT.md`,包含:

- 触发的 ABORT 条款编号
- 当时已完成的 stage / inventory 行数
- 已落盘的中间产物清单(用于下一轮接手)
- 回滚步骤(`git checkout docs/api/openapi.yaml docs/frontend/*.md` · 治理文档保留以便接手)

---

## §8 中间符与最终终止符

| 阶段 | 中间符 |
|---|---|
| P1 完成 | `V1_1_A2_P1_INVENTORY_READY` |
| P2 完成 | `V1_1_A2_P2_PLAN_READY` |
| P3 完成 | `V1_1_A2_P3_OPENAPI_FIXED` |
| P4 完成 | `V1_1_A2_P4_FRONTEND_DOCS_REGENERATED` |
| P5 完成(待 verify) | `V1_1_A2_CONTRACT_DRIFT_PURGED` |

architect verify 通过后由架构师手动签字 `V1_1_A2_DONE_ARCHITECT_VERIFIED`。

---

## §9 你不该做的事(常见踩坑)

- **不要**把 `transport/handler/*.go` 当成主权威 —— 主权威是 `transport/http.go` + handler method 内部 respondOK 的实参类型 + service/struct 的 json tag
- **不要**默认 OpenAPI 的字段命名是对的 —— v1.21 fast path 引入了新 struct,struct json tag 才是命名权威
- **不要**对 v1.21 已经稳定的 RBAC / status code / parameters 段做"顺手优化",违反 §0.2 #5
- **不要**为了"补全 frontend 体验"在 OpenAPI 加 v1.21 实际不返回的字段(那是 V1.2 的事)
- **不要**在 inventory 还没完成时就进 P3 改 OpenAPI · 你会陷入"边扫边改边发现新漂移"的死循环
- **不要**自签 V1.1-A2 PASS · architect verify 是硬门
- **不要**在 ABORT 时半成品 commit · 全部 git checkout 回滚再写 ABORT 报告

---

## §10 回应格式(完成或 ABORT 时给架构师的报告)

成功时简报模板(贴到回复里):

```text
V1.1-A2 contract drift purge 已完成,等待架构师 verify · 未自签 PASS。

inventory:
- 总 path:203
- clean:<n>
- P0:<n>
- P1:<n>
- P2:<n>
- defer-v12:<n>

OpenAPI 修订:
- schema 新增:<n>
- schema 修改:<n>
- schema 删除:<n>
- 行数变化:+<n> / -<n>

Frontend doc:
- 16 份全量重生成
- INDEX 修订历史已追加 v1.21 / V1.1-A2

治理同步:
- ROADMAP v33/v34/v35 已追加
- V1_RETRO_REPORT §X 已关闭 known debt
- V1_TO_V2_MODEL_HANDOFF_MANIFEST 已同步
- V1_NEXT_MODEL_ONBOARDING 已同步
- V1_1_FRONTEND_INTEGRATION_HANDOFF 已通告

验证:
- 业务 sha 锚组 A 11 文件:0 漂移
- OpenAPI 203 path · validator 0/0
- dangling 501:0
- go vet / go build:PASS
- *.go git status:0 改动

落盘材料:
- docs/iterations/V1_1_A2_DRIFT_INVENTORY.md
- docs/iterations/V1_1_A2_FIX_PLAN.md
- docs/iterations/V1_1_A2_RETRO_REPORT.md
- docs/api/openapi.yaml(post V1.1-A2)
- docs/frontend/*.md(16 份)
- 4 份治理文档已同步

终止符:V1_1_A2_CONTRACT_DRIFT_PURGED
```

ABORT 时简报:

```text
V1.1-A2 已 ABORT,触发条款 §7.<N>。

已完成阶段:<P1/P2/...>
已落盘材料:<列出>
回滚状态:<git checkout 的文件清单>
ABORT 报告:docs/iterations/V1_1_A2_ABORT_REPORT.md
```

---

> 起草完毕。本 prompt 是一次性闭环 · 全量回归 · 不允许半成品交付 · 不允许跳过 architect verify。
