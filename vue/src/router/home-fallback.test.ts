import { describe, expect, it } from 'vitest'
import { resolveFirstAccessibleHomeRoute, resolvePostLoginLandingRoute } from './home-fallback'
import type { PermissionEnumValue } from '@/types'
import type { Router } from 'vue-router'

function makePermissionsStore(options: {
  menus?: string[]
  actions?: string[]
}) {
  const menus = new Set(options.menus ?? [])
  const actions = new Set(options.actions ?? [])
  return {
    currentUser: { id: '1' },
    hasMenu: (key: string) => menus.has(key),
    hasPermission: (_perms: PermissionEnumValue | PermissionEnumValue[]) => false,
    hasAction: (key: string) => actions.has(key),
  }
}

describe('home fallback', () => {
  it('lands audit-only users on a plain task list without implicit status filters', () => {
    const route = resolveFirstAccessibleHomeRoute(
      makePermissionsStore({
        menus: ['task_list'],
        actions: ['task.audit.review'],
      }),
    )

    expect(route).toEqual({ name: 'TaskList' })
  })

  it('prefers dashboard when the dashboard menu is available', () => {
    const route = resolveFirstAccessibleHomeRoute(
      makePermissionsStore({
        menus: ['dashboard', 'task_list'],
        actions: ['task.audit.review'],
      }),
    )

    expect(route).toEqual({ name: 'Dashboard' })
  })

  it('does not preserve stale create-task redirects as the login landing page', () => {
    const router = {
      resolve: (path: string) => ({
        matched: [{}],
        meta: { requiresAuth: true },
        path,
        name: 'TaskCreate',
        query: {},
      }),
    } as unknown as Router

    const route = resolvePostLoginLandingRoute(
      router,
      makePermissionsStore({ menus: ['task_list'] }),
      '/tasks/create',
    )

    expect(route).toEqual({ name: 'TaskList' })
  })

  it('does not preserve task-list filter redirects as the login landing page', () => {
    const router = {
      resolve: (path: string) => ({
        matched: [{}],
        meta: { requiresAuth: true },
        path,
        name: 'TaskList',
        query: {
          tab: 'mine',
          task_category: 'normal',
          status: 'PendingAuditA,PendingAuditB',
        },
      }),
    } as unknown as Router

    const route = resolvePostLoginLandingRoute(
      router,
      makePermissionsStore({ menus: ['task_list'] }),
      '/tasks?tab=mine&task_category=normal&status=PendingAuditA,PendingAuditB',
    )

    expect(route).toEqual({ name: 'TaskList' })
  })

  it('keeps draft restore redirects on the create-task route', () => {
    const router = {
      resolve: (path: string) => ({
        matched: [{}],
        meta: { requiresAuth: true },
        path,
        name: 'TaskCreate',
        query: { draft_id: 'draft-1' },
      }),
    } as unknown as Router

    const route = resolvePostLoginLandingRoute(
      router,
      makePermissionsStore({ menus: ['task_list'] }),
      '/tasks/create?create=1&draft_id=draft-1',
    )

    expect(route).toBe('/tasks/create?create=1&draft_id=draft-1')
  })
})
