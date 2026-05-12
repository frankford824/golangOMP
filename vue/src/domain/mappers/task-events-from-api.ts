import type { RecentEvent } from '@/domain/types/dashboard'
import type { LegacyTaskStatus } from '@/domain/types/task'
import { getTaskStatusLabel } from '@/domain/enums/task-status'
import { formatDateTimeBeijingOffsetAware } from '@/utils/date'
import { getTaskEventDisplayTitle } from '@/utils/operation-event-type-labels'
import { assetKindLabelCn } from '@/domain/mappers/read-model-labels-cn'

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
  purchase_task: '采购任务',
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
  warehouse: '仓储',
  retouch: '修图',
  customization: '定制',
  procurement: '采购',
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
  filed: '已建档',
  pending: '待建档',
  pending_filing: '待建档',
  not_filed: '未建档',
  unfilled: '未填报',
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
  return `${n.toFixed(2)} 元`
}

function formatActorSegment(raw: Record<string, unknown>, payload: Record<string, unknown>): string {
  const id =
    pickField(raw, payload, 'operator_id') ??
    pickField(raw, payload, 'creator_id') ??
    pickField(raw, payload, 'actor_id')
  const name =
    (pickField(raw, payload, 'operator_name') ?? pickField(raw, payload, 'creator_name') ?? pickField(
      raw,
      payload,
      'actor_name',
    ))?.trim() ?? ''
  const roleShort =
    (pickField(raw, payload, 'operator_role_label') ??
      pickField(raw, payload, 'actor_role_short') ??
      pickField(raw, payload, 'actor_role'))?.trim() ?? ''

  if (roleShort && id && !name) return `${roleShort} ${id}`
  if (name) return name
  if (id) return `用户 ${id}`
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
  if (s && s.length > 0) return s
  return undefined
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
    const okWord =
      toOutcomeCn(payload.success) ?? toOutcomeCn(payload.ok) ?? toOutcomeCn(payload.result) ?? '成功'
    const bits: string[] = ['创建后已同步至 ERP']
    if (fs) bits.push(`建档状态为「${filingStatusDisplayCn(fs)}」`)
    const tail = okWord.endsWith('。') ? okWord.slice(0, -1) : okWord
    bits.push(`结果：${tail}`)
    return `${bits.join('，')}。`
  }

  if (et === 'task.assigned' || et === 'task.unassigned') {
    const designerId = pickFirst(raw, payload, ['designer_id', 'assignee_id', 'to_user_id', 'current_handler_id'])
    const designerName = pickFirst(raw, payload, ['designer_name', 'assignee_name', 'current_handler_name'])
    const assigneeSeg =
      designerName && designerId
        ? `${designerName}（${designerId}）`
        : designerName || (designerId ? `用户 ${designerId}` : '—')
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
    const fromName = pickFirst(raw, payload, ['from_assignee_name', 'from_designer_name', 'previous_assignee_name'])
    const toId = pickFirst(raw, payload, ['to_assignee_id', 'assignee_id', 'designer_id', 'target_user_id'])
    const toName = pickFirst(raw, payload, ['to_assignee_name', 'assignee_name', 'designer_name'])
    const fromSeg =
      fromName && fromId ? `${fromName}（${fromId}）` : fromName || (fromId ? `用户 ${fromId}` : '—')
    const toSeg = toName && toId ? `${toName}（${toId}）` : toName || (toId ? `用户 ${toId}` : '—')
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
    if (reason) parts.push(`原因：${reason}`)
    if (syncRequested === true || syncRequested === 1 || String(syncRequested).toLowerCase() === 'true') {
      parts.push('已请求同步 ERP')
    }
    return `${parts.join('，')}。`
  }

  if (et === 'task.asset.upload_session.created' ||
    et === 'task.asset.upload_session.completed' ||
    et === 'task.asset.upload_session.cancelled'
  ) {
    const assetType = pickFirst(raw, payload, ['asset_type', 'assetType'])
    const kind = assetType ? assetKindLabelCn(assetType) : ''
    const sessionId = pickFirst(raw, payload, ['upload_session_id', 'session_id', 'upload_request_id'])
    const verbDone =
      et.endsWith('.created') ? '创建了' : et.endsWith('.completed') ? '完成了' : et.endsWith('.cancelled') ? '取消了' : ''
    const toS = pickFirst(raw, payload, ['to_task_status', 'task_status', 'task_task_status'])
    let line = `${actor}${verbDone}${kind || '素材'}上传会话`
    if (sessionId) line += `（${sessionId}）`
    if (toS) line += `，任务状态为「${taskStatusDisplayCn(toS) || toS}」`
    line += workflowDetailSuffix(raw, payload)
    line += '。'
    return line
  }

  if (et === 'task.asset.version.created') {
    const assetType = pickFirst(raw, payload, ['asset_type', 'assetType'])
    const kind = assetType ? assetKindLabelCn(assetType) : '素材'
    return `${actor} 写入 ${kind} 新版本。`
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

  return `${actor} 执行了「${getTaskEventDisplayTitle(et)}」相关操作。`
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
    pickField(raw, payload, 'operator_name') ??
    pickField(raw, payload, 'actor_name') ??
    pickField(raw, payload, 'creator_name') ??
    (operatorId ? `用户 ${operatorId}` : '—')

  const title = titleForEvent(eventType, raw, payload)
  const summary = buildTaskEventSummaryCn(eventType, raw, payload)

  return {
    id,
    type: eventType,
    title,
    summary,
    refId: String(raw.task_id ?? taskId),
    refNo: pickField(raw, payload, 'task_no') ?? '—',
    actor,
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
