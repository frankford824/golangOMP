export interface AuditRecord {
  id: string
  taskId: string
  auditorId: string
  auditorName: string
  action:
    | 'sign'
    | 'pass'
    | 'reject'
    | 'transfer'
    | 'handover'
    | 'takeover'
    | 'complete'
    | 'archive'
    | 'return'
    | 'warehouse_receive'
  comment?: string
  problemCategory?: string
  affectLaunch?: boolean
  needOutsource?: boolean
  createdAt: string
}

export interface AuditHandover {
  id: string
  taskId: string
  fromUserId: string
  fromUserName: string
  toUserId: string
  toUserName: string
  reason: string
  judgment: string
  riskNote?: string
  createdAt: string
}
