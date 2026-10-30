import test from 'node:test'
import assert from 'node:assert/strict'
import {
  deriveMatchConnection,
  isTerminalStatus,
  type MatchConnectionInput,
} from './matchConnection.ts'

const base: MatchConnectionInput = {
  transport: 'live',
  hasState: true,
  matchStatus: 'waiting',
  isParticipant: true,
  playersPresent: 1,
  allReady: false,
}

const derive = (overrides: Partial<MatchConnectionInput> = {}) =>
  deriveMatchConnection({ ...base, ...overrides })

test('a live stream with a waiting room reports waiting_for_opponent', () => {
  const view = derive()
  assert.equal(view.state, 'waiting_for_opponent')
  assert.equal(view.isTransportLive, true)
  assert.equal(view.canPlay, false)
  assert.equal(view.needsSync, false)
})

test('a non-participant that joined but has no seat is joining_room, not ready', () => {
  const view = derive({ isParticipant: false, playersPresent: 1 })
  assert.equal(view.state, 'joining_room')
})

test('both players present without readiness is ready, not in_progress', () => {
  const view = derive({ matchStatus: 'ready', playersPresent: 2, allReady: false })
  assert.equal(view.state, 'ready')
  assert.equal(view.canPlay, false)
})

test('both players ready is starting, not in_progress (server decides the start)', () => {
  const view = derive({ matchStatus: 'ready', playersPresent: 2, allReady: true })
  assert.equal(view.state, 'starting')
  assert.equal(view.canPlay, false)
})

test('in progress with a live transport is playable', () => {
  const view = derive({ matchStatus: 'in_progress', playersPresent: 2 })
  assert.equal(view.state, 'in_progress')
  assert.equal(view.canPlay, true)
  assert.equal(view.isReady, true)
})

test('the reported bug: a dead stream during a live match never reports Connecting forever', () => {
  for (const transport of ['reconnecting', 'connecting', 'failed'] as const) {
    const view = derive({ transport, matchStatus: 'in_progress', playersPresent: 2 })
    assert.notEqual(view.state, 'connected' as never)
    assert.equal(view.canPlay, false, `no play with transport ${transport}`)
    assert.equal(view.needsSync, true, `must resync with transport ${transport}`)
    if (transport === 'failed') {
      assert.equal(view.state, 'failed')
      assert.match(view.description, /retry/i)
    } else {
      assert.equal(view.state, 'reconnecting')
    }
  }
})

test('a live but silent stream is flagged for resync', () => {
  const view = derive({ matchStatus: 'in_progress', playersPresent: 2, msSinceLastSync: 30_000 })
  assert.equal(view.state, 'in_progress')
  assert.equal(view.needsSync, true, 'a silent live stream must request an authoritative refetch')
})

test('a live stream with no room state yet is connecting', () => {
  const view = derive({ matchStatus: null, hasState: false })
  assert.equal(view.state, 'connecting')
  assert.equal(view.canPlay, false)
})

test('a terminal match wins over a broken transport', () => {
  for (const status of ['completed', 'tied', 'forfeited', 'cancelled'] as const) {
    const view = derive({ transport: 'failed', matchStatus: status })
    assert.equal(view.state, 'completed', `${status} must be shown as finished`)
    assert.equal(view.needsSync, false)
  }
})

test('no transport and no state is disconnected', () => {
  const view = derive({ transport: 'closed', hasState: false, matchStatus: null })
  assert.equal(view.state, 'disconnected')
})

test('a failed transport before the first snapshot reports failed with a recovery hint', () => {
  const view = derive({ transport: 'failed', hasState: false, matchStatus: null })
  assert.equal(view.state, 'failed')
  assert.match(view.description, /retry/i)
})

test('isTerminalStatus recognises every terminal backend status', () => {
  assert.equal(isTerminalStatus('completed'), true)
  assert.equal(isTerminalStatus('tied'), true)
  assert.equal(isTerminalStatus('forfeited'), true)
  assert.equal(isTerminalStatus('cancelled'), true)
  assert.equal(isTerminalStatus('in_progress'), false)
  assert.equal(isTerminalStatus('waiting'), false)
  assert.equal(isTerminalStatus(null), false)
})

test('the same inputs always produce the same state (no hidden transport/room conflation)', () => {
  const inputs: Partial<MatchConnectionInput>[] = [
    { transport: 'live', matchStatus: 'in_progress' },
    { transport: 'reconnecting', matchStatus: 'in_progress' },
    { transport: 'live', matchStatus: 'ready', playersPresent: 2, allReady: true },
    { transport: 'live', matchStatus: 'cancelled' },
  ]
  for (const input of inputs) {
    assert.deepEqual(derive(input), derive(input))
  }
})
