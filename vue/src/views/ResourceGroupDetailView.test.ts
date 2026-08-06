// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ get: vi.fn(), costReconciliation: vi.fn(), batchDownload: vi.fn(), downloadBatchAsZip: vi.fn(), push: vi.fn() }))
vi.mock('vue-router', () => ({ useRoute: () => ({ params: { id: '8' } }), useRouter: () => ({ push: mocks.push }) }))
vi.mock('@/services/api/resourceGroupsApi', async (loadOriginal) => {
  const original = await loadOriginal<typeof import('@/services/api/resourceGroupsApi')>()
  return { ...original, resourceGroupsApi: { ...original.resourceGroupsApi, get: mocks.get, costReconciliation: mocks.costReconciliation, batchDownload: mocks.batchDownload } }
})
vi.mock('@/components/task/SkuResourceMatrix.vue', () => ({ default: { props: ['bundle'], template: '<div class="matrix-stub">三阶段资源</div>' } }))
vi.mock('@/utils/batchZipDownload', () => ({ downloadBatchAsZip: mocks.downloadBatchAsZip }))

import ResourceGroupDetailView from './ResourceGroupDetailView.vue'

describe('ResourceGroupDetailView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.get.mockResolvedValue({ id: 8, task_id: 9, task_sku_item_id: 19, task_no: 'RW-009', scope_kind: 'sku', sku_code: 'SKU-009', product_name: '北欧客厅组合', creator_name: '李运营', business_lane: 'customization', lock_version: 1, migration_incomplete: false, sku_profile: { id: 31, sku_code: 'SKU-009', product_i_id: 'ERP-009', category_name: '杯具', cost_price: 8.6, cost_trace: { rule_name: '杯具面积规则', matched_rule_version: 2 }, area_trace: { width_m: 0.2, height_m: 0.3, quantity: 2, area_m2: 0.12 } }, finalized_revision: { id: 70, group_id: 8, revision_no: 2, status: 'finalized', mode: 'set', source_stage: 'audit', references: [], items: [{ id: 1, revision_id: 70, task_asset_id: 101, sort_order: 0 }] } })
    mocks.costReconciliation.mockResolvedValue({ product_management_record_id: 31, sku_code: 'SKU-009', system_cost_price: 8.6, erp_cost_price: 9.1, cost_delta: 0.5, status: 'mismatched', checked_at: '2026-08-05T10:00:00Z', message: 'ERP 实际成本与系统计算成本不一致' })
    mocks.batchDownload.mockResolvedValue({ items: [
      { group_id: 8, revision_id: 70, revision_item_id: 2, task_id: 9, sku_code: 'SKU-009', sort_order: 1, filename: '背面.png', download_url: 'https://files/back' },
      { group_id: 8, revision_id: 70, revision_item_id: 1, task_id: 9, sku_code: 'SKU-009', sort_order: 0, filename: '正面.png', download_url: 'https://files/front' },
    ] })
    mocks.downloadBatchAsZip.mockResolvedValue({ writtenCount: 2, failureCount: 0, failures: [] })
  })

  it('uses product and SKU as the title while keeping task number as provenance', async () => {
    const wrapper = mount(ResourceGroupDetailView)
    await flushPromises()
    expect(wrapper.get('h1').text()).toBe('北欧客厅组合')
    expect(wrapper.text()).toContain('SKU-009')
    expect(wrapper.text()).toContain('RW-009')
    expect(wrapper.text()).toContain('套装资源')
    expect(wrapper.text()).toContain('三阶段资源')
    expect(wrapper.text()).toContain('当前 SKU 成本规则试算与解释')
    expect(wrapper.text()).toContain('杯具面积规则')
    expect(wrapper.text()).toContain('系统计算成本')
    expect(wrapper.text()).toContain('ERP 实际成本')
    expect(wrapper.text()).toContain('¥ 9.10')
    expect(wrapper.text()).toContain('比系统成本高 ¥ 0.50')
    await wrapper.findAll('.hero-actions button')[0].trigger('click')
    expect(mocks.push).toHaveBeenCalledWith('/tasks/9')

    await wrapper.findAll('.hero-actions button').find((item) => item.text() === '下载全部成品')?.trigger('click')
    await flushPromises()
    expect(mocks.downloadBatchAsZip).toHaveBeenCalledWith(expect.objectContaining({
      zipFilename: 'SKU-009-套装成品.zip',
      items: [
        expect.objectContaining({ filename: '正面.png', downloadURL: 'https://files/front' }),
        expect.objectContaining({ filename: '背面.png', downloadURL: 'https://files/back' }),
      ],
    }))
  })
})
