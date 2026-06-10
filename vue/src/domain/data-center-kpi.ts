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

export interface KpiDesignLifecycleEvent {
  taskKey: string
  actorKey: string
  at: number
  kind: 'assignment' | 'submission'
  deadlineMs?: number
  priorityWeight?: number
  inactiveWithoutSubmit?: boolean
}

export interface KpiDesignLifecycleStats {
  designClaims: number
  designCompletedClaims: number
  designTransferredOut: number
  designClosedWithoutSubmit: number
  designInHandClaims: number
  designDeadlineCompletions: number
  designOnTimeCompletions: number
  priorityClaims: number
  priorityInHandClaims: number
  priorityScore: number
  claimToSubmitMs: number[]
}

export function isKpiAssignmentOperation(entry: Pick<OperationLogEntry, 'event_type'>): boolean {
  return ['task.assigned', 'task.reassigned', 'task.batch_assigned'].includes(entry.event_type)
}

export function emptyKpiDesignLifecycleStats(): KpiDesignLifecycleStats {
  return {
    designClaims: 0,
    designCompletedClaims: 0,
    designTransferredOut: 0,
    designClosedWithoutSubmit: 0,
    designInHandClaims: 0,
    designDeadlineCompletions: 0,
    designOnTimeCompletions: 0,
    priorityClaims: 0,
    priorityInHandClaims: 0,
    priorityScore: 0,
    claimToSubmitMs: [],
  }
}

export function summarizeKpiDesignLifecycle(
  events: KpiDesignLifecycleEvent[],
): Map<string, KpiDesignLifecycleStats> {
  interface AssignmentState {
    actorKey: string
    at: number
    deadlineMs: number
    priorityWeight: number
    inactiveWithoutSubmit: boolean
  }

  const stats = new Map<string, KpiDesignLifecycleStats>()
  const currentByTask = new Map<string, AssignmentState>()

  const statFor = (actorKey: string) => {
    const existing = stats.get(actorKey)
    if (existing) return existing
    const next = emptyKpiDesignLifecycleStats()
    stats.set(actorKey, next)
    return next
  }
  const markTransferredOut = (assignment: AssignmentState) => {
    statFor(assignment.actorKey).designTransferredOut += 1
  }

  const sorted = events
    .filter((event) => event.taskKey && event.actorKey && Number.isFinite(event.at) && event.at > 0)
    .sort((a, b) => a.at - b.at)

  for (const event of sorted) {
    if (event.kind === 'assignment') {
      const current = currentByTask.get(event.taskKey)
      if (current) {
        if (current.actorKey === event.actorKey) {
          current.deadlineMs = event.deadlineMs || current.deadlineMs
          current.priorityWeight = Math.max(current.priorityWeight, event.priorityWeight || current.priorityWeight)
          current.inactiveWithoutSubmit = Boolean(event.inactiveWithoutSubmit)
          continue
        }
        markTransferredOut(current)
      }

      const priority = Number(event.priorityWeight || 0)
      const stat = statFor(event.actorKey)
      stat.designClaims += 1
      stat.priorityScore += priority
      if (priority >= 3) stat.priorityClaims += 1
      currentByTask.set(event.taskKey, {
        actorKey: event.actorKey,
        at: event.at,
        deadlineMs: Number(event.deadlineMs || 0),
        priorityWeight: priority,
        inactiveWithoutSubmit: Boolean(event.inactiveWithoutSubmit),
      })
      continue
    }

    const current = currentByTask.get(event.taskKey)
    if (!current || current.actorKey !== event.actorKey || event.at <= current.at) continue

    const stat = statFor(current.actorKey)
    stat.designCompletedClaims += 1
    stat.claimToSubmitMs.push(event.at - current.at)
    if (current.deadlineMs > 0) {
      stat.designDeadlineCompletions += 1
      if (event.at <= current.deadlineMs) stat.designOnTimeCompletions += 1
    }
    currentByTask.delete(event.taskKey)
  }

  for (const assignment of currentByTask.values()) {
    const stat = statFor(assignment.actorKey)
    if (assignment.inactiveWithoutSubmit) {
      stat.designClosedWithoutSubmit += 1
    } else {
      stat.designInHandClaims += 1
      if (assignment.priorityWeight >= 3) stat.priorityInHandClaims += 1
    }
  }

  return stats
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
