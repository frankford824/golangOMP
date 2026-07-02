import http from '@/services/http'

export interface ExperienceRuntimeFlags {
  ui_enabled: boolean
  capture_enabled: boolean
  ai_feedback_enabled: boolean
  worker_enabled: boolean
  behavior_capture_enabled: boolean
  micro_question_enabled: boolean
  review_materialization_enabled: boolean
  behavior_sample_rate: number
  runtime_config_loaded?: boolean
  runtime_config_error?: string
}

export interface ExperienceClientConfig {
  ai_feedback_enabled: boolean
  behavior_capture_enabled: boolean
  micro_question_enabled: boolean
  behavior_sample_rate: number
  enabled_surfaces: string[]
}

export type ExperienceEvidenceLevel = 'L0' | 'L1' | 'L2' | 'L3' | 'L4'

export type AISuggestionFeedbackValue = 'accepted' | 'partially_accepted' | 'rejected'

export interface ExperienceStats {
  flags: ExperienceRuntimeFlags
  total_events: number
  sample_total?: number
  displayed_events?: number
  locatable_samples?: number
  locatable_displayed_events?: number
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
  attribution_total?: number
  attribution_positive?: number
  attribution_weak?: number
  attribution_rejected?: number
  review_items_open?: number
  review_items_approved?: number
  review_items_rejected?: number
  review_items_needs_more_data?: number
  micro_question_answers?: number
  micro_question_answered?: number
  micro_question_dismissed?: number
  micro_question_rate_limited?: number
  task_profiles: number
  asset_quality_labels: number
  worker_last_runs?: ExperienceWorkerRunRecord[]
  latest_profile_rebuilt_at?: string
  generated_at: string
}

export interface ExperienceWorkerRunRecord {
  id?: number
  worker_name: string
  source_name?: string
  started_at: string
  finished_at?: string | null
  status: 'success' | 'partial' | 'failed' | 'locked' | string
  scanned_count: number
  enqueued_count: number
  skipped_count: number
  failed_count: number
  last_error?: string
  metadata?: Record<string, unknown> | unknown
  created_at?: string
}

export interface ExperienceEvent {
  id: number
  event_key: string
  schema_version: number
  event_time: string
  source_type: string
  source_id?: string
  task_id?: number
  target_type?: string
  target_id?: string
  source_watermark?: string
  observed_from?: string
  observed_id?: string
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
  id?: number
  scene: string
  code: string
  name: string
  group: string
  severity?: string
  version?: number
  enabled?: boolean
  deleted_at?: string
  sort_order: number
  created_at?: string
  updated_at?: string
}

export type ExperienceMicroQuestionAnswerValue = 'answered' | 'dismissed'
export type ExperienceReviewDecisionValue = 'approve' | 'reject' | 'needs_more_data'
export type ExperienceReviewItemStatus = 'open' | 'approved' | 'rejected' | 'needs_more_data'
export type ExperienceMicroQuestionEligibilityReason =
  | 'disabled'
  | 'surface_disabled'
  | 'missing_suggestion_event'
  | 'suggestion_not_found'
  | 'not_attribution_eligible'
  | 'suggestion_context_mismatch'
  | 'target_mismatch'
  | 'missing_target'
  | 'already_answered'
  | 'no_supported_attribution'
  | 'rate_limited'

export interface ExperienceMicroQuestionEligibility {
  eligible: boolean
  reason?: ExperienceMicroQuestionEligibilityReason
  answer_event_key?: string
  remaining_daily: number
  reason_tags?: ExperienceReasonTag[]
}

export interface ExperienceMicroQuestionEligibilityParams {
  suggestion_event_id?: string
  suggestion_stable_key?: string
  surface?: string
  target_type?: string
  target_id?: string
}

interface ExperienceMicroQuestionAnswerRequestBase {
  answer_event_key?: string
  suggestion_event_id: string
  suggestion_stable_key?: string
  surface: string
  target_type: string
  target_id: string
  payload?: Record<string, unknown>
}

export type ExperienceMicroQuestionAnswerRequest =
  | (ExperienceMicroQuestionAnswerRequestBase & {
      answer_value: 'answered'
      reason_code: string
    })
  | (ExperienceMicroQuestionAnswerRequestBase & {
      answer_value: 'dismissed'
      reason_code?: string
    })

export interface ExperienceMicroQuestionAnswer {
  id?: number
  answer_event_key: string
  suggestion_event_id?: string
  suggestion_stable_key?: string
  actor_id?: number | null
  surface?: string
  target_type?: string
  target_id?: string
  answer_value: ExperienceMicroQuestionAnswerValue
  reason_code?: string
  payload?: Record<string, unknown> | unknown
  created_at?: string
}

export interface ExperienceReviewItem {
  id?: number
  item_key: string
  item_type: 'attribution_candidate' | string
  status: ExperienceReviewItemStatus | string
  priority?: 'high' | 'medium' | 'low' | string
  evidence_summary?: Record<string, unknown> | unknown
  created_at?: string
  updated_at?: string
}

export interface ExperienceReviewItemsParams {
  status?: ExperienceReviewItemStatus | string
  item_type?: string
  page?: number
  page_size?: number
}

export interface ExperienceReviewDecisionRequest {
  decision: ExperienceReviewDecisionValue
  reason_code?: string
  // Approve requires review_confirmation: true in payload.
  payload?: Record<string, unknown>
}

export interface ExperienceReviewDecision {
  id?: number
  review_item_key: string
  decision: ExperienceReviewDecisionValue
  reason_code?: string
  actor_id?: number | null
  payload?: Record<string, unknown> | unknown
  created_at?: string
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

export type ExperienceBehaviorAction =
  | 'impression'
  | 'visible'
  | 'expand'
  | 'click'
  | 'jump'
  | 'dismiss'
  | 'refresh'
  | 'copy'
  | 'related_action_done'
  | 'ignored_after_timeout'

export interface ExperienceBehaviorEventRequest {
  client_event_id: string
  page_instance_id?: string
  surface?: string
  action: ExperienceBehaviorAction
  target_type?: string
  target_id?: string
  task_id?: number
  suggestion_event_id?: string
  suggestion_stable_key?: string
  occurred_at?: string
  route_name?: string
  component?: string
  dwell_ms?: number
  payload?: Record<string, unknown>
}

export interface ExperienceBehaviorBatchRequest {
  events: ExperienceBehaviorEventRequest[]
}

export interface ExperienceBehaviorBatchResult {
  received: number
  inserted: number
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

  /** GET /v1/experience/client-config */
  clientConfig: (signal?: AbortSignal) =>
    http.get<{ data?: ExperienceClientConfig }>('/v1/experience/client-config', { signal }),

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

  /** POST /v1/experience/behavior-events:batch */
  behaviorEvents: (payload: ExperienceBehaviorBatchRequest, signal?: AbortSignal) =>
    http.post<{ data?: ExperienceBehaviorBatchResult }>('/v1/experience/behavior-events:batch', payload, { signal }),

  /** GET /v1/experience/micro-question-eligibility */
  microQuestionEligibility: (params?: ExperienceMicroQuestionEligibilityParams, signal?: AbortSignal) =>
    http.get<{ data?: ExperienceMicroQuestionEligibility }>('/v1/experience/micro-question-eligibility', {
      params,
      signal,
    }),

  /** POST /v1/experience/micro-question-answers */
  microQuestionAnswer: (payload: ExperienceMicroQuestionAnswerRequest, signal?: AbortSignal) =>
    http.post<{ data?: ExperienceMicroQuestionAnswer }>('/v1/experience/micro-question-answers', payload, { signal }),

  /** GET /v1/reports/experience/review-items */
  reviewItems: (params?: ExperienceReviewItemsParams, signal?: AbortSignal) =>
    http.get<PaginatedEnvelope<ExperienceReviewItem>>('/v1/reports/experience/review-items', { params, signal }),

  /** POST /v1/reports/experience/review-items/{item_key}/decision */
  reviewDecision: (itemKey: string, payload: ExperienceReviewDecisionRequest, signal?: AbortSignal) =>
    http.post<{ data?: ExperienceReviewDecision }>(
      `/v1/reports/experience/review-items/${encodeURIComponent(itemKey)}/decision`,
      payload,
      { signal },
    ),
}
