/**
 * 财务模块领域类型（P2 预留）
 *
 * 说明：当前仅定义数据契约接口，等待后端接口确认后填充实现。
 * 所有金额单位统一为人民币分（整数），前端负责显示层除以 100。
 */

export interface CostSummaryItem {
  taskId: string
  taskNo: string
  sku: string | null
  productName: string
  businessType: string
  groupId: string
  groupName: string
  costPrice: {
    amount: number
    currency: string
  } | null
  closedAt: string | null
}

export interface CostSummaryFilter {
  groupId?: string
  dateStart?: string
  dateEnd?: string
  businessType?: string
  page?: number
  pageSize?: number
}

export interface CostSummaryResult {
  items: CostSummaryItem[]
  total: number
  page: number
  pageSize: number
  totalCostAmount: number
  currency: string
}

export interface FinanceExportParams {
  filter: CostSummaryFilter
  fields: string[]
  format: 'xlsx' | 'csv'
}
