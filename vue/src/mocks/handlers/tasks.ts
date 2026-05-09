import { mockTaskEvents, pushTaskEvent } from '../db/events'
import { instantiateModulesForTask } from '../db/blueprint-resolve'
import { mockTasks, upsertTask, type MockTask, type MockTaskStatus } from '../db/tasks'
import { listTaskModules, mockTaskModules, type MockModuleState } from '../db/taskModules'
import { removeTaskDraft } from '../db/taskDrafts'
import type { MockHandler, MockHttpResponse } from './types'
import { getBeijingDateCompactString, nowISO } from '@/utils/date'

const MOCK_ACTOR = 'ops_demo'
const MOCK_ACTOR_ID = 1
const MOCK_ACTOR_TEAM = 'ungrouped'

function withMockEventPayload(payload: unknown): Record<string, unknown> {
  const base =
    payload && typeof payload === 'object' && !Array.isArray(payload)
      ? (payload as Record<string, unknown>)
      : {}
  return {
    ...base,
    operator_name: MOCK_ACTOR,
    creator_id: MOCK_ACTOR_ID,
    creator_name: MOCK_ACTOR,
  }
}

function parseTaskIdFromRoot(path: string): string | null {
  const match = path.match(/^\/v1\/tasks\/([^/]+)$/)
  return match?.[1] ?? null
}

function filterTasks(
  q: Record<string, unknown>,
): { items: MockTask[]; total: number; page: number; page_size: number } {
  const page = Math.max(1, Number(q.page ?? 1))
  const pageSize = Math.min(100, Math.max(1, Number(q.page_size ?? 20)))
  let items = [...mockTasks]

  const status = String(q.status ?? q.filter_status ?? '')
  const taskStatus = String(q.task_status ?? '').trim()
  const taskType = String(q.task_type ?? '')
  const priority = String(q.priority ?? '')
  const keyword = String(q.keyword ?? '').trim().toLowerCase()
  const filter = String(q.filter ?? '')
  const filterMine = filter === 'mine'
  const filterPool = filter === 'pool' || filter === 'module_pool'
  const actorTeam = String(q.pool_team_code ?? MOCK_ACTOR_TEAM).trim()
  const workflowLane = String(q.workflow_lane ?? '')
  const dateFrom = String(q.date_from ?? '').trim()
  const dateTo = String(q.date_to ?? '').trim()
  const sort = String(q.sort ?? '-updated_at').trim()
  const ownerTeam = String(q.owner_team ?? '').trim()
  const ownerOrgTeam = String(q.owner_org_team ?? '').trim()

  if (filterMine) {
    items = items.filter(
      (t) => t.created_by === MOCK_ACTOR || t.status === 'in_progress' || t.status === 'submitted',
    )
  }
  if (filterPool) {
    const poolTaskIds = new Set(poolRows(actorTeam).map((row) => row.id))
    items = items.filter((t) => poolTaskIds.has(t.id))
  }
  if (workflowLane === 'normal') {
    items = items.filter(
      (t) => t.task_type !== 'customer_customization' && t.task_type !== 'regular_customization',
    )
  } else if (workflowLane === 'customization') {
    items = items.filter(
      (t) => t.task_type === 'customer_customization' || t.task_type === 'regular_customization',
    )
  }
  if (status && status !== 'all') {
    if (status === 'archived') {
      items = items.filter((t) => t.status === 'closed' || t.status === 'cancelled')
    } else {
      items = items.filter((t) => t.status === status)
    }
  }
  if (taskStatus) {
    const statusTokens = taskStatus
      .split(',')
      .map((v) => v.trim().toLowerCase())
      .filter(Boolean)
      .map((token) => (token === 'pendingassign' ? 'pending_claim' : token))
    if (statusTokens.length > 0) {
      items = items.filter((t) => statusTokens.includes(t.status.toLowerCase()))
    }
  }
  if (ownerTeam === '未分配池' || ownerOrgTeam === '未分配池') {
    items = items.filter((t) => t.status === 'pending_claim')
  }
  if (taskType) {
    items = items.filter((t) => t.task_type === taskType)
  }
  if (priority) {
    items = items.filter((t) => t.priority === priority)
  }
  const creatorId = String(q.creator_id ?? '').trim()
  if (creatorId) {
    items = items.filter(
      (t) =>
        String((t as { creator_id?: string }).creator_id ?? t.created_by) === creatorId,
    )
  }
  if (keyword) {
    items = items.filter(
      (t) => t.title.toLowerCase().includes(keyword) || t.task_no.toLowerCase().includes(keyword),
    )
  }
  if (dateFrom) items = items.filter((t) => (t.updated_at ?? '') >= dateFrom)
  if (dateTo) items = items.filter((t) => (t.updated_at ?? '') <= `${dateTo}T23:59:59.999Z`)

  // `-field` 降序、`field` 升序；仅支持 updated_at / created_at / priority
  const sortKey = (sort.startsWith('-') ? sort.slice(1) : sort) as keyof MockTask
  const dir = sort.startsWith('-') ? -1 : 1
  items.sort((a, b) => {
    const va = String(a[sortKey] ?? '')
    const vb = String(b[sortKey] ?? '')
    return va < vb ? -1 * dir : va > vb ? 1 * dir : 0
  })

  const total = items.length
  const start = (page - 1) * pageSize
  items = items.slice(start, start + pageSize)
  return { items, total, page, page_size: pageSize }
}

function modulePoolTeamCode(moduleKey: string): string {
  if (moduleKey === 'design' || moduleKey === 'retouch') return 'design_standard'
  if (moduleKey === 'audit') return 'audit_standard'
  if (moduleKey === 'customization') return 'customization_art'
  if (moduleKey === 'procurement') return 'procurement_standard'
  if (moduleKey === 'warehouse') return 'warehouse_standard'
  return 'ops_standard'
}

function poolRows(actorTeam: string = MOCK_ACTOR_TEAM): Array<MockTask & { module_key: string }> {
  const rows: Array<MockTask & { module_key: string }> = []
  for (const task of mockTasks) {
    const modules = listTaskModules(task.id)
    for (const m of modules) {
      if (m.state === 'pending_claim' && modulePoolTeamCode(m.module_key) === actorTeam) {
        rows.push({
          ...task,
          module_key: m.module_key,
        })
      }
    }
  }
  return rows
}

function retouchWorkflowCodeFromModuleState(state: string | undefined): string {
  const s = String(state ?? 'pending_claim').toLowerCase().replace(/-/g, '_')
  if (s === 'closed') return 'completed'
  return s
}

function applyDesignerAssignToMockTask(
  task: MockTask,
  taskId: string,
  requestBody: { designer_id?: unknown; designer_name?: unknown; remark?: unknown },
): MockHttpResponse | null {
  const taskRecord = task as unknown as Record<string, unknown>
  const incomingDesignerId = requestBody?.designer_id
  const normalizedIncomingDesignerId =
    incomingDesignerId == null || String(incomingDesignerId).trim() === ''
      ? null
      : Number.parseInt(String(incomingDesignerId), 10)
  if (normalizedIncomingDesignerId !== null && Number.isNaN(normalizedIncomingDesignerId)) {
    return { status: 422, data: { code: 'VALIDATION_ERROR', message: 'invalid designer_id' } } satisfies MockHttpResponse
  }

  const existingDesignerId =
    taskRecord.designer_id == null || String(taskRecord.designer_id).trim() === ''
      ? null
      : Number.parseInt(String(taskRecord.designer_id), 10)
  const existingCurrentHandlerId =
    taskRecord.current_handler_id == null || String(taskRecord.current_handler_id).trim() === ''
      ? null
      : Number.parseInt(String(taskRecord.current_handler_id), 10)

  if (
    normalizedIncomingDesignerId !== null &&
    ((existingDesignerId != null && existingDesignerId !== normalizedIncomingDesignerId) ||
      (existingCurrentHandlerId != null &&
        existingCurrentHandlerId !== normalizedIncomingDesignerId))
  ) {
    return {
      status: 409,
      data: { code: 'task_already_claimed', message: 'task already claimed by another actor' },
    } satisfies MockHttpResponse
  }

  if (normalizedIncomingDesignerId === null) {
    taskRecord.designer_id = null
    taskRecord.current_handler_id = null
    taskRecord.designer_name = null
    taskRecord.current_handler_name = null
    task.status = 'pending_claim'
  } else {
    const designerName =
      typeof requestBody?.designer_name === 'string' && requestBody.designer_name.trim() !== ''
        ? requestBody.designer_name.trim()
        : `user_${normalizedIncomingDesignerId}`
    taskRecord.designer_id = normalizedIncomingDesignerId
    taskRecord.current_handler_id = normalizedIncomingDesignerId
    taskRecord.designer_name = designerName
    taskRecord.current_handler_name = designerName
    task.status = 'in_progress'
  }

  task.updated_at = nowISO()
  pushTaskEvent({
    task_id: taskId,
    module_key: task.task_type === 'retouch_task' ? 'retouch' : 'design',
    event_type: normalizedIncomingDesignerId === null ? 'task.unassigned' : 'task.assigned',
    payload: withMockEventPayload({
      designer_id: normalizedIncomingDesignerId,
      designer_name: requestBody?.designer_name,
    }),
  })
  return null
}

function updateModuleState(taskId: string, moduleKey: string, state: MockModuleState): void {
  for (const module of mockTaskModules.filter(
    (item) => item.task_id === taskId && item.module_key === moduleKey,
  )) {
    module.state = state
    module.updated_at = nowISO()
  }
}

function getTaskOr404(taskId: string): MockTask | null {
  return mockTasks.find((item) => item.id === taskId) ?? null
}

export const tasksHandler: MockHandler = (request) => {
  if (request.method === 'GET' && request.path === '/v1/tasks') {
    return {
      status: 200,
      data: {
        ...filterTasks(request.query as Record<string, unknown>),
      },
    }
  }

  if (request.method === 'GET' && request.path === '/v1/tasks/pool') {
    const page = Math.max(1, Number(request.query.page ?? 1))
    const pageSize = Math.min(100, Math.max(1, Number(request.query.page_size ?? 20)))
    const actorTeam = String(request.query.pool_team_code ?? MOCK_ACTOR_TEAM).trim()
    let items = poolRows(actorTeam)
    const moduleKey = String(request.query.module_key ?? '')
    if (moduleKey && moduleKey !== 'any') {
      items = items.filter((item) => item.module_key === moduleKey)
    }
    const total = items.length
    const start = (page - 1) * pageSize
    return {
      status: 200,
      data: {
        items: items.slice(start, start + pageSize),
        total,
        page,
        page_size: pageSize,
      },
    }
  }

  if (request.method === 'GET' && request.path.match(/^\/v1\/tasks\/[^/]+\/events$/)) {
    const taskId = request.path.split('/')[3] ?? ''
    const items = mockTaskEvents
      .filter((e) => e.task_id === taskId)
      .sort((a, b) => b.created_at.localeCompare(a.created_at))
    return { status: 200, data: { items } }
  }

  if (request.method === 'GET' && request.path.match(/^\/v1\/tasks\/[^/]+\/detail$/)) {
    const taskId = request.path.split('/')[3] ?? ''
    const task = mockTasks.find((item) => item.id === taskId)
    if (!task) return { status: 404, data: { message: 'task not found' } }
    const modules = listTaskModules(taskId).map((m) => ({
      module_key: m.module_key,
      state: m.state,
      scope: {
        visible: true,
        in_scope: true,
        deny_code: undefined as string | undefined,
      },
      allowed_actions: { actions: m.allowed_actions ?? [] },
    }))
    const retouchMod = listTaskModules(taskId).find((m) => m.module_key === 'retouch')
    const rtCode =
      task.task_type === 'retouch_task'
        ? retouchWorkflowCodeFromModuleState(retouchMod?.state)
        : null
    const workflow =
      task.task_type === 'retouch_task'
        ? {
            main_status: 'created',
            sub_status: {
              design: { code: rtCode, label: 'Retouch', source: 'retouch_module' },
              retouch: { code: rtCode, label: 'Retouch', source: 'module' },
              audit: { code: 'not_triggered', label: 'Not triggered', source: 'task_type' },
              procurement: { code: 'not_triggered', label: 'Not triggered', source: 'task_type' },
              warehouse: { code: 'not_triggered', label: 'Not triggered', source: 'task_status' },
              customization: { code: 'not_triggered', label: 'Not triggered', source: 'task_type' },
              outsource: { code: 'not_triggered', label: 'Not triggered', source: 'task_type' },
              production: { code: 'reserved', label: 'Reserved', source: 'reserved' },
            },
          }
        : undefined
    return {
      status: 200,
      data: {
        task,
        ...(workflow ? { workflow } : {}),
        task_detail: {
          category_code: 'mock_category',
          design_requirement: 'mock 设计需求',
          remark: 'mock 备注',
          note: '',
          spec_text: 'mock 规格',
        },
        reference_file_refs: [
          {
            filename: 'mock-ref.png',
            mime_type: 'image/png',
            download_url: '/v1/assets/files/mock/ref.png',
          },
        ],
        modules,
      },
    }
  }

  if (request.method === 'GET') {
    const taskId = parseTaskIdFromRoot(request.path)
    if (taskId) {
      const task = mockTasks.find((item) => item.id === taskId)
      if (!task) return { status: 404, data: { message: 'task not found' } }
      return { status: 200, data: task }
    }
  }

  if (request.method === 'POST' && request.path === '/v1/tasks') {
    const taskType = String(request.body?.task_type ?? 'original_product_development')
    const title = String(request.body?.title ?? '新建任务')
    const sourceDraftId = String(request.body?.source_draft_id ?? '').trim()
    if (sourceDraftId) {
      removeTaskDraft(sourceDraftId)
    }
    const id = `task_${Date.now()}`
    const taskNo = `T-${getBeijingDateCompactString()}-${String(mockTasks.length + 1).padStart(4, '0')}`
    const now = nowISO()
    const newTask: MockTask = {
      id,
      task_no: taskNo,
      task_type: taskType,
      title,
      priority: 'normal',
      status: 'pending_claim',
      created_by: MOCK_ACTOR,
      created_at: now,
      updated_at: now,
    }
    upsertTask(newTask)
    instantiateModulesForTask(id, taskType)
    pushTaskEvent({
      task_id: id,
      module_key: 'basic_info',
      event_type: 'task.created',
      payload: withMockEventPayload({ task_type: taskType }),
    })
    return { status: 201, data: newTask }
  }

  if (request.method === 'PATCH' && request.path.match(/^\/v1\/tasks\/[^/]+\/business-info$/)) {
    const taskId = request.path.split('/')[3] ?? ''
    const task = getTaskOr404(taskId)
    if (!task) return { status: 404, data: { message: 'task not found' } }
    task.updated_at = nowISO()
    pushTaskEvent({
      task_id: taskId,
      module_key: 'basic_info',
      event_type: 'business_info.updated',
      payload: withMockEventPayload(request.body),
    })
    return { status: 200, data: { id: task.id, ...request.body } }
  }

  if (request.method === 'POST' && request.path.match(/^\/v1\/tasks\/[^/]+\/submit-design$/)) {
    const taskId = request.path.split('/')[3] ?? ''
    const task = getTaskOr404(taskId)
    if (!task) return { status: 404, data: { message: 'task not found' } }
    const isRetouchTask = task.task_type === 'retouch_task'
    const taskRecord = task as unknown as Record<string, unknown>
    task.status = isRetouchTask ? 'completed' : 'submitted'
    task.updated_at = nowISO()
    if (isRetouchTask) {
      updateModuleState(taskId, 'retouch', 'closed')
      taskRecord.current_handler_id = null
      taskRecord.current_handler_name = null
    } else {
      updateModuleState(taskId, 'design', 'submitted')
      updateModuleState(taskId, 'audit', 'pending_claim')
    }
    pushTaskEvent({
      task_id: taskId,
      module_key: isRetouchTask ? 'retouch' : 'design',
      event_type: 'submitted',
      payload: withMockEventPayload(request.body),
    })
    return { status: 200, data: { id: task.id, status: task.status } }
  }

  if (
    request.method === 'POST' &&
    request.path.match(/^\/v1\/tasks\/[^/]+\/modules\/retouch\/actions\/submit$/)
  ) {
    const taskId = request.path.split('/')[3] ?? ''
    const task = getTaskOr404(taskId)
    if (!task) return { status: 404, data: { message: 'task not found' } }
    task.status = 'completed'
    task.updated_at = nowISO()
    updateModuleState(taskId, 'retouch', 'closed')
    const taskRecord = task as unknown as Record<string, unknown>
    taskRecord.current_handler_id = null
    taskRecord.current_handler_name = null
    pushTaskEvent({
      task_id: taskId,
      module_key: 'retouch',
      event_type: 'submitted',
      payload: withMockEventPayload(request.body),
    })
    return { status: 200, data: { id: task.id, status: task.status } }
  }

  if (request.method === 'POST' && request.path.match(/^\/v1\/tasks\/[^/]+\/modules\/retouch\/reassign$/)) {
    const taskId = request.path.split('/')[3] ?? ''
    const task = getTaskOr404(taskId)
    if (!task) return { status: 404, data: { message: 'task not found' } }
    if (task.task_type !== 'retouch_task') {
      return { status: 409, data: { code: 'INVALID_STATE', message: 'not a retouch task' } }
    }
    const err = applyDesignerAssignToMockTask(task, taskId, request.body ?? {})
    if (err) return err
    return { status: 202, data: { data: task } }
  }

  if (request.method === 'POST' && request.path.match(/^\/v1\/tasks\/[^/]+\/assign$/)) {
    const taskId = request.path.split('/')[3] ?? ''
    const task = getTaskOr404(taskId)
    if (!task) return { status: 404, data: { message: 'task not found' } }
    const err = applyDesignerAssignToMockTask(task, taskId, request.body ?? {})
    if (err) return err
    return { status: 200, data: { data: task } }
  }

  if (request.method === 'POST' && request.path.match(/^\/v1\/tasks\/[^/]+\/audit\/approve$/)) {
    const taskId = request.path.split('/')[3] ?? ''
    const task = getTaskOr404(taskId)
    if (!task) return { status: 404, data: { message: 'task not found' } }
    task.status = 'approved'
    task.updated_at = nowISO()
    updateModuleState(taskId, 'audit', 'approved')
    updateModuleState(taskId, 'warehouse', 'pending')
    pushTaskEvent({
      task_id: taskId,
      module_key: 'audit',
      event_type: 'approved',
      payload: withMockEventPayload(request.body),
    })
    return { status: 200, data: { id: task.id, status: task.status } }
  }

  if (request.method === 'POST' && request.path.match(/^\/v1\/tasks\/[^/]+\/audit\/reject$/)) {
    const taskId = request.path.split('/')[3] ?? ''
    const task = getTaskOr404(taskId)
    if (!task) return { status: 404, data: { message: 'task not found' } }
    task.status = 'rejected'
    task.updated_at = nowISO()
    updateModuleState(taskId, 'audit', 'rejected')
    updateModuleState(taskId, task.task_type === 'regular_customization' || task.task_type === 'customer_customization' ? 'customization' : 'design', 'pending_claim')
    pushTaskEvent({
      task_id: taskId,
      module_key: 'audit',
      event_type: 'rejected',
      payload: withMockEventPayload(request.body),
    })
    return { status: 200, data: { id: task.id, status: task.status } }
  }

  if (request.method === 'POST' && request.path.match(/^\/v1\/tasks\/[^/]+\/warehouse\/receive$/)) {
    const taskId = request.path.split('/')[3] ?? ''
    const task = getTaskOr404(taskId)
    if (!task) return { status: 404, data: { message: 'task not found' } }
    task.status = 'in_progress'
    task.updated_at = nowISO()
    updateModuleState(taskId, 'warehouse', 'in_progress')
    pushTaskEvent({
      task_id: taskId,
      module_key: 'warehouse',
      event_type: 'received',
      payload: withMockEventPayload(request.body),
    })
    return { status: 200, data: { id: task.id, status: task.status, warehouse_status: 'received' } }
  }

  if (request.method === 'POST' && request.path.match(/^\/v1\/tasks\/[^/]+\/warehouse\/complete$/)) {
    const taskId = request.path.split('/')[3] ?? ''
    const task = getTaskOr404(taskId)
    if (!task) return { status: 404, data: { message: 'task not found' } }
    task.status = 'completed'
    task.updated_at = nowISO()
    updateModuleState(taskId, 'warehouse', 'closed')
    pushTaskEvent({
      task_id: taskId,
      module_key: 'warehouse',
      event_type: 'archived',
      payload: withMockEventPayload(request.body),
    })
    return { status: 200, data: { id: task.id, status: task.status } }
  }

  if (request.method === 'POST' && request.path.match(/^\/v1\/tasks\/[^/]+\/warehouse\/reject$/)) {
    const taskId = request.path.split('/')[3] ?? ''
    const task = getTaskOr404(taskId)
    if (!task) return { status: 404, data: { message: 'task not found' } }
    task.status = 'rejected'
    task.updated_at = nowISO()
    updateModuleState(taskId, 'warehouse', 'rejected')
    pushTaskEvent({
      task_id: taskId,
      module_key: 'warehouse',
      event_type: 'rejected',
      payload: withMockEventPayload(request.body),
    })
    return { status: 200, data: { id: task.id, status: task.status } }
  }

  if (request.method === 'POST' && request.path.match(/^\/v1\/tasks\/[^/]+\/close$/)) {
    const taskId = request.path.split('/')[3] ?? ''
    const task = getTaskOr404(taskId)
    if (!task) return { status: 404, data: { message: 'task not found' } }
    task.status = 'closed'
    task.updated_at = nowISO()
    updateModuleState(taskId, 'warehouse', 'closed')
    pushTaskEvent({
      task_id: taskId,
      module_key: 'warehouse',
      event_type: 'close_task',
      payload: withMockEventPayload(request.body),
    })
    return { status: 200, data: { id: task.id, status: task.status } }
  }

  if (request.method === 'POST' && request.path.match(/^\/v1\/tasks\/[^/]+\/cancel$/)) {
    const taskId = request.path.split('/')[3] ?? ''
    const task = mockTasks.find((item) => item.id === taskId)
    if (!task) return { status: 404, data: { message: 'task not found' } }
    const force = Boolean(request.body?.force)
    if (!force) {
      const hasClaimedModule = listTaskModules(taskId).some((module) => Boolean(module.claimed_by))
      if (hasClaimedModule) {
        return {
          status: 409,
          data: {
            code: 'task_already_claimed',
            message: 'task already claimed by another actor',
          },
        }
      }
    }
    const nextStatus: MockTaskStatus = force ? 'closed' : 'cancelled'
    task.status = nextStatus
    task.updated_at = nowISO()
    for (const m of mockTaskModules.filter((x) => x.task_id === taskId)) {
      m.state = 'closed'
      m.updated_at = task.updated_at
    }
    pushTaskEvent({
      task_id: taskId,
      module_key: 'basic_info',
      event_type: force ? 'forcibly_closed' : 'task_cancelled',
      payload: withMockEventPayload({ reason: String(request.body?.reason ?? '') }),
    })
    return { status: 200, data: { id: task.id, status: task.status } }
  }

  return null
}
