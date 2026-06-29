import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

import { getToken } from '@/services/http'
import { canAccessAssetWorkbenchRoute, routeAccessForPath } from './app/access'
import { useAssetWorkbenchSessionStore } from './app/session.store'
import { prefersReducedMotion } from './shared/motion/tokens'

declare module 'vue-router' {
  interface RouteMeta {
    label?: string
    public?: boolean
    anyAuthenticated?: boolean
    simple?: boolean
    requiresAnyCapability?: readonly string[]
  }
}

function accessMeta(path: string) {
  const access = routeAccessForPath(path)
  return access
    ? {
        label: access.label,
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
    path: '/cost-center',
    name: 'asset-cost-center',
    component: () => import('./pages/CostCenterPage.vue'),
    meta: accessMeta('/cost-center'),
  },
  {
    path: '/template-assignments',
    name: 'asset-template-assignments',
    component: () => import('./pages/TemplateAssignPage.vue'),
    meta: accessMeta('/template-assignments'),
  },
  {
    path: '/settlement',
    name: 'asset-settlement',
    component: () => import('./pages/SettlementPage.vue'),
    meta: accessMeta('/settlement'),
  },
  {
    path: '/people',
    name: 'asset-people',
    component: () => import('./pages/PeoplePage.vue'),
    meta: accessMeta('/people'),
  },
  {
    path: '/members',
    name: 'asset-members',
    component: () => import('./pages/MembersPage.vue'),
    meta: accessMeta('/members'),
  },
  {
    path: '/events',
    name: 'asset-events',
    component: () => import('./pages/EventsPage.vue'),
    meta: accessMeta('/events'),
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
  const allowed = canAccessAssetWorkbenchRoute(session.bootstrap, routeAccessForPath(to.path))
  if (!allowed && to.path !== '/403') {
    return { path: '/403', query: { from: to.fullPath } }
  }
  return true
})

router.beforeResolve((to, from, next) => {
  if (!from.matched.length || to.fullPath === from.fullPath || prefersReducedMotion()) {
    next()
    return
  }
  const documentWithTransitions = document as Document & {
    startViewTransition?: (callback: () => Promise<void>) => { finished: Promise<void> }
  }
  if (typeof documentWithTransitions.startViewTransition !== 'function') {
    next()
    return
  }

  const transition = documentWithTransitions.startViewTransition(
    () =>
      new Promise<void>((resolve) => {
        next()
        requestAnimationFrame(() => resolve())
      }),
  )
  transition.finished.catch(() => undefined)
})
