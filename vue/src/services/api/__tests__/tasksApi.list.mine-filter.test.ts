import { describe, expect, it, vi } from 'vitest'

const getMock = vi.fn()

vi.mock('@/services/http', () => ({
  default: {
    get: getMock,
  },
}))

describe('tasksApi.list', () => {
  it('passes filter=mine to GET /v1/tasks', async () => {
    const { tasksApi } = await import('../tasksApi')
    await tasksApi.list({ filter: 'mine', page: 1, page_size: 20 })

    expect(getMock).toHaveBeenCalledTimes(1)
    expect(getMock).toHaveBeenCalledWith('/v1/tasks', {
      params: { filter: 'mine', page: 1, page_size: 20 },
      signal: undefined,
    })
  })
})
