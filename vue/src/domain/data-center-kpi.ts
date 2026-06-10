import type { OperationLogEntry, WorkflowTraceEvent } from '@/services/apiTypes'

export interface KpiUserDirectoryEntry {
  id: string
  username: string
  name: string
  realName: string
  department: string
  team: string
  lastLoginAt?: string
}

export interface KpiOperationTraceOptions {
  rangeStartMs: number
  rangeEndMs: number
  resolveUserById: (id: number | null) => KpiUserDirectoryEntry | undefined
}

export interface UserDirectoryPageState {
  receivedCount: number
  requestedPageSize: number
  totalLoaded: number
  total: number
}

export function isKpiAssignmentOperation(entry: Pick<OperationLogEntry, 'event_type'>): boolean {
  return ['task.assigned', 'task.reassigned', 'task.batch_assigned'].includes(entry.event_type)
}

export function kpiOperationActorId(entry: OperationLogEntry): number | null {
  if (isKpiAssignmentOperation(entry)) {
    const assignedTo = readPayloadNumber(entry.payload, ['designer_id', 'to_handler_id', 'handler_id', 'assignee_id'])
    if (assignedTo > 0) return assignedTo
    return null
  }
  if (entry.event_type === 'task.design.submitted') {
    const designer = readPayloadNumber(entry.payload, ['uploaded_by', 'operator_id', 'designer_id'])
    if (designer > 0) return designer
  }
  if (entry.event_type === 'task.audit.approved' || entry.event_type === 'task.audit.rejected') {
    const auditor = readPayloadNumber(entry.payload, ['auditor_id', 'operator_id'])
    if (auditor > 0) return auditor
  }
  if (entry.event_type === 'task.created' || entry.event_type === 'task.batch_items_created') {
    const creator = readPayloadNumber(entry.payload, ['creator_id', 'operator_id'])
    if (creator > 0) return creator
  }
  const payloadActor = readPayloadNumber(entry.payload, ['operator_id', 'creator_id', 'designer_id', 'auditor_id'])
  if (payloadActor > 0 && !entry.actor_id) return payloadActor
  return entry.actor_id ?? null
}

export function kpiOperationActorDisplayName(
  entry: OperationLogEntry,
  actorId: number | null,
  user: KpiUserDirectoryEntry | undefined,
): string {
  const directoryName = String(user?.name ?? '').trim()
  if (directoryName) return directoryName

  const payloadName = kpiOperationPayloadPersonName(entry)
  if (payloadName) return payloadName

  const originalActorID = Number(entry.actor_id ?? 0)
  const originalActorName = String(entry.actor_username ?? '').trim()
  if (isKpiAssignmentOperation(entry) && actorId && originalActorID > 0 && originalActorID !== actorId) {
    return `人员#${actorId}`
  }
  return originalActorName || (actorId ? `人员#${actorId}` : '')
}

export function buildKpiOperationTraceEvent(
  entry: OperationLogEntry,
  options: KpiOperationTraceOptions,
): WorkflowTraceEvent | null {
  const createdAt = entry.created_at
  const at = createdAt ? new Date(createdAt).getTime() : 0
  if (!Number.isFinite(at) || at < options.rangeStartMs || at > options.rangeEndMs) return null

  const actorId = kpiOperationActorId(entry)
  if (actorId === null && isKpiAssignmentOperation(entry)) return null
  const user = options.resolveUserById(actorId)
  const actorUsername = kpiOperationActorDisplayName(entry, actorId, user)
  const taskID = Number(entry.reference_id)
  return {
    id: Number(entry.log_id) || at,
    event_id: `operation:${entry.source}:${entry.log_id}`,
    event_source: 'system',
    event_type: 'user_action',
    action: entry.event_type,
    actor_id: actorId,
    actor_username: actorUsername,
    actor_source: entry.actor_type,
    actor_department: user?.department || '',
    actor_team: user?.team || '',
    route_method: '',
    route_path: '',
    resource_type: entry.reference_type,
    resource_id: entry.reference_id,
    task_id: Number.isFinite(taskID) && taskID > 0 ? taskID : null,
    outcome: entry.status === 'failed' ? 'failed' : 'succeeded',
    payload: entry.payload,
    occurred_at: createdAt,
    created_at: createdAt,
  }
}

export function shouldContinueUserDirectoryLoad(state: UserDirectoryPageState): boolean {
  if (state.receivedCount <= 0) return false
  if (Number.isFinite(state.total) && state.total > 0) {
    return state.totalLoaded < state.total
  }
  return state.receivedCount >= state.requestedPageSize
}

function kpiOperationPayloadPersonName(entry: OperationLogEntry): string {
  if (isKpiAssignmentOperation(entry)) {
    return readPayloadText(entry.payload, ['designer_name', 'to_handler_name', 'assignee_name', 'handler_name'])
  }
  if (entry.event_type === 'task.design.submitted') {
    return readPayloadText(entry.payload, ['uploaded_by_name', 'operator_name', 'designer_name'])
  }
  if (entry.event_type === 'task.audit.approved' || entry.event_type === 'task.audit.rejected') {
    return readPayloadText(entry.payload, ['auditor_name', 'operator_name'])
  }
  if (entry.event_type === 'task.created' || entry.event_type === 'task.batch_items_created') {
    return readPayloadText(entry.payload, ['creator_name', 'operator_name'])
  }
  return readPayloadText(entry.payload, ['operator_name', 'actor_name', 'name'])
}

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : {}
}

function readPayloadNumber(payload: unknown, keys: string[]): number {
  const record = asRecord(payload)
  for (const key of keys) {
    const raw = record[key]
    if (raw === null || raw === undefined || raw === '') continue
    const value = Number(raw)
    if (Number.isFinite(value) && value > 0) return value
  }
  return 0
}

function readPayloadText(payload: unknown, keys: string[]): string {
  const record = asRecord(payload)
  for (const key of keys) {
    const text = String(record[key] ?? '').trim()
    if (text && text !== 'null') return text
  }
  return ''
}
