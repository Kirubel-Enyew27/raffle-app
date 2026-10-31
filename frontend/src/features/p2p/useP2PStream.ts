import { useCallback, useEffect, useRef, useState } from 'react'
import { sseClient } from '@/lib/sseClient'
import type { SSEStatus } from '@/lib/sseClient'

export interface P2PEvent<T = unknown> {
  type: string
  channel?: string
  payload: T
  created_at?: string
  server_delivery_latency_ms?: number
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
  const target = channel ?? (matchId ? `p2p_match:${matchId}` : '')

  // Seed from the real channel status so an already-live channel (remount,
  // navigating back) never looks stuck on "Connecting…".
  const [status, setStatus] = useState<P2PStreamStatus>(() =>
    enabled && target && token ? sseClient.getStatus(target) : 'closed',
  )
  // Mirror of what React is currently showing, so the poll loop below compares
  // against React's value instead of a value captured at an earlier moment.
  const statusRef = useRef<P2PStreamStatus>(status)

  const onEventRef = useRef(onEvent)
  const onReconnectRef = useRef(onReconnect)

  useEffect(() => {
    onEventRef.current = onEvent
    onReconnectRef.current = onReconnect
  })

  useEffect(() => {
    if (!enabled || !target || !token) return

    let lastStatus: P2PStreamStatus = statusRef.current
    // The SSE client transitions reconnecting → connecting → live, so a
    // 'reconnecting' observation is latched and consumed on the next 'live'.
    let sawReconnecting = lastStatus === 'reconnecting'

    const unsub = sseClient.subscribe(target, token, (ev) => {
      onEventRef.current(ev as P2PEvent)
    })

    const interval = setInterval(() => {
      const s = sseClient.getStatus(target)
      if (s === lastStatus) return

      if (s === 'reconnecting' || s === 'failed') sawReconnecting = true
      if (s === 'live' && sawReconnecting) {
        // Resync authoritative state after any outage, including one that only
        // reached 'failed' and was revived by a retry.
        sawReconnecting = false
        onReconnectRef.current?.()
      }
      lastStatus = s
      statusRef.current = s
      setStatus(s)
    }, 150)

    return () => {
      clearInterval(interval)
      unsub()
    }
  }, [target, token, enabled])

  /** Force an immediate reconnect (used by the "Retry connection" action). */
  const retry = useCallback(() => {
    if (!target) return
    sseClient.retryNow(target)
    const next = sseClient.getStatus(target)
    statusRef.current = next
    setStatus(next)
  }, [target])

  const derived: P2PStreamStatus = !enabled || !target || !token ? 'closed' : status

  return { status: derived, isLive: derived === 'live', retry }
}
