// @vitest-environment jsdom

import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const mocks = vi.hoisted(() => ({
  login: vi.fn(),
  me: vi.fn(),
  refreshAssetCookie: vi.fn(() => Promise.resolve()),
  loadNotifications: vi.fn(() => Promise.resolve()),
  startRealtime: vi.fn(),
  initWebPush: vi.fn(() => Promise.resolve()),
}))

vi.mock('@/services/api/authApi', () => ({
  authApi: {
    login: mocks.login,
    me: mocks.me,
    refreshAssetCookie: mocks.refreshAssetCookie,
    logout: vi.fn(() => Promise.resolve()),
  },
}))

vi.mock('@/stores/notifications.store', () => ({
  useNotificationsStore: () => ({ load: mocks.loadNotifications, reset: vi.fn() }),
}))

vi.mock('@/stores/realtime.store', () => ({
  useRealtimeStore: () => ({ start: mocks.startRealtime, stop: vi.fn() }),
}))

vi.mock('@/stores/webPush.store', () => ({
  useWebPushStore: () => ({ initAfterLogin: mocks.initWebPush, cleanupOnLogout: vi.fn(() => Promise.resolve()) }),
}))

describe('permissions login recovery', () => {
  let usePermissionsStore: (typeof import('@/stores/permissions'))['usePermissionsStore']

  beforeAll(async () => {
    ;({ usePermissionsStore } = await import('@/stores/permissions'))
  })

  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    vi.clearAllMocks()
  })

  it('hydrates the session directly from the successful login response', async () => {
    mocks.login.mockResolvedValue({
      data: {
        data: {
          session: { token: 'login-token' },
          user: {
            id: '17', username: 'designer-17', display_name: '设计十七', department: '设计部', team: '常规组', roles: ['designer'],
            frontend_access: { menus: ['task_list'], pages: ['task_list'], actions: ['task.view'], roles: ['designer'] },
          },
        },
      },
    })

    const store = usePermissionsStore()
    await store.loginWithCredentials('designer-17', 'password-value')

    expect(mocks.me).not.toHaveBeenCalled()
    expect(store.currentUser?.name).toBe('设计十七')
    expect(store.actions).toContain('task.view')
    expect(localStorage.getItem('access_token')).toBe('login-token')
    expect(mocks.startRealtime).toHaveBeenCalledTimes(1)
  })

  it('keeps a newly issued token when compatibility /auth/me is transiently rate limited', async () => {
    mocks.login.mockResolvedValue({ data: { data: { session: { token: 'keep-on-429' } } } })
    mocks.me.mockRejectedValue({ status: 429 })

    const store = usePermissionsStore()
    await expect(store.loginWithCredentials('legacy-user', 'password-value')).rejects.toMatchObject({ status: 429 })

    expect(localStorage.getItem('access_token')).toBe('keep-on-429')
  })

  it('clears a newly issued token when compatibility /auth/me returns 401', async () => {
    mocks.login.mockResolvedValue({ data: { data: { session: { token: 'reject-on-401' } } } })
    mocks.me.mockRejectedValue({ status: 401 })

    const store = usePermissionsStore()
    await expect(store.loginWithCredentials('legacy-user', 'password-value')).rejects.toMatchObject({ status: 401 })

    expect(localStorage.getItem('access_token')).toBeNull()
  })
})
