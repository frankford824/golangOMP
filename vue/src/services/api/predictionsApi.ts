import http from '@/services/http'
import type { V1GlobalSearchScope } from '@/domain/global-search'

export type PredictionType =
  | 'recent'
  | 'search'
  | 'product'
  | 'asset'
  | 'task_create'
  | 'task_next_action'
  | 'management'
  | string

export interface PredictionSuggestion {
  id: string
  type: PredictionType
  title: string
  detail?: string
  action_label?: string
  action_type?: string
  target_type?: 'task' | 'asset' | 'product' | 'task_center' | 'data_center' | 'logs' | string
  target_id?: string
  confidence?: 'high' | 'medium' | 'low' | string
  source?: string
  metadata?: Record<string, string>
}

export interface PredictionBundle {
  suggestions: PredictionSuggestion[]
  generated_at?: string
}

function unwrapPredictionBundle(payload: unknown): PredictionBundle {
  const root = payload && typeof payload === 'object' ? (payload as Record<string, unknown>) : {}
  const data = root.data && typeof root.data === 'object' ? (root.data as Record<string, unknown>) : root
  const suggestions = Array.isArray(data.suggestions)
    ? (data.suggestions as PredictionSuggestion[])
    : []
  return {
    suggestions,
    generated_at: typeof data.generated_at === 'string' ? data.generated_at : undefined,
  }
}

export const predictionsApi = {
  search: async (
    params: { keyword?: string; scope?: V1GlobalSearchScope; limit?: number },
    signal?: AbortSignal,
  ): Promise<PredictionBundle> => {
    const res = await http.get('/v1/predictions/search', {
      params: {
        q: params.keyword ?? '',
        scope: params.scope ?? 'all',
        ...(params.limit != null ? { limit: params.limit } : {}),
      },
      signal,
    })
    return unwrapPredictionBundle(res.data)
  },

  taskCreate: async (
    params: { keyword?: string; taskType?: string; limit?: number },
    signal?: AbortSignal,
  ): Promise<PredictionBundle> => {
    const res = await http.get('/v1/predictions/task-create', {
      params: {
        keyword: params.keyword ?? '',
        task_type: params.taskType ?? '',
        ...(params.limit != null ? { limit: params.limit } : {}),
      },
      signal,
    })
    return unwrapPredictionBundle(res.data)
  },

  taskNextActions: async (taskId: string | number, params?: { limit?: number }, signal?: AbortSignal) => {
    const res = await http.get(`/v1/tasks/${encodeURIComponent(String(taskId))}/predictions`, {
      params: params?.limit != null ? { limit: params.limit } : undefined,
      signal,
    })
    return unwrapPredictionBundle(res.data)
  },

  assets: async (params: { keyword?: string; limit?: number }, signal?: AbortSignal) => {
    const res = await http.get('/v1/predictions/assets', {
      params: {
        q: params.keyword ?? '',
        ...(params.limit != null ? { limit: params.limit } : {}),
      },
      signal,
    })
    return unwrapPredictionBundle(res.data)
  },

  management: async (params: { from?: string; to?: string; limit?: number }, signal?: AbortSignal) => {
    const res = await http.get('/v1/predictions/management', {
      params: {
        ...(params.from ? { from: params.from } : {}),
        ...(params.to ? { to: params.to } : {}),
        ...(params.limit != null ? { limit: params.limit } : {}),
      },
      signal,
    })
    return unwrapPredictionBundle(res.data)
  },
}
