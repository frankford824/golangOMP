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

  it('ends the active editor before the parent validates the latest cell value', async () => {
    vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
      callback(0)
      return 1
    })
    vi.stubGlobal('matchMedia', vi.fn(() => ({
      matches: true,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })))
    const input = document.createElement('input')
    document.body.appendChild(input)
    input.focus()
    const wrapper = mount(UnifiedTaskGrid, {
      props: {
        intent: 'new_design',
        rows: [createComposeRow({ design_requirement: '末尾粘贴编码 CGP000346' })],
      },
    })

    await wrapper.vm.flushRowsFromWorkbook()

    expect(document.activeElement).not.toBe(input)
    input.remove()
    wrapper.unmount()
  })
})
