import http from '@/services/http'

export interface TaskDraftPayload {
  draft_id?: string
  task_type: string
  payload: Record<string, unknown>
}

export const taskDraftsApi = {
  /** 当前用户草稿（Mock：`created_by` 与登录角色对齐） */
  listMine: (signal?: AbortSignal) => http.get('/v1/me/task-drafts', { signal }),
  create: (payload: TaskDraftPayload, signal?: AbortSignal) =>
    http.post('/v1/task-drafts', payload, { signal }),
  getById: (id: string, signal?: AbortSignal) =>
    http.get(`/v1/task-drafts/${encodeURIComponent(id)}`, { signal }),
  update: (id: string, payload: TaskDraftPayload, signal?: AbortSignal) =>
    http.post('/v1/task-drafts', { ...payload, draft_id: id }, { signal }),
  deleteById: (id: string, signal?: AbortSignal) =>
    http.delete(`/v1/task-drafts/${encodeURIComponent(id)}`, { signal }),
}
