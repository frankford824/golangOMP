// @vitest-environment jsdom

import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

const flags = vi.hoisted(() => ({ oaEnabled: false }))
vi.mock('@/oa/access', () => ({ get oaEnabled() { return flags.oaEnabled } }))

import type { Task } from '@/domain/types/task'
import TaskCard from './TaskCard.vue'

function mountCard(priority: Task['priority'], router?: ReturnType<typeof createRouter>) {
  return mount(TaskCard, {
    global: { plugins: router ? [router] : [] },
    props: {
      task: {
        id: '42',
        taskNo: 'RW-TEST-001',
        priority,
        status: 'PendingAssign',
        taskType: 'NEW_PRODUCT_DEV',
      } as Task,
      selected: false,
      overdue: false,
      claimable: false,
      customization: false,
      title: '测试任务',
      sku: 'CGK001067',
      ownership: '运营部',
      creator: '运营测试账号01',
      designer: '-',
      showDesigner: false,
      categoryLabel: '常规任务',
      categoryKind: 'normal',
      showLaneTag: false,
      isRetouch: false,
      isBatch: false,
      batchCount: 0,
      batchPreview: [],
      hasMoreBatch: false,
      claiming: false,
      claimDisabled: false,
      claimLabel: '接单',
      canCopyNo: true,
      canCopyTitle: true,
      canCopySku: true,
      updatedText: '2026/07/29 22:57',
      dueText: '2026/07/31 20:00',
    },
  })
}

describe('TaskCard', () => {
  it('renders a navigable OA title and separate copy buttons', async () => {
    const router = createRouter({ history: createMemoryHistory(), routes: [
      { path: '/', component: { template: '<div />' } },
      { path: '/tasks/:id', component: { template: '<div />' } },
    ] })
    await router.push('/')
    flags.oaEnabled = true
    const wrapper = mountCard('normal', router)
    try {
      expect(wrapper.get('a.tc-title').attributes('href')).toBe('/tasks/42')
      expect(wrapper.find('.tc-copy-toolbar').exists()).toBe(false)
      await wrapper.get('button[aria-label="复制 SKU"]').trigger('click')
      expect(wrapper.emitted('copy')).toEqual([['CGK001067', 'SKU']])
    } finally {
      wrapper.unmount()
      flags.oaEnabled = false
    }
  })

  it('renders drawing priority prominently and preserves update minutes', () => {
    const wrapper = mountCard('drawing')

    expect(wrapper.get('.tc-priority').text()).toBe('出单画图')
    expect(wrapper.get('.tc-priority').classes()).toContain('tc-priority--drawing')
    expect(wrapper.get('.tc-footer').text()).toContain('更新 2026/07/29 22:57')
  })

  it('renders normal priority without using the urgent tone', () => {
    const wrapper = mountCard('normal')

    expect(wrapper.get('.tc-priority').text()).toBe('普通')
    expect(wrapper.get('.tc-priority').classes()).toContain('tc-priority--normal')
  })
})
