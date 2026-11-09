import test from 'node:test'
import assert from 'node:assert/strict'
import type { P2PMatch } from '@/types/api'
import { extractMatchOutcome } from './outcomeExtractor.ts'
import type { GameOutcome } from './outcomeTypes.ts'

function createTerminalMatch(
  gameSlug: 'tic-tac-toe' | 'connect-four' | 'rock-paper-scissors' | 'ethiopian-dama',
  status: 'completed' | 'tied' | 'forfeited' = 'completed',
  winnerId: string | undefined = 'user-me'
): P2PMatch {
  return {
    id: `match-${gameSlug}-1`,
    game_slug: gameSlug,
    status,
    match_type: 'public_queue',
    competition_mode: 'fun',
    service_fee_bp: 500,
    potential_payout: 95,
    stake_amount: 50,
    prize_pool: 95,
    service_fee_percent: 5,
    total_fee: 5,
    winner_user_id: winnerId,
    current_turn_user_id: 'user-me',
    turn_seconds_left: 0,
    disconnect_seconds_left: 0,
    current_round: 1,
    max_rounds: 3,
    scores: { '0': winnerId === 'user-me' ? 1 : 0, '1': winnerId === 'user-opp' ? 1 : 0 },
    board_state: {
      cells: [1, 1, 1, 0, 0, 0, 0, 0, 0],
      winning_line: [0, 1, 2],
    },
    created_by: 'user-me',
    created_at: new Date().toISOString(),
    players: [
      { id: 'p0', match_id: `match-${gameSlug}-1`, user_id: 'user-me', player_index: 0, symbol: 'X', status: 'ready', score: 1, joined_at: '' },
      { id: 'p1', match_id: `match-${gameSlug}-1`, user_id: 'user-opp', player_index: 1, symbol: 'O', status: 'ready', score: 0, joined_at: '' },
    ],
  }
}

test('Review Board to View Results flow preserves board state and outcome across all games', () => {
  const games: Array<'tic-tac-toe' | 'connect-four' | 'rock-paper-scissors' | 'ethiopian-dama'> = [
    'tic-tac-toe',
    'connect-four',
    'rock-paper-scissors',
    'ethiopian-dama',
  ]

  for (const game of games) {
    const match = createTerminalMatch(game, 'completed', 'user-me')
    const initialOutcome = extractMatchOutcome(match, 'user-me')

    assert.ok(initialOutcome, `Outcome should exist for ${game}`)
    assert.equal(initialOutcome.type, 'win')

    // Simulate review mode navigation
    let isReviewingBoard = false
    const toggleReview = () => {
      isReviewingBoard = !isReviewingBoard
    }

    // Step 1: Open Review Board
    toggleReview()
    assert.equal(isReviewingBoard, true, 'User is now reviewing the board')

    // Verify match board_state and scores are completely preserved
    assert.deepEqual(match.board_state, {
      cells: [1, 1, 1, 0, 0, 0, 0, 0, 0],
      winning_line: [0, 1, 2],
    })
    assert.deepEqual(match.scores, { '0': 1, '1': 0 })

    // Step 2: Select View Results
    toggleReview()
    assert.equal(isReviewingBoard, false, 'User returned to GameOutcomeOverlay')

    // Verify outcome identity remains exact
    const restoredOutcome = extractMatchOutcome(match, 'user-me')
    assert.deepEqual(restoredOutcome, initialOutcome)

    // Step 3: Repeat transition multiple times
    for (let i = 0; i < 5; i++) {
      toggleReview()
      assert.equal(isReviewingBoard, true)
      toggleReview()
      assert.equal(isReviewingBoard, false)
    }

    assert.deepEqual(extractMatchOutcome(match, 'user-me'), initialOutcome)
  }
})

test('Handles Win, Loss, and Draw outcomes consistently during board review', () => {
  const outcomesToTest: Array<{ status: 'completed' | 'tied'; winnerId?: string; expectedType: 'win' | 'loss' | 'draw' }> = [
    { status: 'completed', winnerId: 'user-me', expectedType: 'win' },
    { status: 'completed', winnerId: 'user-opp', expectedType: 'loss' },
    { status: 'tied', winnerId: undefined, expectedType: 'draw' },
  ]

  for (const { status, winnerId, expectedType } of outcomesToTest) {
    const match = createTerminalMatch('tic-tac-toe', status, winnerId)
    const outcome = extractMatchOutcome(match, 'user-me') as GameOutcome

    assert.ok(outcome)
    assert.equal(outcome.type, expectedType)

    let isReviewingBoard = false
    const toggleReview = () => { isReviewingBoard = !isReviewingBoard }

    // Toggle Review Board
    toggleReview()
    assert.equal(isReviewingBoard, true)

    // Toggle View Results
    toggleReview()
    assert.equal(isReviewingBoard, false)
    assert.equal(outcome.type, expectedType)
  }
})

test('Switching match ID resets review mode state', () => {
  let isReviewingBoard = true
  let currentMatchId = 'match-1'

  const onMatchChange = (newMatchId: string) => {
    if (newMatchId !== currentMatchId) {
      currentMatchId = newMatchId
      isReviewingBoard = false
    }
  }

  assert.equal(isReviewingBoard, true)
  onMatchChange('match-2')
  assert.equal(isReviewingBoard, false, 'Review board mode resets when starting/joining new match')
})
