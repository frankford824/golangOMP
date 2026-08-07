// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  query: vi.fn(),
  push: vi.fn(),
}))

vi.mock('@/services/api/searchApi', () => ({
  searchApi: { query: mocks.query },
}))
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: mocks.push }),
}))
vi.mock('@/composables/usePermission', () => ({
  usePermission: () => ({ can: () => true }),
}))

import GlobalSearchOverlay from './GlobalSearchOverlay.vue'

describe('GlobalSearchOverlay performance states', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
    Object.defineProperty(navigator, 'onLine', { configurable: true, value: true })
    Object.defineProperty(navigator, 'connection', {
      configurable: true,
      value: {
        effectiveType: '4g',
        saveData: false,
        rtt: 50,
        downlink: 10,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      },
    })
    mocks.query.mockResolvedValue({
      data: {
        query: 'CGK001543',
        results: {
          tasks: [{ id: 3711, task_no: 'RW-20260806-A-003711', title: '医师节常规KT板' }],
          assets: [],
          products: [],
          users: [],
        },
      },
    })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('avoids one-character scans and requests only the preview budget', async () => {
    const wrapper = mount(GlobalSearchOverlay, {
      props: { open: true },
      global: { stubs: { Teleport: true } },
    })
    const input = wrapper.get('input[placeholder="搜索任务、资产、产品、用户"]')

    await input.setValue('医')
    await vi.advanceTimersByTimeAsync(1000)
    expect(mocks.query).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('请输入至少 2 个字')

    await input.setValue('CGK001543')
    await vi.advanceTimersByTimeAsync(219)
    expect(mocks.query).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(1)
    await flushPromises()

    expect(mocks.query).toHaveBeenCalledWith(
      { keyword: 'CGK001543', scope: 'all', limit: 6 },
      expect.any(AbortSignal),
    )
    expect(wrapper.text()).toContain('RW-20260806-A-003711')
    expect(input.attributes('aria-busy')).toBe('false')
  })

  it('uses the constrained-network debounce and exposes failures instead of empty results', async () => {
    Object.defineProperty(navigator, 'connection', {
      configurable: true,
      value: {
        effectiveType: '3g',
        saveData: true,
        rtt: 700,
        downlink: 0.8,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      },
    })
    mocks.query.mockRejectedValueOnce(new Error('timeout'))
    const wrapper = mount(GlobalSearchOverlay, {
      props: { open: true },
      global: { stubs: { Teleport: true } },
    })
    await wrapper.get('input[placeholder="搜索任务、资产、产品、用户"]').setValue('医师节')

    await vi.advanceTimersByTimeAsync(479)
    expect(mocks.query).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(1)
    await flushPromises()

    expect(wrapper.text()).toContain('搜索暂时不可用')
    expect(wrapper.text()).toContain('当前网络较慢，已启用省流量模式')
    expect(wrapper.text()).not.toContain('没有找到匹配内容')
  })

  it('labels retained results while a new weak-network query is updating', async () => {
    Object.defineProperty(navigator, 'connection', {
      configurable: true,
      value: {
        effectiveType: '3g',
        saveData: false,
        rtt: 450,
        downlink: 0.4,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      },
    })
    const wrapper = mount(GlobalSearchOverlay, {
      props: { open: true },
      global: { stubs: { Teleport: true } },
    })
    const input = wrapper.get('input[placeholder="搜索任务、资产、产品、用户"]')

    await input.setValue('医师节')
    await vi.advanceTimersByTimeAsync(480)
    await flushPromises()
    expect(wrapper.text()).toContain('RW-20260806-A-003711')

    mocks.query.mockImplementationOnce(() => new Promise(() => undefined))
    await input.setValue('CGK001543')
    await vi.advanceTimersByTimeAsync(480)

    expect(wrapper.text()).toContain('正在更新“CGK001543”的结果，当前保留上一轮结果。')
    expect(wrapper.text()).toContain('当前网络较慢，已启用省流量模式。')
  })
})
