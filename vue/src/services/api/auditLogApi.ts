/**
 * 审计日志 API
 * 假设后端提供：GET /v1/audit-logs 或 GET /v1/audits/logs
 * 若后端未实现，store 可 fallback 到 mock
 */
import http from '@/services/http'
import type { AuditRecord } from '@/domain/types/audit'

export interface AuditLogListParams {
  taskNo?: string
  auditor?: string
  action?: string
  start?: string
  end?: string
  page?: number
  pageSize?: number
}

export interface AuditLogListResponse {
  data?: AuditRecord[]
  items?: AuditRecord[]
}

export const auditLogApi = {
  list: (params?: AuditLogListParams, signal?: AbortSignal) =>
    http.get<AuditLogListResponse>('/v1/audit-logs', { params, signal }),
}
