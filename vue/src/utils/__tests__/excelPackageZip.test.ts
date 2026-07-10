import { describe, expect, it, vi } from 'vitest'
import {
  countExcelPackageRows,
  createExcelPackageBlobLoader,
  resolveExcelPackageSetFolders,
  resolveExcelPackageZipFilename,
  sumExcelPackageQuantities,
} from '@/utils/excelPackageZip'

function item(overrides: Record<string, unknown> = {}) {
  return {
    asset_id: 25,
    download_url: 'https://oss.test/HSC04325.jpg',
    filename: 'HSC04325-30年再聚首.jpg',
    order_no: 'HSC04325',
    package_folder: '',
    resource_id: 'ext-25',
    row_number: 2,
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

  it('groups multi-image XuKai SKU directories while leaving single images flat', () => {
    const packageFolder = 'HSC33333——真-常规水晶标-卡通喜字款'
    const setItems = [
      item({ asset_id: 31, resource_id: 'ext-31', sku_code: 'HSC33333', order_no: 'HSC33333', filename: '第一张.jpg', package_folder: packageFolder }),
      item({ asset_id: 32, resource_id: 'ext-32', sku_code: 'HSC33333', order_no: 'HSC33333', filename: '第二张.jpg', package_folder: packageFolder }),
      item({ asset_id: 33, resource_id: 'ext-33', sku_code: 'HSC33333', order_no: 'HSC33333', filename: '第三张.jpg', package_folder: packageFolder }),
    ]
    expect(resolveExcelPackageSetFolders(setItems)).toEqual([
      'HSC33333——真-常规水晶标-卡通喜字款',
      'HSC33333——真-常规水晶标-卡通喜字款',
      'HSC33333——真-常规水晶标-卡通喜字款',
    ])
    expect(resolveExcelPackageSetFolders([setItems[0]])).toEqual([''])
    expect(resolveExcelPackageZipFilename(setItems[0], 1, { includeBusinessPrefix: false })).toBe('第一张_1.jpg')
    expect(countExcelPackageRows(setItems)).toBe(1)
  })

  it('creates a distinct folder for a repeated set row', () => {
    const packageFolder = 'HSC33333——套装'
    const setItems = [
      item({ asset_id: 31, resource_id: 'ext-31', row_number: 2, sku_code: 'HSC33333', package_folder: packageFolder }),
      item({ asset_id: 32, resource_id: 'ext-32', row_number: 2, sku_code: 'HSC33333', package_folder: packageFolder }),
      item({ asset_id: 31, resource_id: 'ext-31', row_number: 3, sku_code: 'HSC33333', package_folder: packageFolder }),
      item({ asset_id: 32, resource_id: 'ext-32', row_number: 3, sku_code: 'HSC33333', package_folder: packageFolder }),
    ]
    expect(resolveExcelPackageSetFolders(setItems)).toEqual([
      'HSC33333——套装',
      'HSC33333——套装',
      'HSC33333——套装 (2)',
      'HSC33333——套装 (2)',
    ])
    expect(countExcelPackageRows(setItems)).toBe(2)
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
