import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { TaskDraft } from '@/services/apiTypes'
import { taskDraftsApi, type TaskDraftPayload } from '@/services/api/taskDraftsApi'

function unwrapData(raw: unknown): unknown {
  if (!raw || typeof raw !== 'object') return raw
  const root = raw as Record<string, unknown>
  return root.data ?? raw
}

function normalizeDraft(raw: unknown): TaskDraft {
  const body = unwrapData(raw)
  const obj = body && typeof body === 'object' ? (body as Record<string, unknown>) : {}
  const idRaw = obj.id ?? obj.draft_id ?? ''
  return {
    id: String(idRaw),
    task_type: String(obj.task_type ?? ''),
    payload:
      obj.payload && typeof obj.payload === 'object'
        ? ((obj.payload as Record<string, unknown>).payload as Record<string, unknown>) ??
          (obj.payload as Record<string, unknown>)
        : {},
    created_at: typeof obj.created_at === 'string' ? obj.created_at : undefined,
    updated_at: typeof obj.updated_at === 'string' ? obj.updated_at : undefined,
    expires_at: typeof obj.expires_at === 'string' ? obj.expires_at : undefined,
    created_by: typeof obj.created_by === 'string' ? obj.created_by : undefined,
  }
}

export const useTaskDraftsStore = defineStore('taskDrafts', () => {
  const drafts = ref<TaskDraft[]>([])
  const loading = ref(false)

  async function load(): Promise<void> {
    loading.value = true
    try {
      const res = await taskDraftsApi.listMine()
      const body = unwrapData(res.data)
      const obj = body && typeof body === 'object' ? (body as Record<string, unknown>) : {}
      const items = Array.isArray(body)
        ? body
        : Array.isArray(obj.items)
          ? obj.items
          : []
      drafts.value = items.map(normalizeDraft)
    } finally {
      loading.value = false
    }
  }

  async function save(payload: TaskDraftPayload): Promise<void> {
    const res = await taskDraftsApi.create(payload)
    drafts.value.unshift(normalizeDraft(res.data))
  }

  async function getById(id: string): Promise<TaskDraft> {
    const res = await taskDraftsApi.getById(id)
    return normalizeDraft(res.data)
  }

  async function remove(id: string): Promise<void> {
    await taskDraftsApi.deleteById(id)
    drafts.value = drafts.value.filter((draft) => draft.id !== id)
  }

  return { drafts, loading, load, save, remove, getById }
})
