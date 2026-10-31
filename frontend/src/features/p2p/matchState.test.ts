import assert from 'node:assert/strict'
import test from 'node:test'

import type { P2PMatch } from '@/types/api'
import { hasMatchVersionGap, mergeMatchSnapshot } from './matchState.ts'

function snapshot(id: string, state_version: number, board: string): P2PMatch {
  return {
    id,
    state_version,
    game_slug: 'tic-tac-toe',
    status: 'in_progress',
    match_type: 'private_room',
    stake_amount: 0,
    prize_pool: 0,
    service_fee_percent: 0,
    total_fee: 0,
    turn_seconds_left: 30,
    current_round: 1,
    max_rounds: 1,
    scores: {},
    board_state: { value: board },
    created_by: 'player-a',
    disconnect_seconds_left: 0,
    players: [],
    created_at: '',
  }
}

test('newer authoritative snapshots replace older snapshots', () => {
  const older = snapshot('match-1', 7, 'old')
  const newer = snapshot('match-1', 8, 'new')

  assert.equal(mergeMatchSnapshot(older, newer), newer)
})

test('stale snapshots cannot overwrite current state and duplicate versions need a newer timestamp', () => {
  const current = snapshot('match-1', 8, 'current')

  assert.equal(mergeMatchSnapshot(current, snapshot('match-1', 7, 'stale')), current)
  assert.equal(mergeMatchSnapshot(current, snapshot('match-1', 8, 'duplicate')), current)

  const timestampedCurrent = { ...current, updated_at: '2026-10-31T05:00:01Z' }
  const timestampedNext = {
    ...snapshot('match-1', 8, 'next'),
    updated_at: '2026-10-31T05:00:02Z',
  }
  assert.equal(mergeMatchSnapshot(timestampedCurrent, timestampedNext), timestampedNext)
  assert.equal(mergeMatchSnapshot(timestampedNext, timestampedCurrent), timestampedNext)
})

test('a snapshot for another match starts a separate state stream', () => {
  const current = snapshot('match-1', 8, 'old-match')
  const nextMatch = snapshot('match-2', 1, 'new-match')

  assert.equal(mergeMatchSnapshot(current, nextMatch), nextMatch)
})

test('version gaps are detected only within the same match', () => {
  const current = snapshot('match-1', 8, 'current')

  assert.equal(hasMatchVersionGap(current, snapshot('match-1', 9, 'next')), false)
  assert.equal(hasMatchVersionGap(current, snapshot('match-1', 11, 'gap')), true)
  assert.equal(hasMatchVersionGap(current, snapshot('match-2', 11, 'other')), false)
})
