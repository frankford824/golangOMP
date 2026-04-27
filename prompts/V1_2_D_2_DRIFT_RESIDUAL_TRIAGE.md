# V1.2-D-2 · contract drift 残留 triage · 收口 40 drift + 5 unmapped 至 `drift = 0`

> 前置:V1.2-D-1 ARCHITECT_VERIFIED · V1.2-D ABORT_ARCHITECT_VERIFIED(`drift 71→40 / unmapped 66→5 / clean 85→127`)。
> 范围:残留 40 drift + 5 unmapped + 70 known_gap class 校正。
> 模式:**工具增强优先 → 假 drift 自然消除 → 真 drift 契约跟实现走**。
> 终极目标:`drift = 0` · `unmapped ≤ 2` · `known_gap` 全部带具体 class label。

## §0 输入(`docs/iterations/V1_2_D_FINAL_AUDIT.json` 已落盘 · ground truth)

```text
total_paths           = 242
clean                 = 127
drift                 =  40   ← 本轮目标 = 0
  - clean_empty       =   6   ← 工具误判(P1.1 修)
  - documented_not_found = 14   ← 8 条非业务路由 + 6 条 path-param 漂移/真孤儿(P2/P3)
  - mounted_not_found =   6   ← 6 条 path-param 漂移/真孤儿(P2/P3)
  - only_in_code      =  38   ← 真实 OpenAPI 缺字段(P4 主战场)
  - only_in_openapi   =   2   ← 工具反推 bug(P1.2 修)
unmapped              =   5   ← inferExprType 边界(P1.3 修)
known_gap             =  70   ← 50 真 known_gap class 全 `unknown` 待校正 + 14 doc-not-found + 6 mounted-not-found
```

> **重要**:`known_gap=70` 是 summary 统计口径,实际 verdict 分布是 `verdict==known_gap` 50 条 + `documented_not_found` 14 条 + `mounted_not_found` 6 条,共 70。本轮处理时按 verdict 类别分别处理,不要按 summary 维度。

> 起步前先重跑 audit 落 `tmp/v1_2_d_2_baseline.json`,以工具实测数为准;若数字不匹配 §0,记录差异但继续(数字是否漂移由 §1 SHA 守门)。

## §1 baseline 锚 SHA(P0 · 不一致 ABORT)

### 1.1 业务核心 4 锚(本轮严禁改)

```text
transport/http.go                              9a6d194b54aa8d49dbff3d10f6d91283e07d68f21e117d2ffb9c2f99a72eb396
transport/handler/task_detail.go               704aaa07165996b2a3cf5681d823debd51f492e662e341d22b36041a60044df9
service/identity_service.go                    00ec340a81738a75a88d3b0d32d834b49879bea7df6ac1baa0eb1932d1d47644
service/task_aggregator/detail_aggregator.go   6e10c7e6d3f8096538015385fd317e94715a24568122159154538be17e347c7e
```

### 1.2 domain 3 锚(本轮严禁改)

```text
domain/task.go                                 658a8cdf65c09335ab74176efb4057eff68440537e50ce0d9e550c57413e6e6b
domain/task_detail_aggregate.go                315aef20dc7e34ad3233bf8f3e6bf8ae8e7477103586856d494a8c9e62bb82f0
domain/query_views.go                          7ee9817214464e60294f8bb47b51e6ab7c4cc314d12bc4ce50da56c8ac5af32a
```

### 1.3 V1.2-D 起点(本轮可在 P1/P3/P4 改)

```text
docs/api/openapi.yaml                          e2316b0292a6fee1cedb3631847977562c1811b37fdc30af284998b2dcb660de
tools/contract_audit/main.go                   7dad874a26f7f46f10d17adc02e4757688bdc752d1c0fcdf17a0cdc15a25342b
```

## §2 七阶段计划

```text
P1  工具补强             ← 消除假 drift(clean_empty / only_in_openapi / unmapped) + known_gap class registry
P2  path-param 命名归一   ← OpenAPI 跟齐 transport/http.go 真路径(:asset_id → :id)
P3  documented/mounted_not_found 非业务清理 + 真孤儿决断
P4  only_in_code 38 条 OpenAPI 字段补全(契约跟实现走)
P5  known_gap class 校正(50 条 unknown → 具体 class label)
P6  frontend doc 同步(P2/P4 涉及的 16 doc 段)
P7  整轮 audit + retro
```

每阶段独立 commit · 每阶段完成后跑 contract_audit · drift / unmapped / unknown_class 数字单调下降。

### P1 工具补强(`tools/contract_audit/main.go`)

#### P1.1 `clean_empty` 自动判 clean

当前规则:`code_fields == [] && openapi_fields == []` 仍被判 drift verdict=`clean_empty`。

**修法**:`code_fields == []` 且 `openapi_fields == []` 且 handler 真实存在 → 视为 clean(语义:204/no-content 类 mutation 接口)。

**已知影响 6 条**:全部转 clean。
```text
DELETE /v1/task-drafts/:draft_id
POST   /v1/me/notifications/read-all
POST   /v1/org-move-requests/:id/approve
POST   /v1/org-move-requests/:id/reject
POST   /v1/users/:id/activate
POST   /v1/users/:id/deactivate
```

**测试**:`testdata/clean_empty/` fixture · handler `respondOK(c, gin.H{})` 或 `c.Status(204)` + OpenAPI 仅描述无 schema/properties → verdict=clean。

#### P1.2 `only_in_openapi` 反推修正

当前 2 条 reverse 反推到错误类型:
- `GET /v1/org-move-requests` response_type 错推为 `TaskDraftListItem+pagination`(应该是 `OrgMoveRequest+pagination`)
- `GET /v1/users/designers` response_type 反推为 `designerItem+pagination`(可能 anonymous struct 没 export)

**根因诊断**:工具 `inferExprType` 在 handler 文件内多变量同名时取错绑定;或在 anonymous struct 时落 fallback 到上一个反推到的命名类型。

**修法**:
- handler-by-handler 反推绑定 scope 隔离(每个 handler function 独立 localTypes,函数结束后 reset);
- anonymous struct return 时直接展开匿名 struct 的字段(走 `*ast.StructType` 路径而不是 fallback 到 cached type name);
- 反推到的类型名必须在当前 handler package 的 import 范围内可解析,否则降级 unmapped。

**测试**:`testdata/only_in_openapi_reverse/` fixture · 多 handler 同名变量 + anonymous struct return。

#### P1.3 5 unmapped 根因解决

```text
3× response expression type not inferred
   GET  /v1/assets/search          handler=TaskAssetCenterHandler.SearchGlobalAssets
   GET  /v1/erp/products           handler=ERPBridgeHandler.SearchProducts
   GET  /v1/erp/sync-logs          handler=ERPBridgeHandler.ListSyncLogs
   POST /v1/agent/ack_job          handler=AgentHandler.AckJob
1× response struct not found: TaskDraftListItem
   GET  /v1/me/task-drafts         handler=TaskDraftHandler.MyList
```

**P1.3.1 工具补强**(优先):
- struct lookup 跨 package 搜索(目前只搜 `domain/`,扩展到 `service/` 与 handler 内 inline struct);
- `expression type not inferred` 加深一层:支持 `func() T { ... }()` IIFE / `service.X(...)` 直接 return / `&Foo{...}` 字面量。

**P1.3.2 handler type-hint**(若 P1.3.1 不够):在 5 个 handler 末尾加 `var resp *Type = result` 形式的显式 type-hint,与 V1.2-D-1 `task_detail.go:39` 同款。

> **重要**:**优先走 P1.3.1**(工具改造),只有工具反推确实不了才走 P1.3.2(改 handler)。改 handler 必须只加 type-hint,不改语义,且不动业务核心 4 锚以外的关键 handler。

#### P1.4 `known_gap` class registry

当前 `verdict='known_gap'` 的 50 条全部 `class=unknown` — P5 注册时未带 class label。

**修法**:工具加 `known_gap_class` 字段输出 + 注册规则:

| handler 形态 | known_gap_class |
|---|---|
| `transport/http.go v1R1ReservedHandler` 或类似 placeholder | `reserved_route` |
| handler 内 `respondOK(c, gin.H{...})` 而 OpenAPI 有显式 schema | `dynamic_payload_documented` |
| handler 走 `c.Stream` / `c.File` / `c.Data` 类 streaming/file response | `stream_response` |
| handler 用 SSE 或 long-polling | `streaming_response` |
| 其他无法分类 | `unclassified` |

**测试**:`testdata/known_gap_class/` 4 fixture 各 1 例。

#### P1.5 跑工具自审

```powershell
go run ./tools/contract_audit `
  --transport transport/http.go `
  --handlers transport/handler `
  --domain domain `
  --openapi docs/api/openapi.yaml `
  --output tmp/v1_2_d_2_p1_audit.json `
  --markdown tmp/v1_2_d_2_p1_audit.md
```

**期望**:`drift` 由 40 下降至 ≤ 30(消减 6 clean_empty + 2 only_in_openapi + ~3 unmapped 转 clean)· `unmapped ≤ 2` · `known_gap` 仍 ~70 但 `class=unknown` 数应大幅下降。

落 `docs/iterations/V1_2_D_2_P1_TOOL_DELTA.md` 记录每项工具改动 + drift/unmapped delta。

### P2 path-param 命名归一(OpenAPI 跟齐 handler 真路径)

#### P2.1 完整漂移清单(8 条 = 4 endpoint × 2 verdict)

```text
documented_not_found ↔ mounted_not_found 配对:
  DELETE /v1/assets/:asset_id  (OpenAPI)  ↔  DELETE /v1/assets/:id  (handler)
  GET    /v1/assets/:asset_id              ↔  GET    /v1/assets/:id
  GET    /v1/assets/:asset_id/download     ↔  GET    /v1/assets/:id/download
  GET    /v1/assets/:asset_id/preview      ↔  GET    /v1/assets/:id/preview
```

#### P2.2 决断方向(架构师已定 · `:asset_id` → `:id`)

**理由**:
- `transport/http.go` SHA `9a6d194b...` 锁定不能动(prompt §1.1)。
- handler 真路径用 `:id`,frontend 实际调用走 handler 真路径(curl 时 URL 用 `:id`)。
- frontend doc 若已写 `:asset_id` 是历史 OpenAPI 文档错误,本轮一并修正(P6 同步)。

**操作**:
- `docs/api/openapi.yaml` 中 `/v1/assets/{asset_id}*` 4 个 path 名改为 `/v1/assets/{id}*`,parameter `asset_id` → `id`(name + in:path)。
- 对应 schema/responses 字段中的 `asset_id` 引用保持(因为 response body 里 `asset_id` 字段名是合理的,只是 URL param 改名)。

**期望**:audit 里 4 条 documented_not_found + 4 条 mounted_not_found 全部转 clean(8 条)。

#### P2.3 `frontend/V1_API_*.md` 同步(P6 处理)

记录待改 frontend doc 段名(P6 时执行):
- `docs/frontend/V1_API_ASSETS.md` 4 段中 `:asset_id` → `:id`(URL 与 curl 例子)。

### P3 documented/mounted_not_found 非业务路由清理 + 真孤儿决断

P2 处理后剩 `documented_not_found` 10 条 + `mounted_not_found` 2 条 = 12 条。

#### P3.1 非业务路由(从 OpenAPI 删 + 工具加排除前缀)

```text
documented_not_found:
  GET /health      (cmd/server 主进程级 · 不属业务 contract)
  GET /healthz     (同上)
  GET /ping        (同上)
mounted_not_found:
  GET /internal/jst/ping
  POST /jst/sync/inc
```

**操作**:
- 删 OpenAPI `/health` `/healthz` `/ping` 三条 path entry。
- 不删 transport/http.go 的 `/internal/jst/ping` 与 `/jst/sync/inc`(锁定不能改),改用工具排除前缀:`tools/contract_audit/main.go` 加 ignore prefixes `/health`、`/healthz`、`/ping`、`/internal/`、`/jst/`(只豁免 audit · 不影响实际路由)。

**期望**:5 条转 clean(audit 不再列入 paths)或 known_gap class=`infra_route`。

#### P3.2 真业务孤儿决断(7 条)

```text
documented_not_found · 真业务接口:
  GET    /v1/rule-templates
  GET    /v1/rule-templates/:type
  PUT    /v1/rule-templates/:type
  POST   /v1/incidents/:id/assign
  POST   /v1/incidents/:id/resolve
  POST   /v1/sku/preview_code
  PUT    /v1/policies/:id
```

**逐条诊断**:
对每条 path,grep `transport/http.go` 与 `transport/handler/*.go` 确认 handler 是否真实存在:

(a) 若 handler 存在但路由没挂 → ABORT 报告标"需补 transport/http.go"(留续轮 V1.2-D-3 处理 · 不动 transport/http.go SHA);
(b) 若 handler 不存在 → OpenAPI 删该 path entry,frontend doc 同步删段(P6);
(c) 若 handler 是 `v1R1ReservedHandler` 或 placeholder → OpenAPI 保留,工具注册 known_gap class=`reserved_endpoint`。

**期望**:7 条全部决断,记入 `docs/iterations/V1_2_D_2_P3_2_DECISIONS.md`。

#### P3.3 跑 audit

```powershell
go run ./tools/contract_audit ... --output tmp/v1_2_d_2_p3_audit.json
```

**期望**:drift 下降至 ≤ 18(P1 后 ~30 - P2 -8 - P3.1 -5)。

### P4 only_in_code 38 条 OpenAPI 字段补全

#### P4.1 分批策略

按 family 分 5 批:

| 批 | family | path 数 | 估计字段总数 |
|---|---|---:|---:|
| B1 | tasks 主接口 | 9 | ~150 |
| B2 | sku/agent/audit | 8 | ~30 |
| B3 | assets/upload-requests | 4 | ~50 |
| B4 | categories/cost-rules/integration | 7 | ~100 |
| B5 | export-jobs/auth/admin/policies/incidents | 10 | ~80 |

每批一个 commit · 每批完成跑 audit 验证 `drift` 单调下降 + 不引入新 only_in_openapi。

#### P4.2 操作模板

对每条 path:

1. 读 handler 真实 `respondOK(c, X)` 的 X 类型(audit json `response_type` 字段);
2. 读 X struct 的 json tag 列表(audit json `code_fields` 字段);
3. 修 `docs/api/openapi.yaml` 该 path response schema:
   - 若 schema 直接 inline 字段,补缺失字段;
   - 若 schema 引用 `$ref`(component),修对应 component 加缺失字段;
4. 字段类型从 Go struct 映射(string/int/float/bool/array/object 标准 + `format: date-time` 类 timestamp 字段);
5. 字段 description 留 TODO(本轮不强制写完整中文描述,允许 `description: ""` 占位);
6. 跑 `openapi-validate` 确认 0 error 0 warning。

#### P4.3 高字段数大 path 特别处理

```text
46+ 字段:
  GET  /v1/tasks            (TaskListItem+pagination · 46 字段)
  POST /v1/tasks            (TaskReadModel · 58 字段)
  POST /v1/integration/call-logs                (36 字段)
  POST /v1/export-jobs                          (56 字段)
  POST /v1/cost-rules                           (31 字段)
  POST /v1/assets/upload-requests               (42 字段)
  POST /v1/tasks/:id/assets/mock-upload         (33 字段)
```

这 7 条单独单独 commit(每条一个 commit) · 风险高需独立验证。

#### P4.4 跑 audit

```powershell
go run ./tools/contract_audit ... --output tmp/v1_2_d_2_p4_audit.json
```

**期望**:drift 下降至 ≤ 5(P3 后 ~18 - P4 -38 但可能因新 only_in_openapi 反弹至 ≤ 5);若 only_in_openapi 反弹超 5 条 → ABORT。

### P5 known_gap class 校正(50 条 unknown → 具体 label)

P1.4 工具加了 class registry,本阶段验证生效:

```powershell
go run ./tools/contract_audit ... --output tmp/v1_2_d_2_p5_audit.json
```

抽样 known_gap 记录,确认每条都有具体 class:`reserved_route` / `dynamic_payload_documented` / `stream_response` / `streaming_response` / `infra_route` / `reserved_endpoint`。

**期望**:`class=unknown` 数 = 0;若仍有 unknown,逐条诊断决定加新 class 或转 clean。

### P6 frontend doc 同步

P2/P3.2/P4 涉及的 OpenAPI path 反查 16 doc:

```powershell
$changedPaths = ...  # 从 P2/P3/P4 commit diff 提取
ForEach ($path in $changedPaths) {
  # grep frontend doc 找哪份文档涉及该 path
  Select-String -Path docs/frontend/V1_API_*.md -Pattern $path
}
```

对每份涉及的 frontend doc:
- 添加 `> Updated 2026-04-27 V1.2-D-2 contract residual triage`;
- 修订段内 path 名(P2 的 `:asset_id` → `:id`)与 response schema(P4 字段补全 → 文档 JSON 样本与 field 表格补全)。

### P7 整轮 audit + retro

#### P7.1 最终 audit

```powershell
go run ./tools/contract_audit `
  --transport transport/http.go `
  --handlers transport/handler `
  --domain domain `
  --openapi docs/api/openapi.yaml `
  --output docs/iterations/V1_2_D_2_FINAL_AUDIT.json `
  --markdown docs/iterations/V1_2_D_2_FINAL_AUDIT.md `
  --fail-on-drift true
```

**期望**:exit 0(`drift = 0`)· `unmapped ≤ 2` · `clean ≥ 220` · `known_gap.class != unknown` 全条目带 class。

#### P7.2 retro 报告

新建 `docs/iterations/V1_2_D_2_RETRO_REPORT.md`:

1. date / terminator / scope
2. baseline (V1.2-D final) → final (P7.1) delta 矩阵
3. P1 工具补强 4 项小结(行数 / fixture / drift delta)
4. P2 path-param 归一(8 path · OpenAPI diff)
5. P3 非业务清理 5 + 真孤儿决断 7(逐条决断结果)
6. P4 only_in_code 38 条字段补全(5 批 commit · OpenAPI diff 行数)
7. P5 known_gap class 校正(50 条 unknown → class 分布)
8. P6 frontend doc 同步(16 doc 哪几份改 / sha256 before/after)
9. OpenAPI before/after 行数 + sha256
10. 业务 Go diff(只允许 P1.3.2 的 type-hint;非 type-hint 改动 ABORT)
11. 32 项 verify 矩阵结果
12. terminator `V1_2_D_2_DRIFT_FULLY_TRIAGED`

#### P7.3 治理同步

- `docs/V1_BACKEND_SOURCE_OF_TRUTH.md`:contract state 升级 `V1.2-D-2 CLOSED · drift=0`;
- `docs/iterations/V1_RETRO_REPORT.md` §18:V1.2-D HIGH OPEN 行改 CLOSED + 引用 V1_2_D_2_RETRO_REPORT.md;
- `prompts/V1_ROADMAP.md`:追加 v47~v53 7 行(P1~P7 各一行)。

## §3 ABORT 触发(任意一项立即 ABORT)

| # | 触发条件 | 行为 |
|---|---|---|
| 1 | §1.1 业务核心 4 锚 SHA 漂移 | ABORT |
| 2 | §1.2 domain 3 锚 SHA 漂移 | ABORT |
| 3 | OpenAPI 改动后任一其他 path verdict 由 clean 退化为 drift | ABORT,回退 OpenAPI |
| 4 | handler 改造后 `go test ./... -count=1` 任一 unit/integration FAIL | ABORT |
| 5 | 任一阶段完成后 contract_audit `summary.drift` 比阶段起点更高 | ABORT(本阶段引入新 drift) |
| 6 | P4 中任一 path 改 OpenAPI 后产生新 `only_in_openapi`(过补)| ABORT |
| 7 | P5 后仍有 `known_gap.class==unknown` | ABORT |
| 8 | P7.1 最终 audit `summary.drift > 0` | ABORT |
| 9 | P7.1 最终 audit `summary.unmapped > 2` | ABORT |
| 10 | P1.3.2 在 §1.1 锁定 handler(`task_detail.go` 等)上加 type-hint | ABORT(只能在非锁定 handler 加) |
| 11 | 把 `clean_empty` / `only_in_openapi` / `documented_not_found` 简单注册为 known_gap 而不真实修复(掩盖型 known_gap)| ABORT |
| 12 | 修改 `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` §1~§3 静态权威节(只允许改 §4 已知债务与 contract state 状态行) | ABORT |
| 13 | 任何 `git push` | ABORT |
| 14 | 修改 frontend doc 任一份的非 P6 段(P6 只改 P2/P3/P4 涉及的段)| ABORT |

## §4 verify 矩阵(32 项)

| # | check | 期望 |
|---|---|---|
| 1 | §1.1 业务核心 4 锚 | 0 漂移 |
| 2 | §1.2 domain 3 锚 | 0 漂移 |
| 3 | `go vet ./...` | exit 0 |
| 4 | `go build ./...` | exit 0 |
| 5 | `go test ./...` -count=1 | PASS |
| 6 | `go test ./tools/contract_audit/...` -count=1 -race | PASS(含 P1 新 4 fixture) |
| 7 | OpenAPI validator | 0 error 0 warning |
| 8 | dangling 501 | 0 |
| 9 | P1.1 clean_empty fixture | PASS(6 path 转 clean) |
| 10 | P1.2 only_in_openapi fixture | PASS(2 path 转 clean) |
| 11 | P1.3 unmapped 5 解决 | PASS(unmapped ≤ 2) |
| 12 | P1.4 known_gap class registry fixture | PASS |
| 13 | P2 8 path verdict | 全 clean |
| 14 | P3.1 非业务 5 path | 转 clean 或 known_gap class=`infra_route` |
| 15 | P3.2 真孤儿 7 path | 全部决断写入 P3_2_DECISIONS.md |
| 16 | P4 5 批 commit `drift` 单调下降 | PASS |
| 17 | P4.3 7 大字段 path 单独 commit | PASS |
| 18 | P5 `class=unknown` 数 | 0 |
| 19 | P6 frontend doc 同步 | PASS(每个 P2/P3/P4 涉及 path 都对得上) |
| 20 | P7.1 最终 audit `summary.drift` | 0 |
| 21 | P7.1 最终 audit `summary.clean` | ≥ 220 |
| 22 | P7.1 最终 audit `summary.unmapped` | ≤ 2 |
| 23 | P7.1 `--fail-on-drift true` exit | 0 |
| 24 | P7.1 known_gap 全条目带具体 class | PASS |
| 25 | OpenAPI 行数变化合理(+200~+800)| 视实际 |
| 26 | frontend 16 doc P6 涉及段加 V1.2-D-2 marker | PASS |
| 27 | 业务 Go 文件 diff:**至多** 5 个 handler 加 type-hint(P1.3.2) | PASS |
| 28 | retro 12 段完整 | PASS |
| 29 | V1 SoT contract state 升级 | V1.2-D-2 CLOSED |
| 30 | V1_RETRO §18 HIGH OPEN 行 CLOSED | PASS |
| 31 | V1_ROADMAP v47~v53 7 行已追加 | PASS |
| 32 | `tmp/v1_2_d_2_*.json` audit 工件全落盘(P1/P3/P4/P5 各一)| PASS |

## §5 严禁

- ❌ `git push`
- ❌ 改 §1.1 业务核心 4 锚(transport/http.go / task_detail.go / identity / detail_aggregator)的非 type-hint 行为
- ❌ 改 §1.2 domain 3 锚(任一字段/struct)
- ❌ 删 `domain.TaskDetailAggregate`(其他 handler 在用)
- ❌ 改 `cmd/server/main.go` / `cmd/api/main.go`(本轮无关)
- ❌ 跳过 P1 直接干 P2~P5(工具不准导致后续阶段误报)
- ❌ 任何阶段不写小结 delta 段直接进下一阶段
- ❌ 把 `clean_empty` / `only_in_openapi` / `documented_not_found` 简单注册为 known_gap 而不真实修复
- ❌ P1.3.2 type-hint 改 handler 业务语义(只能加 `var x *Type = result` 一行)
- ❌ P4 加新字段时改 component 中其他 path 已用的字段类型(隐式 regression)
- ❌ 非 P6 范围动 frontend doc

## §6 终止符

完成后输出 `V1_2_D_2_DRIFT_FULLY_TRIAGED` 加 32 项 verify 矩阵结果摘要。不自签 PASS,等架构师独立 verify。

架构师 verify 通过后改签 `V1_2_D_2_DONE_ARCHITECT_VERIFIED`,V1.2 整族(主 + B + C + D + D-1 + D-2)闭环,准入 V1.3 起草。

## §7 执行顺序快查

```text
P0  baseline SHA 校验(§1)             → 不通过 ABORT
P1  工具补强(4 子项)+ 自审 audit       → drift 40→≤30, unmapped 5→≤2
P2  path-param 归一(8 path)           → drift ≤30→≤22
P3.1 非业务路由清理(5 path)            → drift ≤22→≤17
P3.2 真孤儿决断(7 path)               → 决断写报告
P4   only_in_code 字段补全(38 path · 5 批) → drift ≤17→≤5
P5   known_gap class 校正(50 unknown→class) → unknown=0
P6   frontend doc 同步(P2/P3/P4 涉及段)
P7   最终 audit + retro + 治理同步     → drift=0, unmapped≤2
```
