import { mockTaskModules } from '../db/taskModules'
import { pushTaskEvent } from '../db/events'
import type { MockHandler } from './types'
import { nowISO } from '@/utils/date'

const MOCK_ACTOR = 'mock_actor'
const MOCK_ACTOR_ID = 1

function withMockEventPayload(payload: Record<string, unknown>): Record<string, unknown> {
  return {
    ...payload,
    operator_name: MOCK_ACTOR,
    creator_id: MOCK_ACTOR_ID,
    creator_name: MOCK_ACTOR,
  }
}

export const taskModulesHandler: MockHandler = (request) => {
  const claimMatch = request.path.match(/^\/v1\/tasks\/([^/]+)\/modules\/([^/]+)\/claim$/)
  if (request.method === 'POST' && claimMatch) {
    const [, taskId, moduleKey] = claimMatch
    const target = mockTaskModules.find(
      (item) => item.task_id === taskId && item.module_key === moduleKey,
    )
    if (!target) return { status: 404, data: { message: 'module not found' } }
    if (target.claimed_by) {
      return { status: 409, data: { code: 'task_already_claimed', message: 'already claimed' } }
    }
    target.claimed_by = 'mock_actor'
    target.state = 'in_progress'
    target.updated_at = nowISO()
    pushTaskEvent({
      task_id: taskId,
      module_key: moduleKey,
      event_type: 'module.claimed',
      payload: withMockEventPayload({ actor: MOCK_ACTOR }),
    })
    return { status: 200, data: target }
  }

  const actionMatch = request.path.match(/^\/v1\/tasks\/([^/]+)\/modules\/([^/]+)\/actions\/([^/]+)$/)
  if (request.method === 'POST' && actionMatch) {
    const [, taskId, moduleKey, action] = actionMatch
    if (action !== 'submit' || !['customization', 'retouch'].includes(moduleKey)) {
      return { status: 403, data: { code: 'module_action_role_denied', message: '当前节点不支持该操作' } }
    }
    const target = mockTaskModules.find(
      (item) => item.task_id === taskId && item.module_key === moduleKey,
    )
    if (!target) return { status: 404, data: { message: 'module not found' } }
    target.state = 'submitted'
    target.updated_at = nowISO()
    pushTaskEvent({
      task_id: taskId,
      module_key: moduleKey,
      event_type: `module.${action}`,
      payload: withMockEventPayload({ actor: MOCK_ACTOR }),
    })
    return { status: 200, data: target }
  }

  return null
}
