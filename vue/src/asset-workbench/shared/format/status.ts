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

function businessFallback(value: string, label: string) {
  if (!value) return '未设置'
  if (/^[a-z0-9_.\-\s]+$/i.test(value)) return label
  return value
}

function lookup(map: StatusMap, value?: string | null, tone: StatusTone = 'neutral') {
  const key = value || ''
  return map[key] ?? fallback(businessFallback(key, '未归类'), tone)
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
    'profile.upserted': { label: '更新人员档案', tone: 'info' },
    'price.created': { label: '新增价目', tone: 'money' },
    'deduction.created': { label: '新增扣减规则', tone: 'warn' },
    'welfare.created': { label: '新增福利规则', tone: 'money' },
    'promo.created': { label: '新增大促规则', tone: 'money' },
    'upload_session.created': { label: '创建上传', tone: 'info' },
    'upload_session.updated': { label: '更新上传', tone: 'info' },
    'submission.created': { label: '提交作品', tone: 'success' },
    'error_import.created': { label: '导入出错数', tone: 'warn' },
    'settlement.generated': { label: '生成结算', tone: 'money' },
    'settlement.confirmed': { label: '确认结算', tone: 'success' },
    'settlement.cancelled': { label: '取消结算', tone: 'danger' },
    'settlement.adjusted': { label: '结算补差', tone: 'money' },
    'supplement.created': { label: '创建补录', tone: 'info' },
    'supplement_permission.changed': { label: '调整补录权限', tone: 'info' },
    'saved_view.upserted': { label: '保存视图', tone: 'neutral' },
    'file.downloaded': { label: '下载交付文件', tone: 'info' },
    'file.batch_downloaded': { label: '批量下载交付文件', tone: 'info' },
    'system_asset.downloaded': { label: '下载素材', tone: 'info' },
    'system_asset.batch_downloaded': { label: '批量下载素材', tone: 'info' },
    'item.qc_updated': { label: '更新质检', tone: 'success' },
    'item.voided': { label: '作废明细', tone: 'danger' },
    'item.repriced': { label: '重新计价', tone: 'info' },
    'group.upserted': { label: '维护人员分组', tone: 'info' },
    'template.upserted': { label: '维护作品类型', tone: 'info' },
    'template.assigned': { label: '下发作品类型', tone: 'success' },
    'template_assignment.removed': { label: '撤销作品下发', tone: 'danger' },
    'member.identity_changed': { label: '调整成员能力', tone: 'info' },
    'account.merged': { label: '合并账号', tone: 'warn' },
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
  if (/(void|cancel|reverse|delete|reject|fail|error)/i.test(key)) return { label: businessFallback(key, '异常操作'), tone: 'danger' }
  if (/(confirm|approve|check|create|upload|settle)/i.test(key)) return { label: businessFallback(key, '业务操作'), tone: 'success' }
  if (/(price|reprice|import|update|profile|permission)/i.test(key)) return { label: businessFallback(key, '配置维护'), tone: 'info' }
  return fallback(key)
}

export function entityTypeMeta(value?: string | null): StatusMeta {
  return lookup(
    {
      profile: { label: '人员档案', tone: 'info' },
      price_matrix: { label: '价目', tone: 'money' },
      deduction_rule: { label: '扣减规则', tone: 'warn' },
      welfare_rule: { label: '福利规则', tone: 'money' },
      promo_coupon: { label: '大促规则', tone: 'money' },
      upload_session: { label: '上传会话', tone: 'info' },
      submission: { label: '作品提交', tone: 'success' },
      submission_item: { label: '作品明细', tone: 'success' },
      submission_file: { label: '交付文件', tone: 'info' },
      system_asset: { label: '系统素材', tone: 'info' },
      error_import: { label: '出错导入', tone: 'warn' },
      settlement_batch: { label: '结算批次', tone: 'money' },
      settlement_adjustment: { label: '补差冲正', tone: 'money' },
      settlement_supplement: { label: '补录计件', tone: 'info' },
      supplement_permission: { label: '补录权限', tone: 'info' },
      saved_view: { label: '保存视图', tone: 'neutral' },
      group: { label: '人员分组', tone: 'info' },
      template: { label: '作品类型', tone: 'info' },
      template_assignment: { label: '作品下发', tone: 'success' },
      member: { label: '成员权限', tone: 'info' },
      account: { label: '账号', tone: 'warn' },
    },
    value,
  )
}

export function eventReasonText(value?: string | null) {
  const reason = (value || '').trim()
  const known: Record<string, string> = {
    'download manifest issued': '生成下载链接',
    'system asset download manifest issued': '生成素材下载链接',
    'create group': '创建分组',
    'update group': '更新分组',
    'disable group': '停用分组',
    'add group members': '添加分组成员',
    'remove group members': '移除分组成员',
    'create template': '创建作品类型',
    'update template': '更新作品类型',
    'disable template': '停用作品类型',
    'assign template': '下发作品类型',
    'remove assignment': '撤销作品下发',
    'workbench role update': '调整工作台能力',
    'merge account': '合并账号',
    'create submission': '提交作品',
    'update submission': '更新作品提交',
    'create profile': '创建人员档案',
    'update profile': '更新人员档案',
    'create price': '新增价目',
    'create deduction': '新增扣减规则',
    'create welfare': '新增福利规则',
    'create promo': '新增大促规则',
    'confirm settlement': '确认结算',
    'cancel settlement': '取消结算',
    'create supplement': '创建补录',
    'create adjustment': '创建补差',
  }
  return known[reason] ?? businessFallback(reason, '系统操作')
}
