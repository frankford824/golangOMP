import { mockTaskEvents, pushTaskEvent } from '../db/events'
import { instantiateModulesForTask } from '../db/blueprint-resolve'
import { mockTasks, upsertTask, type MockTask, type MockTaskStatus } from '../db/tasks'
import { listTaskModules, mockTaskModules, type MockModuleState } from '../db/taskModules'
import { removeTaskDraft } from '../db/taskDrafts'
import type { MockHandler, MockHttpResponse } from './types'
import { addMillisecondsToNowISO, getBeijingDateCompactString, nowISO } from '@/utils/date'

const MOCK_ACTOR = 'ops_demo'
const MOCK_ACTOR_ID = 1
const MOCK_ACTOR_TEAM = 'ungrouped'
const MOCK_REFERENCE_PREVIEW = 'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" width="320" height="220" viewBox="0 0 320 220"%3E%3Crect width="320" height="220" rx="20" fill="%23eef3fb"/%3E%3Cpath d="M72 161l56-58 39 37 31-29 50 50H72z" fill="%2394a9c8"/%3E%3Ccircle cx="220" cy="70" r="24" fill="%23f6c66d"/%3E%3Ctext x="160" y="194" text-anchor="middle" font-family="sans-serif" font-size="15" fill="%23435b7d"%3E%E8%BF%90%E8%90%A5%E5%8F%82%E8%80%83%E5%9B%BE%3C/text%3E%3C/svg%3E'
const LARGE_SURFACE_AUDIT_TOTAL = Number(import.meta.env.VITE_LARGE_SURFACE_TOTAL ?? 5000)
const LARGE_SURFACE_AUDIT_PAGE_SIZE = Number(import.meta.env.VITE_LARGE_SURFACE_PAGE_SIZE ?? 100)

function isLargeSurfaceAuditEnabled(): boolean {
  return import.meta.env.VITE_LARGE_SURFACE_AUDIT === 'true'
}

function largeSurfacePageSize(raw: unknown): number {
  const fallback = Number.isFinite(LARGE_SURFACE_AUDIT_PAGE_SIZE) ? LARGE_SURFACE_AUDIT_PAGE_SIZE : 100
  const candidate = Math.max(fallback, Number(raw ?? fallback))
  return Math.min(150, Math.max(80, Math.floor(candidate)))
}

function largeSurfaceTasks(q: Record<string, unknown>): { items: MockTask[]; total: number; page: number; page_size: number } {
  const page = Math.max(1, Number(q.page ?? 1))
  const pageSize = largeSurfacePageSize(q.page_size)
  const total = Number.isFinite(LARGE_SURFACE_AUDIT_TOTAL) ? LARGE_SURFACE_AUDIT_TOTAL : 5000
  const statuses: MockTaskStatus[] = ['pending_claim', 'in_progress', 'submitted', 'approved', 'completed']
  const priorities: MockTask['priority'][] = ['normal', 'high', 'drawing']
  const start = (page - 1) * pageSize
  const items = Array.from({ length: pageSize }, (_, index) => {
    const seq = start + index + 1
    return {
      id: String(900000 + seq),
      task_no: `LT-${String(seq).padStart(5, '0')}`,
      task_type: seq % 6 === 0 ? 'customer_customization' : 'original_product_development',
      title: `长列表承载审计任务 ${String(seq).padStart(5, '0')}`,
      priority: priorities[seq % priorities.length],
      status: statuses[seq % statuses.length],
      created_by: seq % 3 === 0 ? 'design_ops' : MOCK_ACTOR,
      created_at: addMillisecondsToNowISO(-seq * 60_000),
      updated_at: addMillisecondsToNowISO(-seq * 30_000),
    }
  })
  return { items, total, page, page_size: pageSize }
}

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

function v8NumericTaskContract(task: MockTask, taskId: string) {
  const isRetouchTask = task.task_type === 'retouch_task'
  return {
    ...task,
    id: Number(taskId),
    task_status: task.status === 'completed' ? 'Completed' : isRetouchTask ? 'InProgress' : 'PendingAudit',
    workflow_revision: 3,
    workflow_contract_version: 2,
    business_lane: task.task_type === 'customer_customization' ? 'customization' : 'normal',
    allowed_actions: isRetouchTask
      ? ['task.design.submit', 'task.assign']
      : ['task.audit.approve', 'task.audit.return_to_design', 'task.audit.handover', 'task.assign'],
    product_name_snapshot: task.title,
    primary_sku_code: 'SKU-MOCK-1002',
    current_handler_name: isRetouchTask ? '修图演示' : '审核演示',
    owner_department: '设计部',
    owner_org_team: '主图组',
    requirement_description: '完成主图设计并核对套装顺序。',
    operation_note: '请在审核通过前检查全部最终成品。',
    reference_file_refs: [{ id: 1, file_name: '参考图.png', download_url: '/mock-assets/reference.png' }],
  }
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
  const businessLane = String(q.business_lane ?? '')
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
  if (businessLane === 'normal') {
    items = items.filter(
      (t) => t.task_type !== 'customer_customization' && t.task_type !== 'regular_customization',
    )
  } else if (businessLane === 'customization') {
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
  if (request.method === 'GET' && request.path === '/v1/task-board/overview') {
    const now = nowISO()
    return {
      status: 200,
      data: {
        data: {
          generated_at: now,
          time_zone: 'Asia/Shanghai',
          period_start: now,
          period_end: now,
          health_status: 'ok',
          counts: {
            total_tasks: 24,
            active_tasks: 12,
            design_pending: 7,
            pending_audit: 2,
            handover: 1,
            customization_in_progress: 2,
            overdue: 1,
            due_today: 4,
            today_created: 5,
            today_completed: 3,
          },
          kpis: {
            week_created: 18,
            week_created_completed: 10,
            week_completion_rate: 55.6,
            week_audit_decisions: 12,
            week_audit_rejected: 1,
            week_reject_rate: 8.3,
            week_completed: 11,
            average_processing_hours: 16.8,
            average_processing_sample_count: 11,
            exact_completion_sample_count: 11,
            fallback_completion_sample_count: 0,
            completion_event_coverage_rate: 100,
          },
          trend: [
            ['2026-07-07', 2, 1, 3],
            ['2026-07-08', 3, 2, 4],
            ['2026-07-09', 2, 1, 2],
            ['2026-07-10', 4, 3, 3],
            ['2026-07-11', 1, 1, 2],
            ['2026-07-12', 3, 2, 3],
            ['2026-07-13', 5, 3, 4],
          ].map(([date, created, completed, due]) => ({ date, created, completed, due })),
          status_distribution: [
            { key: 'design_ops', name: '设计/运营待推进', count: 7 },
            { key: 'audit', name: '待审核', count: 2 },
            { key: 'customization', name: '定制协同', count: 2 },
			{ key: 'blocked', name: '异常待处理', count: 2 },
            { key: 'completed', name: '已完成/终止', count: 11 },
          ],
          recent_tasks: [
            {
              task_id: 1,
              task_no: 'RW-MOCK-001',
              product_name: '演示新品任务',
              owner_name: '演示账号',
              task_status: 'InProgress',
              deadline_at: now,
            },
          ],
          recent_events: [
            {
              id: 'mock-dashboard-event-1',
              event_type: 'task.closed',
              title: '任务结单',
              task_id: 1,
              task_no: 'RW-MOCK-001',
              actor_name: '演示账号',
              created_at: now,
            },
          ],
        },
      },
    }
  }

  if (request.method === 'GET' && request.path === '/v1/tasks') {
    if (isLargeSurfaceAuditEnabled()) {
      return {
        status: 200,
        data: {
          ...largeSurfaceTasks(request.query as Record<string, unknown>),
        },
      }
    }
    return {
      status: 200,
      data: {
        ...filterTasks(request.query as Record<string, unknown>),
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
    const storedTask = mockTasks.find((item) => item.id === taskId || item.id === `task_${taskId}`)
    if (!storedTask) return { status: 404, data: { message: 'task not found' } }
    const task = /^\d+$/.test(taskId) ? v8NumericTaskContract(storedTask, taskId) : storedTask
    const modules = listTaskModules(storedTask.id).map((m) => ({
      module_key: m.module_key,
      state: m.state,
      scope: {
        visible: true,
        in_scope: true,
        deny_code: undefined as string | undefined,
      },
      allowed_actions: { actions: m.allowed_actions ?? [] },
    }))
    return {
      status: 200,
      data: {
        task,
        design_sub_status:
          storedTask.status === 'completed'
            ? 'finalized'
            : storedTask.status === 'submitted'
              ? 'pending_audit'
              : 'in_progress',
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
            download_url: MOCK_REFERENCE_PREVIEW,
          },
        ],
        modules,
      },
    }
  }

  if (request.method === 'GET') {
    const taskId = parseTaskIdFromRoot(request.path)
    if (taskId) {
      const task = mockTasks.find((item) => item.id === taskId || item.id === `task_${taskId}`)
      if (!task) return { status: 404, data: { message: 'task not found' } }
      if (/^\d+$/.test(taskId)) {
        return { status: 200, data: v8NumericTaskContract(task, taskId) }
      }
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

  if (request.method === 'POST' && request.path.match(/^\/v1\/tasks\/[^/]+\/audit\/decision$/)) {
    const taskId = request.path.split('/')[3] ?? ''
    const task = getTaskOr404(taskId)
    if (!task) return { status: 404, data: { message: 'task not found' } }
    const action = String(request.body?.action ?? '')
    if (action !== 'approve' && action !== 'return_to_design') {
      return { status: 400, data: { code: 'INVALID_REQUEST', message: 'invalid audit action' } }
    }
    task.status = action === 'approve' ? 'completed' : 'in_progress'
    task.updated_at = nowISO()
    updateModuleState(taskId, 'audit', action === 'approve' ? 'closed' : 'pending_claim')
    if (action === 'return_to_design') updateModuleState(taskId, 'design', 'in_progress')
    pushTaskEvent({
      task_id: taskId,
      module_key: 'audit',
      event_type: action === 'approve' ? 'task.closed' : 'task.audit.returned_to_design',
      payload: withMockEventPayload(request.body),
    })
    return { status: 200, data: { id: task.id, status: task.status } }
  }

  if (request.method === 'POST' && request.path.match(/^\/v1\/tasks\/[^/]+\/cancel$/)) {
    const taskId = request.path.split('/')[3] ?? ''
    const task = mockTasks.find((item) => item.id === taskId)
    if (!task) return { status: 404, data: { message: 'task not found' } }
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
    task.status = 'cancelled'
    task.updated_at = nowISO()
    for (const m of mockTaskModules.filter((x) => x.task_id === taskId)) {
      m.state = 'closed'
      m.updated_at = task.updated_at
    }
    pushTaskEvent({
      task_id: taskId,
      module_key: 'basic_info',
      event_type: 'task.cancelled',
      payload: withMockEventPayload({ reason: String(request.body?.reason ?? '') }),
    })
    return { status: 200, data: { id: task.id, status: task.status } }
  }

  return null
}
