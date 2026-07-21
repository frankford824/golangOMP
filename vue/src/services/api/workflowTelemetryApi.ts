import http from '@/services/http'

/**
 * 仅记录当前 main-ops 页面访问和用户操作，用于运行质量诊断。
 * 本模块只负责记录当前页面交互事件，不提供查询或管理页面。
 */
export const workflowTelemetryApi = {
  recordEvent: (
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
}
