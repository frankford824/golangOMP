// @vitest-environment jsdom

import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { TOKEN_STORAGE_KEY } from '@/services/http'

const mocks = vi.hoisted(() => ({
  login: vi.fn(),
  me: vi.fn(),
  migrate: vi.fn(),
  refreshAssetCookie: vi.fn(() => Promise.resolve()),
  loadNotifications: vi.fn(() => Promise.resolve()),
  startRealtime: vi.fn(),
  initWebPush: vi.fn(() => Promise.resolve()),
}))

vi.mock('@/services/api/authApi', () => ({
  authApi: {
    login: mocks.login,
    me: mocks.me,
    migrateOASession: mocks.migrate,
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
            id: '17', username: 'designer-17', display_name: '设计十七', department: '设计部', department_id: 23, team: '常规组', roles: ['designer'],
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
    expect(store.actorDepartmentId).toBe(23)
    expect(store.actorDepartment).toBe('设计部')
    expect(localStorage.getItem(TOKEN_STORAGE_KEY)).toBe('login-token')
    expect(mocks.startRealtime).toHaveBeenCalledTimes(1)
  })

  it('keeps a newly issued token when compatibility /auth/me is transiently rate limited', async () => {
    mocks.login.mockResolvedValue({ data: { data: { session: { token: 'keep-on-429' } } } })
    mocks.me.mockRejectedValue({ status: 429 })

    const store = usePermissionsStore()
    await expect(store.loginWithCredentials('legacy-user', 'password-value')).rejects.toMatchObject({ status: 429 })

    expect(localStorage.getItem(TOKEN_STORAGE_KEY)).toBe('keep-on-429')
  })

  it('clears a newly issued token when compatibility /auth/me returns 401', async () => {
    mocks.login.mockResolvedValue({ data: { data: { session: { token: 'reject-on-401' } } } })
    mocks.me.mockRejectedValue({ status: 401 })

    const store = usePermissionsStore()
    await expect(store.loginWithCredentials('legacy-user', 'password-value')).rejects.toMatchObject({ status: 401 })

    expect(localStorage.getItem(TOKEN_STORAGE_KEY)).toBeNull()
  })
})


describe('OA browser session recovery', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.stubEnv('VITE_OA_FEATURES', 'true')
    localStorage.clear()
    vi.clearAllMocks()
  })

  afterEach(() => vi.unstubAllEnvs())

  async function newStore() {
    const { createPinia, setActivePinia } = await import('pinia')
    const { usePermissionsStore } = await import('@/stores/permissions')
    setActivePinia(createPinia())
    return usePermissionsStore()
  }

  const user = { id: 42, username: 'fixture', frontend_access: { menus: ['task_list'] } }

  it('uses the confirmed login response without requesting /auth/me again', async () => {
    const store = await newStore()
    mocks.login.mockResolvedValueOnce({ data: { data: { user } } })
    await store.loginWithCredentials('fixture', 'mock-password')
    expect(store.currentUser?.id).toBe('42')
    expect(mocks.me).not.toHaveBeenCalled()
  })

  it.each([{ status: 429, traceId: 'fixture-trace' }, new Error('timeout')])(
    'lets the same store retry a temporary /auth/me failure', async failure => {
      const store = await newStore()
      mocks.me.mockRejectedValueOnce(failure).mockResolvedValueOnce({ data: { data: user } })
      await expect(store.restoreSession()).rejects.toMatchObject({ cause: failure })
      expect(store.currentUser).toBeNull()
      await store.restoreSession()
      expect(store.currentUser?.id).toBe('42')
    },
  )

  it('holds legacy input in memory for retry without overriding the outage with /auth/me', async () => {
    const store = await newStore()
    const key = 'yb.oa.formal.token.v1'
    localStorage.setItem(key, 'migration-fixture')
    mocks.migrate.mockRejectedValueOnce({ status: 503 }).mockResolvedValueOnce(undefined)
    mocks.me.mockResolvedValueOnce({ data: { data: user } })

    await expect(store.restoreSession()).rejects.toMatchObject({ cause: { status: 503 } })
    expect(mocks.me).not.toHaveBeenCalled()
    expect(localStorage.getItem(key)).toBeNull()
    await store.restoreSession()
    expect(mocks.migrate.mock.calls).toEqual([['migration-fixture'], ['migration-fixture']])
    expect(store.currentUser?.id).toBe('42')
  })

  it('marks an authoritative 403 as access denied rather than retryable', async () => {
    const store = await newStore()
    mocks.me.mockRejectedValueOnce({ status: 403, traceId: 'fixture-trace' })
    await expect(store.restoreSession()).rejects.toMatchObject({
      blocked: true,
      traceId: 'fixture-trace',
      message: '当前账号无法访问 OA。',
    })
    expect(store.currentUser).toBeNull()
  })

  it('passes through an authoritative 401', async () => {
    const store = await newStore()
    mocks.me.mockRejectedValueOnce({ status: 401 })
    await expect(store.restoreSession()).rejects.toMatchObject({ status: 401 })
    expect(store.currentUser).toBeNull()
  })
})
