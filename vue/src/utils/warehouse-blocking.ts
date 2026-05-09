/**
 * 后端 workflow.warehouse_blocking_reasons[].code → 简短中文说明（无则回退 message）。
 * 与 cannot_close 等门禁共用读模型映射，见 read-model-labels-cn。
 */
import { workflowGateReasonLabelCn } from '@/domain/mappers/read-model-labels-cn'

export function warehouseBlockingReasonLine(code: string, message: string): string {
  return workflowGateReasonLabelCn(code, message)
}
