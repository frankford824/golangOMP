// @vitest-environment jsdom
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import SkuResourceMatrix from './SkuResourceMatrix.vue'
import type { ResourceBundle } from '@/services/api/resourceGroupsApi'

const bundle: ResourceBundle = {
  task_id: 9,
  workflow_revision: 4,
  groups: [{
    id: 8,
    task_id: 9,
    task_no: 'RW-009',
    scope_kind: 'sku',
    task_sku_item_id: 11,
    sku_code: 'SKU-009',
    product_name: '北欧客厅组合',
    creator_name: '李运营',
    lock_version: 1,
    migration_incomplete: false,
    finalized_revision: {
      id: 70,
      group_id: 8,
      revision_no: 2,
      status: 'finalized',
      mode: 'set',
      source_stage: 'audit',
      created_by: 7,
      legacy_migration: false,
      created_at: '2026-07-22T08:00:00Z',
      source_file: { task_asset_id: 100, file_name: 'living-room.psd', file_size: 10485760, download_url: '/source' },
      references: [{ id: 1, reference_file_ref_id: 2, sort_order: 0, ref_id: 'ref-1', file_name: 'direction.jpg', preview_url: '/reference' }],
      items: [
        { id: 2, revision_id: 70, task_asset_id: 102, sort_order: 1, file: { task_asset_id: 102, file_name: 'detail.png', preview_url: '/detail' } },
        { id: 1, revision_id: 70, task_asset_id: 101, sort_order: 0, file: { task_asset_id: 101, file_name: 'cover.png', preview_url: '/cover' } },
      ],
    },
  }],
}

describe('SkuResourceMatrix', () => {
  it('renders the three business stages and makes set order immediately visible', async () => {
    const wrapper = mount(SkuResourceMatrix, { props: { bundle }, global: { stubs: { Teleport: true } } })
    expect(wrapper.findAll('.stage-card h3').map((item) => item.text())).toEqual(['运营参考图', '当前有效源文件', '最终成品图'])
    expect(wrapper.text()).toContain('审核人员上传的替换源文件')
    expect(wrapper.text()).toContain('套装 · 2 张')
    expect(wrapper.findAll('.final-tile')[0].text()).toContain('封面')
    expect(wrapper.findAll('.final-tile')[0].text()).toContain('cover.png')
    expect(wrapper.text()).toContain('来源任务 · RW-009')
  })

  it('opens a visual preview without exposing workflow revision or group ids', async () => {
    const wrapper = mount(SkuResourceMatrix, { props: { bundle }, global: { stubs: { Teleport: true } } })
    await wrapper.get('.reference-grid button').trigger('click')
    expect(wrapper.get('.preview-layer img').attributes('src')).toBe('/reference')
    expect(wrapper.text()).not.toContain('工作流修订')
    await wrapper.get('.preview-close').trigger('click')
    expect(wrapper.find('.preview-layer').exists()).toBe(false)
  })

  it('replaces missing snapshot images with a compact file fallback', async () => {
    const wrapper = mount(SkuResourceMatrix, { props: { bundle }, global: { stubs: { Teleport: true } } })
    await wrapper.get('.reference-grid img').trigger('error')
    await wrapper.get('.final-gallery img').trigger('error')

    expect(wrapper.find('.reference-grid img').exists()).toBe(false)
    expect(wrapper.get('.reference-grid .tile-fallback').text()).toBe('JPG')
    expect(wrapper.findAll('.final-gallery img')).toHaveLength(1)
    expect(wrapper.get('.final-gallery .tile-fallback').text()).toBe('PNG')
  })

  it('opens revision history only when the task detail enables it', async () => {
    const wrapper = mount(SkuResourceMatrix, {
      props: { bundle, enableRevisionHistory: true },
      global: { stubs: { Teleport: true, ResourceRevisionDrawer: { props: ['group'], template: '<aside class="revision-drawer-stub">历史修订 {{ group.sku_code }}</aside>' } } },
    })
    expect(wrapper.find('.revision-drawer-stub').exists()).toBe(false)
    await wrapper.get('.revision-history-button').trigger('click')
    expect(wrapper.get('.revision-drawer-stub').text()).toContain('SKU-009')
  })
})
