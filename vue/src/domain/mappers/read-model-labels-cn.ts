/**
 * 读模型展示用中文标签：内存与请求仍保留后端英文 code/枚举，仅在 UI 层映射。
 */

const WORKFLOW_GATE_REASON_CN: Record<string, string> = {
  category_missing: '缺少商品分类',
  spec_text_missing: '缺少规格说明',
  procurement_price_missing: '缺少采购单价（若已填成本单价仍提示，请按后端要求补采购价字段）',
  procurement_quantity_missing: '缺少采购数量',
  warehouse_already_received: '仓库已接收该任务',
  warehouse_already_completed: '仓库流程已完成',
  warehouse_not_received: '仓库尚未确认接收',
  not_pending_close: '任务不在待结单状态',
  filed_at_missing: '缺少建档/归档时间或与 ERP 的关联信息',
  erp_linkage_missing: '缺少与 ERP 的关联信息',
  filed_erp_linkage_missing: '缺少建档或与 ERP 的关联信息',
  design_not_finalized: '设计尚未终稿',
  sku_not_bound: '未绑定 SKU',
  task_no_missing: '缺少任务号',
  audit_not_approved: '审核尚未通过',
  missing_final_design_asset: '缺少最终设计资产',
}

function normSnake(code: string | undefined | null): string {
  return (code ?? '').trim().toLowerCase()
}

/** workflow.cannot_close_reasons / warehouse_blocking_reasons 等门禁 code → 中文短句 */
export function workflowGateReasonLabelCn(
  code: string | undefined | null,
  fallbackMessage?: string | null,
): string {
  const k = normSnake(code)
  if (k && WORKFLOW_GATE_REASON_CN[k]) return WORKFLOW_GATE_REASON_CN[k]
  const msg = (fallbackMessage ?? '').trim()
  if (msg) return msg
  return k || '未知原因'
}

const SKU_GENERATION_STATUS_CN: Record<string, string> = {
  pending: '待生成',
  not_started: '未开始',
  queued: '排队中',
  in_progress: '生成中',
  processing: '生成中',
  running: '生成中',
  partial: '部分完成',
  completed: '已完成',
  done: '已完成',
  success: '已完成',
  failed: '失败',
  error: '失败',
  skipped: '已跳过',
  cancelled: '已取消',
  canceled: '已取消',
}

export function skuGenerationStatusLabelCn(raw: string | undefined | null): string {
  const k = normSnake(raw)
  if (!k) return '—'
  return SKU_GENERATION_STATUS_CN[k] ?? (raw ?? '').trim()
}

const SKU_ITEM_STATUS_CN: Record<string, string> = {
  generated: '已生成',
  draft: '草稿',
  pending: '待处理',
  active: '已生效',
  filed: '建档完成',
  archived: '已归档',
  cancelled: '已取消',
  canceled: '已取消',
  warehouse_received: '仓库已接收',
  received: '已接收',
  pending_audit: '待审核',
  in_audit: '审核中',
  audit_pending: '待审核',
  design_in_progress: '设计中',
}

export function skuItemStatusLabelCn(raw: string | undefined | null): string {
  const k = normSnake(raw)
  if (!k) return '—'
  return SKU_ITEM_STATUS_CN[k] ?? (raw ?? '').trim()
}

const MATERIAL_MODE_CN: Record<string, string> = {
  OTHER: '其他',
  PLASTIC: '塑料',
  METAL: '金属',
  WOOD: '木质',
  GLASS: '玻璃',
  FABRIC: '织物',
  CERAMIC: '陶瓷',
  PAPER: '纸质',
  COMPOSITE: '复合材料',
  LEATHER: '皮革',
  RUBBER: '橡胶',
}

export function materialModeLabelCn(
  raw: string | undefined | null,
  otherDetail?: string | null,
): string {
  const t = (raw ?? '').trim()
  if (!t) return '—'
  const upper = t.toUpperCase()
  if (upper === 'OTHER') return (otherDetail ?? '').trim() || '其他'
  if (MATERIAL_MODE_CN[upper]) return MATERIAL_MODE_CN[upper]
  return t
}

const PRODUCT_SELECTION_MATCH_CN: Record<string, string> = {
  exact: '精确匹配',
  fuzzy: '模糊匹配',
  manual: '手动指定',
  none: '无',
  auto: '自动',
}

export function productSelectionSourceMatchTypeLabelCn(raw: string | undefined | null): string {
  const k = normSnake(raw)
  if (!k) return '—'
  return PRODUCT_SELECTION_MATCH_CN[k] ?? (raw ?? '').trim()
}

const ASSET_KIND_CN: Record<string, string> = {
  reference: '运营参考图',
  source: '设计源文件 / 审核修订源文件',
  delivery: '最终成品图',
  preview: '预览辅助',
  design_thumb: '预览辅助',
}

export function assetKindLabelCn(raw: string | undefined | null): string {
  const k = normSnake(raw)
  if (!k) return '—'
  return ASSET_KIND_CN[k] ?? (raw ?? '').trim()
}

const ASSET_UPLOAD_STATUS_CN: Record<string, string> = {
  pending: '待上传',
  uploading: '上传中',
  uploaded: '已上传',
  processing: '处理中',
  completed: '已完成',
  success: '成功',
  failed: '失败',
  error: '失败',
  cancelled: '已取消',
  canceled: '已取消',
}

export function assetUploadStatusLabelCn(raw: string | undefined | null): string {
  const k = normSnake(raw)
  if (!k) return '—'
  return ASSET_UPLOAD_STATUS_CN[k] ?? (raw ?? '').trim()
}

const ASSET_ARCHIVE_STATUS_CN: Record<string, string> = {
  active: '生效中',
  archived: '已归档',
  pending_archive: '待归档',
  archive_failed: '归档失败',
}

export function assetArchiveStatusLabelCn(raw: string | undefined | null): string {
  const k = normSnake(raw)
  if (!k) return '—'
  return ASSET_ARCHIVE_STATUS_CN[k] ?? (raw ?? '').trim()
}

const ASSET_DOWNLOAD_MODE_CN: Record<string, string> = {
  direct: '直链下载',
  proxy: '代理下载',
  public: '公网访问',
  private_network: '内网访问',
}

export function assetDownloadModeLabelCn(raw: string | undefined | null): string {
  const k = normSnake(raw)
  if (!k) return '—'
  return ASSET_DOWNLOAD_MODE_CN[k] ?? (raw ?? '').trim()
}

const CUSTOMIZATION_JOB_STATUS_CN: Record<string, string> = {
  pending_customization_review: '定制中',
  pending_customization_production: '待定制生产',
  pending_effect_review: '待效果审核',
  pending_effect_revision: '待效果返修',
  pending_production_transfer: '待生产流转',
  pending_warehouse_qc: '待仓库质检',
  rejected_by_warehouse: '仓库驳回',
  completed: '已完成',
}

export function customizationJobStatusLabelCn(raw: string | undefined | null): string {
  const k = normSnake(raw)
  if (!k) return '—'
  return CUSTOMIZATION_JOB_STATUS_CN[k] ?? (raw ?? '').trim()
}

const CUSTOMIZATION_REVIEW_DECISION_CN: Record<string, string> = {
  approved: '通过',
  return_to_designer: '退回设计',
  reviewer_fixed: '审核人修正',
}

export function customizationReviewDecisionLabelCn(raw: string | undefined | null): string {
  const k = normSnake(raw)
  if (!k) return '—'
  return CUSTOMIZATION_REVIEW_DECISION_CN[k] ?? (raw ?? '').trim()
}

const EMPLOYMENT_TYPE_CN: Record<string, string> = {
  full_time: '全职',
  part_time: '兼职',
}

export function employmentTypeLabelCn(raw: string | undefined | null): string {
  const k = normSnake(raw)
  if (!k) return '—'
  return EMPLOYMENT_TYPE_CN[k] ?? (raw ?? '').trim()
}

const WAREHOUSE_REJECT_CATEGORY_CN: Record<string, string> = {
  quality_issue: '质量问题',
  spec_mismatch: '规格不匹配',
  missing_files: '文件缺失',
  wrong_asset: '资产错误',
  other: '其他',
}

export function warehouseRejectCategoryLabelCn(raw: string | undefined | null): string {
  const k = normSnake(raw)
  if (!k) return '—'
  return WAREHOUSE_REJECT_CATEGORY_CN[k] ?? (raw ?? '').trim()
}
