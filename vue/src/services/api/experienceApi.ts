import http from '@/services/http'

export interface ExperienceRuntimeFlags {
  ui_enabled: boolean
  capture_enabled: boolean
  ai_feedback_enabled: boolean
  worker_enabled: boolean
}

export interface ExperienceStats {
  flags: ExperienceRuntimeFlags
  total_events: number
  outbox_queued: number
  outbox_processing: number
  outbox_processed_24h: number
  outbox_failed_24h: number
  outbox_dead_letter: number
  capture_success_rate_24h: number
  capture_failure_rate_24h: number
  tag_total: number
  tag_enabled: number
  tag_coverage_rate: number
  ai_suggestion_events: number
  ai_feedback_events: number
  ai_feedback_rate: number
  task_profiles: number
  asset_quality_labels: number
  latest_profile_rebuilt_at?: string
  generated_at: string
}

export interface ExperienceEvent {
  id: number
  event_key: string
  schema_version: number
  event_time: string
  source_type: string
  source_id?: string
  task_id?: number
  action: string
  outcome?: string
  actor_snapshot?: Record<string, unknown> | unknown
  business_snapshot?: Record<string, unknown> | unknown
  payload?: Record<string, unknown> | unknown
  data_classification?: string
  ground_truth_status?: string
  created_at: string
}

export interface ExperienceReasonTag {
  id: number
  scene: string
  code: string
  name: string
  group: string
  severity?: string
  version: number
  enabled: boolean
  deleted_at?: string
  sort_order: number
  created_at: string
  updated_at: string
}

export interface ExperienceSamplesParams {
  page?: number
  page_size?: number
  source_type?: string
  source_id?: string
  task_id?: number
  action?: string
  outcome?: string
  from?: string
  to?: string
}

export interface PaginationMeta {
  page?: number
  page_size?: number
  total?: number
}

export interface PaginatedEnvelope<T> {
  data?: T[]
  pagination?: PaginationMeta
}

export const experienceApi = {
  /** GET /v1/experience/config */
  config: (signal?: AbortSignal) =>
    http.get<{ data?: ExperienceRuntimeFlags }>('/v1/experience/config', { signal }),

  /** GET /v1/experience/reason-tags */
  reasonTags: (params?: { scene?: string }, signal?: AbortSignal) =>
    http.get<{ data?: ExperienceReasonTag[] }>('/v1/experience/reason-tags', { params, signal }),

  /** GET /v1/reports/experience/stats */
  stats: (signal?: AbortSignal) =>
    http.get<{ data?: ExperienceStats }>('/v1/reports/experience/stats', { signal }),

  /** GET /v1/reports/experience/samples */
  samples: (params?: ExperienceSamplesParams, signal?: AbortSignal) =>
    http.get<PaginatedEnvelope<ExperienceEvent>>('/v1/reports/experience/samples', { params, signal }),
}
