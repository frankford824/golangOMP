/**
 * 日志查询 API
 * 对应文档：v0.4 API 使用说明.md § 6. 日志接口
 * v0.5 对齐：FRONTEND_ALIGNMENT_v0.5.md 第 H 节 server-logs
 */

import http from '@/services/http'

export const logsApi = {
  /**
   * 查询权限变更日志
   * GET /v1/permission-logs
   * 权限：超级管理员
   */
  permissionLogs: (
    params?: { page?: number; page_size?: number; user_id?: string },
    signal?: AbortSignal,
  ) => http.get('/v1/permission-logs', { params, signal }),

  /**
   * 查询操作日志（聚合时间线）
   * GET /v1/operation-logs
   * 权限：超级管理员或人事（HR）；后端文案 "admin or HR access"
   */
  operationLogs: (
    params?: {
      source?: 'task_event' | 'export_event' | 'integration_call'
      event_type?: string
      page?: number
      page_size?: number
    },
    signal?: AbortSignal,
  ) => http.get('/v1/operation-logs', { params, signal }),

  /**
   * 查询全链路事件
   * GET /v1/trace-events
   * 权限：超级管理员或人事（HR）
   */
  traceEvents: (
    params?: {
      trace_id?: string
      event_source?: string
      event_type?: string
      action?: string
      actor_id?: number
      actor_username?: string
      actor_source?: string
      actor_department?: string
      actor_team?: string
      route_path?: string
      task_id?: number
      module_key?: string
      sku_code?: string
      asset_id?: number
      design_asset_id?: number
      task_asset_id?: number
      integration_call_log_id?: number
      resource_type?: string
      resource_id?: string
      outcome?: string
      business_only?: boolean
      from?: string
      to?: string
      page?: number
      page_size?: number
    },
    signal?: AbortSignal,
  ) => http.get('/v1/trace-events', { params, signal }),

  /**
   * 记录前端业务事件
   * POST /v1/trace-events
   */
  recordTraceEvent: (
    payload: {
      event_type: 'page_view' | 'user_action' | string
      action?: string
      page_url?: string
      page_name?: string
      component_id?: string
      task_id?: number
      task_module_id?: number
      module_key?: string
      sku_code?: string
      task_sku_item_id?: number
      asset_id?: number
      design_asset_id?: number
      task_asset_id?: number
      integration_call_log_id?: number
      resource_type?: string
      resource_id?: string
      outcome?: string
      payload?: Record<string, unknown>
      occurred_at?: string
    },
    signal?: AbortSignal,
  ) => http.post('/v1/trace-events', payload, { signal }),

  /**
   * 查询服务器日志
   * v0.5 对齐：FRONTEND_ALIGNMENT_v0.5.md 第 H 节
   * GET /v1/server-logs
   * 权限：Admin
   */
  serverLogs: (
    params?: {
      level?: 'info' | 'warn' | 'error'
      keyword?: string
      since?: string
      until?: string
      page?: number
      page_size?: number
    },
    signal?: AbortSignal,
  ) => http.get('/v1/server-logs', { params, signal }),

  /**
   * 清理旧服务器日志
   * v0.5 对齐：FRONTEND_ALIGNMENT_v0.5.md 第 H 节
   * POST /v1/server-logs/clean
   * body 必含 reason，可选 older_than_hours（默认 24）
   * 权限：Admin
   */
  serverLogsClean: (
    payload: { reason: string; older_than_hours?: number },
    signal?: AbortSignal,
  ) => http.post('/v1/server-logs/clean', payload, { signal }),
}
