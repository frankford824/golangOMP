import type { PlanningSKUInput } from '@/services/api/planningSkuApi'
import type { Task, TaskBatchItem } from '@/domain/types'
import { isErpProductNameTooLong } from '@/domain/erp-product-name'
import { beijingDateTimeLocalToISO, taskInstantMs } from '@/utils/date'
import { generateActionId } from '@/utils/uuid'

export type ComposeIntent = 'modify_existing' | 'new_design' | 'retouch' | 'planning_sku'
export type ComposePriority = 'low' | 'normal' | 'high' | 'critical'
export type ComposeColumnKey =
  | 'erp_sku'
  | 'product_i_id'
  | 'product_name'
  | 'category_code'
  | 'design_requirement'
  | 'description_spec'
  | 'quantity'
  | 'target_price'
  | 'note'
  | 'reference_url'
  | 'width'
  | 'height'
  | 'area'
  | 'special_note'
  | 'reference_assets'
  | 'source_assets'
  | 'set_mode_hint'

export interface ComposeAssetDraft {
  id: string
  file?: File
  name: string
  preview_url?: string
  upload_ref?: string | Record<string, unknown>
  status: 'local' | 'uploading' | 'uploaded' | 'failed'
  error?: string
}

export interface ComposeRow {
  id: string
  erp_product_id?: string
  erp_sku?: string
  erp_product_snapshot?: Record<string, unknown>
  product_i_id?: string
  product_name?: string
  category_code?: string
  design_requirement?: string
  description_spec?: string
  quantity?: number
  target_price?: string
  note?: string
  reference_url?: string
  width?: number
  height?: number
  area?: number
  special_note?: string
  reference_assets: ComposeAssetDraft[]
  source_assets: ComposeAssetDraft[]
  set_mode_hint: boolean
  status?: 'draft' | 'submitting' | 'created' | 'failed'
  result_task_id?: string
  result_sku_code?: string
  error?: string
}

export interface ComposeCommonInfo {
  due_at: string
  priority: ComposePriority
  note: string
  customization_required: boolean
  customization_source_type?: 'new_product' | 'existing_product'
  erp_sync_mode: 'none' | 'async'
  designer_id?: string
}

export interface ComposeColumn {
  key: ComposeColumnKey
  label: string
  width: number
  required?: boolean
  kind?: 'text' | 'number' | 'boolean' | 'asset'
  help?: string
}

export interface ComposeViolation {
  row_id?: string
  row_index?: number
  field: ComposeColumnKey | 'due_at' | 'customization_source_type' | 'rows'
  message: string
}

export interface TaskSubmissionUnit {
  row_ids: string[]
  task: Partial<Task> & {
    skuMode?: 'single' | 'multiple'
    batchItems?: TaskBatchItem[]
  }
}

export const COMPOSE_INTENT_META: Record<ComposeIntent, { title: string; summary: string; badge: string }> = {
  modify_existing: { title: '改现有的款', summary: '选一个已有商品，写清楚要改成什么样', badge: '需设计与审核' },
  new_design: { title: '做新款', summary: '做全新的产品，填几行就出几个新 SKU', badge: '需设计与审核' },
  retouch: { title: '只修图', summary: '只需要修图片，改完直接结单', badge: '无需审核' },
  planning_sku: { title: '只要 SKU 编码', summary: '不用设计，马上拿到一批新编码', badge: '立即出码' },
}

const columns: Record<ComposeIntent, ComposeColumn[]> = {
  modify_existing: [
    { key: 'erp_sku', label: 'ERP 商品 / SKU', width: 170, required: true },
    { key: 'product_name', label: '产品名称', width: 180, required: true },
    { key: 'design_requirement', label: '修改要求', width: 280, required: true },
    { key: 'width', label: '宽', width: 82, kind: 'number' },
    { key: 'height', label: '高', width: 82, kind: 'number' },
    { key: 'area', label: '面积', width: 92, kind: 'number' },
    { key: 'special_note', label: '特殊说明', width: 180 },
    { key: 'reference_assets', label: '参考图', width: 120, kind: 'asset' },
    { key: 'set_mode_hint', label: '建议套装', width: 102, kind: 'boolean', help: '仅供设计师参考，最终由设计阶段判定' },
  ],
  new_design: [
    { key: 'product_i_id', label: '款式编码 i_id', width: 150, required: true },
    { key: 'product_name', label: '产品名称', width: 180, required: true },
    { key: 'design_requirement', label: '设计需求', width: 280, required: true },
    { key: 'width', label: '宽', width: 82, kind: 'number' },
    { key: 'height', label: '高', width: 82, kind: 'number' },
    { key: 'area', label: '面积', width: 92, kind: 'number' },
    { key: 'special_note', label: '特殊说明', width: 180 },
    { key: 'reference_assets', label: '参考图', width: 120, kind: 'asset' },
    { key: 'set_mode_hint', label: '建议套装', width: 102, kind: 'boolean', help: '仅供设计师参考，最终由设计阶段判定' },
  ],
  retouch: [
    { key: 'design_requirement', label: '修图要求', width: 360, required: true },
    { key: 'reference_assets', label: '参考图', width: 140, kind: 'asset' },
    { key: 'source_assets', label: '待修素材', width: 140, kind: 'asset' },
    { key: 'special_note', label: '补充说明', width: 220 },
  ],
  planning_sku: [
    { key: 'category_code', label: '编号类目（非 ERP 款式）', width: 190, required: true, help: '只用于计算 CG/DZ 编号中的 1 位类目短码，不会同步成 ERP 商品编码' },
    { key: 'description_spec', label: '产品描述 / 规格', width: 310, required: true },
    { key: 'quantity', label: '数量', width: 96, required: true, kind: 'number' },
    { key: 'target_price', label: '目标价', width: 110 },
    { key: 'note', label: '备注', width: 210 },
    { key: 'reference_url', label: '参考链接', width: 220 },
    { key: 'reference_assets', label: '产品图片', width: 140, kind: 'asset' },
    { key: 'product_i_id', label: 'ERP 款式编码 i_id', width: 170, help: '仅用于 ERP 建档；ERP 商品名称直接取“产品描述 / 规格”，无需重复填写' },
  ],
}

export function composeColumns(intent: ComposeIntent): ComposeColumn[] {
  return columns[intent]
}

/**
 * 与统一工作台之前的两个入口保持一致：新款设计与策划 SKU 都默认同步 ERP。
 * 策划 SKU 只要求填写 ERP 款式编码，商品名称直接复用已经必填的产品描述 / 规格。
 */
export function defaultErpSyncMode(intent: ComposeIntent): ComposeCommonInfo['erp_sync_mode'] {
  return intent === 'new_design' || intent === 'planning_sku' ? 'async' : 'none'
}

export function createComposeRow(seed: Partial<ComposeRow> = {}): ComposeRow {
  return {
    id: seed.id || generateActionId(),
    reference_assets: seed.reference_assets ? [...seed.reference_assets] : [],
    source_assets: seed.source_assets ? [...seed.source_assets] : [],
    set_mode_hint: seed.set_mode_hint ?? false,
    status: seed.status ?? 'draft',
    ...seed,
  }
}

export function validateCompose(
  intent: ComposeIntent,
  common: ComposeCommonInfo,
  rows: ComposeRow[],
  now: Date = new Date(),
): ComposeViolation[] {
  const violations: ComposeViolation[] = []
  if (!rows.length) violations.push({ field: 'rows', message: '至少添加一行任务内容' })
  if (!common.due_at && intent !== 'planning_sku') violations.push({ field: 'due_at', message: '请选择截止时间' })
  if (common.due_at && intent !== 'planning_sku') {
    const dueAt = beijingDateTimeLocalToISO(common.due_at)
    if (!dueAt) violations.push({ field: 'due_at', message: '截止时间格式无效，请重新选择' })
    else if (taskInstantMs(dueAt) < now.getTime() - 60_000) violations.push({ field: 'due_at', message: '截止时间不能早于当前时间' })
  }
  if (common.customization_required && !common.customization_source_type) {
    violations.push({ field: 'customization_source_type', message: '定制任务请选择来源类型' })
  }
  if (intent === 'planning_sku' && rows.length > 200) {
    violations.push({ field: 'rows', message: '一次最多 200 行，请分批提交' })
  }
  if (intent === 'modify_existing' && rows.length > 50) {
    violations.push({ field: 'rows', message: '一次最多 50 行，请分批提交' })
  }
  if ((intent === 'new_design' || intent === 'retouch') && rows.length > 100) {
    violations.push({ field: 'rows', message: '一次最多 100 行，请分批提交' })
  }
  const dimensionLabels = { width: '宽', height: '高', area: '面积' } as const
  rows.forEach((row, rowIndex) => {
    const add = (field: ComposeViolation['field'], message: string) => violations.push({ row_id: row.id, row_index: rowIndex, field, message })
    for (const key of ['width', 'height', 'area'] as const) {
      const value = row[key]
      if (typeof value === 'number' && Number.isNaN(value)) add(key, `${dimensionLabels[key]}只能填数字`)
      else if (typeof value === 'number' && value < 0) add(key, `${dimensionLabels[key]}不能小于 0`)
    }
    if (intent === 'modify_existing') {
      if (!row.erp_product_id || !row.erp_sku?.trim()) add('erp_sku', '请从 ERP 搜索结果中选择商品，不能只手填 SKU')
      if (!row.product_name?.trim()) add('product_name', '产品名称不能为空')
      if (isErpProductNameTooLong(row.product_name)) add('product_name', '产品名称不能超过 40 个字')
      if (!row.design_requirement?.trim()) add('design_requirement', '请填写修改要求')
    } else if (intent === 'new_design') {
      if (!row.product_i_id?.trim()) add('product_i_id', '请选择款式编码 i_id')
      if (!row.product_name?.trim()) add('product_name', '产品名称不能为空')
      if (isErpProductNameTooLong(row.product_name)) add('product_name', '产品名称不能超过 40 个字')
      if (!row.design_requirement?.trim()) add('design_requirement', '请填写设计需求')
    } else if (intent === 'retouch') {
      if (!row.design_requirement?.trim()) add('design_requirement', '请填写修图要求')
    } else {
      if (!row.category_code?.trim()) add('category_code', '请填写 SKU 类目，用于生成旧采购口径编号')
      const description = row.description_spec?.trim() ?? ''
      if (!description) add('description_spec', '产品描述 / 规格不能为空')
      if (description.length > 4000) add('description_spec', '产品描述 / 规格不能超过 4000 字')
      if (!Number.isInteger(row.quantity) || Number(row.quantity) <= 0) add('quantity', '数量必须是正整数')
      if (row.target_price && !/^\d{1,10}(\.\d{1,2})?$/.test(row.target_price)) add('target_price', '目标价最多 10 位整数、2 位小数')
      if ((row.note?.length ?? 0) > 2000) add('note', '备注不能超过 2000 字')
      if (row.reference_url && !/^https?:\/\//i.test(row.reference_url)) add('reference_url', '参考链接仅支持 HTTP / HTTPS')
      if (common.erp_sync_mode === 'async' && !row.product_i_id?.trim()) add('product_i_id', '开启 ERP 同步时 i_id 必填')
    }
    for (const [field, assets] of [['reference_assets', row.reference_assets], ['source_assets', row.source_assets]] as const) {
      for (const asset of assets) {
        if (asset.status === 'uploading') add(field, '仍有文件正在上传')
        if (asset.status === 'failed') add(field, asset.error || '文件上传失败，请重试')
        if (asset.file && asset.file.size > 20 * 1024 * 1024) add(field, `${asset.name} 超过 20 MB，请压缩或拆分后重试`)
      }
    }
    if (row.reference_assets.length > (intent === 'planning_sku' ? 1 : 5)) add('reference_assets', intent === 'planning_sku' ? '每个策划 SKU 只能上传一张产品图' : '每行最多上传 5 张参考图')
  })
  return violations
}

function uploadedRefs(assets: ComposeAssetDraft[]): Array<string | Record<string, unknown>> {
  return assets.map((asset) => asset.upload_ref).filter((value): value is string | Record<string, unknown> => Boolean(value))
}

function dimensionVariant(row: ComposeRow): Record<string, unknown> | undefined {
  const value: Record<string, unknown> = {}
  if (Number.isFinite(row.width) && Number(row.width) >= 0) value.width = row.width
  if (Number.isFinite(row.height) && Number(row.height) >= 0) value.height = row.height
  if (Number.isFinite(row.area) && Number(row.area) >= 0) value.area = row.area
  if (row.special_note?.trim()) value.special_note = row.special_note.trim()
  if (row.set_mode_hint) value.set_mode_hint = true
  return Object.keys(value).length ? value : undefined
}

function commonTask(common: ComposeCommonInfo): Partial<Task> {
  const dueAt = beijingDateTimeLocalToISO(common.due_at) ?? undefined
  return {
    dueAt,
    priority: common.priority,
    note: common.note,
    customizationRequired: common.customization_required,
    customizationSourceType: common.customization_source_type,
    businessLane: common.customization_required ? 'customization' : 'normal',
    assigneeId: common.designer_id ?? null,
    designerId: common.designer_id ?? null,
  }
}

function rowTaskNote(commonNote: string, specialNote?: string): string {
  const parts = [commonNote.trim(), specialNote?.trim() ?? ''].filter(Boolean)
  return parts.join('\n')
}

/**
 * POST /v1/tasks 的 `client_create_id` 上限是 128 字符（openapi.yaml CreateTaskRequest，
 * 且 task_create_requests.client_create_id 为 VARCHAR(128)）。行 id 是 36 字符 UUID，
 * 直接拼接会在 3 行起越界并让后端在预留幂等记录时报 500，所以这里压成定长摘要。
 * 摘要覆盖行集合本身，"只重试失败行"会得到不同的键，不会被误判为重放。
 */
export function composeIdempotencyKey(sessionId: string, rowIds: string[]): string {
  const canonical = [...rowIds].sort().join(',')
  let hash = 0xcbf29ce484222325n
  for (let index = 0; index < canonical.length; index += 1) {
    hash ^= BigInt(canonical.charCodeAt(index))
    hash = BigInt.asUintN(64, hash * 0x100000001b3n)
  }
  return `compose:${sessionId}:${rowIds.length}-${hash.toString(16).padStart(16, '0')}`
}

export function buildTaskSubmissionUnits(intent: Exclude<ComposeIntent, 'planning_sku'>, common: ComposeCommonInfo, rows: ComposeRow[]): TaskSubmissionUnit[] {
  const shared = commonTask(common)
  if (intent === 'modify_existing') {
    return rows.map((row) => ({
      row_ids: [row.id],
      task: {
        ...shared,
        taskType: 'ORIGINAL_PRODUCT_DEV',
        // The by-code ERP lookup exposes an external product code, not a local
        // products.id. Keep the external identity in the ERP snapshot so the
        // backend can resolve or create the local binding safely.
        productId: null,
        sku: row.erp_sku ?? null,
        productName: row.product_name ?? '',
        erpProductSnapshot: row.erp_product_snapshot,
        designRequirement: row.design_requirement ?? '',
        width: row.width,
        height: row.height,
        area: row.area,
        note: rowTaskNote(common.note, row.special_note),
        referenceFileRefs: uploadedRefs(row.reference_assets),
        setModeHint: row.set_mode_hint,
      } as Partial<Task>,
    }))
  }
  if (intent === 'retouch') {
    return [{
      row_ids: rows.map((row) => row.id),
      task: {
        ...shared,
        taskType: 'RETOUCH_TASK',
        productName: rows.length === 1 ? '修图任务' : `批量修图（${rows.length} 项）`,
        designRequirement: rows.map((row) => row.design_requirement?.trim()).filter(Boolean).join('；'),
        retouchRequirements: rows.map((row, index) => ({
          description: row.design_requirement?.trim() ?? '',
          remark: row.special_note?.trim() ?? '',
          sortOrder: index + 1,
          pendingReferenceFiles: row.reference_assets.map((asset) => asset.file).filter((file): file is File => Boolean(file)),
          pendingSourceFiles: row.source_assets.map((asset) => asset.file).filter((file): file is File => Boolean(file)),
        })),
        referenceFileRefs: rows.flatMap((row) => uploadedRefs(row.reference_assets)),
      } as unknown as Partial<Task>,
    }]
  }
  const batchItems: TaskBatchItem[] = rows.map((row) => ({
    clientKey: row.id,
    productName: row.product_name?.trim() ?? '',
    productIId: row.product_i_id?.trim(),
    categoryCode: row.product_i_id?.trim(),
    designRequirement: row.design_requirement?.trim(),
    referenceFileRefs: uploadedRefs(row.reference_assets),
    variantJson: dimensionVariant(row),
    setModeHint: row.set_mode_hint,
  } as TaskBatchItem))
  if (rows.length === 1) {
    const row = rows[0]
    return [{
      row_ids: [row.id],
      task: {
        ...shared,
        taskType: 'NEW_PRODUCT_DEV',
        productName: row.product_name ?? '',
        category: row.product_i_id,
        designRequirement: row.design_requirement ?? '',
        width: row.width,
        height: row.height,
        area: row.area,
        note: rowTaskNote(common.note, row.special_note),
        referenceFileRefs: uploadedRefs(row.reference_assets),
        setModeHint: row.set_mode_hint,
        syncErpOnCreate: common.erp_sync_mode === 'async',
      } as Partial<Task>,
    }]
  }
  return [{
    row_ids: rows.map((row) => row.id),
    task: {
      ...shared,
      taskType: 'NEW_PRODUCT_DEV',
      skuMode: 'multiple',
      batchItems,
      referenceFileRefs: rows.flatMap((row) => uploadedRefs(row.reference_assets)),
      syncErpOnCreate: common.erp_sync_mode === 'async',
    } as TaskSubmissionUnit['task'],
  }]
}

export function buildPlanningInputs(rows: ComposeRow[], customizationRequired = false): PlanningSKUInput[] {
  return rows.map((row) => ({
    client_item_id: row.id,
    category_code: row.category_code?.trim() ?? '',
    sku_code_type: customizationRequired ? 'customization' : 'regular',
    description_spec: row.description_spec?.trim() ?? '',
    quantity: Number(row.quantity),
    target_price: row.target_price?.trim() || undefined,
    note: row.note?.trim() || undefined,
    reference_url: row.reference_url?.trim() || undefined,
    image_upload_ref: typeof row.reference_assets[0]?.upload_ref === 'string' ? row.reference_assets[0].upload_ref : undefined,
    erp_product_i_id: row.product_i_id?.trim() || undefined,
    erp_product_name: row.product_name?.trim() || row.description_spec?.trim() || undefined,
  }))
}

export function applyBackendViolations(rows: ComposeRow[], raw: unknown): ComposeViolation[] {
  const payload = raw && typeof raw === 'object' ? raw as Record<string, unknown> : {}
  const error = payload.error && typeof payload.error === 'object' ? payload.error as Record<string, unknown> : payload
  const details = error.details && typeof error.details === 'object' ? error.details as Record<string, unknown> : {}
  const items = Array.isArray(details.violations) ? details.violations as Array<Record<string, unknown>> : []
  return items.map((item) => {
    const fieldPath = String(item.field ?? '')
    const match = fieldPath.match(/(?:batch_items|planning_sku_items|retouch_requirements)\[(\d+)]\.([a-z0-9_]+)/i)
    const rowIndex = match ? Number(match[1]) : undefined
    return {
      row_id: rowIndex != null ? rows[rowIndex]?.id : undefined,
      row_index: rowIndex,
      field: (match?.[2] || fieldPath || 'rows') as ComposeViolation['field'],
      message: String(item.message ?? item.reason ?? item.code ?? '提交内容不符合要求'),
    }
  })
}
