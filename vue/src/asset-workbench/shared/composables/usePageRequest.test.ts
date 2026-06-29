// @vitest-environment jsdom
import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import { usePageRequest } from './usePageRequest'

describe('usePageRequest', () => {
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
