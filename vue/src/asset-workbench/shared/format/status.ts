export type StatusTone = 'success' | 'warn' | 'danger' | 'info' | 'neutral' | 'money'

export interface StatusMeta {
  label: string
  tone: StatusTone
}

type StatusMap = Record<string, StatusMeta>

const fallback = (value: string, tone: StatusTone = 'neutral'): StatusMeta => ({
  label: value || '未设置',
  tone,
})

function lookup(map: StatusMap, value?: string | null, tone: StatusTone = 'neutral') {
  const key = value || ''
  return map[key] ?? fallback(key, tone)
}

export function chipClass(tone: StatusTone) {
  return `aw-chip aw-chip--${tone}`
}

export const workerTypeMeta = (value?: string | null) =>
  lookup(
    {
      all: { label: '全部', tone: 'neutral' },
      fulltime: { label: '全职', tone: 'info' },
      parttime: { label: '兼职', tone: 'success' },
    },
    value,
  )

export const submissionStatusMeta = (value?: string | null) =>
  lookup(
    {
      submitted: { label: '已提交', tone: 'info' },
      checked: { label: '已通过', tone: 'success' },
      needs_fix: { label: '需修正', tone: 'warn' },
      voided: { label: '已作废', tone: 'danger' },
    },
    value,
  )

export const qcStatusMeta = (value?: string | null) =>
  lookup(
    {
      pending: { label: '待质检', tone: 'warn' },
      checked: { label: '已通过', tone: 'success' },
      needs_fix: { label: '需修正', tone: 'warn' },
      voided: { label: '已作废', tone: 'danger' },
    },
    value,
  )

export const pricingStatusMeta = (value?: string | null) =>
  lookup(
    {
      priced: { label: '已计价', tone: 'success' },
      unpriced: { label: '待补价', tone: 'warn' },
      pending_grade: { label: '待定级', tone: 'warn' },
      pending: { label: '待计价', tone: 'info' },
    },
    value,
  )

export const settlementStatusMeta = (value?: string | null) =>
  lookup(
    {
      unsettled: { label: '未结算', tone: 'neutral' },
      in_batch: { label: '批次中', tone: 'info' },
      settled: { label: '已结算', tone: 'success' },
      reversed: { label: '已冲正', tone: 'danger' },
    },
    value,
  )

export const previewStatusMeta = (value?: string | null) =>
  lookup(
    {
      pending: { label: '待生成', tone: 'neutral' },
      processing: { label: '生成中', tone: 'info' },
      ready: { label: '可预览', tone: 'success' },
      failed: { label: '失败', tone: 'danger' },
      not_applicable: { label: '不适用', tone: 'neutral' },
    },
    value,
  )

export const batchStatusMeta = (value?: string | null) =>
  lookup(
    {
      generated: { label: '待确认', tone: 'warn' },
      confirmed: { label: '已确认', tone: 'success' },
      cancelled: { label: '已取消', tone: 'neutral' },
      reversed: { label: '已冲正', tone: 'danger' },
    },
    value,
  )

export const supplementStatusMeta = (value?: string | null) =>
  lookup(
    {
      pending: { label: '待审核', tone: 'warn' },
      approved: { label: '已批准', tone: 'success' },
      rejected: { label: '已拒绝', tone: 'danger' },
      reversed: { label: '已冲正', tone: 'danger' },
    },
    value,
  )

export const profileStatusMeta = (value?: string | null) =>
  lookup(
    {
      pending: { label: '待审核', tone: 'warn' },
      active: { label: '已生效', tone: 'success' },
      disabled: { label: '已停用', tone: 'neutral' },
      suspended: { label: '已暂停', tone: 'warn' },
    },
    value,
  )

export const promoModeMeta = (value?: string | null) =>
  lookup(
    {
      fixed_price: { label: '一口价', tone: 'money' },
      markup_amount: { label: '加价', tone: 'info' },
      markup_rate: { label: '涨幅', tone: 'warn' },
    },
    value,
  )

export const duplicateMeta = (hasDuplicate?: boolean) =>
  hasDuplicate ? { label: '可能重复', tone: 'warn' as const } : { label: '无重复提示', tone: 'neutral' as const }

export const enabledMeta = (enabled?: boolean) =>
  enabled ? { label: '启用', tone: 'success' as const } : { label: '停用', tone: 'neutral' as const }

export const systemPreviewMeta = (available?: boolean) =>
  available ? { label: '可预览', tone: 'success' as const } : { label: '只下载', tone: 'neutral' as const }

export const itemTypeMeta = (value?: string | null) =>
  lookup(
    {
      gross_piecework: { label: '正常计件', tone: 'success' },
      error_deduction: { label: '出错扣减', tone: 'danger' },
      welfare: { label: '福利补贴', tone: 'money' },
      supplement: { label: '补录计件', tone: 'info' },
      adjustment: { label: '补差', tone: 'money' },
      reversal: { label: '冲正', tone: 'danger' },
    },
    value,
  )

export const directionMeta = (value?: string | null) =>
  lookup(
    {
      credit: { label: '增加', tone: 'success' },
      debit: { label: '扣回', tone: 'danger' },
    },
    value,
  )

export function eventTypeMeta(value?: string | null): StatusMeta {
  const key = value || ''
  const known: Record<string, StatusMeta> = {
    settlement_batch_confirm: { label: '确认批次', tone: 'success' },
    settlement_batch_cancel: { label: '取消批次', tone: 'danger' },
    settlement_adjustment_create: { label: '结算调整', tone: 'money' },
    submission_item_void: { label: '作废明细', tone: 'danger' },
    submission_item_reprice: { label: '重新计价', tone: 'info' },
    submission_item_qc: { label: '质检更新', tone: 'success' },
    profile_update: { label: '档案维护', tone: 'info' },
    error_import: { label: '出错导入', tone: 'warn' },
  }
  if (known[key]) return known[key]
  if (/(void|cancel|reverse|delete|reject|fail|error)/i.test(key)) return { label: key, tone: 'danger' }
  if (/(confirm|approve|check|create|upload|settle)/i.test(key)) return { label: key, tone: 'success' }
  if (/(price|reprice|import|update|profile|permission)/i.test(key)) return { label: key, tone: 'info' }
  return fallback(key)
}
