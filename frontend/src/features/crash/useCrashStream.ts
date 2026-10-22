import { useEffect, useRef, useState } from 'react'
import type { P2PWinner } from './api'

/**
 * Payload carried by a crash-game realtime event. Every field is optional
 * because each event type publishes a different subset; unknown extra fields
 * are ignored rather than trusted.
 */
export interface CrashEventPayload {
  round_id?: string
  round_number?: number
  state?: string
  server_time?: string
  multiplier?: number
  multiplier_bp?: number
  elapsed_ms?: number
  crash_multiplier?: number
  crash_multiplier_bp?: number
  betting_starts_at?: string
  betting_ends_at?: string
  started_at?: string
  crashed_at?: string
  server_seed?: string
  server_seed_hash?: string
  commitment_published_at?: string
  total_stake?: number
  total_payout?: number
  bet_count?: number
  cashout_count?: number
  user_id?: string
  display_name?: string
  stake?: number
  payout?: number | null
  // P2P fields
  p2p?: boolean
  prize_pool?: number
  service_fee_pct?: number
  total_fee?: number
  winner_count?: number
  winners?: P2PWinner[]
  // Bet placed
  total_prize_pool?: number
  min_participants?: number
}

/** A realtime event as delivered on the shared crash-game channel. */
export interface CrashEvent {
  type: string
  channel?: string
  payload: CrashEventPayload
  created_at?: string
}

export type CrashStreamStatus = 'connecting' | 'live' | 'reconnecting' | 'closed'

interface Options {
  /** Turns the connection on/off (for example while signed out). */
  enabled: boolean
  /** Bearer token used to authenticate the stream. */
  token: string | null
  /** Called for every game event. */
  onEvent: (event: CrashEvent) => void
  /** Called after a dropped connection is restored, to resync authoritative state. */
  onReconnect?: () => void
}

/**
 * Subscribes to the shared crash game room over the project's existing SSE
 * endpoint (`/realtime/stream`).
 */
export function useCrashStream({ enabled, token, onEvent, onReconnect }: Options) {
  const [connection, setConnection] = useState<{ key: string; status: CrashStreamStatus }>({
    key: '',
    status: 'connecting',
  })

  const onEventRef = useRef(onEvent)
  const onReconnectRef = useRef(onReconnect)
  useEffect(() => {
    onEventRef.current = onEvent
    onReconnectRef.current = onReconnect
  })

  const key = enabled && token ? token : ''

  useEffect(() => {
    if (!key) return

    const baseURL = import.meta.env.VITE_API_URL || '/api/v1'
    const root = baseURL.startsWith('http') ? baseURL : `${window.location.origin}${baseURL}`
    const url = `${root}/realtime/stream?channel=crash&token=${encodeURIComponent(key)}`

    const source = new EventSource(url)
    let opened = false

    source.addEventListener('connected', () => {
      if (opened) {
        onReconnectRef.current?.()
      }
      opened = true
      setConnection({ key, status: 'live' })
    })

    source.addEventListener('message', (event) => {
      try {
        const parsed = JSON.parse((event as MessageEvent).data) as CrashEvent
        if (parsed?.type) onEventRef.current(parsed)
      } catch {
        // A malformed frame must never break the stream.
      }
    })

    source.onerror = () => {
      setConnection({ key, status: 'reconnecting' })
    }

    return () => {
      source.close()
    }
  }, [key])

  const status: CrashStreamStatus = !key
    ? 'closed'
    : connection.key === key
      ? connection.status
      : 'connecting'

  return { status }
}
