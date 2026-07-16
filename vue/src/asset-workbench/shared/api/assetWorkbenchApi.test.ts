import { beforeEach, describe, expect, it, vi } from 'vitest'

const postMock = vi.fn()
const getMock = vi.fn()

vi.mock('@/services/http', () => ({
  default: {
    get: getMock,
    post: postMock,
  },
}))

describe('assetWorkbenchApi Excel imports', () => {
  beforeEach(() => {
    getMock.mockReset()
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

describe('assetWorkbenchApi array response normalization', () => {
  beforeEach(() => {
    getMock.mockReset()
  })

  it('normalizes a null overview item list to an empty result', async () => {
    getMock.mockResolvedValue({
      data: {
        data: {
          items: null,
          total: 0,
          page: 1,
          size: 60,
        },
      },
    })
    const { assetWorkbenchApi } = await import('./assetWorkbenchApi')

    await expect(assetWorkbenchApi.overviewSearch({ q: 'DZC000027' })).resolves.toEqual({
      items: [],
      total: 0,
      page: 1,
      size: 60,
    })
  })

  it('normalizes a null client material list to an empty array', async () => {
    getMock.mockResolvedValue({ data: { data: null } })
    const { assetWorkbenchApi } = await import('./assetWorkbenchApi')

    await expect(assetWorkbenchApi.listClientMaterials()).resolves.toEqual([])
  })
})

describe('assetWorkbenchApi material source discriminators', () => {
  beforeEach(() => {
    getMock.mockReset()
    getMock.mockResolvedValue({ data: { data: { download_mode: 'direct', filename: 'external.png', file_size: 1 } } })
  })

  it('downloads migrated external_asset rows through the external resource route', async () => {
    const { assetWorkbenchApi, isExternalMaterialSource } = await import('./assetWorkbenchApi')
    expect(isExternalMaterialSource('external_asset')).toBe(true)

    await assetWorkbenchApi.downloadMaterialAsset({ id: 42, source_type: 'external_asset', resource_id: 'ext-42' })

    expect(getMock).toHaveBeenCalledWith('/v1/assets/ext-42/download', { signal: undefined })
  })
})

describe('assetWorkbenchApi resource-group downloads', () => {
  beforeEach(() => {
    getMock.mockReset()
    postMock.mockReset()
  })

  it('keeps every finalized set item in the download result and uses the selected cover', async () => {
    postMock.mockResolvedValue({ data: { data: { items: [
      { group_id: 8, revision_id: 70, revision_item_id: 701, task_id: 3, sort_order: 1, filename: 'front.png', download_url: 'https://files/front' },
      { group_id: 8, revision_id: 70, revision_item_id: 702, task_id: 3, sort_order: 2, filename: 'side.png', download_url: 'https://files/side' },
    ] } } })
    const { assetWorkbenchApi } = await import('./assetWorkbenchApi')

    const result = await assetWorkbenchApi.downloadMaterialAsset({ id: 8, resource_group_id: 8, cover_revision_item_id: 702 })

    expect(result.download_url).toBe('https://files/side')
    expect(result.items?.map((item) => item.filename)).toEqual(['front.png', 'side.png'])
    expect(postMock).toHaveBeenCalledWith('/v1/resource-groups/batch-download', { group_ids: [8] }, { signal: undefined })
  })

  it('fails the whole resource-group download when the manifest is empty', async () => {
    postMock.mockResolvedValue({ data: { data: { items: [] } } })
    const { assetWorkbenchApi } = await import('./assetWorkbenchApi')

    await expect(assetWorkbenchApi.downloadMaterialAsset({ id: 8, resource_group_id: 8 })).rejects.toThrow('没有可下载的最终成品')
  })
})
