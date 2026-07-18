// @vitest-environment jsdom
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import OrgTreePanel from './OrgTreePanel.vue'
import type { OrgTreeDepartment } from './orgTreeTypes'

const enabledTree: OrgTreeDepartment[] = [
  {
    id: '1',
    value: '设计研发部',
    label: '设计研发部',
    enabled: true,
    memberCount: 12,
    teams: [
      {
        id: '2',
        value: '定制美工组',
        label: '定制美工组',
        department: '设计研发部',
        enabled: true,
        memberCount: 7,
      },
    ],
  },
]

function mountPanel(selectedDepartment = '') {
  return mount(OrgTreePanel, {
    props: {
      enabledTree,
      disabledTree: [],
      selectedDepartment,
      selectedTeam: '',
      showAllEntry: true,
      allActive: !selectedDepartment,
      canManagePolicy: true,
    },
  })
}

describe('OrgTreePanel', () => {
  it('allows a selected department to collapse from the chevron', async () => {
    const wrapper = mountPanel('设计研发部')
    expect(wrapper.text()).toContain('定制美工组')

    await wrapper.find('.org-tree-toggle').trigger('click')

    expect(wrapper.text()).not.toContain('定制美工组')
    expect(wrapper.find('.org-tree-toggle').attributes('aria-expanded')).toBe('false')
  })

  it('toggles collapse when clicking the selected department row again', async () => {
    const wrapper = mountPanel()
    const departmentButton = wrapper.find('.org-filter-item--dept')

    await departmentButton.trigger('click')
    await wrapper.setProps({
      selectedDepartment: '设计研发部',
      allActive: false,
    })
    expect(wrapper.text()).toContain('定制美工组')

    await departmentButton.trigger('click')

    expect(wrapper.text()).not.toContain('定制美工组')
    expect(wrapper.emitted('select-department')?.at(-1)).toEqual(['设计研发部'])
  })

  it('exposes organization policy from both the visible shortcut and context menu', async () => {
    const wrapper = mountPanel()
    const shortcuts = wrapper.findAll('.org-policy-shortcut')

    await shortcuts[0].trigger('click')
    await wrapper.get('.org-tree-toggle').trigger('click')
    await wrapper.get('.org-filter-item--team').trigger('contextmenu')

    expect(wrapper.emitted('manage-policy')).toEqual([
      ['department', 1],
      ['team', 2],
    ])
  })
})
