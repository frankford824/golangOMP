import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useErpIidOptions } from '../src/composables/useErpIidOptions'
import { fetchErpIidOptions, isValidErpIid } from '../src/services/erpIidCatalog'
import { erpApi } from '../src/services/api/erpApi'

vi.mock('../src/services/api/erpApi', () => ({ erpApi: { getIids: vi.fn() } }))

function result(iids: string[], page = 1, total = iids.length, pageSize = 200) {
  return { data: { data: iids.map((i_id) => ({ i_id })), pagination: { page, page_size: pageSize, total } } } as never
}

describe('ERP IID authority shared by selection and submission', () => {
  beforeEach(() => { vi.resetAllMocks() })

  it('loads ERP options even with an empty keyword; never adds hardcoded presets', async () => {
    vi.mocked(erpApi.getIids).mockResolvedValue(result(['常规喷绘布']))
    const { loadIids, items } = useErpIidOptions()
    expect(items.value).toEqual([])
    await loadIids('')
    expect(erpApi.getIids).toHaveBeenCalledWith({ q: '', page: 1, page_size: 200 }, expect.any(AbortSignal))
    expect(items.value.map((item) => item.i_id)).toEqual(['常规喷绘布'])
  })

  it('uses the same successful ERP data for selecting and exact submission validation', async () => {
    vi.mocked(erpApi.getIids).mockResolvedValue(result(['定制喷绘布']))
    const { loadIids, selectOptions } = useErpIidOptions()
    await loadIids('喷绘')
    expect(selectOptions.value).toEqual([{ value: '定制喷绘布', label: '定制喷绘布' }])
    expect(await isValidErpIid('定制喷绘布')).toBe(true)
    expect(await isValidErpIid('喷绘')).toBe(false)
  })

  it('returns an empty list and invalid result when the replica has no match', async () => {
    vi.mocked(erpApi.getIids).mockResolvedValue(result([]))
    const { loadIids, items, hasLoaded, error } = useErpIidOptions()
    await loadIids('定制喷绘布')
    expect(items.value).toEqual([])
    expect(hasLoaded.value).toBe(true)
    expect(error.value).toBe('')
    expect(await isValidErpIid('定制喷绘布')).toBe(false)
  })

  it('distinguishes errors from no match and retries without cached fallback or stale success', async () => {
    vi.mocked(erpApi.getIids)
      .mockResolvedValueOnce(result(['定制喷绘布']))
      .mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValueOnce(result([]))
    const { loadIids, items, error, hasLoaded } = useErpIidOptions()
    await loadIids('定制喷绘布')
    await loadIids('定制喷绘布')
    expect(items.value).toEqual([])
    expect(hasLoaded.value).toBe(false)
    expect(error.value).toContain('查询失败')
    await loadIids('定制喷绘布')
    expect(items.value).toEqual([])
    expect(error.value).toBe('')
    expect(erpApi.getIids).toHaveBeenCalledTimes(3)
  })

  it('ignores an older response after the query changes', async () => {
    let finish!: (value: never) => void
    vi.mocked(erpApi.getIids)
      .mockImplementationOnce(() => new Promise((resolve) => { finish = resolve }))
      .mockResolvedValueOnce(result(['常规喷绘布']))
    const { loadIids, items, error, loading } = useErpIidOptions()
    const first = loadIids('定制')
    const signal = vi.mocked(erpApi.getIids).mock.calls[0]?.[1]
    await loadIids('常规')
    finish(result(['定制喷绘布']))
    await first
    expect(signal?.aborted).toBe(true)
    expect(items.value.map((item) => item.i_id)).toEqual(['常规喷绘布'])
    expect(error.value).toBe('')
    expect(loading.value).toBe(false)
  })

  it('clears and aborts pending requests on reset', async () => {
    let finish!: (value: never) => void
    vi.mocked(erpApi.getIids).mockImplementationOnce(() => new Promise((resolve) => { finish = resolve }))
    const state = useErpIidOptions()
    const pending = state.loadIids()
    state.reset()
    finish(result(['定制喷绘布']))
    await pending
    expect(state.items.value).toEqual([])
    expect(state.loading.value).toBe(false)
  })

  it('reads later pages for both the dropdown and exact validator', async () => {
    vi.mocked(erpApi.getIids).mockImplementation(async (params) =>
      params?.page === 1 ? result(['别的款式'], 1, 2, 1) : result(['定制喷绘布'], 2, 2, 1))
    expect((await fetchErpIidOptions()).map((item) => item.i_id)).toEqual(['别的款式', '定制喷绘布'])
    expect(await isValidErpIid('定制喷绘布')).toBe(true)
    expect(erpApi.getIids).toHaveBeenLastCalledWith({ q: '定制喷绘布', page: 2, page_size: 200 }, undefined)
  })

  it('does not turn a broken later page into an invalid code', async () => {
    vi.mocked(erpApi.getIids).mockResolvedValueOnce(result(['别的款式'], 1, 2, 1)).mockRejectedValueOnce(new Error('offline'))
    await expect(isValidErpIid('定制喷绘布')).rejects.toThrow('offline')
  })

  it('does not accept labels as identifiers and normalizes identifier whitespace/case', async () => {
    vi.mocked(erpApi.getIids).mockResolvedValue({ data: { data: [{ label: '定制喷绘布' }, { i_id: ' KT_STANDARD ' }] } } as never)
    expect(await isValidErpIid('定制喷绘布')).toBe(false)
    expect(await isValidErpIid('kt_standard')).toBe(true)
  })

  it('fails closed for malformed or incomplete replies', async () => {
    vi.mocked(erpApi.getIids).mockResolvedValueOnce({ data: { message: 'bad gateway' } } as never)
    await expect(fetchErpIidOptions()).rejects.toThrow('格式异常')
    vi.mocked(erpApi.getIids).mockResolvedValueOnce(result([], 1, 5))
    await expect(fetchErpIidOptions()).rejects.toThrow('不完整')
  })
})
