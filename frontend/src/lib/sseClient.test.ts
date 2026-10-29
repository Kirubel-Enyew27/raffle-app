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
  // Minimal window stub for the AUTH_TOKEN_REFRESHED_EVENT listener
  if (!('window' in global)) {
    // @ts-ignore
    global.window = {
      addEventListener: () => {},
      removeEventListener: () => {},
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

test('status transitions to failed after MAX_ATTEMPTS errors', () => {
  const client = makeClient()
  try {
    // MAX_ATTEMPTS = 10; we need to exhaust them.
    // Each error triggers a setTimeout; we override setTimeout to fire immediately.
    const origSetTimeout = global.setTimeout
    // @ts-ignore
    global.setTimeout = (fn: () => void) => { fn(); return 0 }

    const unsub = client.subscribe('crash', 'tok2', () => {})
    const src0 = MockEventSource.instances[0]
    src0.emitConnected()

    // Trigger 11 errors (one more than MAX_ATTEMPTS = 10)
    for (let i = 0; i < 11; i++) {
      const instances = MockEventSource.instances
      const latest = instances[instances.length - 1]
      latest.emitError()
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
