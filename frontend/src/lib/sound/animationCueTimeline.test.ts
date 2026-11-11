import test from 'node:test'
import assert from 'node:assert/strict'

import {
  AnimationCueTimeline,
  ANIMATION_MS,
  animationDelay,
  type AnimationCue,
  type AnimationCueSink,
} from './animationCueTimeline.ts'

interface Scheduled {
  id: number
  at: number
  handler: () => void
  cancelled: boolean
}

function createHarness() {
  let now = 0
  let nextId = 1
  const scheduled: Scheduled[] = []
  const calls: AnimationCue[] = []

  const sink: AnimationCueSink = {
    move: () => void calls.push('move'),
    drop: () => void calls.push('drop'),
    land: () => void calls.push('land'),
    capture: () => void calls.push('capture'),
    promotion: () => void calls.push('promotion'),
    select: () => void calls.push('select'),
    reveal: () => void calls.push('reveal'),
  }

  const timeline = new AnimationCueTimeline(sink, {
    setTimer: (handler, delayMs) => {
      const entry: Scheduled = { id: nextId++, at: now + delayMs, handler, cancelled: false }
      scheduled.push(entry)
      return entry as unknown as ReturnType<typeof setTimeout>
    },
    clearTimer: (handle) => {
      const entry = handle as unknown as Scheduled
      entry.cancelled = true
    },
  })

  const advance = (ms: number) => {
    const target = now + ms
    // Fire due timers in chronological order, like a real event loop.
    for (;;) {
      const due = scheduled
        .filter((entry) => !entry.cancelled && entry.at <= target)
        .sort((a, b) => a.at - b.at)[0]
      if (!due) break
      now = due.at
      due.cancelled = true
      due.handler()
    }
    now = target
  }

  return { timeline, calls, scheduled, advance }
}

test('playOnce is idempotent per key and emits immediately', () => {
  const { timeline, calls } = createHarness()

  assert.equal(timeline.playOnce('m1:move', 'move'), true)
  assert.equal(timeline.playOnce('m1:move', 'move'), false)
  assert.equal(timeline.playOnce('m2:move', 'move'), true)

  assert.deepEqual(calls, ['move', 'move'])
})

test('intermediate snapshots between the optimistic move and its confirmation stay silent', () => {
  const { timeline, calls } = createHarness()

  // Optimistic paint, then REST acknowledgement, then a duplicate SSE event:
  // all three describe the same move and must produce exactly one cue.
  timeline.playOnce('c4:12:33:drop', 'drop')
  timeline.playOnce('c4:12:33:drop', 'drop')
  timeline.playOnce('c4:12:33:drop', 'drop')

  assert.deepEqual(calls, ['drop'])
})

test('a scheduled milestone fires exactly at the animation delay, once', () => {
  const { timeline, calls, advance } = createHarness()

  timeline.schedule('c4:5:20:land', 'land', ANIMATION_MS.c4TokenDrop)
  advance(ANIMATION_MS.c4TokenDrop - 1)
  assert.deepEqual(calls, [])

  advance(1)
  assert.deepEqual(calls, ['land'])

  advance(1000)
  assert.deepEqual(calls, ['land'], 'a fired milestone is never repeated')

  // Re-scheduling a played key after the fact must stay silent.
  timeline.schedule('c4:5:20:land', 'land', ANIMATION_MS.c4TokenDrop)
  advance(1000)
  assert.deepEqual(calls, ['land'])
})

test('rescheduling the same key replaces the pending timer instead of stacking', () => {
  const { timeline, calls, advance } = createHarness()

  timeline.schedule('dama:4:capture', 'capture', ANIMATION_MS.damaSlide)
  timeline.schedule('dama:4:capture', 'capture', ANIMATION_MS.damaSlide)

  advance(ANIMATION_MS.damaSlide)
  assert.deepEqual(calls, ['capture'])
})

test('cancel drops one pending cue without forgetting its key', () => {
  const { timeline, calls, advance } = createHarness()

  timeline.schedule('dama:4:capture', 'capture', ANIMATION_MS.damaSlide)
  timeline.cancel('dama:4:capture')
  advance(ANIMATION_MS.damaSlide)
  assert.deepEqual(calls, [])

  // The move itself already played; a retried key must not replay.
  timeline.playOnce('dama:4:move', 'move')
  timeline.playOnce('dama:4:move', 'move')
  assert.deepEqual(calls, ['move'])
})

test('clearPending removes queued sounds but keeps idempotency', () => {
  const { timeline, calls, advance } = createHarness()

  timeline.playOnce('m1:move', 'move')
  timeline.schedule('m1:capture', 'capture', ANIMATION_MS.damaSlide)
  timeline.schedule('m1:promotion', 'promotion', ANIMATION_MS.damaSlide)
  timeline.clearPending()

  advance(ANIMATION_MS.damaSlide)
  assert.deepEqual(calls, ['move'])
  assert.equal(timeline.playOnce('m1:move', 'move'), false)
})

test('reset starts a new match from a clean baseline', () => {
  const { timeline, calls, advance } = createHarness()

  timeline.playOnce('m1:move', 'move')
  timeline.schedule('m1:capture', 'capture', ANIMATION_MS.damaSlide)
  timeline.reset()
  advance(ANIMATION_MS.damaSlide)
  assert.deepEqual(calls, ['move'])

  assert.equal(timeline.playOnce('m1:move', 'move'), true)
  assert.deepEqual(calls, ['move', 'move'])
})

test('a throwing sink never propagates into gameplay', () => {
  const timeline = new AnimationCueTimeline({
    move() {
      throw new Error('audio exploded')
    },
    drop() {},
    land() {},
    capture() {},
    promotion() {},
    select() {},
    reveal() {},
  })

  assert.doesNotThrow(() => timeline.playOnce('m1:move', 'move'))
  assert.equal(timeline.hasPlayed('m1:move'), true)
})

test('animationDelay is a no-op without a reduced-motion preference', () => {
  // Node has no window: the duration is used unchanged.
  assert.equal(animationDelay(ANIMATION_MS.c4TokenDrop), ANIMATION_MS.c4TokenDrop)
})
