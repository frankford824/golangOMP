// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ list: vi.fn(), push: vi.fn() }))
vi.mock('@/services/api/resourceGroupsApi', async (loadOriginal) => {
  const original = await loadOriginal<typeof import('@/services/api/resourceGroupsApi')>()
  return { ...original, resourceGroupsApi: { ...original.resourceGroupsApi, list: mocks.list } }
})
vi.mock('vue-router', () => ({ useRouter: () => ({ push: mocks.push }) }))

import ResourceGroupsView from './ResourceGroupsView.vue'

const group = {
  id: 8,
  task_id: 3,
  task_no: 'RW-008',
  scope_kind: 'sku' as const,
  task_sku_item_id: 11,
  sku_code: 'SKU-008',
  business_lane: 'customization',
  lock_version: 1,
  migration_incomplete: false,
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
    const wrapper = mount(ResourceGroupsView)
    await flushPromises()
    const inputs = wrapper.findAll('.filters input')
    await inputs[0].setValue('RW-008')
    await inputs[1].setValue('keyword')
    await inputs[2].setValue('SKU-008')
    await inputs[3].setValue('12')
    const selects = wrapper.findAll('.filters select')
    await selects[0].setValue('customization')
    await selects[1].setValue('final')
    await selects[2].setValue('design')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(mocks.list).toHaveBeenLastCalledWith({
      q: 'keyword',
      sku_code: 'SKU-008',
      task_no: 'RW-008',
      creator_id: 12,
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
      creator_id: 12,
      business_lane: 'customization',
      resource_role: 'final',
      format_category: 'design',
      page: 2,
    }))
  })

  it('shows a single SKU cover summary card and navigates on click', async () => {
    const wrapper = mount(ResourceGroupsView)
    await flushPromises()
    expect(wrapper.get('.cover img').attributes('src')).toBe('https://img/front.png')
    await wrapper.get('.cover img').trigger('error')
    expect(wrapper.find('.cover img').exists()).toBe(false)
    expect(wrapper.get('.preview-fallback').text()).toBe('FR')
    expect(wrapper.text()).toContain('套装 · 1 张成品')

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
    const wrapper = mount(ResourceGroupsView)
    await flushPromises()
    expect(wrapper.find('.flat-grid').exists()).toBe(true)
    expect(wrapper.text()).toContain('参考图')
    expect(wrapper.get('.flat-card img').attributes('src')).toBe('https://img/ref.png')
    await wrapper.get('.flat-card').trigger('click')
    expect(mocks.push).toHaveBeenCalledWith('/asset-center/8')
  })

  it('renders distinct empty and retryable error states', async () => {
    mocks.list.mockResolvedValueOnce({ items: [], flat_items: [], view_mode: 'group', page: 1, page_size: 24, total: 0 })
    const empty = mount(ResourceGroupsView)
    await flushPromises()
    expect(empty.text()).toContain('没有找到符合条件')

    mocks.list.mockRejectedValueOnce(new Error('资源服务暂不可用'))
    const failed = mount(ResourceGroupsView)
    await flushPromises()
    expect(failed.get('[role="alert"]').text()).toContain('资源服务暂不可用')
    expect(failed.get('[role="alert"] button').text()).toBe('重试')
  })
})
