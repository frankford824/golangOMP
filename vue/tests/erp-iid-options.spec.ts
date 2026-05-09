import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useErpIidOptions } from '../src/composables/useErpIidOptions'
import { ERP_IID_PRESETS } from '../src/domain/erp-iid-presets'
import { erpApi } from '../src/services/api/erpApi'

vi.mock('../src/services/api/erpApi', () => ({
  erpApi: {
    getIids: vi.fn(),
  },
}))

describe('useErpIidOptions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('空关键词时直接使用本地预置且不请求 API', async () => {
    const { loadIids, items, lastSourceMode } = useErpIidOptions()
    await loadIids('')

    expect(erpApi.getIids).not.toHaveBeenCalled()
    expect(lastSourceMode.value).toBe('fallback')
    expect(items.value.length).toBe(ERP_IID_PRESETS.length)
  })

  it('API 成功时使用后端返回的 i_id 选项', async () => {
    vi.mocked(erpApi.getIids).mockResolvedValue({
      data: {
        data: [
          {
            i_id: '常规kt板',
            label: '常规kt板',
            category: '常规kt板',
            category_name: '常规KT板',
            product_count: 12,
          },
        ],
        pagination: { page: 1, page_size: 200, total: 1 },
      },
    } as never)

    const { loadIids, items, lastSourceMode, selectOptions } = useErpIidOptions()
    await loadIids('api-success')

    expect(lastSourceMode.value).toBe('api')
    expect(items.value.length).toBe(1)
    expect(items.value[0]?.i_id).toBe('常规kt板')
    expect(selectOptions.value[0]).toEqual({
      value: '常规kt板',
      label: '常规kt板（常规KT板）',
    })
  })

  it('API 失败时回退到本地 56 项预置', async () => {
    vi.mocked(erpApi.getIids).mockRejectedValue(new Error('network down'))

    const { loadIids, items, lastSourceMode } = useErpIidOptions()
    await loadIids('kt')

    expect(lastSourceMode.value).toBe('fallback')
    expect(items.value.length).toBeGreaterThan(0)
    expect(items.value.some((item) => item.i_id === '常规kt板')).toBe(true)
  })

  it('本地预置种子保持 56 项且包含关键编码', () => {
    expect(ERP_IID_PRESETS.length).toBe(56)
    expect(ERP_IID_PRESETS.some((item) => item.i_id === 'A4纸打印')).toBe(true)
    expect(ERP_IID_PRESETS.some((item) => item.i_id === '定制kt板(覆膜)')).toBe(true)
  })
})
