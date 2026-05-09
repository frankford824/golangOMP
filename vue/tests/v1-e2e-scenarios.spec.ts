import { describe, expect, it } from 'vitest'
import http from '../src/services/http'
import { authApi } from '../src/services/api/authApi'
import { tasksApi } from '../src/services/api/tasksApi'
import { taskDraftsApi } from '../src/services/api/taskDraftsApi'
import { notificationsApi } from '../src/services/api/notificationsApi'

describe('V1 mock E2E flow', () => {
  it('auth login + me returns demo user with super_admin frontend_access', async () => {
    const login = await authApi.login({ username: 'demo', password: 'demo1234' })
    const loginData = login.data as {
      token?: string
      user?: { id?: string; frontend_access?: { is_super_admin?: boolean } }
    }
    expect(loginData.token).toBeTruthy()
    expect(loginData.user?.id).toBe('ops_demo')
    expect(loginData.user?.frontend_access?.is_super_admin).toBe(true)

    const me = await authApi.me()
    const meData = me.data as { frontend_access?: { menus?: string[] } }
    expect(meData.frontend_access?.menus ?? []).toContain('task_list')
  })

  it('loads modules-driven task detail without extra fetch assumptions', async () => {
    const list = await tasksApi.list()
    const taskId = (list.data as { items?: Array<{ id: string }> }).items?.[0]?.id
    expect(taskId).toBeTruthy()
    if (!taskId) return
    const detail = await tasksApi.getDetail(taskId)
    expect((detail.data as { modules?: unknown[] }).modules).toBeTruthy()
  })

  it('draft create then remove works', async () => {
    const created = await taskDraftsApi.create({
      task_type: 'original_product_development',
      payload: { name: 'draft' },
    })
    const id = (created.data as { id: string }).id
    expect(id).toBeTruthy()
    await taskDraftsApi.deleteById(id)
  })

  it('me task drafts returns only current user', async () => {
    const res = await taskDraftsApi.listMine()
    const items = (res.data as { items?: { id: string }[] }).items ?? []
    expect(Array.isArray(items)).toBe(true)
  })

  it('list tasks honors pagination and filters', async () => {
    const res = await tasksApi.list({ page: 1, page_size: 1, sort: '-updated_at' })
    const data = res.data as { items?: unknown[]; total?: number; page?: number; page_size?: number }
    expect(Array.isArray(data.items)).toBe(true)
    expect(data.page).toBe(1)
    expect(data.page_size).toBe(1)
  })

  it('module pool requires pool team match in mock', async () => {
    const emptyPool = await tasksApi.pool()
    const emptyItems = (emptyPool.data as { items?: Array<{ module_key?: string }> }).items ?? []
    expect(emptyItems.length).toBe(0)

    const matchedPool = await tasksApi.pool({ pool_team_code: 'design_standard' })
    const matchedItems = (matchedPool.data as { items?: Array<{ module_key?: string }> }).items ?? []
    expect(matchedItems.length).toBeGreaterThan(0)
    for (const row of matchedItems) {
      expect(typeof row.module_key).toBe('string')
      expect((row.module_key ?? '').length).toBeGreaterThan(0)
    }
  })

  it('pending assign list uses /v1/tasks with task_status', async () => {
    const res = await tasksApi.list({ task_status: 'PendingAssign', page: 1, page_size: 20 })
    const data = res.data as { items?: Array<{ status?: string }>; total?: number }
    const items = data.items ?? []
    expect(Array.isArray(items)).toBe(true)
    expect(Number(data.total ?? 0)).toBeGreaterThanOrEqual(items.length)
  })

  it('cancel returns 409 when task already claimed, force closes it', async () => {
    const res = await tasksApi.list({ status: 'in_progress', page_size: 10 })
    const list = (res.data as { items?: Array<{ id: string }> }).items ?? []
    const taskId = list[0]?.id
    expect(taskId).toBeTruthy()
    if (!taskId) return
    let conflict = false
    try {
      await tasksApi.cancel(taskId, { reason: 'test cancel', force: false })
    } catch (err) {
      const status =
        (err as { status?: number; response?: { status?: number } }).status ??
        (err as { response?: { status?: number } }).response?.status
      if (status === 409) conflict = true
    }
    expect(conflict).toBe(true)
    const forced = await tasksApi.cancel(taskId, { reason: 'admin force close', force: true })
    expect((forced.data as { status?: string }).status).toBe('closed')
  })

  it('excel batch preview returns steps, preview rows, and violations', async () => {
    const [tpl, parsed] = await Promise.all([
      http.get('/v1/tasks/batch-create/template.xlsx'),
      http.post('/v1/tasks/batch-create/parse-excel', { file_name: 'sku.xlsx' }),
    ])
    expect((tpl.data as { file_name?: string }).file_name).toContain('.xlsx')
    const parseData = parsed.data as { preview?: unknown[]; violations?: unknown[] }
    expect(Array.isArray(parseData.preview)).toBe(true)
    expect(Array.isArray(parseData.violations)).toBe(true)
  })

  it('notifications read-all marks unread count to zero', async () => {
    const before = await notificationsApi.unreadCount()
    expect(Number((before.data as { unread_count?: number }).unread_count ?? 0)).toBeGreaterThanOrEqual(0)
    await notificationsApi.readAll()
    const after = await notificationsApi.unreadCount()
    expect(Number((after.data as { unread_count?: number }).unread_count ?? 0)).toBe(0)
  })
})
