/**
 * SSEClient — a shared, singleton EventSource connection manager.
 *
 * Responsibilities:
 *  - Exactly one EventSource per channel, shared by every subscriber.
 *  - One logical session per channel: a token change re-attaches the existing
 *    subscribers to a fresh EventSource instead of stranding them on a
 *    (channel, token) key nobody can find again.
 *  - Bounded exponential backoff (1 s → 30 s, ±20 % jitter) so a server restart
 *    does not produce a reconnect storm.
 *  - The stream is never permanently dead: after `MAX_ATTEMPTS` consecutive
 *    failures it reports 'failed' (so the UI can explain and offer a retry) and
 *    stops the timer, but regaining network, returning to the tab, remounting a
 *    subscriber or calling `retryNow()` starts it again. Before this, a player
 *    whose stream died was stuck on "Connecting…" for the rest of the match.
 *  - `attempts` only resets after a connection has *stayed* live for
 *    `STABLE_RESET_MS`, so a flapping connection cannot loop forever.
 *  - A proactive token refresh is attempted only when the access token is
 *    actually expired: refreshing on every transport error rotated the session
 *    token on each retry and re-ran every token-dependent subscription.
 *
 * Usage:
 *   const unsub = sseClient.subscribe('crash', token, handler)
 *   // later:
 *   unsub()
 */

import { AUTH_TOKEN_REFRESHED_EVENT, authStorage } from './authStorage.ts'

// ─── Token refresh hook ───────────────────────────────────────────────────────
// sseClient may be imported before api.ts initialises. We resolve the refresher
// lazily so there is no circular-import issue. api.ts sets this once it installs
// the interceptors.

let _refreshSession: (() => Promise<boolean>) | null = null

/**
 * Called by api.ts after it creates the sessionRefresher so that sseClient can
 * renew an expired token before retrying a dropped SSE connection.
 */
export function setSseTokenRefresher(fn: (() => Promise<boolean>) | null): void {
  _refreshSession = fn
}

// ─── Public types ────────────────────────────────────────────────────────────

export type SSEStatus = 'connecting' | 'live' | 'reconnecting' | 'failed' | 'closed'

export interface SSEEvent {
  type: string
  channel?: string
  payload: unknown
  created_at?: string
  server_delivery_latency_ms?: number
}

export type SSEEventHandler = (ev: SSEEvent) => void

// ─── Internal types ───────────────────────────────────────────────────────────

interface Subscriber {
  handler: SSEEventHandler
}

interface ChannelListeners {
  connected: () => void
  message: (ev: MessageEvent) => void
  error: () => void
}

interface ChannelState {
  /** The channel name (e.g. 'crash', 'p2p_match:abc'). */
  channel: string
  /** Token used to open the current EventSource. */
  token: string
  source: EventSource | null
  status: SSEStatus
  subscribers: Map<symbol, Subscriber>
  /** Consecutive failed connection attempts. */
  attempts: number
  /** setTimeout handle for the next reconnect. */
  reconnectTimer: ReturnType<typeof setTimeout> | null
  /** setTimeout handle that clears `attempts` after a stable connection. */
  stableTimer: ReturnType<typeof setTimeout> | null
  /** Listeners attached to the current EventSource, so they can be removed. */
  listeners: ChannelListeners | null
  /** Timestamp of the last proactive token refresh attempt. */
  lastRefreshAt: number
}

// ─── Backoff helpers ──────────────────────────────────────────────────────────

const BASE_DELAY_MS = 1_000
const MAX_DELAY_MS = 30_000
/** Consecutive failures before the channel reports 'failed'. */
export const MAX_ATTEMPTS = 10
const JITTER = 0.2 // ±20 %
/** How long a connection must stay live before its retry budget is restored. */
export const STABLE_RESET_MS = 5_000
/** Minimum gap between two proactive token refreshes for the same channel. */
const REFRESH_THROTTLE_MS = 60_000
/** Refresh this long before the access token actually expires. */
const TOKEN_SKEW_MS = 60_000

/**
 * Backoff for the given attempt: 1 s, 2 s, 4 s … 30 s (capped), ±20 % jitter.
 * Exported for tests.
 */
export function backoffDelay(attempt: number, random: () => number = Math.random): number {
  const base = Math.min(BASE_DELAY_MS * 2 ** Math.max(0, attempt), MAX_DELAY_MS)
  const jitter = base * JITTER * (random() * 2 - 1) // [-20 %, +20 %]
  return Math.round(base + jitter)
}

/**
 * True when the JWT is missing, unreadable, or expires within `skewMs`.
 * Unreadable tokens are treated as "not expiring soon" so a malformed token
 * cannot trigger refresh storms; the server rejects it and the UI reports it.
 */
export function tokenExpiresSoon(token: string, skewMs = TOKEN_SKEW_MS): boolean {
  const claims = decodeJwtPayload(token)
  if (!claims || typeof claims.exp !== 'number') return false
  return claims.exp * 1000 - Date.now() <= skewMs
}

function decodeJwtPayload(token: string): { exp?: number } | null {
  const parts = token.split('.')
  if (parts.length < 2) return null
  try {
    const base64 = parts[1].replace(/-/g, '+').replace(/_/g, '/')
    const padded = base64.padEnd(base64.length + ((4 - (base64.length % 4)) % 4), '=')
    if (typeof atob !== 'function') return null
    return JSON.parse(atob(padded)) as { exp?: number }
  } catch {
    return null
  }
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
  /** One entry per channel name. The token lives inside the entry. */
  private readonly channels = new Map<string, ChannelState>()

  private readonly tokenRefreshListener: (ev: Event) => void
  private readonly onlineListener: () => void
  private readonly visibilityListener: () => void

  constructor() {
    this.tokenRefreshListener = (ev: Event) => {
      const newToken = (ev as CustomEvent<{ token: string }>).detail?.token
      if (!newToken) return
      this._applyToken(newToken)
    }

    this.onlineListener = () => this._retryInterrupted()
    this.visibilityListener = () => {
      if (typeof document !== 'undefined' && document.visibilityState === 'visible') {
        this._retryInterrupted()
      }
    }

    if (typeof window !== 'undefined' && typeof window.addEventListener === 'function') {
      window.addEventListener(AUTH_TOKEN_REFRESHED_EVENT, this.tokenRefreshListener)
      window.addEventListener('online', this.onlineListener)
    }
    if (typeof document !== 'undefined' && typeof document.addEventListener === 'function') {
      document.addEventListener('visibilitychange', this.visibilityListener)
    }
  }

  // ── Public API ─────────────────────────────────────────────────────────────

  /**
   * Subscribe to `channel` events authenticated with `token`.
   * Returns an unsubscribe function.
   */
  subscribe(channel: string, token: string, handler: SSEEventHandler): () => void {
    const id = Symbol()
    let state = this.channels.get(channel)

    if (!state) {
      state = this._createState(channel, token)
      this.channels.set(channel, state)
      this._openSource(state)
    } else if (token && token !== state.token) {
      // The access token rotated: re-attach the same channel (and its existing
      // subscribers) to a source authenticated with the new token.
      state.token = token
      this._reopenSource(state)
    } else if (state.status === 'failed') {
      // A component remounting is a user-visible retry signal.
      this._scheduleRetry(state, 0)
    }

    state.subscribers.set(id, { handler })

    // A new subscriber while the channel is dead should also restart it.
    if (state.status === 'failed' && !state.reconnectTimer) {
      this._scheduleRetry(state, 0)
    }

    return () => this._unsubscribe(channel, id)
  }

  /** Returns the current connection status for the given channel name. */
  getStatus(channel: string): SSEStatus {
    return this.channels.get(channel)?.status ?? 'closed'
  }

  /**
   * Force an immediate reconnect attempt for `channel`, resetting its retry
   * budget. Used by the "Retry connection" action instead of reloading the page
   * (a reload throws away the client's in-memory match view).
   */
  retryNow(channel: string): void {
    const state = this.channels.get(channel)
    if (!state) return
    state.attempts = 0
    if (state.reconnectTimer !== null) {
      clearTimeout(state.reconnectTimer)
      state.reconnectTimer = null
    }
    this._openSource(state)
  }

  /** Tear down all connections and remove global listeners. */
  dispose(): void {
    if (typeof window !== 'undefined' && typeof window.removeEventListener === 'function') {
      window.removeEventListener(AUTH_TOKEN_REFRESHED_EVENT, this.tokenRefreshListener)
      window.removeEventListener('online', this.onlineListener)
    }
    if (typeof document !== 'undefined' && typeof document.removeEventListener === 'function') {
      document.removeEventListener('visibilitychange', this.visibilityListener)
    }

    for (const state of Array.from(this.channels.values())) {
      this._closeChannel(state)
    }
    this.channels.clear()
  }

  // ── Internal helpers ───────────────────────────────────────────────────────

  private _createState(channel: string, token: string): ChannelState {
    return {
      channel,
      token,
      source: null,
      status: 'connecting',
      subscribers: new Map(),
      attempts: 0,
      reconnectTimer: null,
      stableTimer: null,
      listeners: null,
      lastRefreshAt: 0,
    }
  }

  /** Opens the first EventSource for a channel. */
  private _openSource(state: ChannelState): void {
    this._detachSource(state)
    this._attachSource(state)
    state.status = 'connecting'
  }

  /** Replaces the current EventSource, keeping subscribers and retry state. */
  private _reopenSource(state: ChannelState): void {
    this._detachSource(state)
    this._attachSource(state)
    state.status = 'connecting'
  }

  private _attachSource(state: ChannelState): void {
    const url = buildUrl(state.channel, state.token)
    const source = new EventSource(url)
    state.source = source

    const listeners: ChannelListeners = {
      connected: () => {
        state.status = 'live'
        // Only restore the retry budget once the connection proves stable;
        // resetting on every connect let a flapping stream retry forever.
        if (state.stableTimer !== null) clearTimeout(state.stableTimer)
        state.stableTimer = setTimeout(() => {
          state.stableTimer = null
          if (state.status === 'live') state.attempts = 0
        }, STABLE_RESET_MS)
      },
      message: (ev: MessageEvent) => {
        try {
          const parsed = JSON.parse(ev.data) as SSEEvent
          if (!parsed?.type) return
          for (const { handler } of state.subscribers.values()) {
            handler(parsed)
          }
        } catch {
          // Ignore unparseable frames.
        }
      },
      error: () => {
        // The browser fires onerror when the server closes the stream and when
        // the connection cannot be established. Both are handled by the same
        // bounded backoff instead of the browser's immediate retry loop.
        this._onChannelError(state)
      },
    }

    state.listeners = listeners
    source.addEventListener('connected', listeners.connected)
    source.addEventListener('message', listeners.message)
    source.onerror = listeners.error
  }

  private _onChannelError(state: ChannelState): void {
    // A late error from a source that has already been replaced must not touch
    // the current connection state.
    this._detachSource(state)
    if (state.status === 'closed' || !this.channels.has(state.channel)) return

    state.attempts += 1

    if (state.attempts > MAX_ATTEMPTS) {
      state.status = 'failed'
      return
    }

    state.status = 'reconnecting'
    this._scheduleRetry(state, backoffDelay(state.attempts - 1))
  }

  /**
   * Schedules the next connection attempt. `delay` of 0 opens immediately on the
   * next macrotask, so callers never connect re-entrantly.
   */
  private _scheduleRetry(state: ChannelState, delay: number): void {
    if (state.reconnectTimer !== null) return
    state.reconnectTimer = setTimeout(async () => {
      state.reconnectTimer = null
      if (!this.channels.has(state.channel) || state.status === 'closed') return

      const proceed = await this._maybeRefreshExpiredToken(state)
      if (!proceed) return
      // The channel may have been closed or replaced while we awaited.
      if (!this.channels.has(state.channel)) return

      this._reopenSource(state)
    }, delay)
  }

  /**
   * Refreshes an expired access token before reconnecting. Failures are
   * swallowed: the reconnect proceeds with the current token and the server's
   * 401 surfaces through the normal status transitions.
   */
  private async _maybeRefreshExpiredToken(state: ChannelState): Promise<boolean> {
    if (!_refreshSession) return true

    let latestToken: string | null = null
    try {
      latestToken = authStorage.getToken()
    } catch {
      // Non-browser environment.
    }

    if (latestToken && latestToken !== state.token) {
      // A concurrent refresh already produced a newer token.
      state.token = latestToken
      return true
    }

    const token = latestToken ?? state.token
    // Refreshing on every transport error rotated the session token on each
    // retry (and re-ran every token-dependent subscription). Only an access
    // token that is actually about to expire is worth refreshing.
    if (!tokenExpiresSoon(token)) return true

    const now = Date.now()
    if (now - state.lastRefreshAt < REFRESH_THROTTLE_MS) return true
    state.lastRefreshAt = now

    try {
      const refreshed = await _refreshSession()
      if (refreshed === false) {
        // The session is gone; the app is being signed out. Stop retrying the
        // stream instead of hammering the server with a dead token.
        state.status = 'failed'
        return false
      }
      const nextToken = authStorage.getToken()
      if (nextToken) state.token = nextToken
      return true
    } catch {
      // Transient refresh failure: retry with the current token.
      return true
    }
  }

  private _detachSource(state: ChannelState): void {
    if (state.reconnectTimer !== null) {
      clearTimeout(state.reconnectTimer)
      state.reconnectTimer = null
    }
    if (state.stableTimer !== null) {
      clearTimeout(state.stableTimer)
      state.stableTimer = null
    }
    if (state.source) {
      if (state.listeners) {
        state.source.removeEventListener('connected', state.listeners.connected)
        state.source.removeEventListener('message', state.listeners.message)
      }
      state.source.onerror = null
      state.source.close()
      state.source = null
      state.listeners = null
    }
  }

  private _unsubscribe(channel: string, id: symbol): void {
    const state = this.channels.get(channel)
    if (!state) return

    state.subscribers.delete(id)

    if (state.subscribers.size === 0) {
      this._closeChannel(state)
      this.channels.delete(channel)
    }
  }

  private _closeChannel(state: ChannelState): void {
    this._detachSource(state)
    state.status = 'closed'
  }

  /**
   * Re-attaches every channel to a new access token. The channel key (and so
   * every subscriber) is preserved, which fixes the leak where an unsubscribe
   * after a refresh could no longer find the migrated entry.
   */
  private _applyToken(newToken: string): void {
    for (const state of this.channels.values()) {
      if (state.token === newToken) continue
      state.token = newToken
      if (state.subscribers.size === 0) continue
      state.attempts = 0
      this._reopenSource(state)
    }
  }

  /**
   * Restarts channels that were waiting on the network or on a hidden tab.
   * 'failed' channels are retried here (and by `retryNow`) rather than on a
   * timer, so the client never retries unboundedly while nothing changes.
   */
  private _retryInterrupted(): void {
    for (const state of this.channels.values()) {
      if (state.subscribers.size === 0) continue
      if (state.status === 'live' || state.status === 'closed') continue
      state.attempts = 0
      if (state.reconnectTimer !== null) {
        clearTimeout(state.reconnectTimer)
        state.reconnectTimer = null
      }
      this._openSource(state)
    }
  }

  // ── Test-only accessors (not part of the public API) ──────────────────────

  /** @internal */
  _channelCount(): number {
    return this.channels.size
  }

  /** @internal */
  _getChannelState(channel: string): ChannelState | undefined {
    return this.channels.get(channel)
  }
}

// ─── Singleton ────────────────────────────────────────────────────────────────

export const sseClient = new SSEClient()
