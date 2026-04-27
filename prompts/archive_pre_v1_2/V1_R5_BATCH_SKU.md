# V1 · R5 · 批量 SKU 二件套(Backend-Only · Excel template + parse)

> 发布:2026-04-25
> 范围:本仓 R5 后端实装 · 仅 2 个 endpoint · 不涉及前端工程
> 上游签字依赖:R4-SA-A v2.1 + Patch-A2 + Patch-A3 + R4-SA-B v1.1 + R4-SA-B.1/B.2 + R4-SA-C v1 + R4-SA-C.1 + R4-SA-D v1.0 + R4-Retro v1.1 全部签字
> 命名注:ROADMAP §32 旧名 `V1_R5_FRONTEND.md` 与本仓范围脱钩(前端工程不在本仓)· 本 prompt 改名为 `V1_R5_BATCH_SKU.md`,ROADMAP §32 同步

---

## 0. 角色与目标

实现 IA §3.5 钦定的 **Excel 唯一入口批量 SKU** 后端二件套:

1. `GET /v1/tasks/batch-create/template.xlsx` — 后端**根据当下数据库字典动态生成** Excel 模板
2. `POST /v1/tasks/batch-create/parse-excel`(multipart)— 解析上传的 Excel,**仅返回**预览 + 逐行错误定位(`{preview, violations: [{row, column, code, message}]}`),**不创建任务**

前端拿预览后走**现有** `POST /v1/tasks`(`batch_sku_mode=multiple`)创建。R5 不新建 batch-create POST endpoint。

**严禁触达**:

- ❌ 任何 SA-A/B/C/D 业务逻辑(本轮零业务变更)
- ❌ `service/task_batch_create.go` 既有 `validateBatchTaskCreateRequest` / `CreateTaskBatchSKUItemParams` 等公共契约的字段语义(只读复用)
- ❌ 任何 migration / schema 变更(本轮零 DDL)
- ❌ 任何前端工程文件 / FE Plan / handoff artifact(前端在外部仓)
- ❌ 任何 R4 报告 / Patch-A2 / Patch-A3 / Retro 报告

**允许写入**:

- ✅ `service/task_batch_excel/`(新建包)
  - `service/task_batch_excel/template.go`(动态生成 Excel)
  - `service/task_batch_excel/parse.go`(解析 + 校验)
  - `service/task_batch_excel/fields.go`(字段元信息 single source of truth · 反射 `CreateTaskBatchSKUItemParams` + 引用 `validateBatchTaskCreateRequest` 的 violation code 字典)
  - 同包 unit test(IA-A10 单测)
  - 同包 integration test(`//go:build integration` · `TestSAEI_*` 命名)
- ✅ `transport/handler/task_batch_excel.go`(新建 · 2 个 handler)
- ✅ `transport/http.go`(挂 2 条路由 · 删 reserved 表 line 566-567 两条 R5 条目)
- ✅ `docs/api/openapi.yaml`(把 line 14028-14043 两个 path 的 501 占位升级为实装 schema · `x-api-readiness: ready_for_frontend` 保持)
- ✅ `docs/iterations/V1_R5_BATCH_SKU_REPORT.md`(新建 · 报告)
- ✅ `prompts/V1_ROADMAP.md` §32 命名同步(由架构师在 R5 签字时一并落)
- ✅ `tmp/r5_batch_sku_*.{sh,log,json,txt,body,pid,xlsx}`(过程产物)
- ✅ `go.mod` / `go.sum`(添加 `github.com/xuri/excelize/v2`)

---

## 1. 必读输入(各读 1 次)

1. **本 prompt 全文**(§0~§9 · 含 §1.5 生产 baseline)
2. **`docs/iterations/r5_batch_sku_probe_pre.log`**(2026-04-25 04:59 UTC 落地 · 全部 baseline 数字 + schema 校正全在此 · §1.5 是它的提炼)
3. `docs/V1_INFORMATION_ARCHITECTURE.md` §3.5(L102~180 · Excel 唯一入口章节 · IA-A9~IA-A14 锚点 · L122~147 路径与契约 · L157 字段单一真源声明)
4. `service/task_service.go` 第 19~34 行(`CreateTaskBatchSKUItemParams` 11 字段定义)
5. `service/task_batch_create.go` 第 16~234 行(`createTaskBatchSKUModeSingle/Multiple` 字面量 + `validateBatchTaskCreateRequest` 完整 violation 链)
6. `domain/task_sku_item.go`(`TaskBatchMode` / `TaskSKUStatus` / `TaskSKUItem` 定义 · 注意 const 是 `single`/`multi_sku` 与 service 字面量错位 · 见 §1.5 C)
7. `transport/http.go` 第 560~575 行(reserved 表两条 R5 条目位置)
8. `docs/api/openapi.yaml` 第 14025~14060 行(两个 R5 path 的 501 占位段)
9. 任意一份现成 SAAI/SABI/SACI/SADI integration test(参考结构)
10. `docs/iterations/V1_R4_SA_D_REPORT.md`(SA-D 单 prompt 微型轮风格 · R5 模仿)
11. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md` 中 `/v1/tasks/batch-create/*` 的 Compatibility 段(若有 · 没有则跳过)

**禁止读**:

- `docs/archive/**` 全部
- `docs/iterations/V1_R0*` / `V1_R1*` / `V1_R2*` / `V1_R3*` 老 prompt
- `prompts/V1_R0*` / `V1_R1*` / `V1_R2*` / `V1_R3*`
- 任何前端工程相关文件(`FRONTEND_*.md` 仓内 5 份只在需要确认 R5 不外溢前端时**最多瞥一眼标题** · 不深读)
- `.gomodcache/**`

---

## 1.5 生产 baseline(2026-04-25 04:59 UTC pre-probe 落地 · `docs/iterations/r5_batch_sku_probe_pre.log`)

**A · 9+1 表行数 baseline**(post diff 必 = 0 · 任何 drift 须 live traffic 解释):

```
tasks=107 · task_modules=300 · task_module_events=300 · task_assets=294 · task_sku_items=91
users=95 · org_departments=11 · org_move_requests=0 · notifications=0 · task_drafts=0 · permission_logs=29890
```

**B · 关键 schema 校正(防 SA-D 同款幻觉 · ABORT 警示)**:

- `tasks.batch_sku_mode` 列**不存在**;实际列 = `batch_mode` (varchar NOT NULL) + `is_batch_task` (tinyint NOT NULL) + `batch_item_count` (int NOT NULL)
- `tasks.task_type` / `tasks.source_mode` 存在
- `task_sku_items` **17 列全 1**(`id` · `task_id` · `sequence_no` · `sku_code` · `sku_status` · `product_name_snapshot` · `product_short_name` · `category_code` · `material_mode` · `cost_price_mode` · `quantity` · `base_sale_price` · `design_requirement` · `variant_json` · `dedupe_key` · `product_id` · `erp_product_id`)

**C · 关键命名错位(R5 必须共存 · 禁修)**:

- service 层 `createTaskBatchSKUModeSingle = "single"` / `createTaskBatchSKUModeMultiple = "multiple"`(`service/task_batch_create.go:17-18` · request 字段 `batch_sku_mode`)
- DB 层 `tasks.batch_mode` 实际值分布 `single=93 / multi_sku=14`(`domain.TaskBatchMode` const 钦定 `single` / `multi_sku`)
- 生产从 `multiple` → `multi_sku` 一定有映射处(可能在 `repo/mysql/task_repo.go` Create/Update),R5 **不得动**这条映射。
- **R5 自身**:request 走 service 字段名(`batch_sku_mode=multiple`)· 生成 Excel 模板时**不暴露** DB 列名(用户填表只用 §3.1 字段集 column 名)· parse 后 violations 报字段路径用 service 层 `batch_items[N].field_name`

**D · multi 任务真实存量**:

- multi(`batch_mode=multi_sku`)= **14 个任务** · NPD 全占(PT multi = 0 个 sample)
- task_sku_items 91 行 · 65 任务 · 行数分布 `6×1 / 4×2 / 3×4 / 2×7 / 1×51`
- NPD multi 抽样(`task_id ∈ {543,539,538}` 各取首 2 行 · 共 9 行):
  - `sku_code` / `product_name_snapshot` / `product_short_name` / `category_code` / `material_mode` / `design_requirement` / `dedupe_key` 7 字段 **9/9 实有值**
  - `variant_json` **0/9 全空** → Excel 列设**可选**(IA §3.5 既有约定)
- 实例:`543` ~ `NSFL000017` "润冰/常规海报/劳动节横幅/..." `FLAG_CLOTH_STANDARD` `other` "出单画图"

**E · 枚举分布**:

- `material_mode` 实际值:`other=69 / ''=12 / preset=10`(IA 钦定 `preset`/`other` · 越界 0)
- `cost_price_mode`:**91 行全空**(无 PT multi sample · R5 PT 单测必须**合成 fixture** 不能从生产读)
- `sku_status`:`generated=91`(domain 4 值之一)
- E.4 三向越界检查 = `0 / 0 / 0`(干净)

**F · 跨域控制字段 baseline**(post 必须不变):

- SA-A:`is_archived=0` · `cleaned_at IS NOT NULL=0` · `deleted_at IS NOT NULL=0`
- SA-B:`org_move_requests total=0` · `users status=deleted=0`
- SA-C:`notifications total=0` · `task_drafts total=0`
- SA-D:`permission_logs.action_type=report_access_denied total=0`
- `task_module_events total=300`

**G · R5 endpoint 流量预检**:

- `permission_logs.action_type LIKE '%batch%' OR '%excel%'` = **0 hit**(R5 上线前 reserved 返 501 · post 也应 0 · 因为 R5 不写 permission_logs)

---

## 2. 验收锚点

| 锚点 | 出处 | R5 落地形式 |
| --- | --- | --- |
| **IA-A10** | `docs/V1_INFORMATION_ARCHITECTURE.md` L176 | unit test:`template.xlsx` 返回的列集合与 `CreateTaskBatchSKUItemParams` 各 task_type 必填字段 100% 一致(NPD/PT 两套独立断言) |
| **IA-A9** | IA §3.5 | template.xlsx 多 sheet:Sheet1 = 主数据 · Sheet2 = 字段说明 · Sheet3 = 枚举字典(material_mode / cost_price_mode 等) |
| **IA-A11** | IA §3.5 L124 | parse-excel 仅返预览 + violations 数组 · 不创建任务 · 不写库 |
| **IA-A12** | IA §124 | violations 形如 `{row: int, column: string, code: string, message: string}` · code 复用 `validateBatchTaskCreateRequest` 既有字典(missing_required_field / invalid_material_mode / batch_not_supported_for_task_type 等) |
| **IA-A13**(自延) | R5 自定义 | parse-excel multipart `task_type` 必传 query/form · 仅接受 `new_product_development` / `purchase_task`(`original_product_development` 显式拒绝 = 复用 L102~113 的 `batch_not_supported_for_task_type`) |
| **R5-A1**(自延) | R5 prompt | OpenAPI 两个 path 401/403/422 ErrorResponse 与 SA-A/B/C/D 风格一致 · `x-api-readiness: ready_for_frontend` |

---

## 3. 实装清单

### 3.1 `service/task_batch_excel/fields.go`(新建)

定义两套字段元信息(NPD + PT),作为 Excel 列、字段说明 sheet、parse 列名映射的**单一真源**。每个字段含:

```go
type FieldSpec struct {
    Column          string             // Excel 列名 · 中文优先(IA §3.5 给运营用)+ 英文 SKUCode 之类保留
    Key             string             // CreateTaskBatchSKUItemParams 字段名(snake_case · 与 OpenAPI 一致)
    Required        bool
    AllowedValues   []string           // 枚举字典(material_mode = preset/other 等)
    Format          FieldFormat        // string/int64/float64/json
    NotAllowed      bool               // 该 task_type 下不允许出现(L164~206 既有 not_allowed_for_task_type 链)
    HelpText        string             // 字段说明 sheet 用
    ViolationCodes  ViolationCodeSet   // missing/invalid 时返回的字段码
}
```

NPD 字段集(参考 L146~175):`product_name` · `product_short_name` · `category_code` · `material_mode`(枚举 preset/other) · `design_requirement` · `variant_json`(可选 JSON) · `new_sku`(可选 · 不指定则后端自动生成)

PT 字段集(参考 L176~206):`product_name` · `category_code` · `cost_price_mode`(枚举 manual/template) · `quantity` · `base_sale_price` · `variant_json` · `purchase_sku`(可选)

**禁止**:
- 重新声明枚举值(直接用 `domain.MaterialMode.Valid()` / `domain.CostPriceMode.Valid()` 校验)
- 字段集字面量与 `CreateTaskBatchSKUItemParams` / `validateBatchTaskCreateRequest` 不一致(IA-A10 单测必须强制对齐)

### 3.2 `service/task_batch_excel/template.go`(新建)

```go
type TemplateService interface {
    Generate(ctx context.Context, taskType domain.TaskType) ([]byte, *domain.AppError)
}
```

- 用 `github.com/xuri/excelize/v2` 生成 .xlsx
- 多 sheet:
  - Sheet1 `Items`(列 = §3.1 字段集 column · 第一行为表头)
  - Sheet2 `Schema`(每行 = 字段元信息 · column / required / allowed_values / help_text)
  - Sheet3 `EnumDict`(material_mode / cost_price_mode / category 等枚举字典 · 给运营查阅 · 不强制做下拉数据校验)
- `task_type` 不在 {NPD, PT} 时返 `ErrCodeInvalidRequest` AppError
- 输出字节流给 handler 写 response

### 3.3 `service/task_batch_excel/parse.go`(新建)

```go
type ParseService interface {
    Parse(ctx context.Context, taskType domain.TaskType, file io.Reader) (*ParseResult, *domain.AppError)
}

type ParseResult struct {
    TaskType   domain.TaskType                       `json:"task_type"`
    Preview    []service.CreateTaskBatchSKUItemParams `json:"preview"`
    Violations []ParseViolation                      `json:"violations"`
}

type ParseViolation struct {
    Row     int    `json:"row"`              // Excel 行号 · 从 2 开始(1 是表头)
    Column  string `json:"column,omitempty"` // §3.1 FieldSpec.Column · 行级错误时为空
    Code    string `json:"code"`             // missing_required_field / invalid_material_mode / duplicate_batch_sku 等
    Message string `json:"message"`
}
```

- 用 `excelize.OpenReader` 解析
- 行 → `CreateTaskBatchSKUItemParams`
- 调 `validateBatchTaskCreateRequest`(包私有 · 需把它**改为可导出** 名 `ValidateBatchTaskCreateRequest` · **唯一允许的既有文件改动** · 见 §3.7)或在 `parse.go` 内引用同等校验逻辑
- 把 violation 转成 `ParseViolation`(field 路径 → row/column 解析)
- 不写任何库

### 3.4 `transport/handler/task_batch_excel.go`(新建)

两个 handler:

```go
func (h *TaskBatchExcelHandler) DownloadTemplate(c *gin.Context) {
    // task_type query param · 默认 new_product_development
    // 调 templateSvc.Generate
    // Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
    // Content-Disposition: attachment; filename="batch_create_template_{task_type}_{yyyymmdd}.xlsx"
    // 200 + body bytes
}

func (h *TaskBatchExcelHandler) ParseUpload(c *gin.Context) {
    // multipart form
    // task_type form field
    // file form field "file"
    // 调 parseSvc.Parse
    // 200 JSON { task_type, preview, violations }
}
```

- 错误处理沿用 SA-A/B/C 同款 `respondError(c, appErr)` / `respondOK(c, data)` helper
- 鉴权:`v1R1AllLoggedInRoles()`(IA §145 写"登录用户")

### 3.5 `transport/http.go` 修改(2 处定点)

**修改 A · reserved 表删 R5 两条**(line 566-567 删除 · 类比 Patch-A2 删 SA-A 7 条的方式):

```diff
-		{GroupBase: "/v1", Method: http.MethodGet, RelativePath: "/tasks/batch-create/template.xlsx", OwnerRound: "R5", RequiredRoles: v1R1AllLoggedInRoles(), SamplePath: "/v1/tasks/batch-create/template.xlsx"},
-		{GroupBase: "/v1", Method: http.MethodPost, RelativePath: "/tasks/batch-create/parse-excel", OwnerRound: "R5", RequiredRoles: v1R1AllLoggedInRoles(), SamplePath: "/v1/tasks/batch-create/parse-excel"},
```

**修改 B · 在 taskGroup(`/v1/tasks` group)内部加 2 条实挂载**(参考既有 `/v1/tasks/{id}/asset-center/*` 等位置):

```go
taskGroup.GET("/batch-create/template.xlsx", access(...), batchExcelH.DownloadTemplate)
taskGroup.POST("/batch-create/parse-excel", access(...), batchExcelH.ParseUpload)
```

**注意**:与 SA-A Patch-A2 同款经验,**确认 Gin wildcard 不冲突**:`/tasks/batch-create/*` 是字面前缀,与 `/tasks/:task_id/*` 不在同一 wildcard 段,无冲突。**仍然在 build 后立即 cmd/server 启动 smoke** 验。

### 3.6 `transport/router_di.go` 或 cmd 接线(轻量)

`taskBatchExcelHandler` 注入到 `Router` 构造路径(参考 SA-A `taskAssetCenterH` 同款注入方式)· DI 链 1 行追加。

### 3.7 改 `service/task_batch_create.go::validateBatchTaskCreateRequest` 可导出(唯一允许的既有文件改动)

```diff
-func validateBatchTaskCreateRequest(p CreateTaskParams) *domain.AppError {
+func ValidateBatchTaskCreateRequest(p CreateTaskParams) *domain.AppError {
```

在同文件中所有内部调用点同步 `validateBatchTaskCreateRequest` → `ValidateBatchTaskCreateRequest`(应只 2~3 处)。

**禁止**:动 validate 内部任何一行业务校验逻辑或 violation code 字符串。

### 3.8 `docs/api/openapi.yaml` 修改(2 path schema 升级)

将第 14025~14060 行两个 path 的:

```yaml
responses:
  '200': { description: Excel template stream }
  '501': { description: Reserved for R5 }
```

升级为完整 schema:

- `GET /v1/tasks/batch-create/template.xlsx`:
  - query `task_type`(enum: new_product_development / purchase_task · default new_product_development)
  - 200:`application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` binary stream
  - 400/401/403:ErrorResponse
  - 删除 501
- `POST /v1/tasks/batch-create/parse-excel`:
  - multipart/form-data:`task_type`(enum)+ `file`(binary)
  - 200:JSON `{ task_type, preview: BatchItem[], violations: ParseViolation[] }`
  - 400(文件格式错 / 解析失败)/ 401 / 403 / 413(文件过大):ErrorResponse
  - 删除 501

新 schema(component 内):

- `BatchItem`:对齐 `CreateTaskBatchSKUItemParams` 字段(snake_case)
- `ParseViolation`:`{row, column?, code, message}`

`x-api-readiness: ready_for_frontend` / `x-owner-round: R5` 保留。

---

## 4. 测试清单(全跑 · 必绿)

### 4.1 unit test(同包 · `_test.go`)

- `TestFieldsAlignWithCreateTaskBatchSKUItemParams`(IA-A10 硬验):反射 `CreateTaskBatchSKUItemParams` 字段集,与 §3.1 NPD ∪ PT 字段集做对齐断言(每个 service 字段都被某个 task_type 覆盖)
- `TestFieldsAlignWithValidateBatchTaskCreateRequest`(IA-A10 硬验):对每个 NPD/PT 必填字段,构造一个仅缺该字段的 Excel 行 + 调 parse · 期望 violation `code = missing_required_field`(对齐 L148~205 既有 violation 字面)
- `TestTemplateGenerateNPD` / `TestTemplateGeneratePT`:生成的字节流可被 excelize 重新打开,Sheet1 第一行 = 字段集 column
- `TestParseValidExcel`:构造合法 Excel(2 行)· parse 返 `len(preview)==2 && len(violations)==0`
- `TestParseMissingRequired`:某行缺 product_name · 返对应 violation
- `TestParseInvalidEnum`:某行 material_mode=foo · 返 `code=invalid_material_mode` violation(对齐 L159)
- `TestParseDuplicateBatchSKU`:两行 NewSKU 重复 · 返 `code=duplicate_batch_sku` violation(对齐 L222)
- `TestParseTaskTypeNotSupported`:`task_type=original_product_development` · 返 `code=batch_not_supported_for_task_type`(对齐 L104)

### 4.2 integration test(`//go:build integration` · 命名 `TestSAEI_*`)

- `TestSAEI_DownloadTemplate_NPD` · `TestSAEI_DownloadTemplate_PT`:走真 router · 200 + Content-Type + Content-Disposition · body 可被 excelize 重新打开
- `TestSAEI_ParseUpload_HappyPath_NPD` · `TestSAEI_ParseUpload_HappyPath_PT`:multipart 上传合法 Excel · 200 + preview 长度 + violations=[]
- `TestSAEI_ParseUpload_RejectedTaskType`:`task_type=original_product_development` 返 422 + `code=batch_not_supported_for_task_type`
- `TestSAEI_ParseUpload_TooLarge`(可选 · 若 multipart 体积限制实装):返 413
- `TestSAEI_DownloadTemplate_Auth_401`:无 token · 返 401
- `TestSAEI_DownloadTemplate_Auth_403_NoLogin`(若有非登录角色限制 · 否则跳过)

测试库范围:沿用既有 `id ∈ [50000, 60000)` 测试用户隔离。本轮零写入业务表(parse 不创建任务),整轮 9 表残留必为 0。

---

## 5. 数据隔离 + 生产 probe(继承 SA-A/B/C/D 套路)

### 5.1 测试库隔离审计

跑完 integration 后,9 表 `[50000, 60000)` 残留必全 0(R5 零业务写入,本来就该 0)。

### 5.2 生产 probe(读-写-读)

**本轮零业务写入**,生产 probe 仅做读基线对比 · 不做写监测:

- `tmp/r5_probe_pre.log` · 记录 baseline_ts + 9 表行数
- 跑完 integration + smoke 后
- `tmp/r5_probe_post.log` · 同 SQL 重跑
- diff:9 表行数应**完全相等**(R5 本仓零业务写入,生产应无任何变化)

任何 1 张表行数变化 → ABORT。

---

## 6. 验证清单(11 步 A~K · 全跑 · 任一 fail 即 ABORT)

```bash
cd /mnt/c/Users/wsfwk/Downloads/yongboWorkflow/go
DSN="$(ssh jst_ecs 'bash /root/ecommerce_ai/r3_5/build_test_dsn.sh')"
export MYSQL_DSN="$DSN"
export R35_MODE=1
export GOPATH=$HOME/.cache/go-path && mkdir -p $GOPATH

# A) build 双 tag 绿
/home/wsfwk/go/bin/go build ./... 2>&1 | tee tmp/r5_batch_sku_build_default.log
/home/wsfwk/go/bin/go build -tags=integration ./... 2>&1 | tee tmp/r5_batch_sku_build_integration.log

# B) 全 unit test
/home/wsfwk/go/bin/go test ./... -count=1 2>&1 | tee tmp/r5_batch_sku_unit.log

# C) 新增 unit target 单跑(IA-A10 锚点)
/home/wsfwk/go/bin/go test -count=1 -run 'TestFieldsAlign|TestTemplateGenerate|TestParse' \
  ./service/task_batch_excel/... 2>&1 | tee tmp/r5_batch_sku_unit_target.log

# D) 新增 SAEI integration 单跑
/home/wsfwk/go/bin/go test -tags=integration -count=1 -run 'TestSAEI_' \
  ./transport/handler/... 2>&1 | tee tmp/r5_batch_sku_integration_target.log

# E) cmd/server 启动 smoke(确认无 Gin wildcard 冲突)
ssh -N -f -L 6379:127.0.0.1:6379 jst_ecs 2>/dev/null || true
sleep 1
fuser -k 8080/tcp 2>/dev/null || true
sleep 1
setsid bash -c '/home/wsfwk/go/bin/go run ./cmd/server' >/tmp/r5_batch_sku_server.log 2>&1 < /dev/null &
echo $! > tmp/r5_batch_sku_server.pid
sleep 8
HEALTH=$(curl -o /dev/null -s -w '%{http_code}' http://127.0.0.1:8080/healthz)
echo "healthz=$HEALTH" | tee tmp/r5_batch_sku_healthz.log
test "$HEALTH" = "200" || { echo SERVER_BAD; tail -50 /tmp/r5_batch_sku_server.log; exit 1; }
grep -i panic /tmp/r5_batch_sku_server.log >/dev/null && { echo SERVER_PANIC; exit 1; }

# F) live smoke 二件套
# 注入 SuperAdmin token(同 retro v1.1 模板 · user_id=49100 · token r5-batch-sku-super-token)
# F1: GET /v1/tasks/batch-create/template.xlsx?task_type=new_product_development → 200 + Content-Type 含 spreadsheetml + body 字节数 > 1024
# F2: GET 同上 task_type=purchase_task → 200
# F3: GET 同上 task_type=invalid → 400
# F4: POST /v1/tasks/batch-create/parse-excel(用 F1 输出做合法 Excel · 转 multipart) → 200 + preview 数组 + violations=[]
# F5: POST 同上 task_type=invalid → 400
# F6: POST 无 file → 400

# G) 关后端
kill -9 $(cat tmp/r5_batch_sku_server.pid) 2>/dev/null || true
pkill -f 'ssh -N -f -L 6379' 2>/dev/null || true

# H) 联合 integration 5 域 + R3 不破坏(4 旧域 + 新 SAEI)
/home/wsfwk/go/bin/go test -tags=integration -count=1 -timeout 30m \
  -run 'SAAI|SABI|SACI|SADI|SAEI|TestModuleAction|TestTaskPool|TestTaskCancel|TestTaskAggregator' \
  ./service/... ./transport/handler/... ./transport/ws/... 2>&1 \
  | tee tmp/r5_batch_sku_integration_full.log

# I) openapi-validate 必 0/0
/home/wsfwk/go/bin/go run ./cmd/tools/openapi-validate docs/api/openapi.yaml 2>&1 \
  | tee tmp/r5_batch_sku_openapi_validate.log

# J) 测试库 [50000,60000) 9 表残留 0
ssh jst_ecs 'bash /root/ecommerce_ai/r3_5/audit_test_isolation.sh' 2>&1 \
  | tee tmp/r5_batch_sku_isolation.log || true

# K) 生产 probe diff(零业务写入硬门)
# pre/post 脚本同 retro v1.1 模板 · 读 9 表行数 · diff 必为 0
```

`F1~F6` 是 R5 核心通过条件:6 条 live smoke 全 ≤ 4xx · 5xx=0 · F1/F2/F4 必为 200。

---

## 7. ABORT 触发清单(命中即停)

- 修改 §0 白名单之外任何文件 → ABORT
- 修改 `validateBatchTaskCreateRequest` 内部任何一行业务校验逻辑 → ABORT(允许仅改函数名为 `ValidateBatchTaskCreateRequest`,见 §3.7)
- 修改 `service/task_batch_create.go` 第 16~19 行常量字面量(`createTaskBatchSKUModeSingle="single"` / `createTaskBatchSKUModeMultiple="multiple"` / `maxBatchSKUGenerateAttempts`)→ ABORT(§1.5 C 钉死:这些是 request 合约 · 与 DB `batch_mode=single/multi_sku` 通过 repo 层映射 · 改这里破坏现有 task POST)
- 修改 `domain.TaskBatchMode` const(`single` / `multi_sku`)→ ABORT(§1.5 C 钉死:这是 DB 真实分布的合约)
- 改 `repo/mysql/task_repo.go` 或任何 multi → multi_sku 映射逻辑 → ABORT
- 修改任何 SA-A/B/C/D handler / service / repo → ABORT
- 任何 migration 新增 → ABORT
- IA-A10 硬验 unit test FAIL → ABORT(`fields.go` NPD/PT 字段集 ≠ `CreateTaskBatchSKUItemParams` 反射结果)
- F1~F6 任一 5xx → ABORT
- 联合 integration 任一旧域 FAIL → ABORT
- openapi-validate 任 1 error → ABORT
- 测试库 [50000,60000) 9 表残留非 0 → ABORT
- 生产 probe pre/post 9+1 表行数 diff 任 1 行 ≠ live traffic 解释范围 → ABORT(§1.5 A baseline 必须严格继承 · multi 任务计数 14 必须不变)
- DSN guard 失效(连到生产)→ ABORT
- 改写 R4 任意报告 → ABORT
- 假设 `tasks.batch_sku_mode` 列存在 → ABORT(§1.5 B 钉死:实际列名 `batch_mode`)

---

## 8. 报告输出(新建 `docs/iterations/V1_R5_BATCH_SKU_REPORT.md`)

11 章必齐:

1. `## Scope` — 本仓 R5 范围(后端二件套 · 命名修正)
2. `## §3.1 fields.go single source of truth` — 字段集 + 与 service 的对齐证据
3. `## §3.2 template.go` — 多 sheet 结构 + 字节流可重打开证据
4. `## §3.3 parse.go` — 解析链 + violation 映射证据
5. `## §3.4~3.7 handler + http.go + DI + service exported`
6. `## §3.8 OpenAPI 升级` — 2 path 删 501 · 加完整 schema · openapi-validate 0/0
7. `## §4 unit + integration test 全绿`
8. `## §5 数据隔离 + 生产 probe diff`
9. `## §6 cmd/server 启动 + F1~F6 live smoke`
10. `## §7 联合 integration + 全 unit + 文件审计`
11. `## §8 sign-off candidate`

---

## 9. 工作流程

1. 读 §1 必读(尤其 IA §3.5 + service/task_batch_create.go validate 链 + service/task_service.go CreateTaskBatchSKUItemParams)
2. 按 §3.1~§3.8 落 8 处实装(顺序:fields → template → parse → handler → http.go → DI → service exported → OpenAPI)
3. 按 §4 写 unit + integration test
4. 按 §6 跑 11 步验证
5. 按 §5 跑 probe pre/post
6. 写 §8 报告
7. 任何步骤 fail 按 §7 ABORT
8. 完成简洁回报 + 退出
