// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest'
import { consumeSSE } from './aiChatApi'

describe('consumeSSE', () => {
  it('decodes split events, joined data lines, and ignores heartbeats', async () => {
    const encoder = new TextEncoder()
    const chunks = [
      'event: meta\ndata: {"conversation_id":"c1",',
      '"user_message_id":"u1","assistant_message_id":"a1"}\n\n: heartbeat\n\nevent: delta\ndata: {"text":"你',
      '好"}\n\nevent: done\ndata: {"message":{"id":"a1","conversation_id":"c1","role":"assistant","content":"你好","status":"completed","created_at":"2026-07-20T00:00:00Z","updated_at":"2026-07-20T00:00:00Z"}}\n\n',
    ]
    const stream = new ReadableStream<Uint8Array>({
      start(controller) {
        chunks.forEach((item) => controller.enqueue(encoder.encode(item)))
        controller.close()
      },
    })
    const handler = vi.fn()
    await consumeSSE(stream, handler)
    expect(handler).toHaveBeenCalledTimes(3)
    expect(handler.mock.calls[0]?.[0]).toMatchObject({ type: 'meta', data: { conversation_id: 'c1' } })
    expect(handler.mock.calls[1]?.[0]).toEqual({ type: 'delta', data: { text: '你好' } })
    expect(handler.mock.calls[2]?.[0]).toMatchObject({ type: 'done', data: { message: { status: 'completed' } } })
  })

  it('turns malformed data into a safe user-facing stream error', async () => {
    const stream = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(new TextEncoder().encode('event: delta\ndata: not-json\n\n'))
        controller.close()
      },
    })
    const events: unknown[] = []
    await consumeSSE(stream, (event) => events.push(event))
    expect(events).toEqual([{ type: 'error', data: { code: 'invalid_stream_event', message: '返回内容格式异常，请稍后重试。' } }])
  })
})
