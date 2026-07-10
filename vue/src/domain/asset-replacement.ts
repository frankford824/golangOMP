export interface AssetReplacementGateInput {
  isExternal?: boolean
  taskId?: unknown
  assetId?: unknown
  assetKind?: unknown
  usableState?: unknown
  taskStatus?: unknown
  isArchived?: unknown
  archiveStatus?: unknown
}

const REPLACEABLE_ASSET_KINDS = new Set(['delivery', 'source', 'reference'])

const TASK_ASSET_UPLOAD_SESSION_STATUSES = new Set([
  'PendingAssign',
  'InProgress',
  'PendingAuditA',
  'RejectedByAuditA',
  'PendingAuditB',
  'RejectedByAuditB',
  'PendingOutsourceReview',
  'PendingCustomizationReview',
  'PendingCustomizationProduction',
  'PendingEffectReview',
  'PendingEffectRevision',
  'PendingProductionTransfer',
  'RejectedByWarehouse',
  'Completed',
])

const TASK_STATUS_ALIASES: Record<string, string> = {
  draft: 'Draft',
  pending_assign: 'PendingAssign',
  assigned: 'Assigned',
  in_progress: 'InProgress',
  pending_audit_a: 'PendingAuditA',
  rejected_by_audit_a: 'RejectedByAuditA',
  pending_audit_b: 'PendingAuditB',
  rejected_by_audit_b: 'RejectedByAuditB',
  pending_outsource: 'PendingOutsource',
  outsourcing: 'Outsourcing',
  pending_outsource_review: 'PendingOutsourceReview',
  pending_customization_review: 'PendingCustomizationReview',
  pending_customization_production: 'PendingCustomizationProduction',
  pending_effect_review: 'PendingEffectReview',
  pending_effect_revision: 'PendingEffectRevision',
  pending_production_transfer: 'PendingProductionTransfer',
  pending_warehouse_qc: 'PendingWarehouseQC',
  rejected_by_warehouse: 'RejectedByWarehouse',
  pending_warehouse_receive: 'PendingWarehouseReceive',
  pending_close: 'PendingClose',
  completed: 'Completed',
  archived: 'Archived',
  blocked: 'Blocked',
  cancelled: 'Cancelled',
}

const TASK_STATUS_LABELS: Record<string, string> = {
  Draft: '草稿',
  Assigned: '已分配',
  PendingOutsource: '待外包',
  Outsourcing: '外包中',
  PendingWarehouseQC: '仓库质检中',
  PendingWarehouseReceive: '仓库待收货',
  PendingClose: '待结单',
  Completed: '已结单',
  Archived: '已归档',
  Blocked: '已阻塞',
  Cancelled: '已取消',
}

function normalizeTaskStatus(value: unknown): string {
  const raw = String(value ?? '').trim()
  if (!raw) return ''
  if (TASK_ASSET_UPLOAD_SESSION_STATUSES.has(raw) || TASK_STATUS_LABELS[raw]) return raw
  const key = raw.replace(/-/g, '_').replace(/([a-z0-9])([A-Z])/g, '$1_$2').toLowerCase()
  return TASK_STATUS_ALIASES[key] ?? raw
}

function normalizeKind(value: unknown): string {
  return String(value ?? '').trim().toLowerCase()
}

function hasPositiveNumericId(value: unknown): boolean {
  if (typeof value === 'number') return Number.isSafeInteger(value) && value > 0
  const raw = String(value ?? '').trim()
  return /^[1-9]\d*$/.test(raw)
}

export function taskStatusBlocksAssetReplacement(statusValue: unknown): string {
  const status = normalizeTaskStatus(statusValue)
  if (!status || TASK_ASSET_UPLOAD_SESSION_STATUSES.has(status)) return ''
  const label = TASK_STATUS_LABELS[status] ?? status
  return `所属任务当前为「${label}」，不能在资产管理直接修改资源`
}

export function assetReplacementUnavailableReason(input: AssetReplacementGateInput): string {
  if (input.isExternal) return '外部资源暂不支持在资产管理直接修改'
  if (!hasPositiveNumericId(input.taskId) || !hasPositiveNumericId(input.assetId)) {
    return '当前资源缺少任务或资产信息，不能在资产管理直接修改'
  }
  if (!REPLACEABLE_ASSET_KINDS.has(normalizeKind(input.assetKind))) {
    return '当前资源不可修改；只有系统内的参考图、源文件、最终成品图可替换'
  }
  const archived = input.isArchived === true || normalizeKind(input.isArchived) === 'true'
  if (archived || normalizeKind(input.archiveStatus) === 'archived') {
    return '已归档资源不可修改，请先由超级管理员恢复资源'
  }
  const usableState = normalizeKind(input.usableState)
  if (usableState === 'history' || usableState === 'cleaned') {
    return '历史版本或已清理资源不可修改，请选择当前有效资源'
  }
  return taskStatusBlocksAssetReplacement(input.taskStatus)
}

export function canReplaceAssetResource(input: AssetReplacementGateInput): boolean {
  return assetReplacementUnavailableReason(input) === ''
}
