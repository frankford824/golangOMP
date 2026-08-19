// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ list: vi.fn(), costReconciliation: vi.fn(), batchSearchAssets: vi.fn(), push: vi.fn() }))
vi.mock('@/services/api/resourceGroupsApi', async (loadOriginal) => {
  const original = await loadOriginal<typeof import('@/services/api/resourceGroupsApi')>()
  return { ...original, resourceGroupsApi: { ...original.resourceGroupsApi, list: mocks.list, costReconciliation: mocks.costReconciliation } }
})
vi.mock('@/services/api/assetsApi', async (loadOriginal) => {
  const original = await loadOriginal<typeof import('@/services/api/assetsApi')>()
  return { ...original, assetsApi: { ...original.assetsApi, batchSearchAssets: mocks.batchSearchAssets } }
})
vi.mock('vue-router', () => ({ useRouter: () => ({ push: mocks.push }) }))
vi.mock('@/composables/useTaskFilterOptions', async () => {
  const { ref } = await import('vue')
  return { useTaskFilterOptions: () => ({ creatorOptions: ref([{ value: '', label: '全部' }, { value: '12', label: '李运营' }]) }) }
})

import ResourceGroupsView from './ResourceGroupsView.vue'

const mountView = () => mount(ResourceGroupsView, {
  global: {
    stubs: {
      Teleport: true,
      AssetPreviewMedia: {
        props: ['assetId', 'taskAssetId', 'fallbackSrc', 'alt'],
        template: '<img class="asset-preview-media-stub" :data-asset-id="assetId || undefined" :data-task-asset-id="taskAssetId || undefined" :src="fallbackSrc || undefined" :alt="alt" />',
      },
    },
  },
})

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
    mocks.costReconciliation.mockResolvedValue({
      product_management_record_id: 91,
      sku_code: 'SKU-008',
      system_cost_price: 18.5,
      erp_cost_price: 20,
      cost_delta: 1.5,
      status: 'mismatched',
      checked_at: '2026-08-05T10:00:00Z',
      message: 'ERP 成本与系统计算成本不一致',
    })
    mocks.batchSearchAssets.mockResolvedValue({
      data: { data: { results: [], matched_count: 0, failed_count: 1 } },
    })
  })

  it('hides the production packaging entry from the asset-center header', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-page-header="asset-center"]').text()).not.toContain('生产打包')
    expect(wrapper.findComponent({ name: 'ProductionPackageDialog' }).exists()).toBe(false)
    expect(wrapper.get('.page-actions').text()).toContain('刷新')
  })

  it('keeps all filters while paging and opens the numeric resource-group route', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('.search-field input').setValue('keyword')
    await wrapper.get('.filter-button').trigger('click')
    const inputs = wrapper.findAll('.filter-drawer input')
    await inputs[0].setValue('2026-08-01')
    await inputs[1].setValue('2026-08-05')
    const selects = wrapper.findAll('.filter-drawer select')
    await selects[0].setValue('final')
    await selects[1].setValue('tif')
    await selects[2].setValue('12')
    await wrapper.get('.filter-drawer form').trigger('submit')
    await flushPromises()

    expect(mocks.list).toHaveBeenLastCalledWith({
      q: 'keyword',
      resource_role: 'final',
      file_format: 'tif',
      created_from: '2026-08-01',
      created_to: '2026-08-05',
      resource_owner_id: '12',
      page: 1,
      page_size: 24,
    })
    const next = wrapper.findAll('.pager button').find((button) => button.text() === '下一页')
    await next?.trigger('click')
    await flushPromises()
    expect(mocks.list).toHaveBeenLastCalledWith(expect.objectContaining({
      q: 'keyword',
      resource_role: 'final',
      file_format: 'tif',
      resource_owner_id: '12',
      page: 2,
    }))
  })

  it('submits an explicit asset search control used by narrow layouts', async () => {
    const wrapper = mountView()
    await flushPromises()

    const searchButton = wrapper.get('.search-row .primary-button')
    expect(searchButton.element.tagName).toBe('BUTTON')
    expect(searchButton.text()).toBe('搜索')

    await wrapper.get('.search-field input').setValue('CGK001500')
    await wrapper.get('.search-row').trigger('submit')
    await flushPromises()

    expect(mocks.list).toHaveBeenLastCalledWith(expect.objectContaining({
      q: undefined,
      sku_code: 'CGK001500',
      page: 1,
    }))
    expect(mocks.costReconciliation).toHaveBeenCalledWith(8)
    expect(wrapper.text()).toContain('ERP 实际成本')
    expect(wrapper.text()).toContain('¥ 20.00')
    expect(wrapper.get('.cost-mismatch').text()).toBe('¥ 20.00')
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

  it('resolves protected cover previews by immutable task-asset id', async () => {
    const protectedGroup = structuredClone(group)
    protectedGroup.finalized_revision.items[0].file.preview_url = '/v1/task-assets/1001/preview'
    mocks.list.mockResolvedValueOnce({ items: [protectedGroup], view_mode: 'group', flat_items: [], page: 1, page_size: 24, total: 1 })

    const wrapper = mountView()
    await flushPromises()

    const preview = wrapper.get('.asset-preview-media-stub')
    expect(preview.attributes('data-task-asset-id')).toBe('1001')
    expect(preview.attributes('data-asset-id')).toBeUndefined()
  })

  it('shows exact external SKU matches in the same asset center and opens their read-only detail', async () => {
    mocks.list.mockResolvedValueOnce({ items: [], view_mode: 'group', flat_items: [], page: 1, page_size: 24, total: 0 })
    mocks.batchSearchAssets.mockResolvedValueOnce({
      data: {
        data: {
          results: [{
            term: 'HSC40222',
            status: 'matched',
            message: '已匹配',
            candidates: 1,
            assets: [{
              id: '77',
              resource_id: 'ext-77',
              source_type: 'external',
              source_label: '外部资源',
              file_role: 'delivery',
              sku_code: 'HSC40222',
              file_name: 'HSC40222-final.tif',
              oss_sync_status: 'ready',
            }],
          }],
          matched_count: 1,
          failed_count: 0,
        },
      },
    })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('.search-field input').setValue('HSC40222')
    await wrapper.get('.search-row').trigger('submit')
    await flushPromises()

    expect(mocks.batchSearchAssets).toHaveBeenLastCalledWith({
      terms: ['HSC40222'],
      format_filter: 'all',
      asset_kind: 'all',
    })
    expect(wrapper.text()).toContain('外部资源匹配')
    expect(wrapper.text()).toContain('HSC40222-final.tif')
    await wrapper.get('.external-card').trigger('click')
    expect(mocks.push).toHaveBeenCalledWith('/asset-center/ext-77')
  })

  it('switches to flat resource grid when the API returns flat view_mode', async () => {
    mocks.list.mockResolvedValueOnce({
      items: [],
      view_mode: 'flat',
      flat_items: [{
        group_id: 8,
        task_id: 3,
        task_no: 'RW-008',
        task_type: 'new_product_development',
        sku_code: 'SKU-008',
        resource_role: 'reference',
        file_name: 'ref.png',
        resource_owner_id: 12,
        resource_owner_name: '李运营',
        resource_created_at: '2026-08-05T10:00:00Z',
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
