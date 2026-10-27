import test from 'node:test'
import assert from 'node:assert/strict'
import { extractMatchOutcome } from './outcomeExtractor.ts'
import type { P2PMatch } from '@/types/api'

const baseMatch: P2PMatch = {
  id: 'match-123',
  game_slug: 'tic-tac-toe',
  status: 'in_progress',
  match_type: 'public_queue',
  stake_amount: 10,
  prize_pool: 19,
  service_fee_percent: 5,
  total_fee: 1,
  winner_user_id: undefined,
  current_turn_user_id: 'user-0',
  turn_seconds_left: 15,
  current_round: 1,
  max_rounds: 3,
  scores: { '0': 0, '1': 0 },
  board_state: {},
  created_by: 'user-0',
  created_at: new Date().toISOString(),
  players: [
    { id: 'p0', match_id: 'match-123', user_id: 'user-0', player_index: 0, symbol: 'X', status: 'ready', score: 0, joined_at: new Date().toISOString() },
    { id: 'p1', match_id: 'match-123', user_id: 'user-1', player_index: 1, symbol: 'O', status: 'ready', score: 0, joined_at: new Date().toISOString() },
  ],
}

test('extractMatchOutcome returns null for in_progress match', () => {
  const result = extractMatchOutcome(baseMatch, 'user-0')
  assert.equal(result, null)
})

test('Tic-Tac-Toe win highlights the 3 decisive winning cells', () => {
  const match: P2PMatch = {
    ...baseMatch,
    status: 'completed',
    winner_user_id: 'user-0',
    scores: { '0': 2, '1': 0 },
    board_state: {
      cells: [1, 1, 1, 2, 2, 0, 0, 0, 0],
      winning_line: [0, 1, 2],
    },
  }

  const outcomeForWinner = extractMatchOutcome(match, 'user-0')
  assert.ok(outcomeForWinner)
  assert.equal(outcomeForWinner.type, 'win')
  assert.equal(outcomeForWinner.title, 'YOU WIN!')
  assert.equal(outcomeForWinner.reason, 'line_completion')
  assert.deepEqual(outcomeForWinner.decisiveIndices, [0, 1, 2])
  assert.match(outcomeForWinner.subtitle, /3 in a Row/i)

  const outcomeForLoser = extractMatchOutcome(match, 'user-1')
  assert.ok(outcomeForLoser)
  assert.equal(outcomeForLoser.type, 'loss')
  assert.equal(outcomeForLoser.title, 'YOU LOSE!')
  assert.deepEqual(outcomeForLoser.decisiveIndices, [0, 1, 2])
})

test('Tic-Tac-Toe draw correctly flags full board stalemate', () => {
  const match: P2PMatch = {
    ...baseMatch,
    status: 'tied',
    winner_user_id: undefined,
    scores: { '0': 1, '1': 1 },
    board_state: {
      cells: [1, 2, 1, 1, 2, 2, 2, 1, 1],
      winning_line: null,
    },
  }

  const outcome = extractMatchOutcome(match, 'user-0')
  assert.ok(outcome)
  assert.equal(outcome.type, 'draw')
  assert.equal(outcome.title, 'DRAW!')
  assert.equal(outcome.reason, 'draw_full_board')
  assert.equal(outcome.decisiveIndices?.length, 9)
})

test('Connect Four win highlights 4 connected tokens', () => {
  const match: P2PMatch = {
    ...baseMatch,
    game_slug: 'connect-four',
    status: 'completed',
    winner_user_id: 'user-1',
    board_state: {
      cells: new Array(42).fill(0),
      winning_line: [35, 36, 37, 38],
    },
  }

  const outcome = extractMatchOutcome(match, 'user-1')
  assert.ok(outcome)
  assert.equal(outcome.type, 'win')
  assert.equal(outcome.reason, 'line_completion')
  assert.deepEqual(outcome.decisiveIndices, [35, 36, 37, 38])
  assert.match(outcome.subtitle, /4 in a Row/i)
})

test('Rock-Paper-Scissors accurately communicates decisive matchup and series score', () => {
  const match: P2PMatch = {
    ...baseMatch,
    game_slug: 'rock-paper-scissors',
    status: 'completed',
    winner_user_id: 'user-0',
    scores: { '0': 2, '1': 1 },
    board_state: {
      last_round_result: {
        p0_choice: 'paper',
        p1_choice: 'rock',
        winner: 0,
        is_tie: false,
      },
    },
  }

  const outcome = extractMatchOutcome(match, 'user-0')
  assert.ok(outcome)
  assert.equal(outcome.type, 'win')
  assert.equal(outcome.reason, 'rps_showdown')
  assert.equal(outcome.rpsData?.myChoice, 'paper')
  assert.equal(outcome.rpsData?.opponentChoice, 'rock')
  assert.match(outcome.rpsData?.interactionText || '', /Paper covers Rock/i)
  assert.equal(outcome.rpsData?.myWins, 2)
  assert.equal(outcome.rpsData?.opponentWins, 1)
})

test('Ethiopian Dama distinguishes elimination from immobilization and draw rules', () => {
  // 1. Elimination
  const eliminationMatch: P2PMatch = {
    ...baseMatch,
    game_slug: 'ethiopian-dama',
    status: 'completed',
    winner_user_id: 'user-0',
    board_state: {
      cells: [1, 0, 0, 0, 0, 0, 0, 0], // Only light pieces left, dark pieces 0
      last_move: { from: 10, to: 1, captures: [5] },
      result: { winner: 0, draw: false, reason: 'no_legal_moves' },
    },
  }

  const elimOutcome = extractMatchOutcome(eliminationMatch, 'user-0')
  assert.ok(elimOutcome)
  assert.equal(elimOutcome.type, 'win')
  assert.equal(elimOutcome.reason, 'elimination')
  assert.match(elimOutcome.subtitle, /All opponent pieces captured/i)
  assert.deepEqual(elimOutcome.decisiveIndices, [10, 1, 5])

  // 2. Immobilization (opponent pieces exist but are blocked)
  const immobilizationMatch: P2PMatch = {
    ...baseMatch,
    game_slug: 'ethiopian-dama',
    status: 'completed',
    winner_user_id: 'user-0',
    board_state: {
      // Light has 2 pieces, dark has 1 piece trapped at index 7
      cells: [1, 1, 0, 0, 0, 0, 0, 3],
      result: { winner: 0, draw: false, reason: 'no_legal_moves' },
    },
  }

  const blockOutcome = extractMatchOutcome(immobilizationMatch, 'user-0')
  assert.ok(blockOutcome)
  assert.equal(blockOutcome.type, 'win')
  assert.equal(blockOutcome.reason, 'immobilization')
  assert.match(blockOutcome.subtitle, /Opponent has no legal moves/i)
  assert.deepEqual(blockOutcome.secondaryIndices, [7])

  // 3. Threefold Repetition Draw
  const drawMatch: P2PMatch = {
    ...baseMatch,
    game_slug: 'ethiopian-dama',
    status: 'tied',
    board_state: {
      cells: [1, 3],
      result: { winner: null, draw: true, reason: 'draw_repetition' },
    },
  }

  const drawOutcome = extractMatchOutcome(drawMatch, 'user-0')
  assert.ok(drawOutcome)
  assert.equal(drawOutcome.type, 'draw')
  assert.equal(drawOutcome.reason, 'draw_repetition')
  assert.match(drawOutcome.subtitle, /Threefold Repetition/i)
})

test('Resignation/forfeit produces clear surrender explanation without fake winning lines', () => {
  const match: P2PMatch = {
    ...baseMatch,
    status: 'forfeited',
    winner_user_id: 'user-0',
  }

  const winnerView = extractMatchOutcome(match, 'user-0')
  assert.ok(winnerView)
  assert.equal(winnerView.type, 'win')
  assert.equal(winnerView.reason, 'forfeited')
  assert.match(winnerView.subtitle, /Opponent surrendered/i)
  assert.equal(winnerView.decisiveIndices, undefined)

  const resignerView = extractMatchOutcome(match, 'user-1')
  assert.ok(resignerView)
  assert.equal(resignerView.type, 'loss')
  assert.equal(resignerView.reason, 'forfeited')
  assert.match(resignerView.subtitle, /You surrendered/i)
})
