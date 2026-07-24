// @vitest-environment jsdom
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import TaskResourceRail from './TaskResourceRail.vue'
import type { ResourceBundle, ResourceRevision } from '@/services/api/resourceGroupsApi'

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
        { id: 2, reference_file_ref_id: 1312, ref_id: 'ref-1312', sort_order: 1, file_name: '第二张.jpg', preview_url: '/second.jpg' },
        { id: 1, reference_file_ref_id: 1311, ref_id: 'ref-1311', sort_order: 0, file_name: '第一张.jpg', preview_url: '/first.jpg' },
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
    const wrapper = mount(TaskResourceRail, { props: { bundle, taskStatus: 'Completed' } })

    expect(wrapper.get('.rail-column.references .column-head small').text()).toBe('2 个附件')
    expect(wrapper.findAll('.rail-column.references .media-strip button').map((item) => item.text())).toEqual([
      '第一张.jpg',
      '第二张.jpg',
    ])
  })

  it('opens the authoritative resource workspace for scoped references', async () => {
    const wrapper = mount(TaskResourceRail, { props: { bundle, taskStatus: 'Completed' } })

    await wrapper.get('.rail-column.references .column-head button').trigger('click')
    await wrapper.get('.rail-column.references .media-strip button').trigger('click')

    expect(wrapper.emitted('openResources')).toHaveLength(2)
    expect(wrapper.emitted('openAttachments')).toBeUndefined()
  })
})
