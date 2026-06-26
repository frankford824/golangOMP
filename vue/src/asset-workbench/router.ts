import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

import { getToken } from '@/services/http'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'asset-login',
    component: () => import('./pages/AuthPage.vue'),
    meta: { label: '登录', public: true },
  },
  {
    path: '/register',
    name: 'asset-register',
    component: () => import('./pages/AuthPage.vue'),
    meta: { label: '注册', public: true },
  },
  {
    path: '/',
    name: 'asset-dashboard',
    component: () => import('./pages/DashboardPage.vue'),
    meta: { label: '总览' },
  },
  {
    path: '/upload',
    name: 'asset-upload',
    component: () => import('./pages/UploadPage.vue'),
    meta: { label: '上传提交' },
  },
  {
    path: '/submissions',
    name: 'asset-submissions',
    component: () => import('./pages/SubmissionsPage.vue'),
    meta: { label: '维护专区' },
  },
  {
    path: '/materials',
    name: 'asset-materials',
    component: () => import('./pages/MaterialsPage.vue'),
    meta: { label: '素材库' },
  },
  {
    path: '/cost-center',
    name: 'asset-cost-center',
    component: () => import('./pages/CostCenterPage.vue'),
    meta: { label: '成本中心' },
  },
  {
    path: '/settlement',
    name: 'asset-settlement',
    component: () => import('./pages/SettlementPage.vue'),
    meta: { label: '结算' },
  },
  {
    path: '/people',
    name: 'asset-people',
    component: () => import('./pages/PeoplePage.vue'),
    meta: { label: '人员' },
  },
  {
    path: '/events',
    name: 'asset-events',
    component: () => import('./pages/EventsPage.vue'),
    meta: { label: '日志' },
  },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})

function normalizeAssetRedirectTarget(raw: unknown): string {
  if (typeof raw !== 'string' || !raw.startsWith('/')) return '/'
  if (raw === '/asset.html' || raw.startsWith('/asset.html?')) return '/'
  return raw
}

router.beforeEach((to) => {
  if (to.path === '/asset.html') {
    return { path: '/', query: to.query }
  }
  const hasToken = Boolean(getToken())
  if (!to.meta.public && !hasToken) {
    return { path: '/login', query: { redirect: normalizeAssetRedirectTarget(to.fullPath) } }
  }
  if (to.meta.public && hasToken) {
    return normalizeAssetRedirectTarget(to.query.redirect)
  }
  return true
})
