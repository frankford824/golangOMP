// @vitest-environment jsdom

import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import type { Task } from '@/domain/types/task'
import TaskCard from './TaskCard.vue'

function mountCard(priority: Task['priority']) {
  return mount(TaskCard, {
    props: {
      task: {
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
  it('renders critical priority prominently and preserves update minutes', () => {
    const wrapper = mountCard('critical')

    expect(wrapper.get('.tc-priority').text()).toBe('加急')
    expect(wrapper.get('.tc-priority').classes()).toContain('tc-priority--critical')
    expect(wrapper.get('.tc-footer').text()).toContain('更新 2026/07/29 22:57')
  })

  it('renders normal priority without using the urgent tone', () => {
    const wrapper = mountCard('normal')

    expect(wrapper.get('.tc-priority').text()).toBe('普通')
    expect(wrapper.get('.tc-priority').classes()).toContain('tc-priority--normal')
  })
})
