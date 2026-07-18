// @vitest-environment jsdom
import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { createComposeRow } from '@/domain/unified-task-compose'
import UnifiedTaskGrid from './UnifiedTaskGrid.vue'

describe('UnifiedTaskGrid responsive authority', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('keeps the mobile row model authoritative instead of booting a hidden workbook', async () => {
    const media = {
      matches: true,
      media: '(max-width: 760px)',
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }
    vi.stubGlobal('matchMedia', vi.fn(() => media))
    const wrapper = mount(UnifiedTaskGrid, {
      props: {
        intent: 'planning_sku',
        rows: [createComposeRow({ description_spec: '移动端保留的内容', quantity: 2 })],
      },
    })
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('卡片填写')
    expect(wrapper.find('.compose-grid__canvas-shell').exists()).toBe(false)
    expect(wrapper.emitted('update:rows')).toBeUndefined()
    wrapper.unmount()
  })
})
