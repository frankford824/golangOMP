// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  config: vi.fn(),
  list: vi.fn(),
  create: vi.fn(),
  get: vi.fn(),
  remove: vi.fn(),
  streamMessage: vi.fn(),
  adminList: vi.fn(),
  adminGet: vi.fn(),
  replace: vi.fn(),
  push: vi.fn(),
}))

vi.mock('@/services/api/aiChatApi', async (loadOriginal) => {
  const original = await loadOriginal<typeof import('@/services/api/aiChatApi')>()
  return {
    ...original,
    aiChatApi: {
      config: mocks.config,
      list: mocks.list,
      create: mocks.create,
      get: mocks.get,
      remove: mocks.remove,
      streamMessage: mocks.streamMessage,
      adminList: mocks.adminList,
      adminGet: mocks.adminGet,
    },
  }
})
vi.mock('vue-router', () => ({
  useRoute: () => ({ path: '/data-center', query: {} }),
  useRouter: () => ({ replace: mocks.replace, push: mocks.push }),
}))

import DataCenterView from './DataCenterView.vue'

const now = '2026-07-20T00:00:00Z'

describe('DataCenterView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.config.mockResolvedValue({ enabled: true, hybrid_search_enabled: true, max_input_chars: 4000, retention_days: 90, max_concurrent_user: 2, can_review_all: false })
    mocks.list.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 60, total_pages: 0 })
    mocks.create.mockResolvedValue({ id: 'conversation-1', owner_user_id: 8, title: '交付问题', status: 'active', lock_version: 0, expires_at: now, created_at: now, updated_at: now, messages: [] })
    mocks.adminList.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 60, total_pages: 0 })
    mocks.streamMessage.mockImplementation(async (_id, _request, emit) => {
      emit({ type: 'meta', data: { conversation_id: 'conversation-1', user_message_id: 'user-1', assistant_message_id: 'assistant-1' } })
      emit({ type: 'status', data: { stage: 'retrieving', label: '正在查找相关业务数据' } })
      emit({ type: 'retrieval', data: { meta: { mode: 'hybrid', degraded: false, candidates: 2 }, sources: [{ source_id: 'S1', entity_type: 'task', entity_id: '42', title: '任务 42', internal_route: '/tasks/42', evidence_excerpt: '审核打回一次', rank: 1 }] } })
      emit({ type: 'delta', data: { text: '审核打回是主要影响环节。' } })
      emit({ type: 'done', data: { message: { id: 'assistant-1', conversation_id: 'conversation-1', role: 'assistant', content: '审核打回是主要影响环节。', status: 'completed', created_at: now, updated_at: now, sources: [{ source_id: 'S1', entity_type: 'task', entity_id: '42', title: '任务 42', internal_route: '/tasks/42', evidence_excerpt: '审核打回一次', rank: 1 }] } } })
    })
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('creates a conversation, streams an answer, and opens server-issued evidence', async () => {
    const wrapper = mount(DataCenterView, { attachTo: document.body })
    await flushPromises()
    expect(wrapper.text()).toContain('从业务问题开始，而不是从报表开始')

    await wrapper.get('#ai-chat-question').setValue('过去一周哪个环节影响交付？')
    await wrapper.get('.chat-composer').trigger('submit')
    await flushPromises()

    expect(mocks.create).toHaveBeenCalledWith('过去一周哪个环节影响交付？')
    expect(mocks.streamMessage).toHaveBeenCalledWith(
      'conversation-1',
      expect.objectContaining({ content: '过去一周哪个环节影响交付？' }),
      expect.any(Function),
      expect.any(AbortSignal),
    )
    expect(wrapper.text()).toContain('审核打回是主要影响环节。')
    expect(wrapper.text()).toContain('检索依据')
    await wrapper.get('.source-link').trigger('click')
    expect(mocks.push).toHaveBeenCalledWith('/tasks/42')
    wrapper.unmount()
  })

  it('shows permission and service errors as business messages', async () => {
    mocks.config.mockRejectedValueOnce(Object.assign(new Error('forbidden'), { status: 403 }))
    const wrapper = mount(DataCenterView)
    await flushPromises()
    expect(wrapper.text()).toContain('当前账号没有数据分析权限')
    expect(wrapper.text()).not.toContain('403')
  })

  it.each([
    [401, '登录状态已失效'],
    [429, '当前生成任务较多'],
    [503, '分析服务正在维护'],
  ])('turns HTTP %s into a non-technical recovery message', async (status, expected) => {
    mocks.config.mockRejectedValueOnce(Object.assign(new Error('technical error'), { status }))
    const wrapper = mount(DataCenterView)
    await flushPromises()
    expect(wrapper.text()).toContain(expected)
    expect(wrapper.text()).not.toContain(String(status))
  })

  it('aborts the active fetch when the user stops generation', async () => {
    let streamSignal: AbortSignal | undefined
    mocks.streamMessage.mockImplementationOnce(async (_id, _request, _emit, signal: AbortSignal) => {
      streamSignal = signal
      await new Promise<void>((_resolve, reject) => {
        signal.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError')), { once: true })
      })
    })
    const wrapper = mount(DataCenterView)
    await flushPromises()
    await wrapper.get('#ai-chat-question').setValue('停止这个分析')
    await wrapper.get('.chat-composer').trigger('submit')
    await flushPromises()
    expect(wrapper.get('.stop-button').text()).toContain('停止生成')
    await wrapper.get('.stop-button').trigger('click')
    await flushPromises()
    expect(streamSignal?.aborted).toBe(true)
    expect(wrapper.text()).toContain('生成已停止')
  })

  it('opens the mobile history drawer and the audited admin review dialog', async () => {
    mocks.config.mockResolvedValueOnce({ enabled: true, hybrid_search_enabled: true, max_input_chars: 4000, retention_days: 90, max_concurrent_user: 2, can_review_all: true })
    const wrapper = mount(DataCenterView, { attachTo: document.body })
    await flushPromises()
    const historyButton = wrapper.get<HTMLButtonElement>('.history-button')
    historyButton.element.focus()
    await historyButton.trigger('click')
    await flushPromises()
    const historyDrawer = document.body.querySelector<HTMLElement>('.history-drawer')
    expect(historyDrawer).not.toBeNull()
    expect(document.activeElement?.getAttribute('aria-label')).toBe('关闭历史对话')
    historyDrawer?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await flushPromises()
    expect(document.body.querySelector('.history-drawer')).toBeNull()
    expect(document.activeElement).toBe(historyButton.element)

    await historyButton.trigger('click')
    await flushPromises()
    const reviewButton = document.body.querySelector<HTMLButtonElement>('.history-drawer .review-button')
    expect(reviewButton).not.toBeNull()
    reviewButton?.click()
    await flushPromises()
    const drawer = document.body.querySelector<HTMLElement>('.review-drawer')
    expect(drawer?.textContent).toContain('跨用户阅读会记录审计')
    expect(document.activeElement?.getAttribute('aria-label')).toBe('关闭对话审阅')
    drawer?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await flushPromises()
    expect(document.body.querySelector('.review-drawer')).toBeNull()
    wrapper.unmount()
  })

  it('renders the disabled state without hiding the rest of the application promise', async () => {
    mocks.config.mockResolvedValueOnce({ enabled: false, hybrid_search_enabled: false, max_input_chars: 4000, retention_days: 90, max_concurrent_user: 2, can_review_all: false })
    const wrapper = mount(DataCenterView)
    await flushPromises()
    expect(wrapper.text()).toContain('数据助手尚未启用')
    expect(wrapper.text()).toContain('任务、资产和报表仍可正常使用')
    expect(wrapper.find('.chat-composer').exists()).toBe(false)
  })
})
