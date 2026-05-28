/**
 * POST /v1/tasks 字段白名单（按 task_type 分区，后端 snake_case）
 *
 * 与后端 `service/task_service.go :: validateTaskTypeFieldWhitelist`
 * 保持一致；任何 task_type 下出现 `forbidden` 列表中的字段时，
 * 后端会返回 400 `INVALID_REQUEST` + `details.violations[*].code =
 * 'field_not_allowed_for_task_type'`。
 *
 * 前端作为「UI 门禁」：
 *   1. 表单渲染层：按 task_type v-if 隔离控件（详见 TaskCreateModal /
 *      TaskCreateView / CustomizationCreateDialog），禁止收集 forbidden 字段。
 *   2. 提交层：在调用 POST 前，`sanitizeCreateTaskPayload` 再过滤
 *      一次，防止遗留字段或回归 UI 把脏数据送出。
 *
 * Round I.g · D2 关键不变式：
 *   `customization_required` 是正交 flag，**不是**新的 task_type。
 *   - original + customization_required=true → 现货改款定制
 *   - new + customization_required=true      → 来图定制
 *   `customization_required` 不会放开任何 forbidden 字段；
 *   sanitizer 内禁止检查 `customization_required`。
 */
export const TASK_TYPE_FIELD_WHITELIST = {
  original_product_development: {
    required: [
      'product_selection.erp_product.product_id',
      'change_request',
    ] as readonly string[],
    optional: [
      'reference_file_refs',
      'reference_link',
      'remark',
	      'customization_required',
	      'customization_source_type',
	      'sku_code_type',
	      'designer_id',
      'priority',
      'deadline_at',
      'due_at',
      'owner_department',
      'owner_org_team',
      'owner_team',
      'requester_id',
      'requester_name',
      'defer_local_product_binding',
      'product_selection',
      'product_id',
      'product_name',
      'product_name_snapshot',
      'sku_code',
      // category_code / product_short_name 需要 ERP 快照（product_selection.erp_product 存在）
      // 时由后端进一步校验；sanitizer 不在此 strip。
      'category_code',
      'category_name',
      'product_short_name',
    ] as readonly string[],
    // Round I.g · D2：严格按「唯一不变式」列出 forbidden，与后端
    // task_service.validateTaskTypeFieldWhitelist(original_product_development) 对齐。
    // 不在 forbidden 内的键均放行，由后端做值域/条件校验。
    forbidden: [
      'material_mode',
      'material',
      'material_other',
      'purchase_sku',
      'product_channel',
      'design_requirement',
    ] as readonly string[],
  },
  new_product_development: {
    required: [
      'i_id',
      'product_name',
      'design_requirement',
    ] as readonly string[],
    optional: [
      'reference_file_refs',
      'reference_link',
      'remark',
	      'customization_required',
	      'customization_source_type',
	      'sku_code_type',
	      'designer_id',
      'priority',
      'deadline_at',
      'due_at',
      'owner_department',
      'owner_org_team',
      'owner_team',
      'requester_id',
      'requester_name',
      'product_selection',
      'new_sku',
      'i_id',
      'product_i_id',
      'category_code',
      'category_name',
      'spec_text',
    ] as readonly string[],
    forbidden: [
      'change_request',
      'purchase_sku',
      'product_channel',
      'product_short_name',
      'material_mode',
      'material',
      'material_other',
    ] as readonly string[],
  },
  purchase_task: {
    required: [
      'i_id',
      'product_name',
      'cost_price_mode',
      'quantity',
    ] as readonly string[],
    optional: [
      'cost_price',
      'remark',
	      'customization_required',
	      'sku_code_type',
	      'designer_id',
      'priority',
      'deadline_at',
      'due_at',
      'owner_department',
      'owner_org_team',
      'owner_team',
      'requester_id',
      'requester_name',
      'i_id',
      'product_i_id',
      'category_code',
      'category_name',
      'purchase_sku',
      'spec_text',
    ] as readonly string[],
    forbidden: [
      'change_request',
      'material_mode',
      'material',
      'material_other',
      'product_short_name',
      'design_requirement',
      'reference_link',
      'reference_file_refs',
      'product_channel',
      'base_sale_price',
    ] as readonly string[],
  },
  retouch_task: {
    // 前端门禁：至少 1 条 retouch_requirements[].description + due_at（见 task-create-rules.ts）。
    // demand_text / design_requirement 由首条需求描述兜底写入，reference_file_refs 可为空。
    required: ['retouch_requirements', 'due_at'] as readonly string[],
    optional: [
      'demand_text',
      'design_requirement',
      'reference_file_refs',
      'remark',
      'priority',
      'deadline_at',
      'owner_department',
      'owner_org_team',
      'owner_team',
      'requester_id',
      'requester_name',
      'product_name',
      'product_name_snapshot',
    ] as readonly string[],
    forbidden: [
      'material_mode',
      'material',
      'material_other',
      'purchase_sku',
      'product_channel',
      'product_short_name',
      'category_code',
      'base_sale_price',
      'cost_price',
      'cost_price_mode',
      'quantity',
    ] as readonly string[],
  },
} as const

export type TaskTypeKey = keyof typeof TASK_TYPE_FIELD_WHITELIST

/**
 * 在提交前移除 `raw` 中对应 `task_type` 下所有 `forbidden` 字段。
 *
 * - 不修改入参（深克隆后操作）。
 * - 仅扫描顶层字段；`product_selection.erp_product.*` 等嵌套字段
 *   由后端校验放行（例如 original 允许 erp_product 内嵌 category_code）。
 * - 未知 task_type 原样返回；由调用方另行校验。
 * - **禁止**在本函数内分支 `customization_required`；forbidden 列表严格按
 *   `task_type` 查表（Round I.g · D2 不变式）。
 */
export function sanitizeCreateTaskPayload<T extends Record<string, unknown>>(
  raw: T,
  taskType: string | undefined,
): T {
  if (!taskType) return raw
  const matrix = (TASK_TYPE_FIELD_WHITELIST as Record<string, { forbidden: readonly string[] }>)[taskType]
  if (!matrix) return raw
  let clone: T
  try {
    clone = JSON.parse(JSON.stringify(raw)) as T
  } catch {
    // raw 含不可序列化字段（如 File / Blob）时退回浅拷贝。
    clone = { ...raw }
  }
  for (const key of matrix.forbidden) {
    if (!Object.prototype.hasOwnProperty.call(clone, key)) continue
    const value = (clone as Record<string, unknown>)[key]
    // 仅在「有值」的时候 strip：undefined / null / "" / 空数组 视为不存在，避免
    // 因显式置空触发 field_not_allowed_for_task_type（与后端行为一致）。
    if (value === undefined || value === null) {
      delete (clone as Record<string, unknown>)[key]
      continue
    }
    if (typeof value === 'string' && value.trim() === '') {
      delete (clone as Record<string, unknown>)[key]
      continue
    }
    if (Array.isArray(value) && value.length === 0) {
      delete (clone as Record<string, unknown>)[key]
      continue
    }
    delete (clone as Record<string, unknown>)[key]
  }
  return clone
}

/**
 * 从后端 violations 数组中提取「字段白名单违规」项。
 * 用于在 UI 上定位到具体出错字段并引导用户。
 */
export function pickFieldWhitelistViolations(
  violations: Array<Record<string, unknown>> | undefined,
): string[] {
  if (!Array.isArray(violations)) return []
  return violations
    .filter((v) => String(v?.code ?? '') === 'field_not_allowed_for_task_type')
    .map((v) => String(v?.field ?? '').trim())
    .filter((f) => f !== '')
}

/**
 * Round I.g · D4：把后端 violations 的英文字段名翻译成中文 label，供 toast 文案使用。
 *
 * 未在映射表里的字段原样返回 snake_case，方便定位。
 */
export const TASK_CREATE_FIELD_CN_LABELS: Readonly<Record<string, string>> = Object.freeze({
  task_type: '任务类型',
  owner_department: '所属部门',
  owner_org_team: '所属运营组',
  owner_team: '所属组',
  change_request: '修改要求',
  material_mode: '材料方式',
  material: '材料',
  material_other: '自定义材料',
  purchase_sku: '采购 SKU',
  retouch_task: 'P 图任务',
  product_channel: '产品渠道',
  design_requirement: '设计要求',
  reference_link: '参考链接',
  reference_file_refs: '参考图',
  product_short_name: '产品简称',
  i_id: '款式编码',
  product_i_id: '款式编码（兼容字段）',
  category_code: '分类编码',
  category_name: '分类名称',
  customization_required: '定制 flag',
  customization_source_type: '定制来源类型',
  product_selection: '产品选品',
  product_id: '产品 ID',
  product_name: '产品名称',
  sku_code: 'SKU 编码',
  cost_price_mode: '成本计价方式',
  cost_price: '成本',
  quantity: '数量',
  base_sale_price: '基本售价',
  spec_text: '规格尺寸',
  new_sku: '新品 SKU',
})

export function humanizeTaskCreateFields(fields: readonly string[]): string[] {
  return fields.map((f) => TASK_CREATE_FIELD_CN_LABELS[f] ?? f)
}

/**
 * 把后端 violations[].code 映射为用户可读的中文提示。
 * 未识别的 code 返回空字符串，由调用方走通用 fallback。
 */
export function humanizeViolationCode(code: string, field: string): string {
  if (code === 'erp_product_name_too_long') return '产品名称不能超过 50 个字符，请精简后再提交，避免同步聚水潭失败'
  if (code === 'insufficient_batch_items') return '批量 SKU 模式下至少需要 2 个商品'
  if (code === 'missing_required_field') {
    const leaf = field.replace(/^.*[\].]\s*/, '') || field
    const label = humanizeTaskCreateFields([leaf])[0] ?? field
    if (leaf === 'i_id') return `必填字段缺失：${label} (i_id)`
    return `必填字段缺失：${label}`
  }
  if (code === 'field_not_allowed_for_task_type') {
    const leaf = field.replace(/^.*[\].]\s*/, '') || field
    const label = humanizeTaskCreateFields([leaf])[0] ?? field
    return `字段不符合当前任务类型：${label}`
  }
  return ''
}
