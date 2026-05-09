/**
 * 财务接口预留（P2 阶段实现）
 * 占位文件：保持模块边界，接入后端时填充具体实现
 */
import type { CostSummaryFilter, CostSummaryResult, FinanceExportParams } from '@/domain/types/finance'

/** 查询成本汇总（占位，P2 阶段实现） */
export async function fetchCostSummary(_filter: CostSummaryFilter): Promise<CostSummaryResult | null> {
  return Promise.resolve(null)
}

/** 导出财务数据（占位，P2 阶段实现） */
export async function exportFinanceData(_payload: FinanceExportParams): Promise<Blob | null> {
  return Promise.resolve(null)
}
