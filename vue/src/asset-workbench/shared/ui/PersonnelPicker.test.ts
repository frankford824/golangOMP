// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { assetWorkbenchApi, type AssetWorkbenchProfile } from '@aw/shared/api/assetWorkbenchApi'
import PersonnelPicker from './PersonnelPicker.vue'

const profile: AssetWorkbenchProfile = {
  id: 7,
  user_id: 42,
  worker_type: 'parttime',
  job_grade: 'J2',
  real_name: '李明',
  status: 'active',
  pii_completed: true,
}

describe('PersonnelPicker', () => {
  afterEach(() => vi.restoreAllMocks())

  it('searches personnel and emits the selected personnel code', async () => {
    const listProfiles = vi.spyOn(assetWorkbenchApi, 'listProfiles').mockResolvedValue({ items: [profile], total: 1 })
    const wrapper = mount(PersonnelPicker, { props: { modelValue: 0, label: '补录人员' } })

    await wrapper.get('button').trigger('click')
    await flushPromises()

    expect(wrapper.get('[role="dialog"]').text()).toContain('李明')
    await wrapper.get('input[type="search"]').setValue('42')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(listProfiles).toHaveBeenLastCalledWith({ page: 1, page_size: 50, user_id: 42 })
    await wrapper.get('[role="option"]').trigger('click')

    expect(wrapper.emitted('update:modelValue')).toEqual([[42]])
    expect(wrapper.emitted('selected')).toEqual([[profile]])
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
  })

  it('searches by name and shows a useful empty state', async () => {
    const listProfiles = vi.spyOn(assetWorkbenchApi, 'listProfiles')
      .mockResolvedValueOnce({ items: [profile], total: 1 })
      .mockResolvedValueOnce({ items: [], total: 0 })
    const wrapper = mount(PersonnelPicker, { props: { modelValue: 0, label: '开放人员' } })

    await wrapper.get('button').trigger('click')
    await flushPromises()
    await wrapper.get('input[type="search"]').setValue('不存在的人')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(listProfiles).toHaveBeenLastCalledWith({ page: 1, page_size: 50, q: '不存在的人' })
    expect(wrapper.text()).toContain('没有找到匹配人员')
  })
})
