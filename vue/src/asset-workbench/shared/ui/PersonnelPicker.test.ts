// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { assetWorkbenchApi, type PersonnelSuggestion } from '@aw/shared/api/assetWorkbenchApi'
import PersonnelPicker from './PersonnelPicker.vue'

const person: PersonnelSuggestion = {
  user_id: 42,
  worker_type: 'parttime',
  job_grade: 'J2',
  name: '李明',
  username: 'qa-person',
  personnel_no: 'QA-42',
  status: 'active',
}

describe('PersonnelPicker', () => {
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('updates matches while typing and emits the explicitly selected user', async () => {
    vi.useFakeTimers()
    const suggest = vi.spyOn(assetWorkbenchApi, 'suggestPersonnel').mockResolvedValue({ items: [person], total: 1 })
    const wrapper = mount(PersonnelPicker, { props: { modelValue: 0, label: '补录人员' } })

    await wrapper.get('button').trigger('click')
    await flushPromises()
    await wrapper.get('input[type="search"]').setValue('QA-42')
    await vi.advanceTimersByTimeAsync(220)
    await flushPromises()
    expect(suggest).toHaveBeenLastCalledWith('payee', 'QA-42', expect.any(AbortSignal))
    expect(wrapper.get('[role="option"]').text()).toContain('人员编号 QA-42')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    await wrapper.get('[role="option"]').trigger('click')

    expect(wrapper.emitted('update:modelValue')).toEqual([[42]])
    expect(wrapper.emitted('selected')).toEqual([[person]])
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('keeps the more recent query results when an older request finishes later', async () => {
    vi.useFakeTimers()
    let finishFirst: ((value: { items: PersonnelSuggestion[]; total: number }) => void) | undefined
    const suggest = vi.spyOn(assetWorkbenchApi, 'suggestPersonnel').mockImplementation(async (_scope, query) => {
      if (query === '李') return new Promise(resolve => { finishFirst = resolve })
      return { items: [], total: 0 }
    })
    const wrapper = mount(PersonnelPicker, { props: { modelValue: 0, label: '扣款人员' } })

    await wrapper.get('button').trigger('click')
    await flushPromises()
    const input = wrapper.get('input[type="search"]')
    await input.setValue('李')
    await vi.advanceTimersByTimeAsync(220)
    const firstSignal = suggest.mock.lastCall?.[2]
    await input.setValue('不存在的人')
    expect(firstSignal?.aborted).toBe(true)
    await vi.advanceTimersByTimeAsync(220)
    await flushPromises()
    finishFirst?.({ items: [person], total: 1 })
    await flushPromises()

    expect(suggest).toHaveBeenLastCalledWith('payee', '不存在的人', expect.any(AbortSignal))
    expect(wrapper.text()).toContain('没有找到匹配人员')
    expect(wrapper.find('[role="option"]').exists()).toBe(false)
    wrapper.unmount()
  })
})
