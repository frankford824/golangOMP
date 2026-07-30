import { beforeEach, describe, expect, it, vi } from 'vitest'

const postMock = vi.fn()

vi.mock('@/services/http', () => ({
  default: {
    post: postMock,
  },
}))

describe('resourceGroupsApi workflow writes', () => {
  beforeEach(() => {
    postMock.mockReset()
    postMock.mockResolvedValue({ data: { data: { task_id: 2891, workflow_revision: 2, groups: [] } } })
  })

  it('submits design when crypto.randomUUID is unavailable on an HTTP host', async () => {
    const originalRandomUUID = globalThis.crypto.randomUUID
    Object.defineProperty(globalThis.crypto, 'randomUUID', { configurable: true, value: undefined })

    try {
      const { resourceGroupsApi } = await import('./resourceGroupsApi')
      await expect(resourceGroupsApi.submitDesign(
        2891,
        { task_id: 2891, workflow_revision: 1, groups: [] },
        [],
      )).resolves.toMatchObject({ task_id: 2891 })

      expect(postMock).toHaveBeenCalledWith('/v1/tasks/2891/submit-design', {
        expected_workflow_revision: 1,
        idempotency_key: expect.stringMatching(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/),
        groups: [],
      })
    } finally {
      Object.defineProperty(globalThis.crypto, 'randomUUID', { configurable: true, value: originalRandomUUID })
    }
  })
})
