// @vitest-environment jsdom
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import TaskResourceRail from './TaskResourceRail.vue'
import type { ResourceBundle, ResourceRevision } from '@/services/api/resourceGroupsApi'

vi.mock('@/components/media/AssetPreviewMedia.vue', () => ({
  default: {
    props: ['taskAssetId', 'fallbackSrc', 'alt'],
    template: '<img class="asset-preview-media-stub" :data-task-asset-id="taskAssetId || undefined" :src="fallbackSrc || undefined" :alt="alt" />',
  },
}))

function revision(id: number, references: ResourceRevision['references']): ResourceRevision {
  return {
    id,
    group_id: id,
    revision_no: 1,
    status: 'finalized',
    mode: 'single',
    source_stage: 'retouch',
    created_by: 1,
    legacy_migration: true,
    items: [],
    references,
    created_at: '2026-07-24T00:00:00Z',
  }
}

const bundle: ResourceBundle = {
  task_id: 1264,
  workflow_revision: 2,
  groups: [
    {
      id: 45,
      task_id: 1264,
      scope_kind: 'retouch_requirement',
      retouch_requirement_id: 45,
      lock_version: 1,
      migration_incomplete: false,
      finalized_revision: revision(45, [
        { id: 2, reference_file_ref_id: 1312, formal_task_asset_id: 202, ref_id: 'ref-1312', sort_order: 1, file_name: '第二张.jpg', preview_url: '/second.jpg' },
        { id: 1, reference_file_ref_id: 1311, formal_task_asset_id: 201, ref_id: 'ref-1311', sort_order: 0, file_name: '第一张.jpg', preview_url: '/first.jpg' },
      ]),
    },
    {
      id: 46,
      task_id: 1264,
      scope_kind: 'retouch_requirement',
      retouch_requirement_id: 46,
      lock_version: 1,
      migration_incomplete: false,
      working_revision: revision(46, [
        { id: 3, reference_file_ref_id: 1312, ref_id: 'ref-1312', sort_order: 0, file_name: '重复引用.jpg', preview_url: '/duplicate.jpg' },
      ]),
    },
  ],
}

describe('TaskResourceRail', () => {
  it('uses current scoped revision references with stable order and de-duplication', () => {
    const wrapper = mount(TaskResourceRail, { props: { bundle, taskStatus: 'Completed', taskType: 'retouch_task' } })

    expect(wrapper.get('.rail-column.references .column-head small').text()).toBe('2 个附件')
    expect(wrapper.findAll('.rail-column.references .media-strip button').map((item) => item.text())).toEqual([
      '第一张.jpg',
      '第二张.jpg',
    ])
  })

  it('opens the authoritative resource workspace for scoped references', async () => {
    const wrapper = mount(TaskResourceRail, { props: { bundle, taskStatus: 'Completed', taskType: 'retouch_task' } })

    await wrapper.get('.rail-column.references .column-head button').trigger('click')
    await wrapper.get('.rail-column.references .media-strip button').trigger('click')

    expect(wrapper.emitted('openResources')).toHaveLength(2)
    expect(wrapper.emitted('openAttachments')).toBeUndefined()
  })

  it('loads formal reference thumbnails through the controlled task-asset path', () => {
    const wrapper = mount(TaskResourceRail, { props: { bundle, taskStatus: 'Completed', taskType: 'retouch_task' } })

    expect(wrapper.findAll('.asset-preview-media-stub').map((item) => item.attributes('data-task-asset-id'))).toEqual(['201', '202'])
  })

  it('explains that retouch source files are optional instead of reporting missing SKU submissions', () => {
    const wrapper = mount(TaskResourceRail, {
      props: {
        bundle,
        taskStatus: 'Completed',
        taskType: 'retouch_task',
        canOperate: true,
      },
    })

    expect(wrapper.get('.rail-column.sources .column-head small').text()).toBe('0 个源文件 · 2 个修图范围')
    expect(wrapper.get('.rail-column.sources .empty-copy').text()).toBe('修图任务无需独立源文件，以参考图与最终成品为准')
    expect(wrapper.get('.resource-rail > footer').text()).toContain('按修图范围提交最终成品；独立源文件可选')
  })

  it('keeps SKU submission copy for design tasks', () => {
    const wrapper = mount(TaskResourceRail, { props: { bundle, taskStatus: 'InProgress', taskType: 'original_product_development' } })

    expect(wrapper.get('.rail-column.sources .column-head small').text()).toBe('0 / 2 个 SKU 已提交')
    expect(wrapper.get('.rail-column.sources .empty-copy').text()).toBe('设计人员尚未提交源文件')
  })
})
