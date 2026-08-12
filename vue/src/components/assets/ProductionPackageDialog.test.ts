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
		const downloadClick = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined)
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
		const search = Array.from(document.body.querySelectorAll<HTMLButtonElement>('.package-dialog button'))
			.find((button) => button.textContent?.includes('生成并下载生产 ZIP'))
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
		expect(document.body.textContent).toContain('再次下载生产 ZIP')
		expect(downloadClick).toHaveBeenCalledOnce()
    expect(document.body.textContent).not.toContain('Excel 仓库外发')
		downloadClick.mockRestore()
    wrapper.unmount()
  })

	it('shows a visible download control before generation and enables it when the package is ready', async () => {
		mocks.createExcelPackageJob.mockResolvedValueOnce({ data: { data: { job_id: 'pkg-visible' } } })
		mocks.getExcelPackageJob.mockResolvedValueOnce({
			data: { data: {
				job_id: 'pkg-visible', status: 'succeeded', total_count: 1, processed_count: 1, failed_count: 0,
				download_url: 'https://oss.test/visible.zip', filename: 'visible.zip',
				manifest: { items: [], failures: [], success_count: 1, failure_count: 0, total_files: 1, total_size: 1 },
			} },
		})
		const downloadClick = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined)
		const wrapper = mount(ProductionPackageDialog, { props: { open: true }, attachTo: document.body })
		const download = Array.from(document.body.querySelectorAll<HTMLButtonElement>('.package-dialog button'))
			.find((button) => button.textContent?.includes('生成后可再次下载'))
		expect(download).toBeDefined()
		expect(download?.disabled).toBe(true)

		const textarea = document.body.querySelector<HTMLTextAreaElement>('.package-dialog textarea')
		const generate = document.body.querySelector<HTMLButtonElement>('.package-dialog .primary-button')
		if (!textarea || !generate) throw new Error('missing production package controls')
		textarea.value = 'CGP000349'
		textarea.dispatchEvent(new Event('input'))
		await wrapper.vm.$nextTick()
		generate.click()
		await flushPromises()

		expect(download?.disabled).toBe(false)
		expect(download?.textContent).toContain('再次下载生产 ZIP')
		expect(downloadClick).toHaveBeenCalledOnce()
		downloadClick.mockRestore()
		wrapper.unmount()
	})

	it('shows per-SKU reasons when no selected-format file can be packaged', async () => {
		mocks.createExcelPackageJob.mockResolvedValueOnce({ data: { data: { job_id: 'pkg-failed' } } })
		mocks.getExcelPackageJob.mockResolvedValueOnce({ data: { data: {
			job_id: 'pkg-failed', status: 'failed', total_count: 1, processed_count: 1, failed_count: 1,
			error_message: '未找到可打包的最终成品：1 行均未匹配所选格式，请查看逐行异常明细。',
			manifest: {
				items: [], success_count: 0, failure_count: 1, total_files: 0, total_size: 0,
				failures: [{ row_number: 1, sku_code: 'CGK001698', reason: 'asset_not_found', message: '未找到匹配的最终成品 TIF 文件' }],
			},
		} } })
		const wrapper = mount(ProductionPackageDialog, { props: { open: true }, attachTo: document.body })
		const textarea = document.body.querySelector<HTMLTextAreaElement>('.package-dialog textarea')
		const generate = document.body.querySelector<HTMLButtonElement>('.package-dialog .primary-button')
		if (!textarea || !generate) throw new Error('missing production package controls')
		textarea.value = 'CGK001698'
		textarea.dispatchEvent(new Event('input'))
		await wrapper.vm.$nextTick()
		generate.click()
		await flushPromises()

		expect(document.body.textContent).toContain('CGK001698：未找到匹配的最终成品 TIF 文件')
		expect(document.body.textContent).toContain('1 行均未匹配所选格式')
		wrapper.unmount()
	})
})
