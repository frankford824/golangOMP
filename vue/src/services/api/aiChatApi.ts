import http, { getToken, type HttpAppError } from '@/services/http'
import { isMockEnabled } from '@/mocks'

export type AIMessageStatus = 'streaming' | 'completed' | 'cancelled' | 'failed'

export interface AIChatConfig {
  enabled: boolean
  hybrid_search_enabled: boolean
  max_input_chars: number
  retention_days: number
  max_concurrent_user: number
  can_review_all: boolean
}

export interface AIMessageSource {
  id?: number
  message_id?: string
  source_id: string
  entity_type: string
  entity_id: string
  title: string
  internal_route?: string
  evidence_excerpt: string
  source_version?: string
  rank: number
}

export interface AIMessage {
  id: string
  conversation_id: string
  reply_to_message_id?: string
  client_message_id?: string
  role: 'user' | 'assistant'
  content: string
  status: AIMessageStatus
  finish_reason?: string
  error_code?: string
  created_at: string
  updated_at: string
  sources?: AIMessageSource[]
}

export interface AIConversation {
  id: string
  owner_user_id: number
  owner_name?: string
  title: string
  status: 'active' | 'deleted'
  lock_version: number
  expires_at: string
  created_at: string
  updated_at: string
  messages?: AIMessage[]
}

export interface AIConversationList {
  items: AIConversation[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export interface AIRetrievalMeta {
  mode: 'exact' | 'hybrid' | string
  degraded: boolean
  candidates: number
  reason?: string
}

export type AIStreamEvent =
  | { type: 'meta'; data: { conversation_id: string; user_message_id: string; assistant_message_id: string; replayed?: boolean } }
  | { type: 'status'; data: { stage: string; label: string } }
  | { type: 'retrieval'; data: { meta: AIRetrievalMeta; sources: AIMessageSource[] } }
  | { type: 'delta'; data: { text: string } }
  | { type: 'done'; data: { message: AIMessage; finish_reason?: string; replayed?: boolean } }
  | { type: 'error'; data: { code: string; message: string } }

export interface AIAdminConversationFilter {
  owner_user_id?: number
  status?: string
  from?: string
  to?: string
  page?: number
  page_size?: number
}

function envelope<T>(response: { data?: { data?: T } }): T {
  return response.data?.data as T
}

function decodeEvent(type: string, raw: string): AIStreamEvent | null {
  if (!type || !raw) return null
  try {
    return { type, data: JSON.parse(raw) } as AIStreamEvent
  } catch {
    return { type: 'error', data: { code: 'invalid_stream_event', message: '返回内容格式异常，请稍后重试。' } }
  }
}

export async function consumeSSE(
  stream: ReadableStream<Uint8Array>,
  onEvent: (event: AIStreamEvent) => void,
): Promise<void> {
  const reader = stream.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  while (true) {
    const { value, done } = await reader.read()
    buffer += decoder.decode(value, { stream: !done }).replace(/\r\n/g, '\n')
    let boundary = buffer.indexOf('\n\n')
    while (boundary >= 0) {
      const block = buffer.slice(0, boundary)
      buffer = buffer.slice(boundary + 2)
      let eventType = ''
      const dataLines: string[] = []
      for (const line of block.split('\n')) {
        if (line.startsWith(':')) continue
        if (line.startsWith('event:')) eventType = line.slice(6).trim()
        if (line.startsWith('data:')) dataLines.push(line.slice(5).trimStart())
      }
      const event = decodeEvent(eventType, dataLines.join('\n'))
      if (event) onEvent(event)
      boundary = buffer.indexOf('\n\n')
    }
    if (done) break
  }
}

async function mockStream(
  conversationID: string,
  content: string,
  onEvent: (event: AIStreamEvent) => void,
  signal?: AbortSignal,
) {
  const sources: AIMessageSource[] = [
    { source_id: 'S1', entity_type: 'task', entity_id: '1001', title: '任务 YB-20260718-001', internal_route: '/tasks/1001', evidence_excerpt: '审核打回与任务完成事件显示，需求澄清是近期返工的主要集中点。', source_version: '7', rank: 1 },
    { source_id: 'S2', entity_type: 'task_resource_group', entity_id: '7001', title: 'SKU YB-A102 资源组', internal_route: '/asset-center/7001', evidence_excerpt: '该资源组包含一份当前源文件与三张最终成品图。', source_version: '12', rank: 2 },
  ]
  const id = `assistant-${Date.now()}`
  onEvent({ type: 'meta', data: { conversation_id: conversationID, user_message_id: `user-${Date.now()}`, assistant_message_id: id } })
  onEvent({ type: 'status', data: { stage: 'retrieving', label: '正在查找相关业务数据' } })
  await new Promise((resolve) => setTimeout(resolve, 80))
  if (signal?.aborted) throw new DOMException('Aborted', 'AbortError')
  onEvent({ type: 'retrieval', data: { meta: { mode: 'hybrid', degraded: false, candidates: 18 }, sources } })
  onEvent({ type: 'status', data: { stage: 'generating', label: '正在整理分析结论' } })
  const answer = `从当前可访问的任务与资源数据看，“${content.slice(0, 20)}”主要涉及两个环节：\n\n1. 审核打回集中在需求说明不完整的任务，建议在创建时补齐交付标准。\n2. 套装资源组的成品顺序需要在设计提交时一次确认，减少审核阶段重复调整。\n\n以上结论来自当前检索到的任务事件与资源组记录。`
  for (const part of answer.match(/.{1,18}/gs) ?? []) {
    if (signal?.aborted) throw new DOMException('Aborted', 'AbortError')
    onEvent({ type: 'delta', data: { text: part } })
    await new Promise((resolve) => setTimeout(resolve, 18))
  }
  const now = new Date().toISOString()
  onEvent({ type: 'done', data: { message: { id, conversation_id: conversationID, role: 'assistant', content: answer, status: 'completed', created_at: now, updated_at: now, sources } } })
}

export const aiChatApi = {
  async config(signal?: AbortSignal): Promise<AIChatConfig> {
    if (isMockEnabled()) {
      return { enabled: true, hybrid_search_enabled: true, max_input_chars: 4000, retention_days: 90, max_concurrent_user: 2, can_review_all: true }
    }
    return envelope<AIChatConfig>(await http.get('/v1/ai/chat/config', { signal }))
  },
  async list(page = 1, pageSize = 30, signal?: AbortSignal): Promise<AIConversationList> {
    if (isMockEnabled()) return { items: mockConversationStore, total: mockConversationStore.length, page, page_size: pageSize, total_pages: 1 }
    return envelope<AIConversationList>(await http.get('/v1/ai/chat/conversations', { params: { page, page_size: pageSize }, signal }))
  },
  async create(title = ''): Promise<AIConversation> {
    if (isMockEnabled()) {
      const now = new Date().toISOString()
      const item: AIConversation = { id: crypto.randomUUID(), owner_user_id: 1, title, status: 'active', lock_version: 0, expires_at: new Date(Date.now() + 90 * 86400000).toISOString(), created_at: now, updated_at: now, messages: [] }
      mockConversationStore.unshift(item)
      return item
    }
    return envelope<AIConversation>(await http.post('/v1/ai/chat/conversations', { title }))
  },
  async get(id: string, signal?: AbortSignal): Promise<AIConversation> {
    if (isMockEnabled()) return mockConversationStore.find((item) => item.id === id) ?? mockConversationStore[0]
    return envelope<AIConversation>(await http.get(`/v1/ai/chat/conversations/${encodeURIComponent(id)}`, { signal }))
  },
  async remove(id: string): Promise<void> {
    if (isMockEnabled()) {
      const index = mockConversationStore.findIndex((item) => item.id === id)
      if (index >= 0) mockConversationStore.splice(index, 1)
      return
    }
    await http.delete(`/v1/ai/chat/conversations/${encodeURIComponent(id)}`)
  },
  async adminList(filter: AIAdminConversationFilter, signal?: AbortSignal): Promise<AIConversationList> {
    if (isMockEnabled()) return { items: mockConversationStore, total: mockConversationStore.length, page: 1, page_size: 30, total_pages: 1 }
    return envelope<AIConversationList>(await http.get('/v1/ai/chat/admin/conversations', { params: filter, signal }))
  },
  async adminGet(id: string, signal?: AbortSignal): Promise<AIConversation> {
    if (isMockEnabled()) return mockConversationStore.find((item) => item.id === id) ?? mockConversationStore[0]
    return envelope<AIConversation>(await http.get(`/v1/ai/chat/admin/conversations/${encodeURIComponent(id)}`, { signal }))
  },
  async streamMessage(
    conversationID: string,
    request: { client_message_id: string; content: string },
    onEvent: (event: AIStreamEvent) => void,
    signal?: AbortSignal,
  ): Promise<void> {
    if (isMockEnabled()) return mockStream(conversationID, request.content, onEvent, signal)
    const token = getToken()
    const response = await fetch(`/v1/ai/chat/conversations/${encodeURIComponent(conversationID)}/messages:stream`, {
      method: 'POST',
      signal,
      headers: {
        'Content-Type': 'application/json',
        Accept: 'text/event-stream',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        'X-Frontend-Version': import.meta.env.VITE_APP_VERSION ?? 'v1',
      },
      body: JSON.stringify(request),
    })
    if (!response.ok) {
      let message = `请求失败（${response.status}）`
      let code = ''
      try {
        const payload = await response.json() as { error?: { message?: string; code?: string } }
        message = payload.error?.message || message
        code = payload.error?.code || ''
      } catch {
        // Keep the status-based fallback when an upstream proxy returns HTML.
      }
      const error = new Error(message) as HttpAppError
      error.status = response.status
      error.code = code
      throw error
    }
    if (!response.body) throw new Error('服务器未返回可读取的分析内容。')
    await consumeSSE(response.body, onEvent)
  },
}

const now = new Date().toISOString()
const mockConversationStore: AIConversation[] = [
  { id: 'mock-conversation-1', owner_user_id: 1, title: '交付瓶颈分析', status: 'active', lock_version: 2, expires_at: new Date(Date.now() + 90 * 86400000).toISOString(), created_at: now, updated_at: now, messages: [] },
  { id: 'mock-conversation-2', owner_user_id: 1, title: '资源复用情况', status: 'active', lock_version: 1, expires_at: new Date(Date.now() + 90 * 86400000).toISOString(), created_at: now, updated_at: now, messages: [] },
]
