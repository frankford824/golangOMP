/**
 * KPI 模块领域类型（P2 预留）
 *
 * 说明：当前仅定义数据契约接口，等待后端接口确认后填充实现。
 */

export interface DesignerKpiItem {
  designerId: string
  designerName: string
  groupId: string
  groupName: string
  period: string
  tasksAssigned: number
  tasksFinalized: number
  tasksRejected: number
  avgTurnaroundHours: number
  onTimeRate: number
}

export interface KpiFilter {
  designerId?: string
  groupId?: string
  period?: string
  page?: number
  pageSize?: number
}

export interface KpiResult {
  items: DesignerKpiItem[]
  total: number
  page: number
  pageSize: number
}
