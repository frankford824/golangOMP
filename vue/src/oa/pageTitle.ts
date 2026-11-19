const titles: Record<string, string> = {
  '/': '任务看板', '/design-dashboard': '设计部门看板', '/403': '访问受限',
  '/operations': '经营总览', '/operations/products': '商品表现', '/operations/pricing': 'SKU 核价', '/operations/rankings': '经营排行',
  '/operations/orders/detail': '订单明细', '/operations/orders/refunds': '售后退款', '/operations/orders/ledger': '经营台账',
  '/me/performance': '我的业绩', '/tasks': '任务中心', '/tasks/create': '创建任务单',
  '/asset-center': '资产管理', '/cost-rules': '成本规则',
  '/users': '人员管理', '/users/shop-bindings': '店铺人员绑定', '/users/roster': '人员花名册', '/me': '个人中心', '/me/org': '我的组织',
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

export function oaReturnLabel(path: string) {
  const url = new URL(path, 'https://oa.invalid')
  if (url.pathname === '/tasks' && url.searchParams.get('tab') === 'drafts') return '返回草稿箱'
  if (url.pathname === '/users' && url.searchParams.has('user_id')) return '返回人员详情'
  if (url.pathname === '/asset-center') return url.searchParams.get('view') === 'ledger' ? '返回规格台账' : '返回资产中心'
  if (url.pathname === '/me/notifications') {
    if (url.searchParams.get('section') === 'cost') return '返回成本复核'
    if (url.searchParams.get('section') === 'expense') return '返回费用动态'
  }
  return '返回' + oaPageTitle(url.pathname)
}
