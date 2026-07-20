const VALUE_LABELS: Record<string, Record<string, string>> = {
  priority: {
    low: '低优先级',
    normal: '普通优先级',
    medium: '普通优先级',
    high: '高优先级',
    critical: '紧急处理',
    urgent: '紧急处理',
  },
  filing_status: {
    pending: '等待 ERP 建档',
    pending_filing: '等待 ERP 建档',
    filing: '正在 ERP 建档',
    filed: '已完成 ERP 建档',
    filing_failed: 'ERP 建档失败',
    failed: 'ERP 建档失败',
    not_required: '无需 ERP 建档',
  },
  erp_sync_status: {
    pending_sync: '等待同步',
    queued: '已进入同步队列',
    syncing: '正在同步',
    synced: '已同步',
    failed: '同步失败',
    cooling_down: '稍后自动重试',
    waiting_image: '等待成品图',
  },
  cost_price_mode: {
    manual: '人工填写',
    template: '按成本规则计算',
    rule: '按成本规则计算',
    automatic: '系统自动计算',
  },
  product_channel: {
    online: '线上渠道',
    offline: '线下渠道',
    domestic: '国内渠道',
    overseas: '海外渠道',
  },
  source_mode: {
    manual: '运营创建',
    excel: '表格导入',
    api: '系统接口创建',
    erp: 'ERP 同步创建',
  },
  handover_status: {
    pending_takeover: '等待接手',
    taken_over: '已接手',
    cancelled: '已取消',
    expired: '已失效',
  },
}

const GENERIC_LABELS: Record<string, string> = {
  true: '是',
  false: '否',
  normal: '普通',
  filed: '已完成',
  failed: '失败',
  pending: '等待处理',
  completed: '已完成',
  active: '启用',
  inactive: '停用',
}

export function taskDetailDisplayValue(key: string, value: unknown, empty = '未填写'): string {
  if (value == null || value === '') return empty
  if (typeof value === 'boolean') return value ? '是' : '否'
  const normalized = String(value).trim()
  if (!normalized) return empty
  const lookup = VALUE_LABELS[key]?.[normalized.toLowerCase()]
  if (lookup) return lookup
  const generic = GENERIC_LABELS[normalized.toLowerCase()]
  if (generic) return generic
  if (/^[a-z][a-z0-9_.-]*$/i.test(normalized)) return '状态待确认'
  return normalized
}

export function handoverStatusLabel(value: unknown): string {
  return taskDetailDisplayValue('handover_status', value, '等待接手')
}

