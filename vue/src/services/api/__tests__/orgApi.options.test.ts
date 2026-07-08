import { beforeEach, describe, expect, it, vi } from 'vitest'

const getMock = vi.fn()

vi.mock('@/services/http', () => ({
  default: {
    get: getMock,
  },
}))

describe('fetchOrgOwnershipOptions', () => {
  beforeEach(() => {
    getMock.mockReset()
  })

  it('requests disabled org master rows and parses team_items ids', async () => {
    getMock.mockResolvedValue({
      data: {
        data: {
          departments: [
            {
              id: 7,
              name: '采购部',
              enabled: false,
              member_count: 0,
              teams: ['旧采购组'],
              team_items: [
                {
                  department_id: 7,
                  team_id: 13,
                  name: '旧采购组',
                  enabled: false,
                  member_count: 0,
                },
              ],
            },
          ],
        },
      },
    })

    const { fetchOrgOwnershipOptions } = await import('../orgApi')
    const parsed = await fetchOrgOwnershipOptions({ includeDisabled: true })

    expect(getMock).toHaveBeenCalledWith('/v1/org/options', {
      signal: undefined,
      params: { include_disabled: true },
    })
    expect(parsed.departmentRecords).toEqual([
      { id: '7', name: '采购部', enabled: false, memberCount: 0 },
    ])
    expect(parsed.teamOptions).toEqual([
      { value: '旧采购组', label: '旧采购组', department: '采购部' },
    ])
    expect(parsed.teamRecords).toEqual([
      {
        id: '13',
        name: '旧采购组',
        departmentId: '7',
        departmentName: '采购部',
        enabled: false,
        memberCount: 0,
      },
    ])
  })
})
