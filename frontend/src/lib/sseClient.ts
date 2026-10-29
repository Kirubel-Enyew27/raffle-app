/**
 * SSEClient — a shared, singleton EventSource connection manager.
 *
 * Features:
 *  - One EventSource per (channel, token) pair; multiple React subscribers
 *    reuse the same underlying connection.
 *  - Exponential backoff: 1 s → 2 s → 4 s → 8 s → 16 s → 30 s (cap), each
 *    delay randomised by ±20 % jitter.
 *  - Maximum 10 reconnect attempts before entering the 'failed' state.
 *  - Token refresh: listens for AUTH_TOKEN_REFRESHED_EVENT and reconnects all
 *    open channels with the new token.
 *  - Clean unsubscribe: the EventSource is closed when the last subscriber
 *    for a channel leaves.
 *
 * Usage:
 *   const unsub = sseClient.subscribe('crash', token, handler)
 *   // later:
 *   unsub()
 */

import { AUTH_TOKEN_REFRESHED_EVENT } from './authStorage.ts'

// ─── Public types ────────────────────────────────────────────────────────────

export type SSEStatus = 'connecting' | 'live' | 'reconnecting' | 'failed' | 'closed'

export interface SSEEvent {
  type: string
  channel?: string
  payload: unknown
  created_at?: string
}

export type SSEEventHandler = (ev: SSEEvent) => void
export type StatusChangeHandler = (status: SSEStatus) => void

// ─── Internal types ───────────────────────────────────────────────────────────

interface Subscriber {
  handler: SSEEventHandler
}

interface ChannelState {
  /** The channel name (e.g. 'crash', 'p2p_match:abc'). */
  channel: string
  /** Token that was used to open this EventSource. */
  token: string
  source: EventSource
  status: SSEStatus
  subscribers: Map<symbol, Subscriber>
  /** Number of consecutive reconnection attempts. */
  attempts: number
  /** setTimeout handle for the next reconnect. */
  reconnectTimer: ReturnType<typeof setTimeout> | null
  /** true between the 'connected' event being received once (to detect reconnects). */
  everConnected: boolean
  /** Listeners added to the EventSource so they can be removed on close. */
  sourceListeners: {
    connected: () => void
    message: (ev: MessageEvent) => void
    error: () => void
  }
}

// ─── Backoff helpers ──────────────────────────────────────────────────────────

const BASE_DELAY_MS = 1_000
const MAX_DELAY_MS = 30_000
const MAX_ATTEMPTS = 10
const JITTER = 0.2 // ±20 %

function backoffDelay(attempt: number): number {
  // attempt 0 → 1 s, 1 → 2 s, 2 → 4 s, 3 → 8 s, 4 → 16 s, 5+ → 30 s
  const base = Math.min(BASE_DELAY_MS * 2 ** attempt, MAX_DELAY_MS)
  const jitter = base * JITTER * (Math.random() * 2 - 1) // [-20 %, +20 %]
  return Math.round(base + jitter)
}

// ─── URL builder ──────────────────────────────────────────────────────────────

function buildUrl(channel: string, token: string): string {
  // In tests `import.meta.env` may be undefined.
  const baseURL =
    typeof import.meta !== 'undefined' && import.meta.env?.VITE_API_URL
      ? import.meta.env.VITE_API_URL
      : '/api/v1'

  const root =
    baseURL.startsWith('http')
      ? baseURL
      : typeof window !== 'undefined'
        ? `${window.location.origin}${baseURL}`
        : baseURL

  return `${root}/realtime/stream?channel=${encodeURIComponent(channel)}&token=${encodeURIComponent(token)}`
}

// ─── SSEClient class ──────────────────────────────────────────────────────────

export class SSEClient {
  /**
   * Map key: `${channel}\0${token}` — one entry per (channel, token) pair.
   * When the token changes the old entry is cleaned up and a new one is opened.
   */
  private readonly channels = new Map<string, ChannelState>()

  /**
   * Maps a channel name to its current key so callers can query status by
   * channel name without knowing the token.
   */
  private readonly channelToKey = new Map<string, string>()

  private readonly tokenRefreshListener: (ev: Event) => void

  constructor() {
    this.tokenRefreshListener = (ev: Event) => {
      const newToken = (ev as CustomEvent<{ token: string }>).detail?.token
      if (!newToken) return
      this._handleTokenRefresh(newToken)
    }

    if (typeof window !== 'undefined') {
      window.addEventListener(AUTH_TOKEN_REFRESHED_EVENT, this.tokenRefreshListener)
    }
  }

  // ── Public API ─────────────────────────────────────────────────────────────

  /**
   * Subscribe to `channel` events authenticated with `token`.
   * Returns an unsubscribe function.
   */
  subscribe(channel: string, token: string, handler: SSEEventHandler): () => void {
    const key = this._key(channel, token)
    const id = Symbol()

    if (!this.channels.has(key)) {
      this._openChannel(channel, token, key)
    }

    const state = this.channels.get(key)!
    state.subscribers.set(id, { handler })

    return () => this._unsubscribe(key, id)
  }

  /** Returns the current connection status for the given channel name. */
  getStatus(channel: string): SSEStatus {
    const key = this.channelToKey.get(channel)
    if (!key) return 'closed'
    return this.channels.get(key)?.status ?? 'closed'
  }

  /** Tear down all connections and remove global listeners. */
  dispose(): void {
    if (typeof window !== 'undefined') {
      window.removeEventListener(AUTH_TOKEN_REFRESHED_EVENT, this.tokenRefreshListener)
    }

    for (const [key] of this.channels) {
      this._closeChannel(key, true)
    }
    this.channels.clear()
    this.channelToKey.clear()
  }

  // ── Internal helpers ───────────────────────────────────────────────────────

  private _key(channel: string, token: string): string {
    return `${channel}\0${token}`
  }

  private _openChannel(channel: string, token: string, key: string): void {
    const url = buildUrl(channel, token)
    const source = new EventSource(url)

    const state: ChannelState = {
      channel,
      token,
      source,
      status: 'connecting',
      subscribers: new Map(),
      attempts: 0,
      reconnectTimer: null,
      everConnected: false,
      // Placeholder — assigned below after state is created so closures can
      // reference `state` by closure.
      sourceListeners: undefined as unknown as ChannelState['sourceListeners'],
    }

    const listeners = {
      connected: () => {
        // On the very first successful open reset attempts.
        state.attempts = 0
        state.status = 'live'
        state.everConnected = true
      },
      message: (ev: MessageEvent) => {
        try {
          const parsed = JSON.parse(ev.data) as SSEEvent
          if (parsed?.type) {
            for (const { handler } of state.subscribers.values()) {
              handler(parsed)
            }
          }
        } catch {
          // Ignore unparseable frames.
        }
      },
      error: () => {
        // The browser fires onerror both when the server closes the connection
        // (readyState → CLOSED) and when a network issue occurs while
        // CONNECTING. In both cases the browser would normally retry
        // immediately; we intercept to apply our own backoff.
        this._onChannelError(key)
      },
    }

    state.sourceListeners = listeners

    // EventSource uses named event listeners for custom event types.
    source.addEventListener('connected', listeners.connected)
    source.addEventListener('message', listeners.message)
    source.onerror = listeners.error

    this.channels.set(key, state)
    this.channelToKey.set(channel, key)
  }

  private _onChannelError(key: string): void {
    const state = this.channels.get(key)
    if (!state) return

    // Close the broken EventSource immediately so the browser stops its own
    // rapid-retry loop — we'll reconnect on our schedule.
    this._detachSource(state)

    if (state.attempts >= MAX_ATTEMPTS) {
      state.status = 'failed'
      return
    }

    state.status = 'reconnecting'
    const delay = backoffDelay(state.attempts)
    state.attempts += 1

    state.reconnectTimer = setTimeout(() => {
      // Verify the channel still exists and is still meant to reconnect.
      const current = this.channels.get(key)
      if (!current || current.status === 'closed' || current.status === 'failed') return

      // Open a fresh EventSource on the same key.
      const url = buildUrl(current.channel, current.token)
      const source = new EventSource(url)
      current.source = source
      current.status = 'connecting'

      const listeners = {
        connected: () => {
          current.attempts = 0
          current.status = 'live'
          current.everConnected = true
        },
        message: (ev: MessageEvent) => {
          try {
            const parsed = JSON.parse(ev.data) as SSEEvent
            if (parsed?.type) {
              for (const { handler } of current.subscribers.values()) {
                handler(parsed)
              }
            }
          } catch {
            // Ignore unparseable frames.
          }
        },
        error: () => {
          this._onChannelError(key)
        },
      }

      current.sourceListeners = listeners
      source.addEventListener('connected', listeners.connected)
      source.addEventListener('message', listeners.message)
      source.onerror = listeners.error
    }, delay)
  }

  private _detachSource(state: ChannelState): void {
    if (state.reconnectTimer !== null) {
      clearTimeout(state.reconnectTimer)
      state.reconnectTimer = null
    }
    if (state.source) {
      state.source.removeEventListener('connected', state.sourceListeners.connected)
      state.source.removeEventListener('message', state.sourceListeners.message)
      state.source.onerror = null
      state.source.close()
    }
  }

  private _unsubscribe(key: string, id: symbol): void {
    const state = this.channels.get(key)
    if (!state) return

    state.subscribers.delete(id)

    if (state.subscribers.size === 0) {
      this._closeChannel(key, false)
    }
  }

  private _closeChannel(key: string, force: boolean): void {
    const state = this.channels.get(key)
    if (!state) return

    if (!force && state.subscribers.size > 0) return

    this._detachSource(state)
    state.status = 'closed'
    this.channels.delete(key)
    // Remove from name→key map only if it still points to this key.
    if (this.channelToKey.get(state.channel) === key) {
      this.channelToKey.delete(state.channel)
    }
  }

  private _handleTokenRefresh(newToken: string): void {
    // Collect existing channels (snapshot) before mutating the map.
    const entries = Array.from(this.channels.entries())

    for (const [oldKey, state] of entries) {
      if (state.token === newToken) continue // already using the new token

      const { channel } = state
      const newKey = this._key(channel, newToken)

      // Migrate subscribers to a new entry with the refreshed token.
      const oldSubscribers = new Map(state.subscribers)

      // Close old channel without removing subscribers (they migrate).
      state.subscribers.clear()
      this._closeChannel(oldKey, true)

      // Open fresh channel if there are subscribers to inherit.
      if (oldSubscribers.size > 0) {
        this._openChannel(channel, newToken, newKey)
        const newState = this.channels.get(newKey)!
        for (const [id, sub] of oldSubscribers) {
          newState.subscribers.set(id, sub)
        }
      }
    }
  }

  // ── Test-only accessors (not part of the public API) ──────────────────────

  /** @internal */
  _channelCount(): number {
    return this.channels.size
  }

  /** @internal */
  _getChannelState(channel: string): ChannelState | undefined {
    const key = this.channelToKey.get(channel)
    if (!key) return undefined
    return this.channels.get(key)
  }
}

// ─── Singleton ────────────────────────────────────────────────────────────────

export const sseClient = new SSEClient()
