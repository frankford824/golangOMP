import { afterEach, expect, it, vi } from 'vitest'
import { useExpenseActivity } from './expenseActivity'

const { api, getToken } = vi.hoisted(() => ({ api: vi.fn(), getToken: vi.fn() }))
vi.mock('./api', () => ({ api }))
vi.mock('@/services/http', () => ({ getToken }))

const activity = useExpenseActivity()

afterEach(() => {
  activity.reset()
  api.mockReset()
  getToken.mockReset()
})

it('drops a previous account response when the current account changes', async () => {
  getToken.mockReturnValue('old-session')
  let completeOldRequest!: (value: { todo_count: number; unread_result_count: number }) => void
  api.mockImplementationOnce(() => new Promise(resolve => { completeOldRequest = resolve }))
  const oldRequest = activity.refresh()

  getToken.mockReturnValue('new-session')
  api.mockResolvedValueOnce({ todo_count: 1, unread_result_count: 2 })
  await activity.refresh()
  completeOldRequest({ todo_count: 9, unread_result_count: 9 })
  await oldRequest

  expect(activity.todoCount).toBe(1)
  expect(activity.unreadResultCount).toBe(2)
  expect(api).toHaveBeenCalledTimes(2)
})

it('refreshes the badge and signals visible lists after a workflow mutation', async () => {
  getToken.mockReturnValue('fixture')
  api.mockResolvedValueOnce({ todo_count: 2, unread_result_count: 1 })
  await activity.refresh()
  const revision = activity.revision
  api.mockResolvedValueOnce({ todo_count: 1, unread_result_count: 0 })

  activity.invalidate()
  await Promise.resolve()

  expect(activity.revision).toBe(revision + 1)
  expect(activity.todoCount).toBe(1)
  expect(activity.unreadResultCount).toBe(0)
})
