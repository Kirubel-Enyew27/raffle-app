import { test, describe } from 'node:test'
import assert from 'node:assert/strict'

import { applyOptimisticMove, buildOptimisticMove } from './optimisticMove.ts'
import type { P2PMatch, P2PMatchPlayer } from '@/types/api'

function player(index: number, userID: string): P2PMatchPlayer {
  return {
    id: `player-${index}`,
    match_id: 'match-1',
    user_id: userID,
    player_index: index,
    symbol: index === 0 ? 'X' : 'O',
    status: 'active',
    score: 0,
    joined_at: '2024-01-01T00:00:00Z',
  }
}

function match(overrides: Partial<P2PMatch> = {}): P2PMatch {
  return {
    id: 'match-1',
    state_version: 7,
    game_slug: 'tic-tac-toe',
    status: 'in_progress',
    match_type: 'private_room',
    competition_mode: 'fun',
    stake_amount: 0,
    prize_pool: 0,
    service_fee_bp: 0,
    service_fee_percent: 0,
    total_fee: 0,
    potential_payout: 0,
    current_turn_user_id: 'user-a',
    turn_seconds_left: 15,
    current_round: 1,
    max_rounds: 3,
    scores: { '0': 0, '1': 0 },
    board_state: {
      cells: new Array(9).fill(0),
      last_move: null,
      move_count: 0,
    },
    created_by: 'user-a',
    disconnect_seconds_left: 0,
    players: [player(0, 'user-a'), player(1, 'user-b')],
    created_at: '2024-01-01T00:00:00Z',
    ...overrides,
  }
}

describe('tic-tac-toe optimistic move', () => {
  test('paints the local marker without changing the authoritative version', () => {
    const current = match()
    const next = applyOptimisticMove(current, {
      game: 'tic-tac-toe',
      playerIndex: 0,
      cell: 4,
    })
    assert.equal(next.board_state.cells[4], 1)
    assert.equal(next.board_state.last_move, 4)
    assert.equal(next.board_state.move_count, 1)
    assert.equal(next.state_version, current.state_version)
    assert.equal(current.board_state.cells[4], 0, 'the authoritative snapshot must stay untouched')
  })

  test('is idempotent once the server snapshot contains the move', () => {
    const settled = match({
      board_state: { cells: [1, 0, 0, 0, 0, 0, 0, 0, 0], last_move: 0, move_count: 1 },
    })
    assert.equal(
      applyOptimisticMove(settled, { game: 'tic-tac-toe', playerIndex: 0, cell: 0 }),
      settled,
    )
  })

  test('builds an overlay only for the seated player and a live match', () => {
    assert.deepEqual(buildOptimisticMove(match(), 'user-a', { cell: '3' }), {
      game: 'tic-tac-toe',
      playerIndex: 0,
      cell: 3,
    })
    assert.equal(buildOptimisticMove(match(), 'outsider', { cell: 3 }), null)
    assert.equal(buildOptimisticMove(match({ status: 'completed' }), 'user-a', { cell: 3 }), null)
  })
})

describe('connect-four optimistic move', () => {
  test('drops the token into the lowest free slot immediately', () => {
    const cells = new Array(42).fill(0)
    const current = match({ game_slug: 'connect-four', board_state: { cells, columns: 7, rows: 6 } })
    const next = applyOptimisticMove(current, {
      game: 'connect-four',
      playerIndex: 0,
      column: 0,
      baseMoveCount: 0,
    })
    assert.equal(next.board_state.cells[35], 1)
    assert.deepEqual(next.board_state.last_move, { row: 5, col: 0, index: 35 })
  })

  test('does not add a second token when the server already applied the drop', () => {
    const cells = new Array(42).fill(0)
    cells[35] = 1
    const settled = match({
      game_slug: 'connect-four',
      board_state: {
        cells,
        columns: 7,
        rows: 6,
        last_move: { row: 5, col: 0, index: 35 },
        move_count: 1,
      },
    })
    assert.equal(
      applyOptimisticMove(settled, {
        game: 'connect-four',
        playerIndex: 0,
        column: 0,
        baseMoveCount: 0,
      }),
      settled,
    )
  })
})

describe('Ethiopian Dama optimistic move', () => {
  test('moves and captures pieces immediately and stages the history animation', () => {
    const cells = new Array(64).fill(0)
    cells[40] = 1 // a3, ivory
    cells[26] = 3 // d6, crimson
    const current = match({
      game_slug: 'ethiopian-dama',
      board_state: {
        cells,
        turn: 0,
        turn_seq: 4,
        move_count: 3,
        legal_moves: [{ from: 40, to: 33, captures: [26], continues: false }],
        move_history: [{ seq: 3, player: 1, from: 20, to: 27 }],
      },
    })
    const next = applyOptimisticMove(current, {
      game: 'ethiopian-dama',
      playerIndex: 0,
      from: 40,
      to: 33,
    })
    assert.equal(next.board_state.cells[40], 0)
    assert.equal(next.board_state.cells[33], 1)
    assert.equal(next.board_state.cells[26], 0)
    assert.equal(next.board_state.turn, 1)
    assert.equal(next.board_state.turn_seq, 5)
    assert.equal(next.board_state.must_continue_from, null)
    assert.deepEqual(next.board_state.move_history.at(-1), {
      seq: 4,
      player: 0,
      from: 40,
      to: 33,
      captures: [26],
    })
  })

  test('promotes a man during the optimistic move and keeps a capture chain with the same player', () => {
    const cells = new Array(64).fill(0)
    cells[8] = 1
    const current = match({
      game_slug: 'ethiopian-dama',
      board_state: {
        cells,
        turn: 0,
        turn_seq: 0,
        move_count: 0,
        legal_moves: [{ from: 8, to: 1, promotion: true, continues: true }],
        move_history: [],
      },
    })
    const next = applyOptimisticMove(current, {
      game: 'ethiopian-dama',
      playerIndex: 0,
      from: 8,
      to: 1,
    })
    assert.equal(next.board_state.cells[1], 2, 'promoted man becomes a king')
    assert.equal(next.board_state.turn, 0, 'a capture chain keeps the turn')
    assert.equal(next.board_state.must_continue_from, 1)
    assert.equal(next.board_state.move_history.at(-1).promotion, true)
  })

  test('carries the complete multi-capture set into the optimistic history', () => {
    const cells = new Array(64).fill(0)
    cells[35] = 1
    cells[26] = 3
    cells[10] = 3
    const current = match({
      game_slug: 'ethiopian-dama',
      board_state: {
        cells,
        turn: 0,
        turn_seq: 0,
        move_count: 0,
        legal_moves: [
          { from: 35, to: 17, captures: [26], continues: true, chain_captures: [10, 26] },
        ],
        move_history: [],
      },
    })
    const next = applyOptimisticMove(current, {
      game: 'ethiopian-dama',
      playerIndex: 0,
      from: 35,
      to: 17,
    })
    const entry = next.board_state.move_history.at(-1) as {
      chain_captures?: number[]
    }
    assert.deepEqual(entry.chain_captures, [10, 26])
  })

  test('is idempotent after the authoritative history contains the hop', () => {
    const cells = new Array(64).fill(0)
    cells[33] = 1
    const settled = match({
      game_slug: 'ethiopian-dama',
      board_state: {
        cells,
        turn: 1,
        turn_seq: 2,
        move_count: 1,
        legal_moves: [],
        move_history: [{ seq: 1, player: 0, from: 40, to: 33 }],
      },
    })
    assert.equal(
      applyOptimisticMove(settled, { game: 'ethiopian-dama', playerIndex: 0, from: 40, to: 33 }),
      settled,
    )
  })
})

describe('rock-paper-scissors optimistic move', () => {
  test('shows the local lock-in immediately without exposing it to the opponent', () => {
    const current = match({
      game_slug: 'rock-paper-scissors',
      board_state: { choices: {}, last_round_result: null, move_count: 0 },
    })
    const next = applyOptimisticMove(current, {
      game: 'rock-paper-scissors',
      playerIndex: 0,
      choice: 'rock',
    })
    assert.equal(next.board_state.choices['0'], 'rock')
    assert.equal(current.board_state.choices['0'], undefined, 'the cached authoritative choice stays hidden')
  })

  test('does not overwrite a choice already acknowledged by the server', () => {
    const settled = match({
      game_slug: 'rock-paper-scissors',
      board_state: { choices: { '0': 'rock' }, last_round_result: null, move_count: 1 },
    })
    assert.equal(
      applyOptimisticMove(settled, {
        game: 'rock-paper-scissors',
        playerIndex: 0,
        choice: 'paper',
      }),
      settled,
    )
  })
})
