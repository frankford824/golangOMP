import { beforeEach, describe, expect, it, vi } from 'vitest'

const postMock = vi.fn()

vi.mock('@/services/http', () => ({
  default: {
    post: postMock,
  },
}))

describe('assetWorkbenchApi Excel imports', () => {
  beforeEach(() => {
    postMock.mockReset()
    postMock.mockResolvedValue({ data: { data: {} } })
  })

  it('sends every Excel import as multipart form data', async () => {
    const { assetWorkbenchApi } = await import('./assetWorkbenchApi')
    const file = new File(['excel'], 'records.xlsx', {
      type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    })

    await assetWorkbenchApi.importSettlementSupplementsExcel('2026-07', file)
    await assetWorkbenchApi.importErrorExcel('2026-07', file)
    await assetWorkbenchApi.importSubmissionItemQCExcel('2026-07', file)

    expect(postMock).toHaveBeenCalledTimes(3)
    for (const [, body, config] of postMock.mock.calls) {
      expect(body).toBeInstanceOf(FormData)
      expect((body as FormData).get('file')).toBe(file)
      expect(config).toMatchObject({
        headers: { 'Content-Type': 'multipart/form-data' },
      })
    }
  })
})
