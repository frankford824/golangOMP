// @vitest-environment jsdom
import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
const mocks = vi.hoisted(() => ({ get: vi.fn(), delete: vi.fn(), post: vi.fn(), resolve: vi.fn() }))
vi.mock('@/services/http', () => ({ default: { get: mocks.get, delete: mocks.delete, post: mocks.post } }))
vi.mock('@/services/mediaDelivery', () => ({ resolveMediaAccess: mocks.resolve, mediaMessage: () => '仍在准备中' }))
import MediaPreparationPanel from './MediaPreparationPanel.vue'

describe('durable preparation panel', () => {
  beforeEach(() => { vi.clearAllMocks() })
  const row = (state: string) => ({ request_id: 'mine', cancelled: false, access_reference: { resource_kind: 'external_asset', resource_id: 'ext-1', purpose: 'download', rendition: 'original' }, job: { job_id: 'job', resource_id: 'ext-1', state, phase: 'uploading', processed_bytes: 50, total_bytes: 100, retryable: true } })

  it('restores owned preparation and clears it on account change', async () => {
    mocks.get.mockResolvedValue({ data: { data: [row('processing')] } })
    const wrapper = mount(MediaPreparationPanel)
    await wrapper.get('button').trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('准备中')
    expect(wrapper.text()).toContain('50 / 100 字节')
    window.dispatchEvent(new Event('asset-media-session-reset')); await flushPromises()
    expect(wrapper.text()).not.toContain('ext-1')
    wrapper.unmount()
  })

  it('requires a user click, hands off natively and never claims disk completion', async () => {
    mocks.get.mockResolvedValue({ data: { data: [row('succeeded')] } })
    mocks.resolve.mockResolvedValue({ state: 'ready', download_url: 'https://files.example.com/original', filename: '原件.psd' })
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    const wrapper = mount(MediaPreparationPanel)
    await wrapper.get('button').trigger('click'); await flushPromises()
    expect(click).not.toHaveBeenCalled()
    const download = wrapper.findAll('button').find((button) => button.text() === '获取链接并下载')!
    await download.trigger('click'); await flushPromises()
    expect(click).toHaveBeenCalledOnce()
    expect(wrapper.text()).toContain('已交给浏览器')
    expect(wrapper.text()).not.toContain('已保存完成')
    wrapper.unmount(); click.mockRestore()
  })
})
