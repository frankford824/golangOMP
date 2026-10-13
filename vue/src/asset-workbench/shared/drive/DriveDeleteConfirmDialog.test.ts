// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import DriveDeleteConfirmDialog from './DriveDeleteConfirmDialog.vue'

describe('DriveDeleteConfirmDialog', () => {
  it('shows cross-page expansion and emits no confirmation on cancel', async () => {
    const wrapper = mount(DriveDeleteConfirmDialog, { attachTo: document.body, props: {
      selection: { fileIds: Array.from({ length: 53 }, (_, index) => index + 1), workCount: 1, extraCount: 3, reason: '错传' }, busy: false,
    }, global: { stubs: { DialogPortal: { template: '<slot />' } } } })
    await flushPromises()
    expect(wrapper.get('[role="dialog"]').text()).toContain('将删除 53 个文件，共 1 件作品')
    expect(wrapper.text()).toContain('已补齐 3 个同作品文件')
    expect(wrapper.text()).toContain('筛选结果之外')
    await wrapper.findAll('button').find((button) => button.text() === '取消')!.trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(1)
    expect(wrapper.emitted('confirm')).toBeUndefined()
    await wrapper.findAll('button').find((button) => button.text() === '确认删除 53 个文件')!.trigger('click')
    expect(wrapper.emitted('confirm')).toHaveLength(1)
    const element = wrapper.element
    wrapper.unmount(); element?.remove()
  })
})
