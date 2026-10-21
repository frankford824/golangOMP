// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import IIdSelector from './IIdSelector.vue'
import { erpApi } from '@/services/api/erpApi'

vi.mock('@/services/api/erpApi', () => ({ erpApi: { getIids: vi.fn() } }))

describe('IIdSelector', () => {
  beforeEach(() => { vi.resetAllMocks(); vi.useFakeTimers() })
  afterEach(() => { vi.useRealTimers(); document.body.innerHTML = '' })

  it('shows server matches by SKU, allows selection, and preserves a value during other searches', async () => {
    vi.mocked(erpApi.getIids).mockResolvedValue({ data: { data: [{ i_id: '定制喷绘布' }] } } as never)
    const wrapper = mount(IIdSelector, { props: { modelValue: '常规喷绘布' }, attachTo: document.body })
    await flushPromises()
    expect(wrapper.get('.base-select-trigger-button').text()).toContain('常规喷绘布')
    await wrapper.get('.base-select-trigger-button').trigger('click')
    const input = document.querySelector('input[type="search"]') as HTMLInputElement
    input.value = 'HQT13217'
    input.dispatchEvent(new Event('input', { bubbles: true }))
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    expect(erpApi.getIids).toHaveBeenLastCalledWith({ q: 'HQT13217', page: 1, page_size: 200 }, expect.any(AbortSignal))
    const option = document.querySelector('[role="listbox"] button') as HTMLButtonElement
    expect(option.textContent).toContain('定制喷绘布')
    option.click()
    await flushPromises()
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual(['定制喷绘布'])
    wrapper.unmount()
  })

  it('separates query failure from an empty result and exposes a working retry', async () => {
    vi.mocked(erpApi.getIids).mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce({ data: { data: [] } } as never)
    const wrapper = mount(IIdSelector, { props: { modelValue: '定制喷绘布' }, attachTo: document.body })
    await flushPromises()
    expect(wrapper.text()).toContain('ERP 款式查询失败')
    expect(wrapper.get('.base-select-trigger-button').text()).toContain('定制喷绘布')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    await wrapper.get('.base-select-trigger-button').trigger('click')
    const retry = document.querySelector('[role="listbox"] button') as HTMLButtonElement
    expect(retry.textContent).toBe('重试查询')
    retry.click()
    await flushPromises()
    expect(wrapper.text()).not.toContain('查询失败')
    expect(wrapper.text()).toContain('未找到已同步的 ERP 款式')
    expect(document.querySelector('[role="listbox"]')).not.toBeNull()
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    wrapper.unmount()
  })
})
