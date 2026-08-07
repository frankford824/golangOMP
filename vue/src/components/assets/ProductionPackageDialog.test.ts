// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  batchSearchAssets: vi.fn(),
}))

vi.mock('@/services/api/assetsApi', async (loadOriginal) => {
  const original = await loadOriginal<typeof import('@/services/api/assetsApi')>()
  return {
    ...original,
    assetsApi: {
      ...original.assetsApi,
      batchSearchAssets: mocks.batchSearchAssets,
    },
  }
})

import ProductionPackageDialog from './ProductionPackageDialog.vue'

describe('ProductionPackageDialog batch search', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('automatically exposes a real asset hidden by the default production filters', async () => {
    mocks.batchSearchAssets
      .mockResolvedValueOnce({
        data: {
          data: {
            results: [{
              term: 'RW-20260806-A-003753',
              status: 'not_found',
              message: '找到了资产，但没有符合当前格式或资源类型筛选的可下载资源',
              candidates: 0,
            }],
            matched_count: 0,
            failed_count: 1,
          },
        },
      })
      .mockResolvedValueOnce({
        data: {
          data: {
            results: [{
              term: 'RW-20260806-A-003753',
              status: 'matched',
              message: '已匹配',
              candidates: 1,
              assets: [{
                id: 'asset-753',
                resource_id: 'system:asset-753',
                task_id: 753,
                asset_type: 'source',
                file_name: '审核修改源文件.zip',
                source_type: 'system',
              }],
            }],
            matched_count: 1,
            failed_count: 0,
          },
        },
      })

    const wrapper = mount(ProductionPackageDialog, {
      props: { open: true },
      attachTo: document.body,
    })
    const textarea = document.body.querySelector<HTMLTextAreaElement>('.package-dialog textarea')
    if (!textarea) throw new Error('missing package search textarea')
    textarea.value = 'RW-20260806-A-003753'
    textarea.dispatchEvent(new Event('input'))
    await wrapper.vm.$nextTick()
    const search = Array.from(document.body.querySelectorAll<HTMLButtonElement>('button')).find((button) => button.textContent?.includes('查询资源'))
    if (!search) throw new Error('missing search button')
    search.click()
    await flushPromises()

    expect(mocks.batchSearchAssets).toHaveBeenNthCalledWith(1, {
      terms: ['RW-20260806-A-003753'],
      format_filter: 'image',
      asset_kind: 'delivery',
    })
    expect(mocks.batchSearchAssets).toHaveBeenNthCalledWith(2, {
      terms: ['RW-20260806-A-003753'],
      format_filter: 'all',
      asset_kind: 'all',
    })
    expect(document.body.textContent).toContain('审核修改源文件.zip')
    expect(document.body.textContent).toContain('已自动放宽筛选')
    expect(document.body.textContent).toContain('按全部格式找到 1 个资源')
    wrapper.unmount()
  })
})
