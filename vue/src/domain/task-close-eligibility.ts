import axios from 'axios'
import type { Task } from '@/domain/types/task'
import { checkTaskCompletion } from '@/domain/task-completion'
import { workflowGateReasonLabelCn } from '@/domain/mappers/read-model-labels-cn'

/** 任务已处于完成/关闭等终态，不应再展示「去结单」或主档补录为「待完成」 */
export function isTaskCloseFlowTerminal(task: Task): boolean {
  if (task.status === 'Archived' || task.status === 'Cancelled') return true
  if (task.status === 'Completed') return true
  if (task.mainStatus === 'CLOSED') return true
  if (task.workflowMainStatus?.toLowerCase() === 'closed') return true
  return false
}

/**
 * 是否允许发起结单（archive / closeTask）。
 * 若任务带 workflow.can_close（GET 详情/列表归一化），与后端结单门禁一致，优先于纯前端启发式。
 */
export function canCloseTaskForArchive(task: Task): { allowed: boolean; reasons: string[] } {
  if (isTaskCloseFlowTerminal(task)) {
    return { allowed: false, reasons: [] }
  }
  const local = checkTaskCompletion(task)
  if (task.workflowCanClose !== undefined) {
    if (!task.workflowCanClose) {
      const reasons =
        task.cannotCloseReasons
          ?.map((r) => workflowGateReasonLabelCn(r.code, r.message).trim())
          .filter((s) => s.length > 0) ?? []
      if (reasons.length === 0 && task.missing_fields_summary_cn?.trim()) {
        reasons.push(task.missing_fields_summary_cn.trim())
      }
      if (reasons.length === 0) {
        reasons.push('当前不满足服务端结单条件，请补全主档信息后重试')
      }
      return { allowed: false, reasons }
    }
    return { allowed: true, reasons: [] }
  }
  return { allowed: local.canComplete, reasons: local.reasons }
}

/** 解析 POST close 等返回的 409，展示 cannot_close_reasons / missing_fields_summary_cn */
export function formatCloseArchiveError(e: unknown): string {
  if (axios.isAxiosError(e)) {
    const data = e.response?.data as
      | {
          error?: {
            message?: string
            details?: {
              cannot_close_reasons?: Array<{ code?: string; message?: string }>
              missing_fields_summary_cn?: string
            }
          }
        }
      | undefined
    const details = data?.error?.details
    const reasons = details?.cannot_close_reasons
    if (Array.isArray(reasons) && reasons.length > 0) {
      const lines = reasons
        .map((r) =>
          workflowGateReasonLabelCn(
            typeof r?.code === 'string' ? r.code : '',
            typeof r?.message === 'string' ? r.message : null,
          ),
        )
        .filter((x) => x.trim() !== '')
      if (lines.length > 0) return lines.join('；')
    }
    if (typeof details?.missing_fields_summary_cn === 'string' && details.missing_fields_summary_cn.trim()) {
      return details.missing_fields_summary_cn.trim()
    }
    const msg = data?.error?.message
    if (typeof msg === 'string' && msg.trim()) return msg
    if (e.response?.status === 409) return '当前不可结单（409），请查看结单条件或补全主档信息'
  }
  return e instanceof Error ? e.message : '结单失败'
}
