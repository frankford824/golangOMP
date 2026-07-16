import type { RecentEvent } from '@/domain/types/dashboard'
import type { LegacyTaskStatus } from '@/domain/types/task'
import { getTaskStatusLabel } from '@/domain/enums/task-status'
import { formatDateTimeBeijingOffsetAware } from '@/utils/date'
import { getTaskEventDisplayTitle } from '@/utils/operation-event-type-labels'
import { assetKindLabelCn } from '@/domain/mappers/read-model-labels-cn'
import { userAccountDisplay, userAccountOrEmpty } from '@/domain/user-display'

export function extractTaskEventsList(body: unknown): Record<string, unknown>[] {
  if (!body || typeof body !== 'object') return []
  const root = body as Record<string, unknown>
  const data = root.data
  if (Array.isArray(data)) {
    return data.filter((x): x is Record<string, unknown> => x != null && typeof x === 'object') as Record<
      string,
      unknown
    >[]
  }
  if (data && typeof data === 'object' && !Array.isArray(data)) {
    const items = (data as Record<string, unknown>).items
    if (Array.isArray(items)) {
      return items.filter((x): x is Record<string, unknown> => x != null && typeof x === 'object') as Record<
        string,
        unknown
      >[]
    }
  }
  if (Array.isArray(root)) return root as Record<string, unknown>[]
  return []
}

export function extractCostOverrideEventsList(body: unknown): Record<string, unknown>[] {
  if (!body || typeof body !== 'object') return []
  const root = body as Record<string, unknown>
  const data = root.data
  if (data && typeof data === 'object' && !Array.isArray(data)) {
    const events = (data as Record<string, unknown>).events
    if (Array.isArray(events)) {
      return events.filter((x): x is Record<string, unknown> => x != null && typeof x === 'object')
    }
  }
  const events = root.events
  if (Array.isArray(events)) {
    return events.filter((x): x is Record<string, unknown> => x != null && typeof x === 'object')
  }
  return []
}

function payloadObject(raw: Record<string, unknown>): Record<string, unknown> {
  const p = raw.payload
  if (p && typeof p === 'object' && !Array.isArray(p)) return p as Record<string, unknown>
  return {}
}

function pickField(raw: Record<string, unknown>, payload: Record<string, unknown>, key: string): string | undefined {
  const direct = raw[key]
  if (direct != null && String(direct).trim() !== '') return String(direct).trim()
  const nested = payload[key]
  if (nested != null && String(nested).trim() !== '') return String(nested).trim()
  const camel = key.replace(/_([a-z])/g, (_, c: string) => c.toUpperCase())
  const camelRaw = raw[camel]
  if (camelRaw != null && String(camelRaw).trim() !== '') return String(camelRaw).trim()
  const camelPay = payload[camel]
  if (camelPay != null && String(camelPay).trim() !== '') return String(camelPay).trim()
  return undefined
}

function pickFirst(
  raw: Record<string, unknown>,
  payload: Record<string, unknown>,
  keys: string[],
): string | undefined {
  for (const k of keys) {
    const v = pickField(raw, payload, k)
    if (v) return v
  }
  return undefined
}

function toOutcomeCn(raw: unknown): string | undefined {
  if (raw === true || raw === 1) return '成功'
  if (raw === false || raw === 0) return '失败'
  const s = String(raw ?? '').trim().toLowerCase()
  if (!s) return undefined
  if (['ok', 'success', 'successful', 'normal', 'completed', 'done', 'filed'].includes(s)) return '正常'
  if (['failed', 'error', 'failure'].includes(s)) return '失败'
  return undefined
}

const TASK_TYPE_API_CN: Record<string, string> = {
  original_product_development: '原品开发任务',
  new_product_development: '新品任务',
  sku_planning: '策划 SKU',
  retouch_task: 'P 图任务',
}

function taskTypeLabelCn(code: string | undefined): string {
  if (!code) return ''
  const k = code.trim().toLowerCase()
  return TASK_TYPE_API_CN[k] ?? code.trim()
}

const MODULE_KEY_CN: Record<string, string> = {
  basic_info: '基础信息',
  design: '设计',
  audit: '审核',
  retouch: '修图',
  customization: '定制',
}

function moduleKeyLabelCn(key: string | undefined): string {
  if (!key) return ''
  const k = key.trim().toLowerCase()
  return MODULE_KEY_CN[k] ?? key.trim()
}

function workflowDetailSuffix(raw: Record<string, unknown>, payload: Record<string, unknown>): string {
  let design = ''
  let audit = ''
  const tryWf = (w: unknown) => {
    if (!w || typeof w !== 'object' || Array.isArray(w)) return
    const o = w as Record<string, unknown>
    const d = o.design_sub_status ?? o.design
    const a = o.audit_sub_status ?? o.audit
    if (!design && d != null) design = String(d).trim()
    if (!audit && a != null) audit = String(a).trim()
  }
  tryWf(payload.workflow)
  const detail = payload.detail
  if (detail && typeof detail === 'object' && !Array.isArray(detail)) {
    tryWf((detail as Record<string, unknown>).workflow)
  }
  if (!design) design = pickField(raw, payload, 'design_sub_status') ?? pickField(raw, payload, 'design_state') ?? ''
  if (!audit) audit = pickField(raw, payload, 'audit_sub_status') ?? pickField(raw, payload, 'audit_state') ?? ''
  if (!design && !audit) return ''
  return `，明细：设计 ${design || '—'}、审核 ${audit || '—'}`
}

function taskStatusDisplayCn(code: string | undefined): string {
  if (!code?.trim()) return ''
  return getTaskStatusLabel(code.trim() as LegacyTaskStatus)
}

const FILING_STATUS_CN: Record<string, string> = {
  filed: '已同步',
  pending: '待建档',
  pending_filing: '待建档',
  not_filed: '未同步',
  unfilled: '未填报',
  filing_failed: '同步失败',
  error: '异常',
}

function filingStatusDisplayCn(raw: string | undefined): string {
  if (!raw?.trim()) return ''
  const k = raw.trim().toLowerCase()
  return FILING_STATUS_CN[k] ?? raw.trim()
}

function moneyDisplay(raw: unknown): string {
  if (raw == null || raw === '') return '—'
  const n = typeof raw === 'number' ? raw : Number(raw)
  if (!Number.isFinite(n)) return String(raw)
  return `${n.toFixed(3)} 元`
}

function isTechnicalActorName(value: string): boolean {
  const text = value.trim()
  if (!text) return true
  if (/^未知用户$/i.test(text)) return true
  if (/^用户\s*#?\d+$/i.test(text)) return true
  if (/^#?\d+$/.test(text)) return true
  if (/^session_actor\s*#?\d+$/i.test(text)) return true
  return false
}

function businessActorDisplay(...candidates: unknown[]): string {
  for (const candidate of candidates) {
    const text = String(candidate ?? '').trim()
    if (!text || isTechnicalActorName(text)) continue
    const display = userAccountDisplay(text)
    if (display && !isTechnicalActorName(display)) return display
  }
  return ''
}

function cleanInlineBusinessText(raw: unknown): string {
  return businessReadableEventSummary(String(raw ?? ''))
}

function formatActorSegment(raw: Record<string, unknown>, payload: Record<string, unknown>): string {
  const name =
    (pickField(raw, payload, 'operator_username') ??
      pickField(raw, payload, 'actor_username') ??
      pickField(raw, payload, 'creator_username') ??
      pickField(raw, payload, 'operator_name') ??
      pickField(raw, payload, 'creator_name') ??
      pickField(
      raw,
      payload,
      'actor_name',
    ))?.trim() ?? ''
  const roleShort =
    (pickField(raw, payload, 'operator_role_label') ??
      pickField(raw, payload, 'actor_role_short') ??
      pickField(raw, payload, 'actor_role'))?.trim() ?? ''

  const actor = businessActorDisplay(name, roleShort)
  if (actor) return actor
  return '系统'
}

function titleForEvent(
  eventType: string,
  raw: Record<string, unknown>,
  payload: Record<string, unknown>,
): string {
  const t = eventType.toLowerCase()
  const action = String(pickField(raw, payload, 'action') ?? '').toLowerCase()
  if (
    t.includes('replace') ||
    t.includes('replacement') ||
    action.includes('replace') ||
    pickField(raw, payload, 'previous_asset_id')
  ) {
    return '稿件替换'
  }
  return getTaskEventDisplayTitle(eventType)
}

function preferredApiSummary(raw: Record<string, unknown>, payload: Record<string, unknown>): string | undefined {
  const s =
    pickField(raw, payload, 'summary') ??
    pickField(raw, payload, 'message') ??
    pickField(raw, payload, 'description')
  if (s && s.length > 0) return businessReadableEventSummary(s)
  return undefined
}

const UUID_PATTERN = /[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}/gi
const LONG_HEX_PATTERN = /\b[0-9a-f]{24,}\b/gi
const STORAGE_PATH_PATTERN = /\b(?:tasks|objects)\/[^\s，。；,;）)]+/gi
const TECH_FIELD_PATTERN =
  /\b(?:upload_session_id|remote_upload_id|remote_file_id|storage_key|file_hash|trace_id|asset_id|source_asset_id|asset_version_id|task_asset_id|design_asset_id|ref_id|timeline_version|retouch_requirement_id)\b\s*[:：=]?\s*["']?[\w\-./:%]+["']?/gi
const BUSINESS_FIELD_NAME_CN: Record<string, string> = {
  filing_status: '同步状态',
  erp_sync_status: 'ERP 同步状态',
  base_sync_status: '基础资料状态',
  image_sync_status: 'ERP 图片状态',
  sync_status: '同步状态',
  c_price: '成本价',
  s_price: '售价',
  sku_id: 'SKU 编码',
  i_id: '款式编码',
}
const BUSINESS_FIELD_NAME_PATTERN = new RegExp(
  `\\b(${Object.keys(BUSINESS_FIELD_NAME_CN)
    .map((key) => key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
    .join('|')})\\b`,
  'gi',
)
const BUSINESS_STATUS_CODE_CN: Record<string, string> = {
  filed: '已同步',
  pending: '待处理',
  pending_filing: '待补齐后同步',
  not_filed: '未同步',
  unfilled: '未填报',
  filing: '同步中',
  filing_failed: '同步失败',
  queued: '排队中',
  syncing: '同步中',
  synced: '已同步',
  pending_sync: '待同步',
  waiting_image: '待上传图片',
  pending_upload: '待上传',
  cooling_down: '稍后重试',
  failed: '失败',
  error: '异常',
  completed: '已完成',
  in_progress: '处理中',
  draft: '草稿',
  prepared: '已备货',
  received: '已接收',
  rejected: '已拒收',
  cancelled: '已取消',
  canceled: '已取消',
  idempotent_skip_same_payload: '内容无变化，已跳过重复同步',
}
const BUSINESS_STATUS_CODE_PATTERN = new RegExp(
  `\\b(${Object.keys(BUSINESS_STATUS_CODE_CN)
    .map((key) => key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
    .join('|')})\\b`,
  'gi',
)

function replaceBusinessStatusCodes(text: string): string {
  return text.replace(
    BUSINESS_STATUS_CODE_PATTERN,
    (matched) => BUSINESS_STATUS_CODE_CN[matched.toLowerCase()] ?? matched,
  )
}

function replaceBusinessFieldNames(text: string): string {
  return text.replace(
    BUSINESS_FIELD_NAME_PATTERN,
    (matched) => BUSINESS_FIELD_NAME_CN[matched.toLowerCase()] ?? matched,
  )
}

function businessReadableEventSummary(summary: string): string {
  return replaceBusinessFieldNames(replaceBusinessStatusCodes(summary))
    .replace(/\bmanual\s+cost\s+override\b/gi, '人工维护成本')
    .replace(/\boperator\s*:\s*\d+\b/gi, '操作人')
    .replace(/\bERP readback:\s*/gi, 'ERP 状态确认：')
    .replace(/未知用户/g, '待确认人员')
    .replace(/用户\s*#?\d+/gi, '待确认人员')
    .replace(/session_actor\s*#?\d+/gi, '待确认人员')
    .replace(/上传会话\s*[（(]\s*[0-9a-f-]{32,36}\s*[）)]/gi, '上传记录')
    .replace(/上传会话/g, '上传记录')
    .replace(TECH_FIELD_PATTERN, '')
    .replace(STORAGE_PATH_PATTERN, '文件记录')
    .replace(UUID_PATTERN, '')
    .replace(LONG_HEX_PATTERN, '')
    .replace(/\s*[（(]\s*[）)]/g, '')
    .replace(/\s*[，,]\s*[，,]+/g, '，')
    .replace(/[，,]\s*。/g, '。')
    .replace(/\s{2,}/g, ' ')
    .trim()
}

function buildTaskEventSummaryCn(
  eventType: string,
  raw: Record<string, unknown>,
  payload: Record<string, unknown>,
): string | undefined {
  const fromApi = preferredApiSummary(raw, payload)
  if (fromApi) return fromApi

  const et = eventType.trim()
  const actor = formatActorSegment(raw, payload)

  if (et === 'task.created') {
    const ttRaw = pickFirst(raw, payload, ['task_type', 'taskType'])
    const tt = taskTypeLabelCn(ttRaw)
    const sku = pickFirst(raw, payload, ['sku_code', 'primary_sku_code', 'product_sku'])
    const outcomeRaw = pickField(raw, payload, 'outcome')
    const tail =
      toOutcomeCn(payload.result) ??
      toOutcomeCn(payload.success) ??
      toOutcomeCn(payload.ok) ??
      toOutcomeCn(outcomeRaw) ??
      (outcomeRaw ? outcomeRaw : undefined) ??
      '正常'
    const parts: string[] = [`${actor} 创建了任务`]
    if (tt) parts[0] = `${actor} 创建了${tt}`
    if (sku) parts.push(`SKU ${sku}`)
    return `${parts.join('，')}，结果：${tail}。`
  }

  if (et === 'task.filing.triggered') {
    const fs = pickFirst(raw, payload, ['filing_status', 'filingStatus'])
    const fsKey = String(fs ?? '').trim().toLowerCase()
    const attemptedRaw = payload.attempted ?? payload.attemptedSync
    const attempted =
      attemptedRaw === true ||
      attemptedRaw === 1 ||
      String(attemptedRaw).trim().toLowerCase() === 'true'
    const skippedReason = pickFirst(raw, payload, ['skipped_reason', 'skip_reason'])
    const itemPayload = JSON.stringify(payload.erp_filing_items ?? payload.erpFilingItems ?? '')
    const failed =
      fsKey === 'filing_failed' ||
      fsKey === 'failed' ||
      fsKey === 'error' ||
      /failure|failed|error|timed out/i.test(itemPayload)
    if (!attempted && skippedReason) {
      const bits: string[] = ['ERP 商品资料已是最新，无需重复同步']
      if (fs) bits.push(`同步状态为「${filingStatusDisplayCn(fs)}」`)
      return `${bits.join('，')}。`
    }
    if (failed) {
      const bits: string[] = ['ERP 商品资料同步失败']
      if (fs) bits.push(`同步状态为「${filingStatusDisplayCn(fs)}」`)
      return `${bits.join('，')}。`
    }
    const okWord =
      toOutcomeCn(payload.success) ?? toOutcomeCn(payload.ok) ?? toOutcomeCn(payload.result) ?? '成功'
    const bits: string[] = ['ERP 商品资料已同步']
    if (fs) bits.push(`建档状态为「${filingStatusDisplayCn(fs)}」`)
    const tail = okWord.endsWith('。') ? okWord.slice(0, -1) : okWord
    bits.push(`结果：${tail}`)
    return `${bits.join('，')}。`
  }

  if (et === 'task.filing.readback_confirmed') {
    const fs = pickFirst(raw, payload, ['filing_status', 'filingStatus'])
    const sku = pickFirst(raw, payload, ['sku_code', 'primary_sku_code', 'product_sku'])
    const parts = ['ERP 同步结果已确认']
    if (sku) parts.push(`SKU ${sku}`)
    if (fs) parts.push(`建档状态为「${filingStatusDisplayCn(fs)}」`)
    return `${parts.join('，')}。`
  }

  if (et === 'task.assigned' || et === 'task.unassigned') {
    const designerId = pickFirst(raw, payload, ['designer_id', 'assignee_id', 'to_user_id', 'current_handler_id'])
    const designerName = pickFirst(raw, payload, [
      'designer_username',
      'assignee_username',
      'current_handler_username',
      'designer_name',
      'assignee_name',
      'current_handler_name',
    ])
    const assigneeSeg =
      businessActorDisplay(designerName) || (designerId ? '待确认人员' : '—')
    const selfRaw = payload.self_assign ?? payload.self_claim ?? payload.is_self_claim
    const self =
      selfRaw === true ||
      selfRaw === 1 ||
      String(selfRaw).toLowerCase() === 'true' ||
      pickField(raw, payload, 'claim_mode')?.toLowerCase() === 'self'
    const fromS = pickFirst(raw, payload, ['from_task_status', 'previous_task_status', 'old_task_status'])
    const toS = pickFirst(raw, payload, ['to_task_status', 'task_status', 'new_task_status', 'task_task_status'])
    const tail = toOutcomeCn(payload.success) ?? toOutcomeCn(payload.result) ?? '正常'
    let line = ''
    if (et === 'task.unassigned') {
      line = `${actor} 解除了任务指派`
    } else if (self) {
      line = `${assigneeSeg} 认领了任务（自接单）`
    } else {
      line = `${actor} 将任务指派给 ${assigneeSeg}`
    }
    if (fromS && toS) {
      line += `，任务状态由「${taskStatusDisplayCn(fromS) || fromS}」变为「${taskStatusDisplayCn(toS) || toS}」`
    } else if (toS) {
      line += `，任务状态为「${taskStatusDisplayCn(toS) || toS}」`
    }
    line += `，${tail}。`
    return line
  }

  if (et === 'task.reassigned' || et === 'module.reassigned') {
    const fromId = pickFirst(raw, payload, ['from_assignee_id', 'from_designer_id', 'previous_assignee_id'])
    const fromName = pickFirst(raw, payload, [
      'from_assignee_username',
      'from_designer_username',
      'previous_assignee_username',
      'from_assignee_name',
      'from_designer_name',
      'previous_assignee_name',
    ])
    const toId = pickFirst(raw, payload, ['to_assignee_id', 'assignee_id', 'designer_id', 'target_user_id'])
    const toName = pickFirst(raw, payload, [
      'to_assignee_username',
      'assignee_username',
      'designer_username',
      'target_username',
      'to_assignee_name',
      'assignee_name',
      'designer_name',
    ])
    const fromSeg = businessActorDisplay(fromName) || (fromId ? '待确认人员' : '—')
    const toSeg = businessActorDisplay(toName) || (toId ? '待确认人员' : '—')
    const mk =
      moduleKeyLabelCn(pickField(raw, payload, 'module_key')) ||
      moduleKeyLabelCn(raw.module_key != null ? String(raw.module_key) : '')
    const tail = toOutcomeCn(payload.success) ?? toOutcomeCn(payload.result) ?? '正常'
    return `${actor} 将任务由 ${fromSeg} 改派至 ${toSeg}${mk ? `（${mk}）` : ''}，${tail}。`
  }

  if (et === 'module.pool_reassigned') {
    const team = pickField(raw, payload, 'pool_team_code')
    const tail = toOutcomeCn(payload.success) ?? '已回到任务池'
    return `${actor} 在任务池内执行了改派${team ? `，目标组为 ${team}` : ''}，${tail}。`
  }

  if (et === 'task.business_info.updated' || et === 'business_info.updated') {
    return `${actor} 更新了业务信息${pickField(raw, payload, 'note') ? `（${pickField(raw, payload, 'note')}）` : ''}。`
  }

  if (et === 'task.cost.updated') {
    const previous = payload.previous_cost_price ?? payload.previousCostPrice
    const current = payload.cost_price ?? payload.costPrice
    const estimated = payload.estimated_cost ?? payload.estimatedCost
    const manual = payload.manual_cost_override ?? payload.manualCostOverride
    const reason = pickFirst(raw, payload, ['manual_cost_override_reason', 'override_reason', 'remark'])
    const syncRequested = payload.erp_sync_requested ?? payload.erpSyncRequested
    const parts = [`${actor} 更新了成本价：${moneyDisplay(previous)} → ${moneyDisplay(current)}`]
    if (estimated != null && estimated !== '') parts.push(`系统预估 ${moneyDisplay(estimated)}`)
    if (manual === true || manual === 1 || String(manual).toLowerCase() === 'true') parts.push('人工覆盖')
    if (reason) parts.push(`原因：${cleanInlineBusinessText(reason)}`)
    if (syncRequested === true || syncRequested === 1 || String(syncRequested).toLowerCase() === 'true') {
      parts.push('已请求同步 ERP')
    }
    return `${parts.join('，')}。`
  }

  if (et === 'task.sku_item.updated') {
    const sku = pickFirst(raw, payload, ['sku_code', 'item_sku_code', 'target_sku_code'])
    const name = pickFirst(raw, payload, ['product_name', 'product_name_snapshot', 'item_name'])
    const parts = [`${actor} 更新了子项信息`]
    if (sku) parts.push(`SKU ${sku}`)
    if (name) parts.push(cleanInlineBusinessText(name))
    return `${parts.join('，')}。`
  }

  if (et === 'task.batch_items_created') {
    const count = pickFirst(raw, payload, ['batch_item_count', 'item_count', 'count'])
    const primarySku = pickFirst(raw, payload, ['primary_sku_code', 'sku_code'])
    const parts = [`${actor} 生成了批量任务子项`]
    if (count) parts.push(`共 ${count} 项`)
    if (primarySku) parts.push(`主 SKU ${primarySku}`)
    return `${parts.join('，')}。`
  }

  if (et === 'task.asset.upload_session.created' ||
    et === 'task.asset.upload_session.completed' ||
    et === 'task.asset.upload_session.cancelled'
  ) {
    const assetType = pickFirst(raw, payload, ['asset_type', 'assetType'])
    const kind = assetType ? assetKindLabelCn(assetType) : ''
    const verbDone =
      et.endsWith('.created') ? '创建了' : et.endsWith('.completed') ? '完成了' : et.endsWith('.cancelled') ? '取消了' : ''
    const toS = pickFirst(raw, payload, ['to_task_status', 'task_status', 'task_task_status'])
    let line = `${actor} ${verbDone}${kind || '素材'}上传记录`
    if (toS) line += `，任务状态为「${taskStatusDisplayCn(toS) || toS}」`
    line += workflowDetailSuffix(raw, payload)
    line += '。'
    return line
  }

  if (et === 'task.asset.version.created') {
    const assetType = pickFirst(raw, payload, ['asset_type', 'assetType'])
    const kind = assetType ? assetKindLabelCn(assetType) : '素材'
    const filename = pickFirst(raw, payload, ['filename', 'remark', 'file_name'])
    const isDerived =
      String(payload.derived_async ?? payload.derivedAsync ?? '').toLowerCase() === 'true' ||
      String(payload.derivation_reason ?? payload.derivationReason ?? '').trim() !== ''
    if (isDerived) return kind === '预览辅助' ? '系统生成了预览图。' : `系统生成了${kind}预览。`
    return `${actor} 保存了${kind}${filename ? `：${cleanInlineBusinessText(filename)}` : ''}。`
  }

  if (et === 'task.reference.asset.formalized' || et === 'task.reference.asset.bulk_formalized') {
    const count = pickFirst(raw, payload, ['count', 'ref_count', 'asset_count'])
    const action = et.endsWith('bulk_formalized') ? '批量接入' : '接入'
    const parts = [`${actor} 已将创建任务时上传的参考图${action}任务`]
    if (count) parts.push(`共 ${count} 张`)
    return `${parts.join('，')}。`
  }

  if (et === 'task.reference.asset.formalize_failed') {
    const msg = pickFirst(raw, payload, ['error_message', 'message', 'reason'])
    return `${actor} 接入创建任务参考图失败${msg ? `：${cleanInlineBusinessText(msg)}` : '，可重新上传或刷新后重试'}。`
  }

  if (et === 'task.design.submitted') {
    const assetType = pickFirst(raw, payload, ['asset_type', 'assetType'])
    const kind = assetType ? assetKindLabelCn(assetType) : '设计稿'
    const sku = pickFirst(raw, payload, ['target_sku_code', 'sku_code'])
    return `${actor} 提交了${kind}${sku ? `（SKU ${sku}）` : ''}，进入审核。`
  }

  if (
    et === 'task.audit.claimed' ||
    et === 'task.audit.approved' ||
    et === 'task.audit.rejected' ||
    et === 'task.audit.transferred' ||
    et === 'task.audit.handed_over' ||
    et === 'task.audit.taken_over'
  ) {
    const comment = pickFirst(raw, payload, ['comment', 'reason', 'remark'])
    const issue = pickFirst(raw, payload, ['issue_types', 'issue_type', 'reject_reason'])
    const actionLabel: Record<string, string> = {
      'task.audit.claimed': '领取了审核任务',
      'task.audit.approved': '通过了审核',
      'task.audit.rejected': '驳回了审核',
      'task.audit.transferred': '转交了审核',
      'task.audit.handed_over': '交接了审核',
      'task.audit.taken_over': '接管了审核',
    }
    const parts = [`${actor} ${actionLabel[et] ?? '处理了审核'}`]
    if (issue) parts.push(`原因：${cleanInlineBusinessText(issue)}`)
    if (comment) parts.push(`说明：${cleanInlineBusinessText(comment)}`)
    return `${parts.join('，')}。`
  }

  if (et === 'task.completed') {
    const remark = pickFirst(raw, payload, ['remark', 'reason'])
    const sku = pickFirst(raw, payload, ['sku_code', 'primary_sku_code'])
    const parts = [`${actor} 已结单`]
    if (sku) parts.push(`SKU ${sku}`)
    if (remark) parts.push(cleanInlineBusinessText(remark))
    return `${parts.join('，')}。`
  }

  if (
    et === 'task.customization.reviewed' ||
    et === 'task.customization.effect_preview_submitted' ||
    et === 'task.customization.effect_reviewed' ||
    et === 'task.customization.production_transferred'
  ) {
    const status = pickFirst(raw, payload, ['status', 'action', 'review_result'])
    const comment = pickFirst(raw, payload, ['comment', 'remark', 'reason'])
    const actionLabel: Record<string, string> = {
      'task.customization.reviewed': '处理了定制需求审核',
      'task.customization.effect_preview_submitted': '提交了定制效果图',
      'task.customization.effect_reviewed': '处理了定制效果图审核',
      'task.customization.production_transferred': '将定制任务转入生产',
    }
    const parts = [`${actor} ${actionLabel[et] ?? '处理了定制任务'}`]
    if (status) parts.push(`结果：${cleanInlineBusinessText(status)}`)
    if (comment) parts.push(`说明：${cleanInlineBusinessText(comment)}`)
    return `${parts.join('，')}。`
  }

  if (
    et === 'task.erp_image.auto_synced' ||
    et === 'task.erp_image.auto_sync_failed' ||
    et === 'task.erp_image.awaiting_upload'
  ) {
    const sku = pickFirst(raw, payload, ['sku_code', 'primary_sku_code'])
    const reason = pickFirst(raw, payload, ['reason', 'message'])
    if (et === 'task.erp_image.auto_synced') return `ERP 图片已自动同步${sku ? `：SKU ${sku}` : ''}。`
    if (et === 'task.erp_image.awaiting_upload') return `未找到可同步的 ERP 商品图${sku ? `：SKU ${sku}` : ''}，待人工上传。`
    return `ERP 图片自动同步失败${sku ? `：SKU ${sku}` : ''}${reason ? `，${cleanInlineBusinessText(reason)}` : ''}。`
  }

  if (et === 'task.status.changed') {
    const fromS = pickFirst(raw, payload, ['from_task_status', 'previous_task_status'])
    const toS = pickFirst(raw, payload, ['to_task_status', 'task_status', 'new_task_status'])
    if (fromS && toS) {
      const fromCn = taskStatusDisplayCn(fromS) || fromS
      const toCn = taskStatusDisplayCn(toS) || toS
      return `${actor} 变更任务状态：由「${fromCn}」变为「${toCn}」。`
    }
    if (toS) {
      const toCn = taskStatusDisplayCn(toS) || toS
      return `${actor} 变更任务状态为「${toCn}」。`
    }
    if (fromS) {
      const fromCn = taskStatusDisplayCn(fromS) || fromS
      return `${actor} 触发状态流转（原状态为「${fromCn}」）。`
    }
  }

  return `${actor} 记录了${getTaskEventDisplayTitle(et)}。`
}

/** GET /v1/tasks/{id}/events 单条 → 抽屉展示模型 */
export function mapTaskEventRowToRecentEvent(raw: Record<string, unknown>, taskId: string): RecentEvent {
  const payload = payloadObject(raw)
  const id = String(raw.id ?? raw.sequence ?? `${taskId}-${raw.created_at ?? Math.random()}`)
  const eventType = String(raw.event_type ?? raw.type ?? 'event')
  const created = String(raw.created_at ?? '')
  const at = created ? formatDateTimeBeijingOffsetAware(created) : '—'
  const operatorId =
    pickField(raw, payload, 'operator_id') ??
    pickField(raw, payload, 'replacement_actor_id') ??
    pickField(raw, payload, 'creator_id')
  const actor =
    businessActorDisplay(
      pickField(raw, payload, 'operator_username'),
      pickField(raw, payload, 'actor_username'),
      pickField(raw, payload, 'creator_username'),
      pickField(raw, payload, 'operator_name'),
      pickField(raw, payload, 'actor_name'),
      pickField(raw, payload, 'creator_name'),
    ) || (operatorId ? '系统记录' : '—')

  const title = titleForEvent(eventType, raw, payload)
  const summary = buildTaskEventSummaryCn(eventType, raw, payload)

  return {
    id,
    type: eventType,
    title,
    summary,
    refId: String(raw.task_id ?? taskId),
    refNo: pickField(raw, payload, 'task_no') ?? '—',
    actor: userAccountDisplay(actor),
    at,
    ...(created ? { createdAtIso: created } : {}),
    previous_asset_id: pickField(raw, payload, 'previous_asset_id'),
    current_asset_id: pickField(raw, payload, 'current_asset_id'),
    replacement_actor_id: pickField(raw, payload, 'replacement_actor_id'),
    replacement_actor_name: pickField(raw, payload, 'replacement_actor_name'),
    replacement_note: pickField(raw, payload, 'note') ?? pickField(raw, payload, 'replacement_note'),
    replacement_task_id: pickField(raw, payload, 'replacement_task_id') ?? pickField(raw, payload, 'task_id'),
    workflow_lane: pickField(raw, payload, 'workflow_lane'),
    source_department: pickField(raw, payload, 'source_department'),
  }
}

const COST_OVERRIDE_EVENT_CN: Record<string, string> = {
  override_applied: '成本人工覆盖',
  override_updated: '成本覆盖更新',
  override_released: '成本覆盖解除',
}

export function mapCostOverrideEventToRecentEvent(
  raw: Record<string, unknown>,
  taskId: string,
  taskNo?: string,
): RecentEvent {
  const eventType = String(raw.event_type ?? raw.eventType ?? 'cost_override')
  const id = String(raw.event_id ?? raw.eventId ?? raw.sequence ?? `${taskId}-cost-${Math.random()}`)
  const created = String(raw.override_at ?? raw.overrideAt ?? raw.occurred_at ?? raw.occurredAt ?? '')
  const actor =
    userAccountOrEmpty(
      raw.override_actor_username,
      raw.overrideActorUsername,
      raw.actor_username,
      raw.actorUsername,
      raw.override_actor,
      raw.overrideActor,
      raw.actor,
    ) || '系统'
  const title = COST_OVERRIDE_EVENT_CN[eventType] ?? '成本操作'
  const previous = raw.previous_cost_price ?? raw.previousCostPrice
  const current = raw.result_cost_price ?? raw.resultCostPrice ?? raw.cost_price ?? raw.costPrice
  const overrideCost = raw.override_cost ?? raw.overrideCost
  const reason = String(raw.override_reason ?? raw.reason ?? raw.note ?? '').trim()
  const governance = String(raw.governance_status ?? raw.governanceStatus ?? '').trim()
  const parts = [`${actor} 执行了「${title}」`]
  if (previous != null || current != null) parts.push(`成本 ${moneyDisplay(previous)} → ${moneyDisplay(current)}`)
  else if (overrideCost != null) parts.push(`覆盖成本 ${moneyDisplay(overrideCost)}`)
  if (reason) parts.push(`原因：${reason}`)
  if (governance) parts.push(`规则状态：${governance}`)
  return {
    id,
    type: `task.cost.${eventType}`,
    title,
    summary: `${parts.join('，')}。`,
    refId: taskId,
    refNo: taskNo || '—',
    actor,
    at: created ? formatDateTimeBeijingOffsetAware(created) : '—',
    ...(created ? { createdAtIso: created } : {}),
  }
}
