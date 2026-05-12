import { getToken } from '@/services/http'

export type V1WsEventType =
  | 'task_pool_count_changed'
  | 'my_task_updated'
  | 'notification_arrived'

export interface V1WsEventDetail {
  type: V1WsEventType
  payload: Record<string, unknown>
}

export interface V1SocketOptions {
  onMessage: (event: V1WsEventDetail) => void
  onFallbackPoll?: () => void
}

const MAX_RECONNECT_DELAY_MS = 30_000
const FALLBACK_POLL_INTERVAL_MS = 15_000

function resolveWsUrl(): string {
  const token = getToken()
  const explicit = String(import.meta.env.VITE_WS_BASE_URL ?? '').trim()
  const base = explicit || `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}`
  const url = new URL('/ws/v1', base)
  if (token) url.searchParams.set('access_token', token)
  return url.toString()
}

export class V1SocketClient {
  private socket: WebSocket | null = null
  private reconnectTimer: number | undefined
  private fallbackTimer: number | undefined
  private reconnectDelay = 1_000
  private consecutiveFailures = 0
  private manuallyClosed = false

  constructor(private readonly options: V1SocketOptions) {}

  connect(): void {
    this.manuallyClosed = false
    this.clearReconnectTimer()

    try {
      this.socket = new WebSocket(resolveWsUrl())
    } catch {
      this.scheduleReconnect()
      return
    }

    this.socket.addEventListener('open', () => {
      this.consecutiveFailures = 0
      this.reconnectDelay = 1_000
      this.stopFallbackPolling()
    })

    this.socket.addEventListener('message', (event) => {
      const parsed = parseWsMessage(event.data)
      if (parsed) this.options.onMessage(parsed)
    })

    this.socket.addEventListener('error', () => {
      this.consecutiveFailures += 1
      if (this.consecutiveFailures >= 3) this.startFallbackPolling()
    })

    this.socket.addEventListener('close', () => {
      if (!this.manuallyClosed) this.scheduleReconnect()
    })
  }

  disconnect(): void {
    this.manuallyClosed = true
    this.clearReconnectTimer()
    this.stopFallbackPolling()
    this.socket?.close()
    this.socket = null
  }

  private scheduleReconnect(): void {
    this.clearReconnectTimer()
    this.consecutiveFailures += 1
    if (this.consecutiveFailures >= 3) this.startFallbackPolling()
    const delay = this.reconnectDelay
    this.reconnectDelay = Math.min(this.reconnectDelay * 2, MAX_RECONNECT_DELAY_MS)
    this.reconnectTimer = window.setTimeout(() => this.connect(), delay)
  }

  private clearReconnectTimer(): void {
    if (this.reconnectTimer === undefined) return
    window.clearTimeout(this.reconnectTimer)
    this.reconnectTimer = undefined
  }

  private startFallbackPolling(): void {
    if (this.fallbackTimer !== undefined || !this.options.onFallbackPoll) return
    this.options.onFallbackPoll()
    this.fallbackTimer = window.setInterval(
      () => this.options.onFallbackPoll?.(),
      FALLBACK_POLL_INTERVAL_MS,
    )
  }

  private stopFallbackPolling(): void {
    if (this.fallbackTimer === undefined) return
    window.clearInterval(this.fallbackTimer)
    this.fallbackTimer = undefined
  }
}

function parseWsMessage(data: unknown): V1WsEventDetail | null {
  if (typeof data !== 'string') return null
  try {
    const parsed = JSON.parse(data) as unknown
    if (!parsed || typeof parsed !== 'object') return null
    const event = parsed as Partial<V1WsEventDetail>
    if (
      event.type !== 'task_pool_count_changed' &&
      event.type !== 'my_task_updated' &&
      event.type !== 'notification_arrived'
    ) {
      return null
    }
    return {
      type: event.type,
      payload:
        event.payload && typeof event.payload === 'object'
          ? (event.payload as Record<string, unknown>)
          : {},
    }
  } catch {
    return null
  }
}
