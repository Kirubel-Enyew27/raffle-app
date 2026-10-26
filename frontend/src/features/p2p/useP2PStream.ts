import { useEffect, useRef, useState } from 'react'

export interface P2PEvent<T = any> {
  type: string
  channel?: string
  payload: T
  created_at?: string
}

export type P2PStreamStatus = 'connecting' | 'live' | 'reconnecting' | 'closed'

interface Options {
  /** Subscribes to `p2p_match:<matchId>` when no explicit channel is given. */
  matchId?: string
  /** Explicit channel, e.g. the shared `p2p_lobby` room. */
  channel?: string
  token: string | null
  enabled: boolean
  onEvent: (event: P2PEvent) => void
  onReconnect?: () => void
}

export function useP2PStream({ matchId, channel, token, enabled, onEvent, onReconnect }: Options) {
  const [status, setStatus] = useState<P2PStreamStatus>('connecting')
  const onEventRef = useRef(onEvent)
  const onReconnectRef = useRef(onReconnect)

  useEffect(() => {
    onEventRef.current = onEvent
    onReconnectRef.current = onReconnect
  })

  useEffect(() => {
    const target = channel ?? (matchId ? `p2p_match:${matchId}` : '')
    if (!enabled || !target || !token) {
      setStatus('closed')
      return
    }

    const baseURL = import.meta.env.VITE_API_URL || '/api/v1'
    const root = baseURL.startsWith('http') ? baseURL : `${window.location.origin}${baseURL}`
    const url = `${root}/realtime/stream?channel=${encodeURIComponent(target)}&token=${encodeURIComponent(token)}`

    let opened = false
    const source = new EventSource(url)

    source.addEventListener('connected', () => {
      if (opened) {
        onReconnectRef.current?.()
      }
      opened = true
      setStatus('live')
    })

    source.addEventListener('message', (event) => {
      try {
        const parsed = JSON.parse((event as MessageEvent).data) as P2PEvent
        if (parsed?.type) {
          onEventRef.current(parsed)
        }
      } catch {
        // Ignore unparseable frames
      }
    })

    source.onerror = () => {
      setStatus('reconnecting')
    }

    return () => {
      source.close()
      setStatus('closed')
    }
  }, [matchId, channel, token, enabled])

  return { status, isLive: status === 'live' }
}
