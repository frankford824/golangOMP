const titles: Record<string, string> = {
  '/': '任务看板', '/design-dashboard': '设计部门看板', '/403': '访问受限',
  '/operations': '经营总览', '/operations/products': '商品表现', '/operations/pricing': 'SKU 核价', '/operations/rankings': '经营排行',
  '/operations/orders/detail': '订单明细', '/operations/orders/refunds': '售后退款', '/operations/orders/ledger': '经营台账',
  '/me/performance': '我的业绩', '/tasks': '任务中心', '/tasks/create': '创建任务单',
  '/asset-center': '资产管理', '/cost-rules': '成本规则', '/data-center': 'AI 数据助手',
  '/users': '人员管理', '/users/roster': '人员花名册', '/me': '个人中心', '/me/org': '我的组织',
  '/me/security': '安全设置', '/me/notifications': '通知中心', '/me/task-drafts': '任务草稿',
  '/fees': '费用中心', '/fees/apply': '发起费用申请', '/fees/mine': '我的申请', '/fees/approval': '审批待办',
  '/fees/report': '费用报表', '/fees/templates': '模板与授权', '/fees/document': '费用单据',
}
export function oaPageTitle(path: string) {
  if (titles[path]) return titles[path]
  if (path.startsWith('/asset-center/')) return '资产详情'
  if (path.startsWith('/operations/products/')) return '商品详情'
  if (path.startsWith('/tasks/')) return path.endsWith('/assets') ? '任务资产' : '任务详情'
  return '永箔运营'
}
