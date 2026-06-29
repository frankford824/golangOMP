// @vitest-environment jsdom
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AsyncBoundary from './AsyncBoundary.vue'

describe('AsyncBoundary', () => {
  it('renders loading, error, empty, and default states', async () => {
    const wrapper = mount(AsyncBoundary, {
      props: { loading: true, loadingLabel: '加载中' },
      slots: { default: '<section>内容</section>' },
    })

    expect(wrapper.text()).toContain('加载中')

    await wrapper.setProps({ loading: false, error: '出错了' })
    expect(wrapper.text()).toContain('出错了')
    await wrapper.find('button').trigger('click')
    expect(wrapper.emitted('retry')).toHaveLength(1)

    await wrapper.setProps({ error: '', empty: true, emptyLabel: '没有内容' })
    expect(wrapper.text()).toContain('没有内容')

    await wrapper.setProps({ empty: false })
    expect(wrapper.text()).toContain('内容')
  })
})
