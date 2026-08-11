// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
	createExcelPackageJob: vi.fn(),
	getExcelPackageJob: vi.fn(),
}))

vi.mock('@/services/api/assetsApi', async (loadOriginal) => {
  const original = await loadOriginal<typeof import('@/services/api/assetsApi')>()
  return {
    ...original,
    assetsApi: {
      ...original.assetsApi,
		createExcelPackageJob: mocks.createExcelPackageJob,
		getExcelPackageJob: mocks.getExcelPackageJob,
    },
  }
})

import ProductionPackageDialog from './ProductionPackageDialog.vue'

describe('ProductionPackageDialog unified production package', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('preserves duplicate SKU rows and sends the selected TIF-only production filter', async () => {
		mocks.createExcelPackageJob.mockResolvedValueOnce({ data: { data: { job_id: 'pkg-1' } } })
		mocks.getExcelPackageJob.mockResolvedValueOnce({
			data: {
				data: {
					job_id: 'pkg-1',
					status: 'succeeded',
					total_count: 2,
					processed_count: 2,
					failed_count: 0,
					download_url: 'https://oss.test/package.zip',
					filename: 'package.zip',
					manifest: {
					items: [
            {
              row_number: 1,
              order_no: 'HSC34548',
              sku_code: 'HSC34548',
              sku_name: '镂空模板',
              quantity: 1,
              asset_id: 753,
              task_id: 753,
              filename: '镂空文件.tif',
              file_size: 1024,
              download_url: 'https://oss.test/final.tif',
            },
          ],
          success_count: 2,
          failure_count: 0,
					total_files: 2,
					total_size: 2048,
					},
				},
      },
    })

    const wrapper = mount(ProductionPackageDialog, {
      props: { open: true },
      attachTo: document.body,
    })
    const textarea = document.body.querySelector<HTMLTextAreaElement>('.package-dialog textarea')
    if (!textarea) throw new Error('missing package search textarea')
    textarea.value = 'HSC34548\nHSC34548'
    textarea.dispatchEvent(new Event('input'))
    await wrapper.vm.$nextTick()
		const search = document.body.querySelector<HTMLButtonElement>('.package-dialog .secondary-button')
    if (!search) throw new Error('missing search button')
    search.click()
    await flushPromises()

		expect(mocks.createExcelPackageJob).toHaveBeenCalledWith(
      [
        {
          row_number: 1,
          order_no: 'HSC34548',
          sku_code: 'HSC34548',
          quantity: 1,
        },
        {
          row_number: 2,
          order_no: 'HSC34548',
          sku_code: 'HSC34548',
          quantity: 1,
        },
			],
			'tif',
			expect.any(AbortSignal),
		)
		expect(mocks.getExcelPackageJob).toHaveBeenCalledWith('pkg-1', expect.any(AbortSignal))
    expect(document.body.textContent).toContain('匹配行')
    expect(document.body.textContent).toContain('生产文件')
    expect(document.body.textContent).not.toContain('Excel 仓库外发')
    wrapper.unmount()
  })
})
