import { onBeforeUnmount } from 'vue'
import { V1SocketClient, type V1WsEventDetail } from '@/services/ws/v1Socket'

export interface UseWebSocketOptions {
  onMessage?: (event: V1WsEventDetail) => void
  onFallbackPoll?: () => void
}

export function useWebSocket(options: UseWebSocketOptions = {}): {
  disconnect: () => void
} {
  const client =
    options.onMessage
      ? new V1SocketClient({
          onMessage: options.onMessage,
          onFallbackPoll: options.onFallbackPoll,
        })
      : null

  if (client) client.connect()

  const disconnect = (): void => {
    client?.disconnect()
  }

  onBeforeUnmount(disconnect)
  return { disconnect }
}
