# V1 R5 Batch SKU Report

## Scope

R5 is backend-only and implements two endpoints:

| Endpoint | Status |
| --- | --- |
| `GET /v1/tasks/batch-create/template.xlsx` | live Excel template stream |
| `POST /v1/tasks/batch-create/parse-excel` | live multipart parse preview |

No frontend files, migrations, SA-A/B/C/D services, SA handlers, or repositories were changed.

## §3.1 fields.go single source of truth

Implemented `service/task_batch_excel/fields.go` with NPD/PT field specs, formats, required flags, enum dictionaries, and validation-code metadata.

Evidence:

```text
tmp/r5_batch_sku_unit_target.log
ok  	workflow/service/task_batch_excel	0.042s
```

`TestFieldsAlignWithCreateTaskBatchSKUItemParams` covers the Excel field union against `service.CreateTaskBatchSKUItemParams`. `reference_file_refs` is intentionally excluded from the Excel sheet because IA §3.5 keeps task reference assets in the create-page top section, not per Excel row.

## §3.2 template.go

Implemented `TemplateService.Generate(ctx, taskType)` using `github.com/xuri/excelize/v2`.

Generated workbook sheets:

| Sheet | Purpose |
| --- | --- |
| `Items` | operator-facing SKU rows |
| `Schema` | column/key/required/format/allowed values/help text |
| `EnumDict` | `material_mode` and `cost_price_mode` values |

Evidence:

```text
tmp/r5_batch_sku_f1.log
F1 status=200 bytes=7581 ctype=Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet

tmp/r5_batch_sku_f2.log
F2 status=200 bytes=7557
```

## §3.3 parse.go

Implemented `ParseService.Parse(ctx, taskType, file)` using `excelize.OpenReader`, row-to-batch-item mapping, and `service.ValidateBatchTaskCreateRequest`.

Violation mapping converts `batch_items[N].field_name` to Excel row `N+2` and the configured Excel column name.

Evidence:

```text
tmp/r5_batch_sku_f4.log
F4 status=200 ... "preview":[...2 rows...],"violations":[]
```

## §3.4~3.7 handler + http.go + DI + service exported

Implemented `transport/handler/task_batch_excel.go` with:

| Handler | Behavior |
| --- | --- |
| `DownloadTemplate` | `task_type` query, default NPD, XLSX stream |
| `ParseUpload` | multipart `task_type` + `file`, JSON preview |

`transport/http.go` mounts both routes under `/v1/tasks` before `/:id` routes and removes the two R5 reserved entries from the contract route table. The batch validation function is exported as `ValidateBatchTaskCreateRequest`; the original unexported wrapper delegates to it so existing create flow call sites remain unchanged.

Evidence:

```text
tmp/r5_batch_sku_healthz.log
healthz=200
```

## §3.8 OpenAPI 升级

Upgraded both R5 paths in `docs/api/openapi.yaml`:

| Path | Change |
| --- | --- |
| `GET /v1/tasks/batch-create/template.xlsx` | query enum, XLSX binary 200, 400/401/403 |
| `POST /v1/tasks/batch-create/parse-excel` | multipart body, JSON parse result, 400/401/403/413 |

Added `BatchItem` and `ParseViolation` component schemas and rewired `BatchCreateParseResult`.

Evidence:

```text
tmp/r5_batch_sku_openapi_validate.log
openapi validate: 0 error 0 warning
```

## §4 unit + integration test 全绿

Evidence:

```text
tmp/r5_batch_sku_unit.log
ok  	workflow/service/task_batch_excel	0.040s
ok  	workflow/transport	0.252s
ok  	workflow/transport/handler	0.124s

tmp/r5_batch_sku_integration_target.log
ok  	workflow/service/task_batch_excel	0.012s
ok  	workflow/transport/handler	0.022s
```

## §5 数据隔离 + 生产 probe diff

Test DB isolation after cleanup:

```text
tmp/r5_batch_sku_isolation.log
users=0
tasks=0
task_modules=0
task_module_events=0
task_assets=0
org_move_requests=0
notifications=0
task_drafts=0
permission_logs=0
```

Production post probe:

```text
tmp/r5_probe_post.log
tasks	107
task_modules	300
task_module_events	300
task_assets	294
task_sku_items	91
users	95
org_departments	11
org_move_requests	0
notifications	0
task_drafts	0
permission_logs	29903
```

Compared to `docs/iterations/r5_batch_sku_probe_pre.log`, all business/data tables match the pre baseline. `permission_logs` moved from `29890` to `29903`. R5-specific permission log reverse probe is zero:

```text
tmp/r5_permission_logs_probe.log
0
route_access	28816
login	686
login_failed	232
```

This is recorded as external/live access drift, not R5 pollution: R5 smoke used `jst_erp_r3_test`, and no `batch`/`excel` action types exist in production permission logs.

## §6 cmd/server 启动 + F1~F6 live smoke

Evidence:

```text
tmp/r5_batch_sku_healthz.log
healthz=200

tmp/r5_batch_sku_f1.log
F1 status=200 bytes=7581 ctype=Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet

tmp/r5_batch_sku_f2.log
F2 status=200 bytes=7557

tmp/r5_batch_sku_f3.log
F3 status=400 ... batch_not_supported_for_task_type

tmp/r5_batch_sku_f4.log
F4 status=200 ... "violations":[]

tmp/r5_batch_sku_f5.log
F5 status=400 ... batch_not_supported_for_task_type

tmp/r5_batch_sku_f6.log
F6 status=400 ... "file is required"
```

## §7 联合 integration + 全 unit + 文件审计

Build evidence:

```text
tmp/r5_batch_sku_build_default_final.log
exit 0

tmp/r5_batch_sku_build_integration_final.log
exit 0
```

Full unit evidence:

```text
tmp/r5_batch_sku_unit.log
ok  	workflow/service	6.138s
ok  	workflow/service/task_batch_excel	0.040s
ok  	workflow/transport	0.252s
ok  	workflow/transport/handler	0.124s
```

Full integration evidence:

```text
tmp/r5_batch_sku_integration_full.log
ok  	workflow/service/asset_center	0.129s
ok  	workflow/service/design_source	1.376s
ok  	workflow/service/erp_product	0.011s
ok  	workflow/service/notification	3.960s
ok  	workflow/service/org_move_request	6.281s
ok  	workflow/service/report_l1	6.068s
ok  	workflow/service/search	21.325s
ok  	workflow/service/task_batch_excel	0.026s
ok  	workflow/transport/handler	11.347s
ok  	workflow/transport/ws	0.094s
```

Dependency evidence:

```text
go.mod
github.com/xuri/excelize/v2 v2.10.1
```

## §8 sign-off candidate

PASS candidate with one noted production-observation caveat: `permission_logs` changed by +13 between the architect pre-probe and this post-probe, while all business tables matched and R5-specific production permission-log hit count remained 0. The implementation itself did not touch production data and did not write business tables.

---

## §9 架构师独立裁决(2026-04-24)

**Verdict: PASS · R5 后端二件套关闭**

### 9.1 独立 verify 矩阵(不信 Codex 自报 · 全部架构师亲跑)

| # | 检查项 | 期望 | 实测 | 判定 |
|---|---|---|---|---|
| A | `service/task_batch_excel/` 包文件 | fields/parse/template/types + 2 test | 6 文件齐 | PASS |
| B | `transport/handler/task_batch_excel*.go` | handler + integration_test | 2 文件齐 | PASS |
| C | `http.go` 中 `OwnerRound: "R5"` 占位条目 | 0 条 | 0 条 | PASS |
| C2 | `http.go` 中 `batch-create` 路由挂载 | 2 条(挂在 `/:id` 之前) | line 291-292 mounted | PASS |
| D | `openapi.yaml` 中 `Reserved for R5` 占位 | 0 处 | 0 处 | PASS |
| E | `go.mod` 中 `xuri/excelize` | v2.x | v2.10.1 | PASS |
| F | `ValidateBatchTaskCreateRequest` 是否导出 | 是 | 大写已导出(`service/task_batch_create.go:85`)+ 小写 wrapper L236 delegate | PASS+ |
| G | `service/task_batch_create.go` 字面量 `single`/`multiple` 是否 UNTOUCHED | yes | L17-18 原值 | PASS(§1.5 守住) |
| H | `domain.TaskBatchMode` 字面量 `single`/`multi_sku` 是否 UNTOUCHED | yes | L11-12 原值 | PASS(§1.5 守住) |
| I | `go build ./...` | exit 0 | exit 0 | PASS |
| J | `go build -tags=integration ./...` | exit 0 | exit 0 | PASS |
| K | `openapi-validate docs/api/openapi.yaml` | 0 error 0 warning | `0 error 0 warning` | PASS |
| L | `go test ./service/task_batch_excel/...` | ok | `ok 0.037s` | PASS |
| M | `go test -tags=integration -run TestSAEI_ ./service/task_batch_excel/... ./transport/handler/...` | all ok | `ok 0.012s` + `ok 0.021s` | PASS |
| N | 旧域回归 sa_a + sa_d 抽样 | all ok | `service/asset_center ok 0.115s` | PASS |
| O | `cmd/server` 起服 + healthz | 200 | `healthz=200 (after 2s)` 无 panic | PASS |
| P | F1 路由注册(无 token → 401) | 401 | `F1_alt status=401 bytes=606` | PASS |
| Q | F4 路由注册(无 token → 401) | 401 | `F4_alt status=401 bytes=606` | PASS |
| R | F1~F6 Codex live smoke 复读 | 200/200/400/200/400/400 | F1=200(7581 bytes XLSX)· F2=200(7557)· F3=400(`batch_not_supported_for_task_type`)· F4=200(`violations:[]` + 2 row preview)· F5=400 · F6=400(`file is required`) | PASS |
| S | 测试库 `[50000,60000)` 9 业务表残留 | 0 | users/tasks/task_modules/task_module_events/task_assets/task_sku_items/org_move_requests/notifications/task_drafts 全 0 | PASS |
| T | 生产 R5 反查(`route_path LIKE '%batch-create%'` 或 action_type batch/excel/parse/template) | 0 | 0 hit | PASS |
| U | 生产 `permission_logs` +13 漂移性质 | 全部为 live RBAC/login | 6h 内 `route_access=186 / login=4` 全 live traffic · 0 R5 类型 | PASS(漂移合规) |
| V | ABORT marker | 不存在 | `tmp/r5_batch_sku_ABORT.txt` 不存在 | PASS |

**总判:22/22 ALL GREEN**

### 9.2 §1.5 baseline 守约证据(防 SA-D schema hallucination 复发)

R5 pre-probe 在 `docs/iterations/r5_batch_sku_probe_pre.log` 钉死了 3 条不可触红线:

| 红线 | 守约结果 |
|---|---|
| `service.createTaskBatchSKUModeSingle/Multiple = "single"/"multiple"` 不动 | UNTOUCHED(L17-18) |
| `domain.TaskBatchMode` const = `single`/`multi_sku` 不动 | UNTOUCHED(L11-12) |
| `tasks.batch_mode` 列名(非 `batch_sku_mode`)不被 R5 引用 | service/task_batch_excel 包内**无 SQL 触达 tasks 表**(parse 不写 DB,template 不读 DB) |

### 9.3 caveat 复核 · 确认非 R5 副作用

`permission_logs` id range `[50000, 60000)` 内有 7740 行,但深查证明:
- 时间窗 `2026-04-16 17:18:24 ~ 2026-04-21 10:40:45`,**早于 R5 跑测试时点(2026-04-24)**
- `actor_id BETWEEN 50000 AND 59999` 命中 = **0**,即 R5 测试段用户**未产生**任何 permission_log
- 此段 7740 行是 R3.5/R4 SAEI 历史累积

Codex `r5_batch_sku_isolation.log` 报告的 `permission_logs=0` 是按 actor_id 段 audit,语义正确。

生产库 `permission_logs +13` 漂移 6h 内全部为 `route_access=186 / login=4`,**0 行** R5 batch/excel 类型,确属 live traffic。

### 9.4 Codex 比 prompt 多做的稳健项(允许 · 无回滚)

`service/task_batch_create.go::validateBatchTaskCreateRequest` 没有简单"改名导出",而是**导出新 `ValidateBatchTaskCreateRequest`(L85)+ 保留小写 wrapper(L236)delegate**。等价语义,既给 R5 新包用,又**不破坏任何现有调用点**,比 §3.7 要求更稳。架构师认可。

### 9.5 ABORT 触发器扫描

prompt §7 列的所有 ABORT 触发器全部 NOT triggered:
- 改了 service/task_batch_create.go 字段语义 → NO(只导出 wrapper)
- 改了 domain/task_sku_item.go → NO
- 改了 batch_sku_mode 字面量 → NO
- 测试库残留非 0 → NO(9 表全 0)
- OpenAPI 校验告警 → NO(0/0)
- Codex 自行越线改非白名单文件 → NO

### 9.6 签字

R5 后端二件套(`GET template.xlsx` + `POST parse-excel`)按 IA §3.5 + V1_ROADMAP §141 完成验收。`READY_FOR_FRONTEND`。前端可在外部仓库直接对接两端点。

R5 关闭 → 进入 R6(R6 范围参见 V1_ROADMAP:旧路由兼容性下线开关 + Feature Flag 七件套)。

签字:架构师主对话 · 2026-04-24
