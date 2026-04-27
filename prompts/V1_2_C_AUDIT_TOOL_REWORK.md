## V1.2-C · contract_audit 工具回炉重写 · 真实三向 diff 引擎落地

> 起草人:架构师(Claude Opus 4.7)
> 起草时间:2026-04-26 PT
> 性质:**codex TUI / codex exec autopilot 单 prompt** · 单一目标 · 强 ABORT 守卫
> 前置:`V1_2_PARTIAL_PASS_AUDIT_TOOL_REWORK_REQUIRED`(架构师裁决 · 2026-04-26)
> 接续来源:`prompts/V1_2_RESUME_FROM_P2.md`(V1.2 主体 PARTIAL PASS)
> 终止符(成功):`V1_2_C_AUDIT_TOOL_REWORKED`
> 范围严格收窄:**只动 `tools/contract_audit/` 目录** · 不动 OpenAPI / 不动业务 Go 代码 / 不动 frontend doc / 不动治理文档

---

## §0 为什么要回炉(必读)

### §0.1 V1.2 P4 实质问题

架构师独立 verify 后发现 `tools/contract_audit/main.go` 是空壳工具:

```99:126:tools/contract_audit/main.go
	sort.Slice(ops, func(i, j int) bool {
		if ops[i].Path == ops[j].Path {
			return ops[i].Method < ops[j].Method
		}
		return ops[i].Path < ops[j].Path
	})
	paths := make([]PathReport, 0, len(ops))
	for _, op := range ops {
		fields := op.Fields
		paths = append(paths, PathReport{
			Method:        op.Method,
			Path:          op.Path,
			Handler:       "transport/http.go",
			ResponseType:  "openapi.response.200",
			CodeFields:    append([]string(nil), fields...),
			OpenAPIFields: append([]string(nil), fields...),
			Verdict:       "clean",
		})
	}
	return Report{
		Version:         "v1.2",
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
		OpenAPISHA256:   fileSHA(openapiPath),
		TransportSHA256: fileSHA(transport),
		Summary: Summary{
			TotalPaths: len(paths),
			Clean:      len(paths),
		},
```

致命缺陷:

1. **L102-114** `op.Fields` 来自 OpenAPI yaml 抽取,`CodeFields` 与 `OpenAPIFields` 同源 → diff 永远空 · `Verdict` 写死 `"clean"`
2. **L62-63** `_ = handlers / _ = domain` · `--handlers` `--domain` flag 永远丢弃 · transport AST / handler AST / domain struct json tag 从未被读取
3. **`Summary.Drift` 字段从未赋值** · 默认 0 · `--fail-on-drift true` 永远 exit 0
4. `main_test.go` 三个测试只覆盖孤立 `StructJSONFields` / `DiffFields`,从未覆盖 `BuildReport` 主流程

后果:`summary.drift=0` 是数学保证,不是 audit 结论。CI hook 的 drift seed 仅 code-only-changed 路径有效,字段不匹配路径完全失守。V1.1-A2 Q-1 仍 OPEN,转本轮(V1.2-C-1)。

### §0.2 V1.2-C 必达目标

把 main 流程改造为真实 4 段流水:

```
transport/http.go         (AST: 抽 mount → handler 函数全名映射)
        ↓
transport/handler/*.go    (AST: 抽 handler 函数体内 respondOK/respondCreated/respondOKWithPagination 第二参数 → 反推 Go 类型)
        ↓
domain/*.go + service/.../*.go   (AST: 抽该 Go 类型的 json tag 顶层字段集)
        ↓
docs/api/openapi.yaml     (yaml: 抽 path/method 200 响应展开 schema 顶层字段集 · 处理 $ref 闭包)
        ↓
真三向 diff → Verdict (clean / only_in_code / only_in_openapi / both_diff / unmapped_handler) → Summary.Drift 真实计数
```

无法启发式反推的 handler 标 `unmapped_handler` 不计 drift,**单独计入 `summary.unmapped` 段并显式列出**,供后续手工 review。

---

## §1 ABORT 触发(任一命中立即停 · 写 ABORT 报告 · 不签终止符)

| # | 条件 |
|---|---|
| 1 | 业务 SHA 锚组 A(下方 §2 表 10 文件)任意 1 文件 sha 漂移 |
| 2 | `docs/api/openapi.yaml` sha 漂移(必须保持 `80730ec3d272e4124ab95244feb0c1daf499d4c0a032f47b70179cdd4189488f`) |
| 3 | `transport/http.go` sha 漂移(必须保持 `9a6d194b54aa8d49dbff3d10f6d91283e07d68f21e117d2ffb9c2f99a72eb396`) |
| 4 | `domain/` 目录 任意 `.go` 文件被改动 |
| 5 | `transport/handler/` 目录 任意 `.go` 文件被改动 |
| 6 | `service/` 目录 任意 `.go` 文件被改动 |
| 7 | `docs/frontend/*.md` 任意 1 文件被改动 |
| 8 | 治理 4 件套(`docs/V1_BACKEND_SOURCE_OF_TRUTH.md` / `docs/V1_TO_V2_MODEL_HANDOFF_MANIFEST.md` / `prompts/V1_NEXT_MODEL_ONBOARDING.md` / `prompts/V1_1_FRONTEND_INTEGRATION_HANDOFF.md`)任意 1 被改动 |
| 9 | `tools/contract_audit/main.go` rework 后仍含 `_ = handlers` / `_ = domain` 死赋值 |
| 10 | `tools/contract_audit/main.go` rework 后仍把 OpenAPI 抽出的字段同时赋给 `CodeFields` 与 `OpenAPIFields` |
| 11 | `tools/contract_audit/main_test.go` 不含主流程集成测试(必须真调用 `BuildReport` 并断言 `Summary.Drift > 0` 在 testdata phantom 注入场景) |
| 12 | 工具针对 §6.4 `TestMainFlow_FieldDriftSeed` 的 phantom 注入 testdata 不报 `drift > 0` |
| 13 | 工具针对真实仓库跑(主仓 transport + OpenAPI)输出 `summary.unmapped > 0.5 * mounted 总数` |
| 14 | `go vet ./...` / `go build ./...` / `go test ./...`(尤其 `./tools/contract_audit/...`)任意 1 失败 |

---

## §2 业务 SHA 锚组 A · 必须 0 漂移

| 文件 | 必须 sha (lower hex) |
|---|---|
| `service/asset_lifecycle/cleanup_job.go` | (沿用 V1.1-A2 baseline · codex P0 校验时锁定) |
| `service/task_draft/service.go` | (沿用 V1.1-A2 baseline) |
| `service/task_lifecycle/auto_archive_job.go` | (沿用 V1.1-A2 baseline) |
| `repo/mysql/task_auto_archive_repo.go` | (沿用 V1.1-A2 baseline) |
| `domain/task.go` | `658a8cdf65c09335ab74176efb4057eff68440537e50ce0d9e550c57413e6e6b` |
| `cmd/server/main.go` | (沿用 V1.1-A2 baseline) |
| `service/task_aggregator/detail_aggregator.go` | (沿用 V1.1-A1 baseline) |
| `repo/mysql/task_detail_bundle.go` | (沿用 V1.1-A1 baseline) |
| `service/identity_service.go` | `00ec340a81738a75a88d3b0d32d834b49879bea7df6ac1baa0eb1932d1d47644` |
| `repo/mysql/identity_actor_bundle.go` | (沿用 V1.1-A1 baseline) |
| `transport/http.go` | `9a6d194b54aa8d49dbff3d10f6d91283e07d68f21e117d2ffb9c2f99a72eb396` |
| `docs/api/openapi.yaml` | `80730ec3d272e4124ab95244feb0c1daf499d4c0a032f47b70179cdd4189488f` |

P0 step-0 必须用 `Get-FileHash` 或 `sha256sum` 全部重算并打印,任意一项不一致 → ABORT。

---

## §3 已知事实(P0 之前可读 · 不必再算)

- transport 风格:`group.METHOD("/path", middleware..., handler.Method)` · 最后一个 arg 是 handler 函数引用 · 形如 `tasksH.GetByTaskID` / `authH.Login`
- handler OK 出口统一三个函数:`respondOK(c, data)` / `respondCreated(c, data)` / `respondOKWithPagination(c, data, pagination)` · 见 `transport/handler/response.go` L11-24
- `respondOK` 包装为 `{"data": data}` · OpenAPI schema 顶层永远有 `data` 子段 · 工具应该在 OpenAPI 侧抽 `responses.200.content.application/json.schema.properties.data.properties.*` 顶层 keys(沿用 V1.2 原有 `responseFields` 函数,见 `tools/contract_audit/main.go` L157-170)
- `respondOKWithPagination` 顶层是 `data + pagination` · 工具应识别这种 wrap 并取 `data` 段字段做对账,`pagination` 段字段交给 verdict 但不计 drift
- domain struct json tag 已有现成抽取器 `StructJSONFields`(`tools/contract_audit/main.go` L172-208)· 直接复用
- domain 目录文件清单 ≈72 文件 · handler 目录 ≈73 文件(含 *_test.go)· 工具不应解析 *_test.go

---

## §4 阶段 P1~P5 顺序执行(任一阶段失败立即 ABORT)

### §4.1 P1 baseline 校验

1. 拉 §2 表 12 文件实际 sha · 与 baseline 对比 · 任一漂移 → ABORT
2. 不动任何文件,只读

### §4.2 P2 transport AST 解析(替换 `OpenAPIOperations` 流程)

新增函数 `ParseTransportRoutes(transportPath string) ([]Route, error)`:

```go
type Route struct {
    Method      string  // GET / POST / PUT / PATCH / DELETE
    Path        string  // 完整路径,例如 /v1/tasks/:id/detail (gin 风格)
    HandlerExpr string  // handler 引用表达式 · 例如 "tasksH.GetByTaskID"
    Mount       string  // 文件:行,例如 "transport/http.go:287"
}
```

实现要求:

- 用 `go/parser.ParseFile` 解析 `transport/http.go` 抽 `*ast.CallExpr`
- 识别形如 `<receiver>.<METHOD>(<path>, <middlewares...>, <handler>)` 的调用
- METHOD ∈ {GET, POST, PUT, PATCH, DELETE, HEAD}
- 第一参数必须是字符串字面量 → 取 path
- 最后一参数是 `*ast.SelectorExpr` 形如 `tasksH.GetByTaskID` → 取 receiver + 方法名
- 中间参数(middlewares)忽略
- group 路径前缀:识别 `g := v1.Group("/foo")` 这种 var 赋值,后续 `g.GET("/bar", ...)` 拼接为 `/v1/foo/bar`
- 注意 V1 主仓使用嵌套 group + 直挂混合 · 把所有路径标准化为 gin `:param` 风格(不要转 OpenAPI `{param}`,留给字段对账层做归一)

约束:必须能枚举至少 200 条 mount(主仓 OpenAPI documented 是 206 path)· 否则 ABORT。

### §4.3 P3 handler AST 解析(handler 函数 → 响应类型)

新增函数 `ResolveHandlerResponseType(handlerDir string, route Route, fset *token.FileSet) (typeName string, pkg string, verdict string)`:

实现要求:

- 解析 `transport/handler/*.go`(跳过 `*_test.go`)
- 把 handler 函数全名(`tasksH.GetByTaskID`)映射到具体 `*ast.FuncDecl`(receiver type 找包级 type,方法名找方法)
- 在 FuncDecl 体内找 **所有** `respondOK(c, X)` / `respondCreated(c, X)` / `respondOKWithPagination(c, X, _)` 调用
  - 多个 OK 出口:取**最后一个**(顺序遍历 body)
- 反推 X 表达式的类型(启发式):
  - X 是 `&Foo{...}` → `Foo`
  - X 是 `Foo{...}` → `Foo`
  - X 是 ident `x` → 在函数局部找最近的 `var x Foo` / `x := ...` / `x, _ := svc.Method(...)` · 后者通过 `svc.Method` 的返回类型签名(查同包或 import 包的 type 定义)
  - X 是 `make(map[string]any, ...)` 或 `gin.H{...}` → 标 `verdict = "unmapped_handler_dynamic_payload"` 跳过
  - 反推失败 → `verdict = "unmapped_handler"` 并记录原因
- 返回 typeName 时一并返回 pkg(import 路径,例如 `workflow/service/task_aggregator.AggregateDetail`)

不要追求完美类型推断 · 启发式覆盖率达到 50% 即可 · 剩余进 `unmapped_handler` 段。

约束:`summary.unmapped < 0.5 * mounted_total` 否则 ABORT(条款 §1 #13)。

### §4.4 P4 字段抽取与归一

字段抽取两侧:

A. **code 侧** · `ResolveHandlerResponseType` 拿到 `(typeName, pkg)` 后:

   - 在 `domain/`、`service/`(递归)、`transport/handler/`(同包定义)中查 `type Foo struct {...}` 定义
   - 复用 `StructJSONFields` 抽顶层 json tag 字段名(忽略嵌入字段的递归展开 · V1.2-C 阶段只对账顶层)
   - tag `-` / 空 / 无 tag 不计入
   - tag `omitempty` 截断后取字段名

B. **OpenAPI 侧** · 复用现有 `responseFields` 函数(`main.go` L157-170)抽 `responses.200.content.application/json.schema.properties.data.properties.*` 顶层 keys
   - 但需要扩展:支持 schema 是 `$ref` 时去 `components.schemas[name]` 展开
   - 支持 `allOf` 合并(取并集)
   - 支持 `data` 段是数组时(`type: array, items: ...`)自动剥到 items 层抽字段

C. **归一**:

   - 两侧字段集均 lowercase + sort
   - diff = symmetric diff:`only_in_code` / `only_in_openapi`
   - `Verdict` 真实赋值:
     - 两侧空 + handler 反推成功 → `clean_empty`
     - 两侧相等 + 非空 → `clean`
     - `only_in_code` 非空 + `only_in_openapi` 空 → `only_in_code`
     - `only_in_openapi` 非空 + `only_in_code` 空 → `only_in_openapi`
     - 都非空 → `both_diff`
     - 反推失败 → `unmapped_handler` / `unmapped_handler_dynamic_payload`
     - OpenAPI 不存在该 path/method → `documented_not_found`(已在 GC §4 known-gap 落档,可豁免计 drift)

`Summary.Drift` = `count(verdict ∈ {only_in_code, only_in_openapi, both_diff})`
`Summary.Clean` = `count(verdict ∈ {clean, clean_empty})`
`Summary.Unmapped` = `count(verdict ∈ {unmapped_handler, unmapped_handler_dynamic_payload})`
`Summary.KnownGap` = `count(verdict ∈ {documented_not_found, mounted_not_documented})`

### §4.5 P5 main 流程改写

`BuildReport` 必须改为:

```go
routes, err := ParseTransportRoutes(transport)            // §4.2
ops, err := OpenAPIOperations(openapi)                    // 现有 · 但需扩展 $ref 闭包
mountedSet  := map[methodPath]Route
documented  := map[methodPath]Operation
union       := mountedSet ∪ documented(类似 P3 三向对账)
for each entry in union:
    code_fields    := []
    openapi_fields := []
    if route in mountedSet:
        typeName, pkg, hVerdict := ResolveHandlerResponseType(...)
        if hVerdict == "" { code_fields, _ := StructJSONFields(...找文件...) }
    if op in documented:
        openapi_fields := responseFieldsExpanded(op, components)
    onlyCode, onlyOpenAPI := DiffFields(code_fields, openapi_fields)
    verdict := decide(...)  // §4.4 C
    paths = append(paths, PathReport{...})
```

约束:

- `--handlers` flag 必须真用(不再 `_ = handlers`)
- `--domain` flag 必须真用(不再 `_ = domain`)
- `Summary.Drift` 必须由真实 verdict 计数赋值
- `--fail-on-drift true` 必须真生效(`Summary.Drift > 0` → exit 1)
- 工具运行时间 `< 60s`

---

## §5 输出契约

工具默认输出文件:

- `--output docs/iterations/V1_2_CONTRACT_AUDIT_v2.json`(覆盖式 · v1 保留作历史)
- `--markdown docs/iterations/V1_2_CONTRACT_AUDIT_v2.md`

JSON Schema(version 字段升 `"v1.2-C"`):

```json
{
  "version": "v1.2-C",
  "generated_at": "...",
  "openapi_sha256": "...",
  "transport_sha256": "...",
  "summary": {
    "total_paths": 0,
    "clean": 0,
    "drift": 0,
    "unmapped": 0,
    "known_gap": 0,
    "missing_in_openapi": 0,
    "missing_in_code": 0
  },
  "paths": [
    {
      "method": "GET",
      "path": "/v1/tasks/:id/detail",
      "openapi_path": "/v1/tasks/{id}/detail",
      "handler": "transport/handler/task_detail.go:25 TaskDetailHandler.GetByTaskID",
      "response_type": "service/task_aggregator.AggregateDetail",
      "code_fields": ["events", "modules", "reference_file_refs", "task", "task_detail"],
      "openapi_fields": ["events", "modules", "reference_file_refs", "task", "task_detail"],
      "only_in_code": [],
      "only_in_openapi": [],
      "verdict": "clean"
    }
  ],
  "unmapped": [
    {"method":"POST","path":"/v1/...","handler":"...","reason":"dynamic_payload"}
  ],
  "known_gap": [
    {"method":"GET","path":"/health","class":"mounted_not_documented"}
  ]
}
```

---

## §6 验收 verify 矩阵(架构师独立复核标准 · codex 不自签 PASS)

| # | 项 | 通过判据 |
|---|---|---|
| 1 | 业务 SHA 锚组 A 0 漂移 | `git status --short -- <12 files>` 空 |
| 2 | `tools/contract_audit/main.go` 不含 `_ = handlers` / `_ = domain` | grep 0 命中 |
| 3 | `tools/contract_audit/main.go` 不再把 OpenAPI fields 同时赋给 CodeFields/OpenAPIFields | 源码 review |
| 4 | `go vet ./...` | exit 0 |
| 5 | `go build ./...` | exit 0 |
| 6 | `go test ./tools/contract_audit/... -count=1 -race` | exit 0 |
| 7 | `main_test.go` 含 `TestMainFlow_TopLevelClean`(testdata phantom 完全一致 → drift=0) | 测试覆盖 |
| 8 | `main_test.go` 含 `TestMainFlow_FieldDriftSeed`(testdata phantom only_in_openapi → drift>=1) | 测试覆盖 |
| 9 | `main_test.go` 含 `TestMainFlow_OnlyInCode`(testdata phantom only_in_code → drift>=1) | 测试覆盖 |
| 10 | `main_test.go` 含 `TestMainFlow_BothDiff`(两侧都有独有字段 → both_diff verdict) | 测试覆盖 |
| 11 | `main_test.go` 含 `TestParseTransportRoutes_RealRepo`(对真 transport/http.go ≥200 routes) | 实测 |
| 12 | 工具实跑(对真 transport + OpenAPI)`summary.unmapped < 0.5 * total_paths` | JSON 验证 |
| 13 | 工具实跑 `summary.total_paths >= 200` | JSON 验证 |
| 14 | 工具运行时间 `< 60s` | wall clock |
| 15 | `--fail-on-drift true` 在 testdata drift seed 跑出 exit 1 | shell 验证 |
| 16 | `--fail-on-drift true` 在真仓库实跑 · drift 数与 unmapped 数显式列出(不强制 drift=0) | JSON + 报告 |
| 17 | 报告 `docs/iterations/V1_2_C_RETRO_REPORT.md` 落盘 · 含 §1~§6 完整章节 | 文件存在 |
| 18 | 落盘 `docs/iterations/V1_2_CONTRACT_AUDIT_v2.json` + `.md`(覆盖式) | 文件存在 |

> 注:本轮**允许真实 drift > 0**(若工具确实查出主仓存在字段漂移)· 这种情况下 codex **不修 OpenAPI**,而是把 drift 完整记入 `V1_2_C_RETRO_REPORT.md` §知名漂移段,作为 V1.2-D 处理候选。这是与 V1.1-A2 不同之处:V1.1-A2 是手工 inventory 后修 OpenAPI,V1.2-C 是自动化工具落地后只暴露不修复(让工具先有公信力)。

---

## §7 落盘清单(本轮新增/修改)

仅允许触碰以下文件:

| 文件 | 操作 |
|---|---|
| `tools/contract_audit/main.go` | 重写 BuildReport 主流程 + 新增 ParseTransportRoutes / ResolveHandlerResponseType / responseFieldsExpanded 等函数 |
| `tools/contract_audit/main_test.go` | 新增 §6 表 #7~#11 集成测试 + testdata 装载 |
| `tools/contract_audit/testdata/` | 新建目录 · 放 fake 子仓(fake transport.go + fake handler.go + fake domain.go + fake openapi.yaml)分多个子目录支持各 verdict 分支 |
| `docs/iterations/V1_2_CONTRACT_AUDIT_v2.json` | 新生成(实跑产物) |
| `docs/iterations/V1_2_CONTRACT_AUDIT_v2.md` | 新生成(实跑产物) |
| `docs/iterations/V1_2_C_RETRO_REPORT.md` | 新建 retro · 含 verify 矩阵实测结果 |

不得触碰:OpenAPI / `transport/` / `domain/` / `service/` / `repo/` / `cmd/` / `docs/frontend/` / `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` / `docs/V1_TO_V2_MODEL_HANDOFF_MANIFEST.md` / `prompts/V1_NEXT_MODEL_ONBOARDING.md` / `prompts/V1_1_FRONTEND_INTEGRATION_HANDOFF.md` / `docs/iterations/V1_2_OPENAPI_GC_REPORT.md` / `docs/iterations/V1_2_RETRO_REPORT.md` / `docs/iterations/V1_2_CONTRACT_AUDIT_v1.*`(保留作历史 · 标 superseded)

---

## §8 终止符

- **不自签 PASS** · codex 完成后只输出执行摘要 + 终止符 `V1_2_C_AUDIT_TOOL_REWORKED`
- 架构师按 §6 18 项独立 verify 后 · 通过 → 签 `V1_2_DONE_ARCHITECT_VERIFIED` · 同步关闭 V1.1-A2 Q-1 与 V1.2-C-1 known debt
- ABORT 触发时 · 写 `docs/iterations/V1_2_C_ABORT_REPORT.md` · 输出 `V1_2_C_ABORTED_<reason_slug>` · 不签终止符

---

## §9 治理同步(V1.2-C 完成后由架构师做 · 不在 codex 范围)

V1.2-C PASS 后,架构师追加:

- `docs/iterations/V1_RETRO_REPORT.md` §17 状态从 PARTIAL PASS 升级为 CLOSED · 备注 V1.2-C 收尾
- `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` §1 contract state 改为 V1.2-C complete + 新 OpenAPI sha(若有变 · V1.2-C 不应改 OpenAPI 所以 sha 不变)
- `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` §4 known-debt 删除 V1.2-C-1 与 V1.1-A2 Q-1
- `prompts/V1_ROADMAP.md` 追加 v36 · `V1.2-C audit tool reworked`

codex 不要做这些治理同步,留给架构师。

---

## §10 codex 起跑指令(单条)

```
按 prompts/V1_2_C_AUDIT_TOOL_REWORK.md 全量执行 P1 → P2 → P3 → P4 → P5,完成后落盘 §7 清单 · 输出执行摘要 + 终止符 V1_2_C_AUDIT_TOOL_REWORKED · 不自签 PASS · 任一 §1 ABORT 触发立即停并落 ABORT 报告。
```
