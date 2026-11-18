import type { RouteRecordRaw } from 'vue-router'
import { oaEnabled } from './access'

const Commerce = () => import('./pages/Commerce.vue')
const People = () => import('./pages/People.vue')
const Fees = () => import('./pages/Fees.vue')
const OrderData = () => import('./pages/OrderData.vue')
const ordersMeta = { requiresAuth: true, oaAction: 'orders.view' }
const peopleMeta = { requiresAuth: true, oaAction: 'people.manage', oaFallback: 'UserManagement' }
const feeActions = { apply: 'finance.apply', mine: 'finance.apply', approval: ['finance.approve', 'finance.pay'], templates: ['finance.templates', 'finance.admin'], report: 'finance.report' }

export const oaRoutes: RouteRecordRaw[] = oaEnabled ? [
  { path: 'operations', name: 'OperationsOverview', component: Commerce, props: { mode: 'overview' }, meta: ordersMeta },
  { path: 'operations/products', component: Commerce, props: { mode: 'products' }, meta: ordersMeta },
  { path: 'operations/products/:id', component: Commerce, props: route => ({ mode: 'detail', productId: route.params.id }), meta: ordersMeta },
  { path: 'operations/rankings', component: Commerce, props: { mode: 'ranking' }, meta: ordersMeta },
  { path: 'operations/orders/:mode(detail|refunds|ledger)', component: OrderData, props: true, meta: ordersMeta },
  { path: 'me/performance', component: Commerce, props: { mode: 'personal' }, meta: ordersMeta },
  { path: 'users/roster', name: 'PeopleRoster', component: People, props: { mode: 'roster' }, meta: peopleMeta },
  { path: 'people', redirect: to => ({ name: 'UserManagement', query: to.query, hash: to.hash }) },
  { path: 'people/roster', redirect: to => ({ name: 'PeopleRoster', query: to.query, hash: to.hash }) },
  { path: 'people/:id(\\d+)', redirect: to => ({ name: 'UserManagement', query: { ...to.query, user_id: String(to.params.id), tab: 'terms' }, hash: to.hash }) },
  ...Object.entries(feeActions).map(([mode, oaAction]) => ({
    path: 'fees/' + mode, component: Fees, props: { mode }, meta: { requiresAuth: true, oaAction, oaFallback: 'Me' },
  })),
  { path: 'fees/document', component: Fees, props: { mode: 'document' }, meta: { requiresAuth: true, oaAction: ['finance.apply', 'finance.approve', 'finance.pay', 'finance.report'], oaFallback: 'MeNotifications' } },
] : []
