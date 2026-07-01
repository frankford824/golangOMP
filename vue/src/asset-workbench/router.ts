import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

import { getToken } from '@/services/http'
import {
  canAccessAssetWorkbenchRoute,
  firstAccessibleSettingsPath,
  isConfigOnlyAdmin,
  routeAccessForPath,
} from './app/access'
import { useAssetWorkbenchSessionStore } from './app/session.store'

declare module 'vue-router' {
  interface RouteMeta {
    label?: string
    subtitle?: string
    public?: boolean
    anyAuthenticated?: boolean
    simple?: boolean
    requiresAnyCapability?: readonly string[]
    settings?: boolean
  }
}

function accessMeta(path: string) {
  const access = routeAccessForPath(path)
  return access
    ? {
        label: access.label,
        subtitle: access.subtitle,
        anyAuthenticated: access.anyAuthenticated,
        simple: access.simple,
        requiresAnyCapability: access.requiresAnyCapability,
      }
    : {}
}

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
    component: () => import('./pages/HomePage.vue'),
    meta: accessMeta('/'),
  },
  {
    path: '/upload',
    name: 'asset-upload',
    component: () => import('./pages/UploadPage.vue'),
    meta: accessMeta('/upload'),
  },
  {
    path: '/my-settlement',
    name: 'asset-my-settlement',
    component: () => import('./pages/MySettlementPage.vue'),
    meta: accessMeta('/my-settlement'),
  },
  {
    path: '/account',
    name: 'asset-account',
    component: () => import('./pages/AccountPage.vue'),
    meta: accessMeta('/account'),
  },
  {
    path: '/notifications',
    name: 'asset-notifications',
    component: () => import('./pages/NotificationsPage.vue'),
    meta: accessMeta('/notifications'),
  },
  {
    path: '/submissions',
    name: 'asset-submissions',
    component: () => import('./pages/SubmissionsPage.vue'),
    meta: accessMeta('/submissions'),
  },
  {
    path: '/materials',
    name: 'asset-materials',
    component: () => import('./pages/MaterialsPage.vue'),
    meta: accessMeta('/materials'),
  },
  {
    path: '/overview',
    name: 'asset-overview',
    component: () => import('./pages/OverviewPage.vue'),
    meta: accessMeta('/overview'),
  },
  {
    path: '/settlement',
    name: 'asset-settlement',
    component: () => import('./pages/SettlementPage.vue'),
    meta: accessMeta('/settlement'),
  },
  {
    path: '/reports',
    name: 'asset-reports',
    component: () => import('./pages/ReportsPage.vue'),
    meta: accessMeta('/reports'),
  },
  {
    path: '/settings',
    component: () => import('./shell/SettingsShell.vue'),
    meta: { ...accessMeta('/settings'), settings: true },
    children: [
      {
        path: 'pricing',
        name: 'asset-settings-pricing',
        component: () => import('./pages/CostCenterPage.vue'),
        meta: accessMeta('/settings/pricing'),
      },
      {
        path: 'people',
        name: 'asset-settings-people',
        component: () => import('./pages/PeoplePage.vue'),
        meta: accessMeta('/settings/people'),
      },
      {
        path: 'members',
        name: 'asset-settings-members',
        component: () => import('./pages/MembersPage.vue'),
        meta: accessMeta('/settings/members'),
      },
      {
        path: 'events',
        name: 'asset-settings-events',
        component: () => import('./pages/EventsPage.vue'),
        meta: accessMeta('/settings/events'),
      },
    ],
  },
  {
    path: '/cost-center',
    redirect: '/settings/pricing',
  },
  {
    path: '/people',
    redirect: '/settings/people',
  },
  {
    path: '/members',
    redirect: '/settings/members',
  },
  {
    path: '/events',
    redirect: '/settings/events',
  },
  {
    path: '/403',
    name: 'asset-forbidden',
    component: () => import('./pages/ForbiddenPage.vue'),
    meta: accessMeta('/403'),
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

router.beforeEach(async (to) => {
  if (to.meta.public || !getToken()) return true
  const session = useAssetWorkbenchSessionStore()
  const entry = await session.loadEntry()
  if (entry && entry.state !== 'ready') return true
  const bootstrap = session.bootstrap ?? (await session.refresh())

  if (to.path === '/settings') {
    const settingsPath = firstAccessibleSettingsPath(bootstrap)
    if (settingsPath) return { path: settingsPath, query: to.query, hash: to.hash }
    return { path: '/403', query: { from: to.fullPath } }
  }

  if (to.path === '/' && isConfigOnlyAdmin(bootstrap)) {
    const settingsPath = firstAccessibleSettingsPath(bootstrap)
    if (settingsPath) return settingsPath
  }

  const allowed = canAccessAssetWorkbenchRoute(bootstrap, routeAccessForPath(to.path))
  if (!allowed && to.path !== '/403') {
    return { path: '/403', query: { from: to.fullPath } }
  }
  return true
})
