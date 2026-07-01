import http from '@/services/http'

export interface ExperienceRuntimeFlags {
  ui_enabled: boolean
  capture_enabled: boolean
  ai_feedback_enabled: boolean
  worker_enabled: boolean
}

export type ExperienceEvidenceLevel = 'L0' | 'L1' | 'L2' | 'L3' | 'L4'

export type AISuggestionFeedbackValue = 'accepted' | 'partially_accepted' | 'rejected'

export interface ExperienceStats {
  flags: ExperienceRuntimeFlags
  total_events: number
  sample_total?: number
  displayed_events?: number
  locatable_samples?: number
  feedback_samples?: number
  reasoned_feedback_samples?: number
  reusable_samples?: number
  feedback_accepted?: number
  feedback_partially_accepted?: number
  feedback_rejected?: number
  reason_coverage_rate?: number
  reusable_rate?: number
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
  evidence_level?: ExperienceEvidenceLevel
  feedback_value?: AISuggestionFeedbackValue
  feedback_reason_code?: string
  feedback_created_at?: string
  missing_signals?: string[]
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
  min_evidence_level?: ExperienceEvidenceLevel
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

export interface AISuggestionFeedbackPayload {
  surface: string
  target_type?: string
  target_id?: string
  suggestion_id?: string
  suggestion_type?: string
  source?: string
  action_type?: string
  action_label?: string
  route?: string
}

export interface AISuggestionFeedbackRequest {
  suggestion_event_id?: string
  feedback_value: AISuggestionFeedbackValue
  reason_code?: string
  reason_note?: string
  outcome_source_type?: string
  outcome_source_id?: string
  payload?: AISuggestionFeedbackPayload
}

export interface AISuggestionFeedback {
  id: number
  suggestion_event_id: string
  feedback_value: AISuggestionFeedbackValue
  reason_code?: string
  reason_note?: string
  outcome_source_type?: string
  outcome_source_id?: string
  actor_id?: number | null
  payload?: AISuggestionFeedbackPayload
  created_at: string
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

  /** POST /v1/ai-suggestions/{suggestion_event_id}/feedback */
  feedback: (suggestionEventId: string, payload: AISuggestionFeedbackRequest, signal?: AbortSignal) =>
    http.post<{ data?: AISuggestionFeedback }>(
      `/v1/ai-suggestions/${encodeURIComponent(suggestionEventId)}/feedback`,
      payload,
      { signal },
    ),
}
