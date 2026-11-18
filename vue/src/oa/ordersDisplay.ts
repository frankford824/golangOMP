const orderStates = {
  sent: { label: '已发货', tone: 'success' },
  split: { label: '已拆单', tone: 'info' },
  merged: { label: '已合单', tone: 'info' },
  cancelled: { label: '已取消', tone: 'neutral' },
  question: { label: '问题单', tone: 'warning' },
  waitpay: { label: '待付款', tone: 'warning' },
  delivering: { label: '发货中', tone: 'info' },
  waitconfirm: { label: '待确认', tone: 'info' },
  paid: { label: '已确认', tone: 'success' },
  confirmed: { label: '已确认', tone: 'success' },
  refunded: { label: '已退款', tone: 'neutral' },
}

const ledgerStates = {
  open: { label: '核算中', tone: 'warning' },
  closed: { label: '已封账', tone: 'success' },
}

export function orderState(column: string, value: unknown) {
  if (typeof value !== 'string' || !value.trim()) return undefined
  const states: Record<string, { label: string; tone: string }> = column === 'state' ? ledgerStates : orderStates
  const code = value.trim().toLowerCase()
  return Object.prototype.hasOwnProperty.call(states, code)
    ? states[code]
    : { label: /[一-鿿]/.test(value) ? value.trim() : '其他状态', tone: 'neutral' }
}
