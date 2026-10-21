// @vitest-environment jsdom
import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import { usePageRequest } from './usePageRequest'

describe('usePageRequest', () => {
  it('ignores late responses after a newer request or reset', async () => {
    const responses: Array<(value: string) => void> = []
    const Host = defineComponent({ setup() { return usePageRequest(() => new Promise<string>((resolve) => responses.push(resolve))) }, template: '<div />' })
    const wrapper = mount(Host)
    const older = wrapper.vm.run()
    const newer = wrapper.vm.run()
    responses[1]!('new')
    await newer
    responses[0]!('old')
    await older
    expect(wrapper.vm.data).toBe('new')
    const last = wrapper.vm.run()
    wrapper.vm.reset()
    responses[2]!('private old data')
    await last
    expect(wrapper.vm.data).toBeNull()
    wrapper.unmount()
  })
  it('loads data and clears the loading state', async () => {
    const Host = defineComponent({
      setup() {
        return usePageRequest(async () => 'ready')
      },
      template: '<button type="button" @click="run">{{ loading ? "loading" : data }}</button>',
    })

    const wrapper = mount(Host)
    await wrapper.find('button').trigger('click')

    expect(wrapper.text()).toBe('ready')
    expect(wrapper.vm.loading).toBe(false)
    expect(wrapper.vm.error).toBe('')
  })

  it('keeps previous data and exposes request errors', async () => {
    const Host = defineComponent({
      setup() {
        return usePageRequest<string>(async () => {
          throw new Error('加载失败')
        }, 'cached')
      },
      template: '<button type="button" @click="run">{{ error || data }}</button>',
    })

    const wrapper = mount(Host)
    await wrapper.find('button').trigger('click')

    expect(wrapper.text()).toBe('加载失败')
    expect(wrapper.vm.data).toBe('cached')
  })
})
