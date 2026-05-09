export type AuditAction = 'sign' | 'pass' | 'reject' | 'transfer' | 'handover' | 'takeover'

export const AUDIT_ACTION_LABELS: Record<AuditAction, string> = {
  sign: '签收',
  pass: '通过',
  reject: '打回',
  transfer: '转交',
  handover: '交班',
  takeover: '接手',
}

export function getAuditActionLabel(action: AuditAction): string {
  return AUDIT_ACTION_LABELS[action] ?? action
}
