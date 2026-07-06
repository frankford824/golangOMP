import { describe, expect, it, vi } from 'vitest'

const patchMock = vi.fn()

vi.mock('@/services/api/usersApi', () => ({
  usersApi: {
    patch: patchMock,
  },
}))

describe('useOrgPermissionData membership helpers', () => {
  it('clearUserMembership sends the backend ungrouped alias only', async () => {
    const { clearUserMembership } = await import('../useOrgPermissionData')

    await clearUserMembership('42')

    expect(patchMock).toHaveBeenCalledTimes(1)
    expect(patchMock).toHaveBeenCalledWith('42', { team: 'ungrouped' })
  })
})
