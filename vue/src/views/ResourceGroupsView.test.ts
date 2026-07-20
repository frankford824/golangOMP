// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ list: vi.fn(), push: vi.fn() }))
vi.mock('@/services/api/resourceGroupsApi', async (loadOriginal) => {
  const original = await loadOriginal<typeof import('@/services/api/resourceGroupsApi')>()
  return { ...original, resourceGroupsApi: { ...original.resourceGroupsApi, list: mocks.list } }
})
vi.mock('vue-router', () => ({ useRouter: () => ({ push: mocks.push }) }))
vi.mock('@/composables/useTaskFilterOptions', async () => {
  const { ref } = await import('vue')
  return { useTaskFilterOptions: () => ({ creatorOptions: ref([{ value: '', label: '全部' }, { value: '12', label: '李运营' }]) }) }
})

import ResourceGroupsView from './ResourceGroupsView.vue'

const mountView = () => mount(ResourceGroupsView, { global: { stubs: { Teleport: true } } })

const group = {
  id: 8,
  task_id: 3,
  task_no: 'RW-008',
  scope_kind: 'sku' as const,
  task_sku_item_id: 11,
  sku_code: 'SKU-008',
  business_lane: 'customization',
  product_name: '北欧沙发组合',
  creator_id: 12,
  creator_name: '李运营',
  lock_version: 1,
  migration_incomplete: false,
  sku_profile: {
    id: 91,
    task_id: 3,
    task_sku_item_id: 11,
    sku_code: 'SKU-008',
    product_i_id: 'STYLE-88',
    combo_sku_codes: ['COMBO-01'],
    cost_price: 18.5,
    spec_text: '80 × 120 cm',
    area_trace: { area_m2: 0.96 },
    cost_trace: { rule_name: '喷绘面积规则', matched_rule_version: 3 },
    erp_sync_status: 'synced',
  },
  finalized_revision: {
    id: 70,
    group_id: 8,
    revision_no: 2,
    status: 'finalized' as const,
    mode: 'set' as const,
    source_stage: 'audit' as const,
    source_file: null,
    items: [{ id: 701, revision_id: 70, task_asset_id: 1001, sort_order: 1, file: { task_asset_id: 1001, file_name: 'front.png', preview_url: 'https://img/front.png' } }],
    references: [],
  },
}

describe('ResourceGroupsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.list.mockResolvedValue({ items: [group], view_mode: 'group', flat_items: [], page: 1, page_size: 24, total: 48 })
  })

  it('keeps all filters while paging and opens the numeric resource-group route', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('.search-field input').setValue('keyword')
    await wrapper.get('.filter-button').trigger('click')
    const inputs = wrapper.findAll('.filter-drawer input')
    await inputs[0].setValue('SKU-008')
    await inputs[1].setValue('RW-008')
    const selects = wrapper.findAll('.filter-drawer select')
    await selects[0].setValue('final')
    await selects[1].setValue('design')
    await selects[2].setValue('customization')
    await selects[3].setValue('12')
    await wrapper.get('.filter-drawer form').trigger('submit')
    await flushPromises()

    expect(mocks.list).toHaveBeenLastCalledWith({
      q: 'keyword',
      sku_code: 'SKU-008',
      task_no: 'RW-008',
      creator_id: '12',
      business_lane: 'customization',
      resource_role: 'final',
      format_category: 'design',
      page: 1,
      page_size: 24,
    })
    const next = wrapper.findAll('.pager button').find((button) => button.text() === '下一页')
    await next?.trigger('click')
    await flushPromises()
    expect(mocks.list).toHaveBeenLastCalledWith(expect.objectContaining({
      q: 'keyword',
      sku_code: 'SKU-008',
      task_no: 'RW-008',
      creator_id: '12',
      business_lane: 'customization',
      resource_role: 'final',
      format_category: 'design',
      page: 2,
    }))
  })

  it('shows a single SKU cover summary card and navigates on click', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('.cover img').attributes('src')).toBe('https://img/front.png')
    await wrapper.get('.cover img').trigger('error')
    expect(wrapper.find('.cover img').exists()).toBe(false)
    expect(wrapper.get('.preview-fallback').text()).toContain('PNG')
    expect(wrapper.text()).toContain('北欧沙发组合')
    expect(wrapper.text()).toContain('任务 RW-008')
    expect(wrapper.text()).toContain('套装')
    expect(wrapper.text()).toContain('1 张成品')
    expect(wrapper.text()).toContain('0.960 ㎡')
    expect(wrapper.text()).toContain('¥ 18.50')
    expect(wrapper.text()).toContain('COMBO-01')
    expect(wrapper.text()).toContain('成本规则 · 喷绘面积规则（第 3 版）')
    expect(wrapper.text()).toContain('ERP 已同步')

    const card = wrapper.get('.resource-card')
    expect(card.element.tagName).toBe('BUTTON')
    await card.trigger('click')
    expect(mocks.push).toHaveBeenCalledWith('/asset-center/8')
  })

  it('switches to flat resource grid when the API returns flat view_mode', async () => {
    mocks.list.mockResolvedValueOnce({
      items: [],
      view_mode: 'flat',
      flat_items: [{
        group_id: 8,
        task_id: 3,
        task_no: 'RW-008',
        sku_code: 'SKU-008',
        resource_role: 'reference',
        file_name: 'ref.png',
        preview_url: 'https://img/ref.png',
      }],
      page: 1,
      page_size: 24,
      total: 1,
    })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('.flat-grid').exists()).toBe(true)
    expect(wrapper.text()).toContain('参考图')
    expect(wrapper.get('.flat-card img').attributes('src')).toBe('https://img/ref.png')
    await wrapper.get('.flat-card').trigger('click')
    expect(mocks.push).toHaveBeenCalledWith('/asset-center/8')
  })

  it('renders distinct empty and retryable error states', async () => {
    mocks.list.mockResolvedValueOnce({ items: [], flat_items: [], view_mode: 'group', page: 1, page_size: 24, total: 0 })
    const empty = mountView()
    await flushPromises()
    expect(empty.text()).toContain('没有找到符合条件')

    mocks.list.mockRejectedValueOnce(new Error('资源服务暂不可用'))
    const failed = mountView()
    await flushPromises()
    expect(failed.get('[role="alert"]').text()).toContain('资源服务暂不可用')
    expect(failed.get('[role="alert"] button').text()).toBe('重试')
  })
})
