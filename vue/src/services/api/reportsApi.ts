import http from '@/services/http'

/** Query for L1 throughput and module-dwell (OpenAPI: from/to required) */
export interface L1ReportRangeParams {
  from: string
  to: string
  department_id?: number
  task_type?: string
}

export interface KpiAiAnalysisHighlight {
  title: string
  value?: string
  note?: string
}

export interface KpiAiAnalysisPersonInsight {
  role?: string
  name: string
  metric?: string
  signal: string
  action?: string
}

export interface KpiAiAnalysisTaskSample {
  task_no: string
  task_name?: string
  task_type?: string
  timeline?: string[]
  observation?: string
}

export interface KpiAiAnalysisRisk {
  level?: string
  title: string
  reason?: string
}

export interface KpiAiAnalysisAction {
  owner?: string
  action: string
  timing?: string
}

export interface KpiAiAnalysisResponse {
  headline: string
  overview: string
  highlights?: KpiAiAnalysisHighlight[]
  people_insights?: KpiAiAnalysisPersonInsight[]
  task_samples?: KpiAiAnalysisTaskSample[]
  risks?: KpiAiAnalysisRisk[]
  actions?: KpiAiAnalysisAction[]
  evidence?: string[]
  confidence?: 'high' | 'medium' | 'low' | string
  generated_at?: string
  model?: string
  provider?: string
}

export interface KpiTaskEvent {
  id: string
  task_id: number
  task_no?: string
  sku_code?: string
  product_name?: string
  task_type?: string
  business_lane?: string
  category_name?: string
  task_status?: string
  priority?: string
  deadline_at?: string
  event_type: string
  operator_id?: number | null
  operator_name?: string
  operator_department?: string
  operator_team?: string
  payload?: Record<string, unknown> | unknown
  created_at: string
}

export interface BusinessTrendHotspot {
  topic: string
  count: number
  signal?: string
  keywords?: string[]
  task_samples?: string[]
}

export interface BusinessTrendMatch {
  topic: string
  source: string
  signal?: string
  business_meaning?: string
  evidence?: string[]
}

export interface BusinessTrendDirection {
  title: string
  reason?: string
  suggested_action?: string
  priority?: 'high' | 'medium' | 'low' | string
}

export interface BusinessTrendRisk {
  level?: 'high' | 'medium' | 'low' | string
  title: string
  reason?: string
}

export interface BusinessTrendSourceStatus {
  source: string
  status: 'used' | 'skipped' | 'failed' | string
  message: string
  items?: number
}

export interface BusinessTrendEvidenceSample {
  task_no?: string
  task_name?: string
  source?: string
  note: string
  created_at?: string
}

export interface BusinessTrendPilotResponse {
  headline: string
  overview: string
  internal_hotspots?: BusinessTrendHotspot[]
  external_matches?: BusinessTrendMatch[]
  business_directions?: BusinessTrendDirection[]
  risks?: BusinessTrendRisk[]
  source_statuses?: BusinessTrendSourceStatus[]
  evidence_samples?: BusinessTrendEvidenceSample[]
  confidence?: 'high' | 'medium' | 'low' | string
  generated_at?: string
}

export interface BusinessTrendPilotParams {
  from: string
  to: string
  mode: 'internal' | 'external'
  sources?: string[]
}

export const reportsApi = {
  /** GET /v1/reports/l1/cards */
  l1Cards: (signal?: AbortSignal) => http.get('/v1/reports/l1/cards', { signal }),

  /** GET /v1/reports/l1/throughput */
  l1Throughput: (params: L1ReportRangeParams, signal?: AbortSignal) =>
    http.get('/v1/reports/l1/throughput', { params, signal }),

  /** GET /v1/reports/l1/module-dwell */
  l1ModuleDwell: (params: L1ReportRangeParams, signal?: AbortSignal) =>
    http.get('/v1/reports/l1/module-dwell', { params, signal }),

  /** GET /v1/reports/l1/kpi-events */
  l1KpiEvents: (params: Pick<L1ReportRangeParams, 'from' | 'to'> & { limit?: number }, signal?: AbortSignal) =>
    http.get<{ data?: KpiTaskEvent[] }>('/v1/reports/l1/kpi-events', { params, signal }),

  /** POST /v1/reports/l1/kpi-ai-analysis */
  kpiAiAnalysis: (params: Pick<L1ReportRangeParams, 'from' | 'to'>, signal?: AbortSignal) =>
    http.post<{ data?: KpiAiAnalysisResponse }>('/v1/reports/l1/kpi-ai-analysis', params, {
      signal,
      timeout: 120_000,
    }),

  /** POST /v1/reports/business-trends/pilot-analysis */
  businessTrendPilotAnalysis: (params: BusinessTrendPilotParams, signal?: AbortSignal) =>
    http.post<{ data?: BusinessTrendPilotResponse }>('/v1/reports/business-trends/pilot-analysis', params, {
      signal,
      timeout: 120_000,
    }),
}
