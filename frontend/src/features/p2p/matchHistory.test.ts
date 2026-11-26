import test from 'node:test'
import assert from 'node:assert/strict'
import { matchOutcome, isTerminalMatch } from './matchHistory.ts'
import type { P2PMatch } from '@/types/api'

const baseMatch: P2PMatch = {
  id: 'match-hist-1',
  game_slug: 'tic-tac-toe',
  status: 'completed',
  match_type: 'public_queue',
  competition_mode: 'fun',
  service_fee_bp: 500,
  potential_payout: 0,
  stake_amount: 0,
  prize_pool: 0,
  service_fee_percent: 5,
  total_fee: 0,
  winner_user_id: undefined,
  current_turn_user_id: null,
  turn_seconds_left: 0,
  disconnect_seconds_left: 0,
  current_round: 1,
  max_rounds: 3,
  scores: { '0': 0, '1': 0 },
  board_state: {},
  created_by: 'user-a',
  created_at: new Date().toISOString(),
  players: [
    { id: 'p0', match_id: 'match-hist-1', user_id: 'user-a', player_index: 0, symbol: 'X', status: 'ready', score: 0, joined_at: '' },
    { id: 'p1', match_id: 'match-hist-1', user_id: 'user-b', player_index: 1, symbol: 'O', status: 'ready', score: 0, joined_at: '' },
  ],
}

test('matchOutcome accurately distinguishes won, lost, draw, and cancelled', () => {
  // 1. Won
  const wonMatch: P2PMatch = { ...baseMatch, status: 'completed', winner_user_id: 'user-a' }
  assert.equal(matchOutcome(wonMatch, 'user-a').label, 'Won')
  assert.equal(matchOutcome(wonMatch, 'user-b').label, 'Lost')

  // 2. Draw
  const drawMatch: P2PMatch = { ...baseMatch, status: 'tied', winner_user_id: undefined }
  assert.equal(matchOutcome(drawMatch, 'user-a').label, 'Draw')
  assert.equal(matchOutcome(drawMatch, 'user-b').label, 'Draw')

  // 3. Cancelled
  const cancelledMatch: P2PMatch = { ...baseMatch, status: 'cancelled', winner_user_id: undefined }
  const creatorOutcome = matchOutcome(cancelledMatch, 'user-a')
  assert.equal(creatorOutcome.label, 'Cancelled')
  assert.notEqual(creatorOutcome.label, 'Lost')
  assert.notEqual(creatorOutcome.label, 'Won')

  const opponentOutcome = matchOutcome(cancelledMatch, 'user-b')
  assert.equal(opponentOutcome.label, 'Cancelled')
  assert.notEqual(opponentOutcome.label, 'Lost')
  assert.notEqual(opponentOutcome.label, 'Won')

  // 4. Terminal status check
  assert.equal(isTerminalMatch(cancelledMatch), true)
  assert.equal(isTerminalMatch(wonMatch), true)
  assert.equal(isTerminalMatch(drawMatch), true)
})
