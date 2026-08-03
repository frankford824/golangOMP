// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ get: vi.fn(), batchDownload: vi.fn(), push: vi.fn() }))
vi.mock('vue-router', () => ({ useRoute: () => ({ params: { id: '8' } }), useRouter: () => ({ push: mocks.push }) }))
vi.mock('@/services/api/resourceGroupsApi', async (loadOriginal) => {
  const original = await loadOriginal<typeof import('@/services/api/resourceGroupsApi')>()
  return { ...original, resourceGroupsApi: { ...original.resourceGroupsApi, get: mocks.get, batchDownload: mocks.batchDownload } }
})
vi.mock('@/components/task/SkuResourceMatrix.vue', () => ({ default: { props: ['bundle'], template: '<div class="matrix-stub">三阶段资源</div>' } }))

import ResourceGroupDetailView from './ResourceGroupDetailView.vue'

describe('ResourceGroupDetailView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.get.mockResolvedValue({ id: 8, task_id: 9, task_sku_item_id: 19, task_no: 'RW-009', scope_kind: 'sku', sku_code: 'SKU-009', product_name: '北欧客厅组合', creator_name: '李运营', business_lane: 'customization', lock_version: 1, migration_incomplete: false, sku_profile: { sku_code: 'SKU-009', product_i_id: 'ERP-009', category_name: '杯具', cost_price: 8.6, cost_trace: { rule_name: '杯具面积规则', matched_rule_version: 2 }, area_trace: { width_m: 0.2, height_m: 0.3, quantity: 2, area_m2: 0.12 } }, finalized_revision: { id: 70, group_id: 8, revision_no: 2, status: 'finalized', mode: 'set', source_stage: 'audit', references: [], items: [{ id: 1, revision_id: 70, task_asset_id: 101, sort_order: 0 }] } })
    mocks.batchDownload.mockResolvedValue({ items: [] })
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
    await wrapper.findAll('.hero-actions button')[0].trigger('click')
    expect(mocks.push).toHaveBeenCalledWith('/tasks/9')
  })
})
