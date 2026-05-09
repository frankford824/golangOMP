/**
 * KPI 接口预留（P2 阶段实现）
 * 占位文件：保持模块边界，接入后端时填充具体实现
 */
import type { KpiFilter, KpiResult } from '@/domain/types/kpi'

/** 查询设计师 KPI（占位，P2 阶段实现） */
export async function fetchDesignerKpi(_params: KpiFilter): Promise<KpiResult | null> {
  return Promise.resolve(null)
}
