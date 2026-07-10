import { describe, expect, it, vi } from 'vitest'
import {
  createExcelPackageBlobLoader,
  resolveExcelPackageZipFilename,
  sumExcelPackageQuantities,
} from '@/utils/excelPackageZip'

function item(overrides: Record<string, unknown> = {}) {
  return {
    asset_id: 25,
    download_url: 'https://oss.test/HSC04325.jpg',
    filename: 'HSC04325-30年再聚首.jpg',
    order_no: 'HSC04325',
    resource_id: 'ext-25',
    sku_code: 'HSC04325',
    sku_name: '',
    source_type: 'external',
    ...overrides,
  }
}

describe('excelPackageZip', () => {
  it('preserves the source filename without duplicating the SKU prefix', () => {
    expect(resolveExcelPackageZipFilename(item(), 1)).toBe('HSC04325-30年再聚首_1.jpg')
    expect(resolveExcelPackageZipFilename(item({ filename: '30年再聚首.jpg' }), 2)).toBe('HSC04325_30年再聚首_2.jpg')
    expect(resolveExcelPackageZipFilename(item({ filename: 'HSC04325.jpg' }), 3)).toBe('HSC04325_3.jpg')
  })

  it('keeps a distinct order number in flat package names', () => {
    expect(resolveExcelPackageZipFilename(item({ order_no: 'SO-100' }), 1)).toBe('SO-100_HSC04325-30年再聚首_1.jpg')
  })

  it('counts requested image quantities instead of rows', () => {
    expect(sumExcelPackageQuantities([{ quantity: 6 }, { quantity: 3 }, { quantity: 0 }])).toBe(10)
  })

  it('reconciles the reported 52 requested images into 41 matched and 11 unmatched', () => {
    const matched = Array.from({ length: 41 }, () => ({ quantity: 1 }))
    const unmatched = Array.from({ length: 11 }, () => ({ quantity: 1 }))
    expect(sumExcelPackageQuantities([...matched, ...unmatched])).toBe(52)
    expect(sumExcelPackageQuantities(matched)).toBe(41)
    expect(sumExcelPackageQuantities(unmatched)).toBe(11)
  })

  it('downloads the same source once and shares the blob across duplicate rows', async () => {
    const blob = new Blob(['image'])
    const fetcher = vi.fn(async () => ({ ok: true, status: 200, blob: async () => blob }) as Response)
    const load = createExcelPackageBlobLoader(fetcher)

    const [first, second, third] = await Promise.all([load(item()), load(item()), load(item())])

    expect(fetcher).toHaveBeenCalledTimes(1)
    expect(first).toBe(blob)
    expect(second).toBe(blob)
    expect(third).toBe(blob)
  })
})
