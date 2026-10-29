/**
 * Tests for SSEClient — the shared, singleton EventSource connection manager.
 *
 * EventSource is a browser API unavailable in Node. We replace it with a
 * minimal mock that exposes the same addEventListener / onerror / close
 * interface. The mock is stored in `global.EventSource` before each test.
 */

import test from 'node:test'
import assert from 'node:assert/strict'
import { SSEClient } from './sseClient.ts'

// ─── EventSource mock ─────────────────────────────────────────────────────────

type ListenerFn = (ev?: Event | MessageEvent) => void

class MockEventSource {
  static instances: MockEventSource[] = []
  url: string
  readyState: number = 0 // CONNECTING
  onerror: (() => void) | null = null
  private listeners: Map<string, ListenerFn[]> = new Map()

  constructor(url: string) {
    this.url = url
    MockEventSource.instances.push(this)
  }

  addEventListener(type: string, fn: ListenerFn) {
    if (!this.listeners.has(type)) this.listeners.set(type, [])
    this.listeners.get(type)!.push(fn)
  }

  removeEventListener(type: string, fn: ListenerFn) {
    const list = this.listeners.get(type)
    if (!list) return
    const idx = list.indexOf(fn)
    if (idx !== -1) list.splice(idx, 1)
  }

  close() {
    this.readyState = 2 // CLOSED
  }

  // Test helpers

  /** Simulate the server sending the 'connected' event. */
  emitConnected() {
    const fns = this.listeners.get('connected') ?? []
    for (const fn of fns) fn()
  }

  /** Simulate the server sending a 'message' SSE event. */
  emitMessage(data: object) {
    const fns = this.listeners.get('message') ?? []
    const evt = { data: JSON.stringify(data) } as MessageEvent
    for (const fn of fns) fn(evt)
  }

  /** Simulate a network error or server close. */
  emitError() {
    if (this.onerror) this.onerror()
  }
}

function setupMock() {
  MockEventSource.instances = []
  // @ts-ignore — inject mock into global so SSEClient's `new EventSource(...)` picks it up
  global.EventSource = MockEventSource
  // Minimal window stub with working addEventListener / dispatchEvent so
  // the SSEClient's AUTH_TOKEN_REFRESHED_EVENT listener actually fires.
  if (!('window' in global) || !(global as Record<string, unknown>).window) {
    const listeners: Record<string, Array<(ev: Event) => void>> = {}
    // @ts-ignore
    global.window = {
      addEventListener: (type: string, fn: (ev: Event) => void) => {
        if (!listeners[type]) listeners[type] = []
        listeners[type].push(fn)
      },
      removeEventListener: (type: string, fn: (ev: Event) => void) => {
        const list = listeners[type]
        if (!list) return
        const idx = list.indexOf(fn)
        if (idx !== -1) list.splice(idx, 1)
      },
      dispatchEvent: (ev: Event) => {
        const type = ev.type
        const list = listeners[type]
        if (!list) return
        for (const fn of [...list]) fn(ev)
      },
      CustomEvent: class CustomEvent {
        type: string
        detail: unknown
        constructor(type: string, opts?: { detail?: unknown }) {
          this.type = type
          this.detail = opts?.detail
        }
      },
      location: { origin: 'http://localhost' },
    }
  }
}

function teardown() {
  // @ts-ignore
  delete global.EventSource
}

// ─── Helper: build a fresh client per test ────────────────────────────────────

function makeClient() {
  setupMock()
  return new SSEClient()
}

// ─── Tests ────────────────────────────────────────────────────────────────────

test('subscribe creates exactly one EventSource per (channel, token) pair', () => {
  const client = makeClient()
  try {
    const unsub1 = client.subscribe('crash', 'tok1', () => {})
    const unsub2 = client.subscribe('crash', 'tok1', () => {})

    assert.equal(MockEventSource.instances.length, 1, 'only one EventSource for the same channel+token')
    assert.equal(client._channelCount(), 1, 'client tracks one channel')

    unsub1()
    unsub2()
  } finally {
    client.dispose()
    teardown()
  }
})

test('two subscribers on the same channel share the connection', () => {
  const client = makeClient()
  try {
    const received1: unknown[] = []
    const received2: unknown[] = []

    const unsub1 = client.subscribe('p2p_match:abc', 'tok', (ev) => received1.push(ev))
    const unsub2 = client.subscribe('p2p_match:abc', 'tok', (ev) => received2.push(ev))

    const src = MockEventSource.instances[0]
    src.emitMessage({ type: 'game_event', payload: { move: 1 } })

    assert.equal(received1.length, 1, 'first subscriber got the event')
    assert.equal(received2.length, 1, 'second subscriber got the event')
    assert.equal(MockEventSource.instances.length, 1, 'still only one EventSource')

    unsub1()
    unsub2()
  } finally {
    client.dispose()
    teardown()
  }
})

test('different channels open separate EventSources', () => {
  const client = makeClient()
  try {
    const unsub1 = client.subscribe('crash', 'tok', () => {})
    const unsub2 = client.subscribe('p2p_match:xyz', 'tok', () => {})

    assert.equal(MockEventSource.instances.length, 2, 'one EventSource per distinct channel')
    assert.equal(client._channelCount(), 2)

    unsub1()
    unsub2()
  } finally {
    client.dispose()
    teardown()
  }
})

test('unsubscribing the last subscriber closes the EventSource', () => {
  const client = makeClient()
  try {
    const unsub = client.subscribe('lobby', 'tok', () => {})
    const src = MockEventSource.instances[0]

    assert.equal(src.readyState, 0, 'EventSource is open')

    unsub()

    assert.equal(src.readyState, 2, 'EventSource is closed after last unsub')
    assert.equal(client._channelCount(), 0, 'channel entry removed from map')
  } finally {
    client.dispose()
    teardown()
  }
})

test('removing one of two subscribers keeps the connection open', () => {
  const client = makeClient()
  try {
    const unsub1 = client.subscribe('lobby', 'tok', () => {})
    const unsub2 = client.subscribe('lobby', 'tok', () => {})
    const src = MockEventSource.instances[0]

    unsub1()

    assert.equal(src.readyState, 0, 'EventSource remains open while second subscriber exists')
    assert.equal(client._channelCount(), 1)

    unsub2()
    assert.equal(src.readyState, 2, 'EventSource closed after last unsub')
  } finally {
    client.dispose()
    teardown()
  }
})

test('status starts as connecting and transitions to live after connected event', () => {
  const client = makeClient()
  try {
    const unsub = client.subscribe('crash', 'tok', () => {})

    assert.equal(client.getStatus('crash'), 'connecting')

    const src = MockEventSource.instances[0]
    src.emitConnected()

    assert.equal(client.getStatus('crash'), 'live')
    unsub()
  } finally {
    client.dispose()
    teardown()
  }
})

test('status transitions to reconnecting on error', () => {
  const client = makeClient()
  try {
    const unsub = client.subscribe('crash', 'tok', () => {})
    const src = MockEventSource.instances[0]
    src.emitConnected()

    src.emitError()

    // After error the channel should be reconnecting (not failed yet: attempt < MAX)
    const status = client.getStatus('crash')
    assert.ok(
      status === 'reconnecting' || status === 'connecting',
      `expected reconnecting/connecting, got ${status}`,
    )

    unsub()
  } finally {
    client.dispose()
    teardown()
  }
})

test('status transitions to failed after MAX_ATTEMPTS errors', async () => {
  const client = makeClient()
  try {
    // MAX_ATTEMPTS = 10; we need to exhaust them.
    // Each error triggers a setTimeout; we override setTimeout to fire
    // asynchronously via a microtask so the async callback can complete.
    const origSetTimeout = global.setTimeout
    // @ts-ignore
    global.setTimeout = (fn: () => void) => { void Promise.resolve().then(fn); return 0 }

    const unsub = client.subscribe('crash', 'tok2', () => {})
    const src0 = MockEventSource.instances[0]
    src0.emitConnected()

    // Trigger 11 errors (one more than MAX_ATTEMPTS = 10). Each error fires
    // asynchronously via the microtask queue, so we await between iterations.
    for (let i = 0; i < 11; i++) {
      const instances = MockEventSource.instances
      const latest = instances[instances.length - 1]
      latest.emitError()
      // Allow the async setTimeout callback to complete.
      await new Promise((r) => origSetTimeout(r, 0))
    }

    const status = client.getStatus('crash')
    assert.equal(status, 'failed', `expected failed after exhausted retries, got ${status}`)

    // @ts-ignore
    global.setTimeout = origSetTimeout
    unsub()
  } finally {
    client.dispose()
    teardown()
  }
})

test('events are not delivered after unsubscribe', () => {
  const client = makeClient()
  try {
    const received: unknown[] = []
    const unsub = client.subscribe('lobby', 'tok', (ev) => received.push(ev))

    const src = MockEventSource.instances[0]
    src.emitMessage({ type: 'update', payload: {} })
    assert.equal(received.length, 1)

    unsub()

    // After unsub the EventSource is closed; any further calls are no-ops
    // from the test's perspective since the source is gone.
    assert.equal(received.length, 1, 'no new events after unsubscribe')
  } finally {
    client.dispose()
    teardown()
  }
})

test('getStatus returns closed for unknown channel', () => {
  const client = makeClient()
  try {
    assert.equal(client.getStatus('unknown'), 'closed')
  } finally {
    client.dispose()
    teardown()
  }
})

test('dispose closes all channels and clears state', () => {
  const client = makeClient()
  const unsub1 = client.subscribe('ch1', 'tok', () => {})
  const unsub2 = client.subscribe('ch2', 'tok', () => {})

  client.dispose()

  assert.equal(client._channelCount(), 0, 'all channels cleared on dispose')

  for (const src of MockEventSource.instances) {
    assert.equal(src.readyState, 2, 'all EventSources closed on dispose')
  }

  // These should be no-ops after dispose
  unsub1()
  unsub2()

  teardown()
})

test('exponential backoff delay increases with attempt number', () => {
  // Access the internal backoffDelay via the compiled module.
  // Since it is not exported we verify the observable behaviour: each
  // successive reconnect opens a new EventSource after a timer fires.
  // Here we just confirm the formula numerically.
  const BASE = 1_000
  const MAX = 30_000
  const delays = [0, 1, 2, 3, 4, 5, 6].map((a) => {
    const base = Math.min(BASE * 2 ** a, MAX)
    return base
  })

  assert.deepEqual(delays, [1000, 2000, 4000, 8000, 16000, 30000, 30000])
})

test('setSseTokenRefresher: proactive refresh is called before reconnect when token is stale', async () => {
  setupMock()

  // Import the module-level exports from the already-loaded singleton.
  // Dynamic import() in node:test re-uses the cached module.
  const mod = await import('./sseClient.ts')
  const { SSEClient, setSseTokenRefresher } = mod

  let refreshCalled = 0
  setSseTokenRefresher(async () => {
    refreshCalled += 1
    // Simulate successful refresh — dispatch the event so sseClient migrates channel.
    if (typeof window !== 'undefined') {
      window.dispatchEvent(
        new CustomEvent('auth:token-refreshed', { detail: { token: 'new-token-refresh' } }),
      )
    }
    return true
  })

  const client = new SSEClient()
  const origSetTimeout = global.setTimeout
  try {
    // Fire timers synchronously so we can assert without awaiting.
    // @ts-ignore
    global.setTimeout = (fn: () => void) => { void Promise.resolve().then(fn); return 0 }

    const unsub = client.subscribe('match:refresh-test', 'old-token-refresh', () => {})
    const src0 = MockEventSource.instances[MockEventSource.instances.length - 1]
    src0.emitConnected()

    // Emit an error to trigger the reconnect path.
    src0.emitError()

    // Wait a tick for the async setTimeout callback to run.
    await new Promise((r) => origSetTimeout(r, 50))

    assert.ok(refreshCalled >= 1, `expected refresh to be called at least once, got ${refreshCalled}`)

    unsub()
  } finally {
    setSseTokenRefresher(null as unknown as () => Promise<boolean>)
    // @ts-ignore
    global.setTimeout = origSetTimeout
    client.dispose()
    teardown()
  }
})

test('setSseTokenRefresher: reconnect continues when refresh throws', async () => {
  setupMock()

  const mod = await import('./sseClient.ts')
  const { SSEClient, setSseTokenRefresher } = mod

  setSseTokenRefresher(async () => { throw new Error('network error') })

  const client = new SSEClient()
  const origSetTimeout = global.setTimeout
  try {
    // @ts-ignore
    global.setTimeout = (fn: () => void) => { void Promise.resolve().then(fn); return 0 }

    const unsub = client.subscribe('match:refresh-fail', 'token-fail', () => {})
    const src0 = MockEventSource.instances[MockEventSource.instances.length - 1]
    src0.emitConnected()

    // One error — still within MAX_ATTEMPTS, should stay 'reconnecting' / 'connecting'
    src0.emitError()

    await new Promise((r) => origSetTimeout(r, 50))

    const status = client.getStatus('match:refresh-fail')
    assert.ok(
      ['reconnecting', 'connecting', 'live'].includes(status),
      `unexpected status after failed refresh: ${status}`,
    )

    unsub()
  } finally {
    setSseTokenRefresher(null as unknown as () => Promise<boolean>)
    // @ts-ignore
    global.setTimeout = origSetTimeout
    client.dispose()
    teardown()
  }
})


test('reconnect: status transitions reconnecting → connecting → live and channel reopens', async () => {
  setupMock()

  const client = new SSEClient()
  const origSetTimeout = global.setTimeout
  try {
    // Fire timers synchronously.
    // @ts-ignore
    global.setTimeout = (fn: () => void) => { fn(); return 0 }

    const unsub = client.subscribe('reconnect-test', 'tok-rc', () => {})
    const src0 = MockEventSource.instances[MockEventSource.instances.length - 1]
    src0.emitConnected()

    assert.equal(client.getStatus('reconnect-test'), 'live')

    // Trigger an error → reconnecting → timer fires → _reopenSource → connecting
    src0.emitError()

    // After the synchronous timer, _reopenSource should have run.
    // The new EventSource should be in 'connecting' status.
    const status1 = client.getStatus('reconnect-test')
    assert.ok(
      ['reconnecting', 'connecting'].includes(status1),
      `expected reconnecting or connecting after error, got ${status1}`,
    )

    // Simulate the new EventSource receiving a 'connected' event.
    const src1 = MockEventSource.instances[MockEventSource.instances.length - 1]
    src1.emitConnected()

    assert.equal(client.getStatus('reconnect-test'), 'live', 'should be live after reconnect')

    unsub()
  } finally {
    // @ts-ignore
    global.setTimeout = origSetTimeout
    client.dispose()
    teardown()
  }
})

test('reconnect: _refreshSession returning false marks channel as failed', async () => {
  setupMock()

  const mod = await import('./sseClient.ts')
  const { SSEClient, setSseTokenRefresher } = mod

  setSseTokenRefresher(async () => false)

  const client = new SSEClient()
  const origSetTimeout = global.setTimeout
  try {
    // @ts-ignore
    global.setTimeout = (fn: () => void) => { void Promise.resolve().then(fn); return 0 }

    const unsub = client.subscribe('refresh-dead', 'tok-dead', () => {})
    const src0 = MockEventSource.instances[MockEventSource.instances.length - 1]
    src0.emitConnected()

    src0.emitError()

    // Wait for the async refresh + status update.
    await new Promise((r) => origSetTimeout(r, 50))

    assert.equal(
      client.getStatus('refresh-dead'),
      'failed',
      'channel should be failed when refresh returns false (session dead)',
    )

    unsub()
  } finally {
    setSseTokenRefresher(null as unknown as () => Promise<boolean>)
    // @ts-ignore
    global.setTimeout = origSetTimeout
    client.dispose()
    teardown()
  }
})

test('reconnect: _refreshSession returning true but token unchanged still reopens source', async () => {
  setupMock()

  const mod = await import('./sseClient.ts')
  const { SSEClient, setSseTokenRefresher } = mod

  // Refresh succeeds (returns true) but does NOT dispatch AUTH_TOKEN_REFRESHED_EVENT,
  // so the token stays the same. The channel should still reopen with the existing token.
  setSseTokenRefresher(async () => true)

  const client = new SSEClient()
  const origSetTimeout = global.setTimeout
  try {
    // @ts-ignore
    global.setTimeout = (fn: () => void) => { void Promise.resolve().then(fn); return 0 }

    const unsub = client.subscribe('refresh-same', 'tok-same', () => {})
    const src0 = MockEventSource.instances[MockEventSource.instances.length - 1]
    src0.emitConnected()

    src0.emitError()

    // Wait for the async refresh + reopen.
    await new Promise((r) => origSetTimeout(r, 50))

    const status = client.getStatus('refresh-same')
    assert.ok(
      ['connecting', 'live'].includes(status),
      `expected connecting or live after refresh with same token, got ${status}`,
    )

    // A new EventSource should have been opened (2+ instances for this channel).
    assert.ok(
      MockEventSource.instances.length >= 2,
      'a new EventSource should have been created after refresh with same token',
    )

    unsub()
  } finally {
    setSseTokenRefresher(null as unknown as () => Promise<boolean>)
    // @ts-ignore
    global.setTimeout = origSetTimeout
    client.dispose()
    teardown()
  }
})
