# V1.2-D · contract drift triage 整轮 · 处理 V1.2-C 工具暴露的 71 真实 drift + 66 unmapped

> 前置:V1.2-D-1(`task_detail.go` fallback 切除)已 ARCHITECT_VERIFIED · drift 已从 72 → 71 · `/v1/tasks/:id/detail` clean。
> 范围:剩余 71 drift + 66 unmapped 全量 triage,分 3 阶段(HIGH / MED / LOW)。
> 模式:**契约跟实现走**(V1.1-A2 同款) — handler 真实出现的字段优先 · OpenAPI 跟齐;handler 反推不到的 unmapped 用工具增强或显式 known_gap 注册。
> 终极目标:`drift = 0` 且 `unmapped` 仅剩有真实理由的 reserved/streaming/dynamic_payload。

## §0 输入(V1.2-D-1 完成后的真实仓 audit baseline · 架构师 2026-04-27 verify 确认)

```
total_paths = 242
clean       = 85   ← V1.2-D-1 +1
drift       = 71   ← V1.2-D-1 -1
  - only_in_code        = 39
  - only_in_openapi     = 2   ← GET /v1/tasks/:id + POST /v1/tasks/:id/close (TaskReadModel 字段空 · P2 处理)
  - both_diff           = 30
unmapped    = 66
  - unmapped_handler                = 35
  - unmapped_handler_dynamic_payload = 31
known_gap   = 20
```

> 起步前先跑 `go run ./tools/contract_audit ...` 把 baseline 落 `tmp/v1_2_d_baseline.json`,以工具实测数为准。文档数字已与 `tmp/v1_2_d_1_audit.json` 对齐,但仍以阶段起点工具实测为准(其他模型若先动了 OpenAPI/handler 数字会变)。

## §1 baseline 锚 SHA(P0 · 不一致 ABORT)

```
docs/api/openapi.yaml                          80730ec3d272e4124ab95244feb0c1daf499d4c0a032f47b70179cdd4189488f
                                               (V1.2-D-1 不改,这一轮要改)
transport/http.go                              9a6d194b54aa8d49dbff3d10f6d91283e07d68f21e117d2ffb9c2f99a72eb396
domain/task.go                                 658a8cdf65c09335ab74176efb4057eff68440537e50ce0d9e550c57413e6e6b
                                               (这一轮按需可改 - 但需架构师确认)
service/identity_service.go                    00ec340a81738a75a88d3b0d32d834b49879bea7df6ac1baa0eb1932d1d47644
service/task_aggregator/detail_aggregator.go   6e10c7e6d3f8096538015385fd317e94715a24568122159154538be17e347c7e
tools/contract_audit/main.go                   fc86c550622c3fcdbcd59beca8fe08e7a44b1fecd33c3c9f42dc116ac9f6455d
                                               (这一轮可在 P1 做工具增强)
```

V1.2-D-1 完成后 `transport/handler/task_detail.go` 应为新 SHA(由 V1.2-D-1 报告记录)· cmd/server/main.go 与 cmd/api/main.go 也应有新 SHA。

## §2 三阶段计划

```
P1  工具增强             ← 把假 drift 噪声去掉(anonymous embed / pagination wrap)
P2  HIGH drift triage    ← TaskReadModel 字段补全(2 path)
P3  MED drift triage     ← 跨家族 30 both_diff(契约跟实现走 · 含 auth/tasks/products/users/upload-sessions/categories 等)
P4  MED drift triage     ← pagination wrap 7 list 接口 only_in_code
P5  LOW drift triage     ← 35 unmapped_handler 真实改造(handler 走 respondOK 标准出口)
                         + 31 dynamic_payload 标注 acceptable known_gap
P6  整轮 audit + retro
```

每阶段独立 commit · 每阶段完成后跑 contract_audit · drift / unmapped 数字单调下降。

### P1 工具增强(`tools/contract_audit/main.go`)

#### P1.1 `BuildStructIndex` 支持 anonymous embedded struct 字段展开

当前 `structFields(st)` 只处理有 `field.Tag` 的字段 · 嵌入字段 `Task` 没有 tag 但 JSON 序列化时会平铺。

**修法**:
- 嵌入字段(field.Names == nil 即 anonymous embed)若没 `json:"name"` tag,递归展开嵌入类型的 fields。
- 嵌入字段若有 `json:"name"` tag(显式 nested),用 tag name 作为顶层 key。
- 跨包嵌入(如 `domain.Task` 在 `domain/query_views.go` 的 `TaskReadModel` 中)需要 `StructIndex.FieldsByType` 二次查找。

**测试**:`testdata/anonymous_embed/` 新 fixture · 验证 `TaskReadModel { Task; ReferenceFileRefs []ReferenceFileRef }` 平铺出 Task 全字段 + `reference_file_refs`。

#### P1.2 `responseFieldsExpanded` 支持 pagination wrapper schema

当前若 OpenAPI 顶层是 `{ data: [...], pagination: {...} }`(自定义 wrap),工具会把 pagination 字段当作 data 同级输出,与 handler `respondOKWithPagination(c, data, pagination)` 的 wrap 形态匹配。

**修法**:
- 检测 op response schema 顶层若同时有 `data` 与 `pagination` properties,把 `pagination` 字段也加入 openapiFields(以 `pagination.X` 形式)或与 handler `respondOKWithPagination` 的格式一致(`pagination` 内部展开)。
- 默认改成 handler 实测格式:openapiFields 仅 `data` 内字段(已现有逻辑) + `pagination`(单字段)。
- 让 `respondOKWithPagination` 调用方在 codeFields 同样产出 `pagination` 字段(handler 端不展开 pagination 内部)。

**测试**:`testdata/pagination_wrap/` fixture。

#### P1.3 `lastRespondExpr` 多出口归并

V1.2-C 用最后一个 respondOK,有多 fast-path/slow-path 出口的 handler 会单点反推。

**修法**:收集**所有** respondOK / respondCreated / respondOKWithPagination 出口,反推每个的类型,**取并集**作为 codeFields。多出口产出不一致字段时,verdict 加新值 `multi_exit_inconsistent` 并在 GapReport.Reason 里列出每个出口的类型。

**测试**:`testdata/multi_exit/`。

#### P1.4 跑工具自审

```powershell
go run ./tools/contract_audit --transport transport/http.go --handlers transport/handler --domain domain --openapi docs/api/openapi.yaml --output tmp/v1_2_d_p1_audit.json --markdown tmp/v1_2_d_p1_audit.md
```

**期望**:`summary.drift` 由 71 下降(噪声型 drift 被工具修复)· 实际数额视改造效果而定 · 落 `docs/iterations/V1_2_D_P1_TOOL_DELTA.md` 记录下降 N 条。

### P2 HIGH drift triage · `TaskReadModel` 字段补全

经 P1.1 工具增强后,`GET /v1/tasks/:id` 与 `POST /v1/tasks/:id/close` 应自动 clean(因为 `TaskReadModel { Task; ReferenceFileRefs }` 嵌入展开后 codeFields 与 OpenAPI 32 字段对齐)。

如果还残留 1~2 字段差(例如 `reference_file_refs` 类型不同),用契约跟实现走原则修 OpenAPI 让其对齐 handler。

**验证**:`tmp/v1_2_d_p2_audit.json` 中 `GET /v1/tasks/:id` 与 `POST /v1/tasks/:id/close` verdict = clean。

### P3 MED drift triage · 跨家族 30 both_diff

抽样(类目家族 + auth/tasks/products/users/upload-sessions 等):

```
GET /v1/categories         oc=11(全是 incident 字段·error)  oo=12(category 字段·正确)
GET /v1/categories/:id     oc=26(全是 task aggregate 字段·error)  oo=15(category 字段·正确)
GET /v1/category-mappings  同样模式
```

> 完整 30 条清单以 `tmp/v1_2_d_baseline.json` `paths[].verdict == both_diff` 为准。涉及家族:`/v1/auth/*`、`/v1/categories*`、`/v1/category-mappings*`、`/v1/products/*`、`/v1/users*`、`/v1/upload-sessions*` 与少量 tasks 域接口。

**根因诊断**(通过 P3.1 起步必做):oc 字段对不上时多数是工具反推到错误类型(handler 共享 generic 类型 / inferExprType 走偏);少数是真实漂移。

**P3.1 诊断**:对每条 30 both_diff 路径,检查 handler 实际 `respondOK(c, X)` 的 X 真实类型 · 如果不是工具反推的类型,这是工具 bug → 退回 P1 加强反推 · 否则是真实漂移 → 修 OpenAPI 跟齐。

**P3.2 修复**:契约跟实现走 — 修 `docs/api/openapi.yaml` 让 OpenAPI schema 与 handler 真实 struct 字段对齐。

**P3.3 frontend doc 同步**:被改的 OpenAPI path 对应的 frontend doc 段(`docs/frontend/V1_API_*.md` 中相应家族段)同步更新为新 schema。

**验证**:`tmp/v1_2_d_p3_audit.json` 30 条 both_diff 全 clean。

### P4 MED drift triage · pagination wrap 7 list 接口

抽样:

```
GET /v1/admin/jst-users    only_in_code = [count, current_page, datas, page_size, pages]
GET /v1/erp/users          同上
... 7 个共用 datas/pages/page_size/count/current_page wrap
```

**根因**:这 7 个接口 handler 用了 JSDT erp 系列的非标准 pagination wrap(`{ datas: [...], count: N, current_page: N, page_size: N, pages: N }`),而非 V1 标准 `respondOKWithPagination(c, items, paginationStruct)`。

**两条路径选一**(架构师决断 · 默认走 b):

(a) **handler 改成标准 wrap**:把 7 接口改用 `respondOKWithPagination`。需要确认前端是否依赖 `datas/pages/page_size/count/current_page` 字面 — 如果前端已上线绑定这些字段,**禁止改 handler**(会让前端崩),走 (b)。
(b) **OpenAPI 跟齐 handler 真实 wrap**:为这 7 接口在 OpenAPI 单独写 pagination wrap schema(`{ datas, count, current_page, page_size, pages }`),frontend doc 同步标注"该路径使用 ERP-style pagination wrap"。

**默认走 (b)** — 前端已上线 v1.21,任何 handler 改造可能破前端。

**P4.1 抽样实战**:让 codex 跑 `curl http://prod-or-staging /v1/admin/jst-users?...` 抓真实响应,确认 wrap 形态(确认是否是 datas 而非 data 等)。

**P4.2 修 OpenAPI**:加 7 个 wrap schema(或共享一个 `JstErpPaginationEnvelope` 公共 schema)。

**验证**:`tmp/v1_2_d_p4_audit.json` 7 接口全 clean。

### P5 LOW · 35 unmapped_handler 真实改造 + 31 dynamic_payload 注册 known_gap

#### P5.1 35 unmapped_handler 分类

`tmp/v1_2_d_baseline.json` `unmapped[]` 段(reason 字段):

```
29  no respondOK/respondCreated/respondOKWithPagination call
11  route handler is inline or middleware slice
10  reserved route handler                       ← intentional
10  dynamic payload                              ← gin.H{...}
 3  response expression type not inferred
 2  handler receiver type not resolved
 1  response struct not found: <name>
```

#### P5.2 处理矩阵

| 类别 | 数量 | 处理 |
|---|---:|---|
| reserved route handler | 10 | 注册 known_gap class=`reserved_route` · 不强制改 |
| dynamic payload (gin.H) | 10 | OpenAPI 单独写显式 schema · 注册 known_gap class=`dynamic_payload_documented` |
| inline / middleware slice | 11 | 把 inline handler 抽成命名方法,使工具可反推 · 视复杂度可拆 LOW-2 子轮 |
| no respondOK | 29 | 改成 respondOK(streaming/file download 类除外,这些注册 known_gap class=`stream_response`) |
| 反推不到(3+2+1=6) | 6 | 工具补丁(P1.5)或在 handler 加显式类型注解 |

#### P5.3 验证

`tmp/v1_2_d_p5_audit.json` 中 `unmapped[]` 数 ≤ 25(reserved 10 + stream/dynamic_documented 15) · 其余转 clean 或 known_gap。

### P6 整轮 audit + retro

#### P6.1 最终 audit

```powershell
go run ./tools/contract_audit --transport transport/http.go --handlers transport/handler --domain domain --openapi docs/api/openapi.yaml --output docs/iterations/V1_2_D_FINAL_AUDIT.json --markdown docs/iterations/V1_2_D_FINAL_AUDIT.md --fail-on-drift true
```

**期望**:exit 0(`drift = 0`)· `unmapped ≤ 25` · `clean ≥ 217`(242 - 25)。

#### P6.2 retro 报告

新建 `docs/iterations/V1_2_D_RETRO_REPORT.md`,含:

1. date / terminator / scope
2. baseline(V1.2-D-1 完成后)→ final(P6.1)delta 矩阵
3. P1 工具增强 4 项小结(行数 / 测试 / fixture)
4. P2 / P3 / P4 / P5 每阶段:
   - 修复路径列表(method+path)
   - 修复方式(handler 改 / OpenAPI 改 / frontend doc 改)
   - drift 与 unmapped 数额变化
5. OpenAPI before/after 行数 + sha256
6. frontend 16 doc before/after sha256
7. 业务 Go 文件 git diff 摘要(行数 / 函数 / risk)
8. 28 项 verify 矩阵结果
9. 已知 known_gap 25 条登记表(逐条标 class + reason)
10. terminator `V1_2_D_DRIFT_TRIAGED`

#### P6.3 治理同步

- `docs/V1_BACKEND_SOURCE_OF_TRUTH.md`:contract state 升级 `V1.2-D CLOSED · drift=0`
- `docs/iterations/V1_RETRO_REPORT.md` §18:把 HIGH/MED/MED/LOW 4 行全改 CLOSED · 引用 V1_2_D_RETRO_REPORT.md
- `prompts/V1_ROADMAP.md`:追加 v45~v50 6 行(P1~P6 各一行)

## §3 ABORT 触发(任意一项立即 ABORT)

| # | 触发条件 | 行为 |
|---|---|---|
| 1 | §1 任一锁定 SHA 在阶段开始时漂移 | ABORT |
| 2 | OpenAPI 改动后任一其他 path verdict 由 clean 退化为 drift(回归)| ABORT,回退 OpenAPI |
| 3 | handler 改造后 `go test ./... -count=1` 任一 unit/integration FAIL | ABORT |
| 4 | 任一阶段完成后 contract_audit `summary.drift` 比阶段起点更高 | ABORT(本阶段引入新 drift) |
| 5 | P3 修 OpenAPI 时未同步 frontend doc | ABORT |
| 6 | P6.1 最终 audit `summary.drift > 0` | ABORT(目标未达成 · 不允许签字) |
| 7 | P6.1 最终 audit `summary.unmapped > 25` | ABORT(known_gap 注册不全) |
| 8 | 修改 `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` 中 §1~§3 静态权威节(只允许改 §4 known debt 与 §1 contract state 状态行) | ABORT |
| 9 | 任何 `git push` | ABORT |

## §4 verify 矩阵(28 项)

| # | check | 期望 |
|---|---|---|
| 1 | §1 baseline SHA P0 | 0 漂移 |
| 2 | `go vet ./...` | exit 0 |
| 3 | `go build ./...` | exit 0 |
| 4 | `go test ./...` | PASS |
| 5 | `go test ./tools/contract_audit/... -count=1` | PASS(含 P1 新 fixture) |
| 6 | OpenAPI validator `go run ./cmd/tools/openapi-validate docs/api/openapi.yaml` | 0 error 0 warning |
| 7 | dangling 501 | 0 |
| 8 | 工具 P1.1 anonymous embed fixture | PASS |
| 9 | 工具 P1.2 pagination wrap fixture | PASS |
| 10 | 工具 P1.3 multi exit fixture | PASS |
| 11 | P2 `GET /v1/tasks/:id` verdict | clean |
| 12 | P2 `POST /v1/tasks/:id/close` verdict | clean |
| 13 | P3 跨家族 30 条 both_diff verdict | 全 clean |
| 14 | P4 7 个 list 接口 verdict | 全 clean |
| 15 | P5.2 reserved 10 条 known_gap class | reserved_route |
| 16 | P5.2 dynamic_payload 10 条 known_gap class | dynamic_payload_documented |
| 17 | P5.2 stream/file download known_gap class | stream_response |
| 18 | P6.1 最终 audit `summary.drift` | 0 |
| 19 | P6.1 最终 audit `summary.clean` | ≥ 217 |
| 20 | P6.1 最终 audit `summary.unmapped` | ≤ 25 |
| 21 | P6.1 `--fail-on-drift true` exit | 0 |
| 22 | OpenAPI 行数变化合理(≤ +500 行) | 视实际 |
| 23 | frontend 16 doc 全部加 V1.2-D revision marker | PASS |
| 24 | 业务 Go 文件 diff 仅命中 P5 列出的 inline → named handler 与 P5.4 streaming 标注 | PASS |
| 25 | retro 10 段完整 | PASS |
| 26 | V1 SoT contract state 已升级 | V1.2-D CLOSED |
| 27 | V1_RETRO §18 4 行全 CLOSED | PASS |
| 28 | V1_ROADMAP v45~v50 已追加 | PASS |

## §5 严禁

- ❌ `git push`
- ❌ 改 `service/identity_service.go`(本轮无关)
- ❌ 改 `service/task_aggregator/detail_aggregator.go`(V1.1-A1 fast-path · 已稳定)
- ❌ 改 V1.2-D-1 已修过的 `transport/handler/task_detail.go`
- ❌ 删除 `domain.TaskDetailAggregate`(其他 handler 在用)
- ❌ 改 `cmd/server/main.go` / `cmd/api/main.go` 中 `task_aggregator.NewDetailService` 构造逻辑
- ❌ 跳过 P1 直接干 P2~P5(工具不准导致后续阶段误报)
- ❌ 任何阶段不写 retro 段直接进下一阶段
- ❌ 把 known_gap 写成 "TODO" 而无 class+reason

## §6 终止符

完成后输出 `V1_2_D_DRIFT_TRIAGED` 加 28 项 verify 矩阵结果摘要。不自签 PASS,等架构师独立 verify。

架构师 verify 通过后改签 `V1_2_D_DONE_ARCHITECT_VERIFIED`,V1.2 整族(主+B+C+D)闭环,准入 V1.3 起草。
