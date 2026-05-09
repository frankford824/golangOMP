import http from '@/services/http'

/** Query for L1 throughput and module-dwell (OpenAPI: from/to required) */
export interface L1ReportRangeParams {
  from: string
  to: string
  department_id?: number
  task_type?: string
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
}
