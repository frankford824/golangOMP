import { mockTaskDrafts, removeTaskDraft, type MockTaskDraft } from '../db/taskDrafts'
import type { MockHandler } from './types'
import { addMillisecondsToNowISO, nowISO } from '@/utils/date'

const MOCK_USER = 'ops_demo'

function findDraft(id: string): MockTaskDraft | undefined {
  return mockTaskDrafts.find((d) => d.id === id)
}

export const taskDraftsHandler: MockHandler = (request) => {
  if (request.method === 'GET' && request.path === '/v1/me/task-drafts') {
    const items = mockTaskDrafts.filter((d) => d.created_by === MOCK_USER)
    return { status: 200, data: { items, total: items.length } }
  }

  const detailGetPut = request.path.match(/^\/v1\/task-drafts\/([^/]+)$/)
  if (detailGetPut) {
    const id = detailGetPut[1] ?? ''
    if (request.method === 'GET') {
      const draft = findDraft(id)
      if (!draft) return { status: 404, data: { message: 'draft not found' } }
      return { status: 200, data: draft }
    }
    if (request.method === 'DELETE') {
      const deleted = removeTaskDraft(id)
      return deleted
        ? { status: 200, data: { success: true } }
        : { status: 404, data: { message: 'draft not found' } }
    }
  }

  if (request.path === '/v1/task-drafts' && request.method === 'POST') {
    const now = nowISO()
    const incomingId = String(request.body?.draft_id ?? '').trim()
    if (incomingId) {
      const draft = findDraft(incomingId)
      if (!draft) return { status: 404, data: { message: 'draft not found' } }
      draft.task_type = String((request.body as { task_type?: string })?.task_type ?? draft.task_type)
      draft.payload = (request.body as { payload?: Record<string, unknown> })?.payload ?? draft.payload
      draft.updated_at = now
      return { status: 200, data: draft }
    }
    const id = `draft_${Date.now()}`
    const next: MockTaskDraft = {
      id,
      task_type: String((request.body as { task_type?: string })?.task_type ?? 'original_product_development'),
      payload: (request.body as { payload?: Record<string, unknown> })?.payload ?? (request.body as Record<string, unknown>) ?? {},
      created_by: MOCK_USER,
      created_at: now,
      updated_at: now,
      expires_at: addMillisecondsToNowISO(7 * 24 * 60 * 60_000),
    }
    mockTaskDrafts.unshift(next)
    return { status: 201, data: next }
  }

  return null
}
