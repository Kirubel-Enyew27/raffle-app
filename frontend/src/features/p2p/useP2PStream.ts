import { useEffect, useRef, useState } from 'react'
import { sseClient } from '@/lib/sseClient'
import type { SSEStatus } from '@/lib/sseClient'

export interface P2PEvent<T = unknown> {
  type: string
  channel?: string
  payload: T
  created_at?: string
}

export type P2PStreamStatus = SSEStatus

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
  // Track whether we've seen a 'live' event before so we can fire onReconnect.
  const everLiveRef = useRef(false)

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

    let lastStatus: P2PStreamStatus = 'connecting'
    setStatus('connecting')

    const unsub = sseClient.subscribe(target, token, (ev) => {
      // Forward game events.
      onEventRef.current(ev as P2PEvent)

      // Track live/reconnect transitions via connection events if the server
      // publishes them as SSE events (type: 'connected').
      if (ev.type === 'connected') {
        if (everLiveRef.current) {
          onReconnectRef.current?.()
        }
        everLiveRef.current = true
        lastStatus = 'live'
        setStatus('live')
      }
    })

    // Poll the client status because EventSource status changes are not
    // surfaced as events on the handler — they come from the source itself.
    // We attach a small interval to mirror sseClient status into React state.
    const interval = setInterval(() => {
      const s = sseClient.getStatus(target)
      if (s !== lastStatus) {
        if (s === 'live' && lastStatus === 'reconnecting') {
          onReconnectRef.current?.()
        }
        lastStatus = s
        setStatus(s)
      }
    }, 200)

    return () => {
      clearInterval(interval)
      unsub()
      setStatus('closed')
    }
  }, [matchId, channel, token, enabled])

  return { status, isLive: status === 'live' }
}
