import type { RouteRecordRaw } from 'vue-router'
import { oaEnabled } from './access'

const Commerce = () => import('./pages/Commerce.vue')
const ShopBindings = () => import('./pages/ShopBindings.vue')
const People = () => import('./pages/People.vue')
const Fees = () => import('./pages/Fees.vue')
const Pricing = () => import('./pages/Pricing.vue')
const OrderData = () => import('./pages/OrderData.vue')
const ordersMeta = { requiresAuth: true, oaAction: 'orders.view' }
const peopleMeta = { requiresAuth: true, oaAction: 'people.manage', oaFallback: 'UserManagement' }
export const feeTabs = [
  { mode: 'approval', label: '审批待办', actions: ['finance.approve', 'finance.pay'] },
  { mode: 'mine', label: '我的申请', actions: ['finance.apply'] },
  { mode: 'templates', label: '模板与授权', actions: ['finance.templates', 'finance.admin'] },
  { mode: 'report', label: '费用报表', actions: ['finance.report'] },
] as const

export const oaRoutes: RouteRecordRaw[] = oaEnabled ? [
  { path: 'operations', name: 'OperationsOverview', component: Commerce, props: { mode: 'overview' }, meta: ordersMeta },
  { path: 'operations/products', component: Commerce, props: { mode: 'products' }, meta: ordersMeta },
  { path: 'operations/products/:id', component: Commerce, props: route => ({ mode: 'detail', productId: route.params.id }), meta: ordersMeta },
  { path: 'operations/pricing', name: 'OperationsPricing', component: Pricing, meta: { requiresAuth: true, oaAction: 'pricing.view', title: 'SKU 核价' } },
  { path: 'operations/rankings', component: Commerce, props: { mode: 'ranking' }, meta: ordersMeta },
  { path: 'operations/orders/:mode(detail|refunds|ledger)', component: OrderData, props: true, meta: ordersMeta },
  { path: 'me/performance', component: Commerce, props: { mode: 'personal' }, meta: ordersMeta },
  { path: 'users/shop-bindings', name: 'PeopleShopBindings', component: ShopBindings, meta: peopleMeta },
  { path: 'users/roster', name: 'PeopleRoster', component: People, props: { mode: 'roster' }, meta: peopleMeta },
  { path: 'people', redirect: to => ({ name: 'UserManagement', query: to.query, hash: to.hash }) },
  { path: 'people/roster', redirect: to => ({ name: 'PeopleRoster', query: to.query, hash: to.hash }) },
  { path: 'people/:id(\\d+)', redirect: to => ({ name: 'UserManagement', query: { ...to.query, user_id: String(to.params.id), tab: 'terms' }, hash: to.hash }) },
  { path: 'fees', name: 'FeesHome', component: Fees, props: { mode: 'home' }, meta: { requiresAuth: true } },
  { path: 'fees/apply', component: Fees, props: { mode: 'apply' }, meta: { requiresAuth: true, title: '发起费用申请' } },
  ...feeTabs.map(({ mode, label }) => ({
    path: 'fees/' + mode, component: Fees, props: { mode }, meta: { requiresAuth: true, title: label },
  })),
  { path: 'fees/document', component: Fees, props: { mode: 'document' }, meta: { requiresAuth: true, title: '费用单据' } },
] : []
