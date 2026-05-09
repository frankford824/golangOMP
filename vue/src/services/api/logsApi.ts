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
