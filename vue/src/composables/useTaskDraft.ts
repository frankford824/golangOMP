import { ref } from 'vue'
import { taskDraftsApi, type TaskDraftPayload } from '@/services/api/taskDraftsApi'
import type { TaskDraft } from '@/services/apiTypes'

function unwrapData(raw: unknown): unknown {
  if (!raw || typeof raw !== 'object') return raw
  const root = raw as Record<string, unknown>
  return root.data ?? raw
}

function normalizeDraft(raw: unknown): TaskDraft {
  const body = unwrapData(raw)
  const obj = body && typeof body === 'object' ? (body as Record<string, unknown>) : {}
  const idRaw = obj.id ?? obj.draft_id ?? ''
  const payloadRaw = obj.payload
  const nestedPayload =
    payloadRaw && typeof payloadRaw === 'object'
      ? (payloadRaw as Record<string, unknown>).payload
      : undefined
  const payload =
    nestedPayload && typeof nestedPayload === 'object'
      ? (nestedPayload as Record<string, unknown>)
      : payloadRaw && typeof payloadRaw === 'object'
        ? (payloadRaw as Record<string, unknown>)
        : {}

  return {
    id: String(idRaw),
    task_type: String(obj.task_type ?? ''),
    payload,
    created_at: typeof obj.created_at === 'string' ? obj.created_at : undefined,
    updated_at: typeof obj.updated_at === 'string' ? obj.updated_at : undefined,
    expires_at: typeof obj.expires_at === 'string' ? obj.expires_at : undefined,
    created_by: typeof obj.created_by === 'string' ? obj.created_by : undefined,
  }
}

export function useTaskDraft() {
  const drafts = ref<TaskDraft[]>([])
  const saving = ref(false)

  async function load(): Promise<void> {
    const res = await taskDraftsApi.listMine()
    const body = unwrapData(res.data)
    const obj = body && typeof body === 'object' ? (body as Record<string, unknown>) : {}
    const items = Array.isArray(body)
      ? body
      : Array.isArray(obj.items)
        ? obj.items
        : []
    drafts.value = items.map(normalizeDraft)
  }

  async function save(payload: TaskDraftPayload): Promise<TaskDraft> {
    saving.value = true
    try {
      const res = await taskDraftsApi.create(payload)
      const created = normalizeDraft(res.data)
      drafts.value.unshift(created)
      return created
    } finally {
      saving.value = false
    }
  }

  async function getById(id: string): Promise<TaskDraft> {
    const res = await taskDraftsApi.getById(id)
    return normalizeDraft(res.data)
  }

  async function remove(id: string): Promise<void> {
    await taskDraftsApi.deleteById(id)
    drafts.value = drafts.value.filter((draft) => draft.id !== id)
  }

  async function update(id: string, payload: TaskDraftPayload): Promise<TaskDraft> {
    saving.value = true
    try {
      const res = await taskDraftsApi.update(id, payload)
      const updated = normalizeDraft(res.data)
      const index = drafts.value.findIndex((draft) => draft.id === updated.id || draft.id === id)
      if (index === -1) drafts.value.unshift(updated)
      else drafts.value[index] = updated
      return updated
    } finally {
      saving.value = false
    }
  }

  return { drafts, saving, load, save, remove, update, getById }
}
