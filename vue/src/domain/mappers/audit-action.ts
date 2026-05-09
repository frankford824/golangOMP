import type { AuditRecord } from '@/domain/types/audit'

/** 审计动作 DTO -> UI 文案映射，禁止在组件内硬编码 */
export const AUDIT_ACTION_LABELS: Record<AuditRecord['action'], string> = {
  sign: '提交审核',
  pass: '通过',
  reject: '打回',
  transfer: '转交',
  handover: '交班',
  takeover: '接手',
  complete: '结单',
  archive: '归档',
  return: '退回',
  warehouse_receive: '仓库接收',
}

export function getAuditActionLabel(action: AuditRecord['action']): string {
  return AUDIT_ACTION_LABELS[action] ?? action
}
