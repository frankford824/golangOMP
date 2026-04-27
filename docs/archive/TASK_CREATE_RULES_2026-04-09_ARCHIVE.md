# Task Create Rules - 三种任务创建规则

## 概述

系统支持三种任务类型，每种任务有明确的必填/条件必填/可选字段规则。

**所有任务类型的公共必填字段：**
- `task_type` - 任务类型
- `owner_team` - 所属运营组（必须是合法配置组）
- `due_at` / `deadline_at` - 任务截止时间（RFC3339）

**当前合法的 owner_team 值：**
- 人力行政组
- 设计组
- 内贸运营组
- 采购仓储组
- 总经办组

---

## 一、原品开发 (original_product_development)

### 前端字段口径
- 绑定 ERP 产品（可查询搜索）
- 如有图则展示预览图，无图则占位图
- 修改要求
- 参考图（可选）
- 所属运营组
- 指派设计师（默认请选择设计师）
- 任务截止时间
- 优先级（默认 low）
- 外包按钮（可选，默认 false）
- 备注

### 后端创建规则

**必填：**
| 字段 | JSON Key | 说明 |
|------|----------|------|
| 产品 ID | `product_id` | 绑定的 ERP 产品 ID |
| 修改要求 | `change_request` | 原品修改要求描述 |
| 所属组 | `owner_team` | 必须是合法配置组 |
| 截止时间 | `due_at` | RFC3339 格式 |

**可选：**
| 字段 | JSON Key | 说明 |
|------|----------|------|
| 参考图 | `reference_file_refs` | reference 对象数组 |
| 设计师 | `designer_id` | 指派设计师 ID |
| 优先级 | `priority` | low/normal/high/urgent，默认 low |
| 外包 | `is_outsource` | 布尔值，默认 false |
| 备注 | `remark` | 自由文本 |

**自动推断：**
- `source_mode` 自动设为 `existing_product`
- `sku_code` 从绑定产品获取
- `product_name_snapshot` 从绑定产品获取
- 前端不应显式传 `source_mode`；后端根据 `task_type` 自动推断

**ERP 产品快照：**
创建时会固化以下 ERP 产品信息：
- product_id
- sku_code
- product_name
- category_name
- image_url（可空）

---

## 二、新品开发 (new_product_development)

### 前端字段口径
- 产品分类编码
- 产品材质（preset 或 other 手动输入）
- 新品 SKU
- 产品名称
- 产品简称
- 成本单价（manual/template/非必填）
- 数量（非必填）
- 基本售价（非必填）
- 设计需求说明
- 参考图（可选）
- 产品参考链接
- 所属运营组
- 指派设计师
- 任务截止时间
- 优先级（默认 low）
- 外包按钮（默认 false）
- 备注

### 后端创建规则

**必填：**
| 字段 | JSON Key | 说明 |
|------|----------|------|
| 分类编码 | `category_code` | 产品分类编码 |
| 材质模式 | `material_mode` | `preset` 或 `other` |
| 产品名称 | `product_name` | 产品名称 |
| 产品简称 | `product_short_name` | 产品简称 |
| 设计需求 | `design_requirement` | 设计需求说明 |
| 所属组 | `owner_team` | 必须是合法配置组 |
| 截止时间 | `due_at` | RFC3339 格式 |

**条件必填：**
| 字段 | JSON Key | 条件 |
|------|----------|------|
| 预设材质 | `material` | material_mode = preset 时必填 |
| 手动材质 | `material_other` | material_mode = other 时必填 |
| 成本单价 | `cost_price` | cost_price_mode = manual 时必填 |

**可选：**
| 字段 | JSON Key | 说明 |
|------|----------|------|
| 新品 SKU | `new_sku` | 省略时后端自动生成 |
| 成本模式 | `cost_price_mode` | manual 或 template |
| 数量 | `quantity` | 整数 |
| 基本售价 | `base_sale_price` | 数值 |
| 参考图 | `reference_file_refs` | reference 对象数组 |
| 参考链接 | `reference_link` | URL |
| 设计师 | `designer_id` | 指派设计师 ID |
| 优先级 | `priority` | 默认 low |
| 外包 | `is_outsource` | 默认 false |
| 备注 | `remark` | 自由文本 |

**自动推断：**
- `source_mode` 自动设为 `new_product`
- `sku_code` 如未提供 `new_sku`，自动从编码规则生成
- 前端不应显式传 `source_mode`；后端根据 `task_type` 自动推断

---

## 三、采购任务 (purchase_task)

### 前端字段口径
- 采购 SKU
- 产品名称
- 产品渠道（非必填）
- 成本单价（manual/template）
- 数量
- 基本售价
- 参考图（可选）
- 所属组（必填）
- 任务截止时间
- 优先级（默认 low）
- 备注

### 后端创建规则

**必填：**
| 字段 | JSON Key | 说明 |
|------|----------|------|
| 采购 SKU | `purchase_sku` | 采购产品 SKU |
| 产品名称 | `product_name` | 产品名称 |
| 成本模式 | `cost_price_mode` | `manual` 或 `template` |
| 数量 | `quantity` | 整数 |
| 基本售价 | `base_sale_price` | 数值 |
| 所属组 | `owner_team` | 必须是合法配置组 |
| 截止时间 | `due_at` | RFC3339 格式 |

**条件必填：**
| 字段 | JSON Key | 条件 |
|------|----------|------|
| 成本单价 | `cost_price` | cost_price_mode = manual 时必填 |

**可选：**
| 字段 | JSON Key | 说明 |
|------|----------|------|
| 产品渠道 | `product_channel` | 渠道名称 |
| 参考图 | `reference_file_refs` | reference 对象数组 |
| 优先级 | `priority` | 默认 low |
| 备注 | `remark` | 自由文本 |

**自动推断：**
- `source_mode` 自动设为 `new_product`
- 创建时自动初始化 procurement 记录（draft 状态）
- 前端不应显式传 `source_mode`；后端根据 `task_type` 自动推断

---

## API 调用示例

### 原品开发
```json
{
  "task_type": "original_product_development",
  "product_id": 123,
  "change_request": "修改包装颜色为蓝色",
  "owner_team": "内贸运营组",
  "due_at": "2026-04-01T00:00:00Z",
  "priority": "high"
}
```

### 新品开发
```json
{
  "task_type": "new_product_development",
  "category_code": "HBJ",
  "material_mode": "preset",
  "material": "PU皮",
  "product_name": "新款手提包",
  "product_short_name": "手提包A",
  "design_requirement": "简约风格，主色调为米白色",
  "owner_team": "内贸运营组",
  "due_at": "2026-04-15T00:00:00Z"
}
```

### 采购任务
```json
{
  "task_type": "purchase_task",
  "purchase_sku": "PUR-2026-001",
  "product_name": "标准包装盒",
  "cost_price_mode": "manual",
  "cost_price": 5.50,
  "quantity": 1000,
  "base_sale_price": 12.00,
  "owner_team": "采购仓储组",
  "due_at": "2026-03-30T00:00:00Z"
}
```

---

## 错误响应格式

校验失败时返回带有 `violations` 的错误详情：

```json
{
  "error": {
    "code": "INVALID_REQUEST",
    "message": "purchase_task validation failed",
    "details": {
      "task_type": "purchase_task",
      "source_mode": "new_product",
      "violations": [
        {
          "field": "owner_team",
          "code": "missing_owner_team",
          "message": "owner_team (所属组) is required for task creation"
        }
      ]
    }
  }
}
```
## 2026-03-19 task-create reference upload closure
- Formal reference-image input on `POST /v1/tasks`: `reference_file_refs`
- Formal element type of `reference_file_refs`: reference 对象数组
- `reference_images` is no longer accepted on task creation and now returns `400 INVALID_REQUEST`
- Formal pre-task ref flow:
  - `POST /v1/tasks/reference-upload`
  - request content-type: `multipart/form-data`
  - file field name: `file`
  - append the returned ref object directly into `reference_file_refs`
- Compatibility-only pre-task ref flow:
  - `POST /v1/task-create/asset-center/upload-sessions`
  - `POST /v1/task-create/asset-center/upload-sessions/{session_id}/complete`
- Backend validates every ref against completed legal upload records before create-tx:
  - ref must exist
  - ref must come from a completed upload
  - ref must be reference-eligible
  - forged refs return `400 INVALID_REQUEST` with `invalid_reference_file_refs`

## 2026-03-30 Batch SKU Update

- `owner_team` still uses the legacy compatibility enum validation. This task-create path is not switched to `/v1/org/options`.
- Create-time compatibility now normalizes supported org-team values into legacy task `owner_team` values before validation:
  - `运营一组` ~ `运营七组` -> `内贸运营组`
  - `定制美工组` / `设计审核组` -> `设计组`
  - `采购组` / `仓储组` / `烘焙仓储组` -> `采购仓储组`
- This is only a task-create compatibility bridge. It does not unify task `owner_team` semantics with the account org model, and it does not rewrite historical task data.
- `original_product_development` does not support `batch_sku_mode=multiple` and does not accept `batch_items`.
- `new_product_development` supports one mother task with multiple SKU child items through:
  - `batch_sku_mode=multiple`
  - `batch_items[]` with `product_name`, `product_short_name`, `category_code`, `material_mode`, `design_requirement`, optional `new_sku`, optional `variant_json`
- `purchase_task` supports one mother task with multiple SKU child items through:
  - `batch_sku_mode=multiple`
  - `batch_items[]` with `product_name`, optional `purchase_sku`, `cost_price_mode`, `quantity`, `base_sale_price`, optional `variant_json`
- Batch mode validation rules:
  - `batch_items` is required and must contain at least 2 items
  - top-level single-SKU fields cannot be mixed into multi-SKU create, such as top-level `new_sku`, `purchase_sku`, or `sku_code`
  - request-level duplicate child items are rejected through machine-readable `error.details.violations`
  - batch child SKU codes remain globally unique through `task_sku_items.sku_code`
- Single-SKU creation remains backward-compatible:
  - existing single create payloads keep using `POST /v1/tasks`
  - backend still returns one task record, while new read models also expose additive `sku_items`

## owner_team compatibility guardrail

- task-side `owner_team` is still a legacy compatibility field.
- create-time normalization may map only the explicitly approved org teams below into task-side legacy owner teams:
  - `运营一组` -> `内贸运营组`
  - `运营三组` -> `内贸运营组`
  - `运营七组` -> `内贸运营组`
  - `定制美工组` -> `设计组`
  - `设计审核组` -> `设计组`
  - `采购组` -> `采购仓储组`
  - `仓储组` -> `采购仓储组`
  - `烘焙仓储组` -> `采购仓储组`
- normalization outcomes must stay observable through `mapping_source=legacy_direct`, `mapping_source=org_team_compat`, or `mapping_source=invalid`.
- unsupported values still return `400 INVALID_REQUEST` with `violations[].field=owner_team` and `violations[].code=invalid_owner_team`.
- this is not a full org-model unification.
- `/v1/org/options` teams must not be treated as task `owner_team` values automatically.
- any newly introduced org team that should be accepted by task create must also add an explicit task mapping and regression coverage.
- historical `tasks.owner_team` rows are not rewritten by this guardrail.

## 2026-03-31 canonical task org ownership update

- `tasks.owner_team` is still retained as the legacy task compatibility field.
- New canonical task-side ownership fields now exist:
  - `owner_department`
  - `owner_org_team`
- Create-time behavior is now:
  - backend still accepts legacy `owner_team` values directly
  - backend still accepts explicitly mapped org-team values such as `运营三组` and normalizes them into legacy `owner_team`
  - new tasks also persist canonical `owner_department` / `owner_org_team` when the mapping is deterministic
- Recommended frontend behavior:
  - continue sending `owner_team` for compatibility
  - optionally send `owner_department` / `owner_org_team` as canonical hints
  - read back `owner_team`, `owner_department`, and `owner_org_team` from task detail/list instead of assuming one field is enough
- Reverse mapping from legacy `owner_team` is intentionally conservative:
  - if one legacy owner-team maps to exactly one department, backend may backfill only `owner_department`
  - if one legacy owner-team could map to multiple org teams, backend leaves `owner_org_team` empty rather than fabricating a team
- Historical tasks are not fully rewritten in this round.
- `/v1/tasks` now also supports canonical ownership filters:
  - `owner_department`
  - `owner_org_team`
- Minimal visibility in this round is list/detail read-side only:
  - view-all roles still see all tasks
  - department/team scoped management roles are filtered by canonical task ownership
  - this is still not a full ABAC or row-level visibility engine
## v0.9 authoritative supplement

- Authoritative source-of-truth document: `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
- Official pre-task reference upload route: `POST /v1/tasks/reference-upload`
- Compatibility pre-task reference upload route: `/v1/task-create/asset-center/upload-sessions*`
- Official task-create reference field: `reference_file_refs`
- Rejected create field: `reference_images`
- Canonical task ownership fields: `owner_department`, `owner_org_team`
- Compatibility task ownership field retained at runtime: `owner_team`
- Official task product-code preview route: `POST /v1/tasks/prepare-product-codes`
- `GET/PUT /v1/rule-templates/product-code` is deprecated and returns explicit errors

Use the rest of this file as historical/task-shape detail only. If any older wording below conflicts with the runtime or the v0.9 source-of-truth doc, prefer the runtime and the v0.9 source-of-truth doc.
