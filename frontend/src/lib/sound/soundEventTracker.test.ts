import test from 'node:test'
import assert from 'node:assert/strict'

import { SoundEventTracker, type MatchSnapshot, type SoundSink } from './soundEventTracker.ts'

interface Calls {
  matchStart: number
  move: number
  turnAlert: number
  ticks: number[]
  victory: number
  defeat: number
  draw: number
}

function createSink() {
  const calls: Calls = {
    matchStart: 0,
    move: 0,
    turnAlert: 0,
    ticks: [],
    victory: 0,
    defeat: 0,
    draw: 0,
  }
  const sink: SoundSink = {
    playMatchStart: () => void calls.matchStart++,
    playMove: () => void calls.move++,
    playTurnAlert: () => void calls.turnAlert++,
    playTimerTick: (sec?: number) => void calls.ticks.push(sec ?? 0),
    playVictory: () => void calls.victory++,
    playDefeat: () => void calls.defeat++,
    playDraw: () => void calls.draw++,
  }
  return { calls, sink }
}

function trackerFor(userId = 'alice', sink?: SoundSink) {
  const created = createSink()
  const tracker = new SoundEventTracker(sink ?? created.sink)
  tracker.setUser(userId)
  return { tracker, calls: created.calls }
}

const inProgress = (overrides: Partial<MatchSnapshot> = {}): MatchSnapshot => ({
  id: 'match-1',
  status: 'in_progress',
  current_turn_user_id: 'bob',
  board_state: { cells: [0, 0, 0, 0, 0, 0, 0, 0, 0] },
  current_round: 1,
  ...overrides,
})

test('historical completed match stays silent on load and on refresh', () => {
  const { tracker, calls } = trackerFor()
  const finished: MatchSnapshot = {
    id: 'match-1',
    status: 'completed',
    winner_user_id: 'alice',
    board_state: { cells: [1, 1, 1, 2, 2, 0, 0, 0, 0] },
  }

  tracker.update(finished)
  tracker.update(finished)
  tracker.update({ ...finished })

  assert.equal(calls.victory, 0)
  assert.equal(calls.defeat, 0)
  assert.equal(calls.move, 0)
  assert.equal(calls.matchStart, 0)
})

test('first snapshot of an in-progress match does not alert even when it is my turn', () => {
  const { tracker, calls } = trackerFor('alice')
  tracker.update(inProgress({ current_turn_user_id: 'alice' }))
  assert.equal(calls.turnAlert, 0)
})

test('turn alert fires only on transition to the local player, once per transition', () => {
  const { tracker, calls } = trackerFor('alice')

  tracker.update(inProgress())
  tracker.update(inProgress())
  assert.equal(calls.turnAlert, 0, 'opponent turn must stay silent')

  // Opponent moved; it is now Alice's turn.
  tracker.update(inProgress({ current_turn_user_id: 'alice', board_state: { cells: [2, 0, 0, 0, 0, 0, 0, 0, 0] } }))
  assert.equal(calls.turnAlert, 1)
  assert.equal(calls.move, 1)

  // Duplicate SSE event / re-render must not repeat either sound.
  tracker.update(inProgress({ current_turn_user_id: 'alice', board_state: { cells: [2, 0, 0, 0, 0, 0, 0, 0, 0] } }))
  assert.equal(calls.turnAlert, 1)
  assert.equal(calls.move, 1)
})

test('dama captures and promotions use their dedicated sounds', () => {
  let moves = 0
  let captures = 0
  let promotions = 0
  const sink: SoundSink = {
    playMatchStart() {},
    playMove() {
      moves++
    },
    playTurnAlert() {},
    playTimerTick() {},
    playVictory() {},
    playDefeat() {},
    playDraw() {},
    playCapture() {
      captures++
    },
    playPromotion() {
      promotions++
    },
  }
  const tracker = new SoundEventTracker(sink)
  tracker.setUser('alice')

  tracker.update(inProgress({ board_state: { cells: [0], last_move: null } }))
  tracker.update(
    inProgress({ board_state: { cells: [1], last_move: { from: 1, to: 2, captures: [3] } } })
  )
  assert.equal(captures, 1)
  assert.equal(moves, 0, 'a capture must not also play the plain placement cue')

  tracker.update(
    inProgress({
      board_state: { cells: [1, 1], last_move: { from: 2, to: 3, captures: [4], promotion: true } },
    })
  )
  assert.equal(promotions, 1)
  assert.equal(captures, 1)

  tracker.update(inProgress({ board_state: { cells: [1, 1, 1], last_move: { from: 3, to: 4 } } }))
  assert.equal(moves, 1, 'ordinary moves keep the placement cue')
})

test('match start plays once when the match transitions into progress', () => {
  const { tracker, calls } = trackerFor('alice')
  tracker.update(inProgress({ status: 'ready', current_turn_user_id: null, board_state: {} }))
  tracker.update(inProgress())
  assert.equal(calls.matchStart, 1)
  tracker.update(inProgress())
  assert.equal(calls.matchStart, 1)
})

test('victory and defeat fire exactly once for the correct player', () => {
  const winner = trackerFor('alice')
  winner.tracker.update(inProgress({ current_turn_user_id: 'alice' }))
  winner.tracker.update(inProgress({ status: 'completed', winner_user_id: 'alice' }))
  assert.equal(winner.calls.victory, 1)
  winner.tracker.update(inProgress({ status: 'completed', winner_user_id: 'alice' }))
  assert.equal(winner.calls.victory, 1, 'duplicate completion event must be ignored')

  const loser = trackerFor('bob')
  loser.tracker.update(inProgress())
  loser.tracker.update(inProgress({ status: 'completed', winner_user_id: 'alice' }))
  assert.equal(loser.calls.defeat, 1)
  assert.equal(loser.calls.victory, 0)
})

test('tie plays the draw sound once, not victory/defeat', () => {
  const { tracker, calls } = trackerFor('alice')
  tracker.update(inProgress())
  tracker.update(inProgress({ status: 'tied', winner_user_id: null }))
  tracker.update(inProgress({ status: 'tied', winner_user_id: null }))
  assert.equal(calls.draw, 1)
  assert.equal(calls.victory, 0)
  assert.equal(calls.defeat, 0)
})

test('countdown ticks once per second inside the threshold and never after the match ends', () => {
  const { tracker, calls } = trackerFor('alice')

  tracker.update(inProgress(), 15)
  tracker.update(inProgress(), 6)
  assert.deepEqual(calls.ticks, [])

  tracker.update(inProgress(), 5)
  tracker.update(inProgress(), 5) // re-render at the same second
  assert.deepEqual(calls.ticks, [5])

  tracker.update(inProgress(), 4)
  tracker.update(inProgress(), 3)
  tracker.update(inProgress(), 2)
  tracker.update(inProgress(), 1)
  assert.deepEqual(calls.ticks, [5, 4, 3, 2, 1])

  tracker.update(inProgress(), 0)
  assert.deepEqual(calls.ticks, [5, 4, 3, 2, 1])

  tracker.update(inProgress({ status: 'completed', winner_user_id: 'bob' }), 1)
  assert.deepEqual(calls.ticks, [5, 4, 3, 2, 1], 'no ticks after completion')
})

test('switching to a different match resets baselines and stays silent for history', () => {
  const { tracker, calls } = trackerFor('alice')
  tracker.update(inProgress({ id: 'match-1' }))
  tracker.update(inProgress({ id: 'match-1', current_turn_user_id: 'alice' }))
  assert.equal(calls.turnAlert, 1)

  // Navigating to an already-finished historical match must not replay sounds.
  tracker.update(inProgress({ id: 'match-2', status: 'completed', winner_user_id: 'alice' }))
  tracker.update(inProgress({ id: 'match-2', status: 'completed', winner_user_id: 'alice' }))
  assert.equal(calls.victory, 0)
  assert.equal(calls.turnAlert, 1)
})

test('a throwing sink never propagates into gameplay', () => {
  const sink: SoundSink = {
    playMatchStart() {},
    playMove() {
      throw new Error('audio exploded')
    },
    playTurnAlert() {},
    playTimerTick() {},
    playVictory() {},
    playDefeat() {},
    playDraw() {},
  }
  const tracker = new SoundEventTracker(sink)
  tracker.setUser('alice')

  tracker.update(inProgress())
  assert.doesNotThrow(() =>
    tracker.update(inProgress({ board_state: { cells: [1, 0, 0, 0, 0, 0, 0, 0, 0] } }))
  )
})
