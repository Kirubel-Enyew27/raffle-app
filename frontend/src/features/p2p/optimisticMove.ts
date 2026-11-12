import type { P2PMatch } from '@/types/api'
import { landingIndex } from './connectFour.ts'
import { findStep, pieceSide } from './dama.ts'
import type { RpsChoice } from './rockPaperScissors.ts'

/**
 * A move that has been painted locally but not yet acknowledged by the server.
 *
 * This overlay is presentation-only. It never changes the cached authoritative
 * match, never advances the saved game, and is discarded if the server rejects
 * the action. The server still validates membership, turn, legality, version
 * and outcome before the move exists anywhere durable.
 */
export type OptimisticMove =
  | {
      game: 'tic-tac-toe'
      playerIndex: number
      cell: number
    }
  | {
      game: 'connect-four'
      playerIndex: number
      column: number
      /** Moves already committed when the drop was painted. */
      baseMoveCount: number
    }
  | {
      game: 'ethiopian-dama'
      playerIndex: number
      from: number
      to: number
    }
  | {
      game: 'rock-paper-scissors'
      playerIndex: number
      choice: RpsChoice
    }

function numeric(value: unknown, fallback = 0): number {
  const parsed = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(parsed) ? parsed : fallback
}

function asCells(value: unknown): number[] {
  return Array.isArray(value) ? value.map((cell) => numeric(cell)) : []
}

function boardOf(match: P2PMatch): Record<string, unknown> {
  return match.board_state ?? {}
}

/**
 * Applies the overlay to a copy of the newest authoritative snapshot.
 *
 * Each branch first checks whether the authoritative snapshot already contains
 * the move. That makes the overlay idempotent: if an SSE event wins the race
 * against the REST acknowledgement, removing the overlay cannot move a second
 * piece or drop a second Connect Four token.
 */
export function applyOptimisticMove(match: P2PMatch, move: OptimisticMove | null): P2PMatch {
  if (!move || match.status !== 'in_progress' || match.game_slug !== move.game) return match
  const board = boardOf(match)

  if (move.game === 'tic-tac-toe') {
    const cells = asCells(board.cells)
    if (cells.length !== 9 || !Number.isInteger(move.cell) || move.cell < 0 || move.cell > 8) {
      return match
    }
    if (cells[move.cell] !== 0) return match

    const nextCells = [...cells]
    nextCells[move.cell] = move.playerIndex + 1
    return {
      ...match,
      board_state: {
        ...board,
        cells: nextCells,
        last_move: move.cell,
        move_count: numeric(board.move_count) + 1,
      },
    }
  }

  if (move.game === 'connect-four') {
    const columns = numeric(board.columns, 7)
    const rows = numeric(board.rows, 6)
    const cells = asCells(board.cells)
    if (cells.length !== columns * rows || !Number.isInteger(move.column)) return match
    // A newer commit means the server has already applied this drop (or moved
    // past it); painting another token would double-drop the same column.
    if (numeric(board.move_count) > move.baseMoveCount) return match

    const index = landingIndex(cells, move.column, columns, rows)
    // The column is full, or the authoritative snapshot already contains the
    // token this overlay was about to add.
    if (index === null || cells[index] !== 0) return match

    const nextCells = [...cells]
    nextCells[index] = move.playerIndex + 1
    return {
      ...match,
      board_state: {
        ...board,
        cells: nextCells,
        last_move: {
          row: Math.floor(index / columns),
          col: move.column,
          index,
        },
        move_count: numeric(board.move_count) + 1,
      },
    }
  }

  if (move.game === 'ethiopian-dama') {
    const cells = asCells(board.cells)
    if (cells.length !== 64 || !Number.isInteger(move.from) || !Number.isInteger(move.to)) {
      return match
    }
    if (pieceSide(cells[move.from]) !== move.playerIndex) return match

    const history = Array.isArray(board.move_history) ? board.move_history : []
    const last = history[history.length - 1] as
      | { seq?: number; player?: number; from?: number; to?: number }
      | undefined
    // The server already applied this hop.
    if (last && last.player === move.playerIndex && last.from === move.from && last.to === move.to) {
      return match
    }

    const step = findStep(board as never, move.from, move.to)
    if (!step) return match

    const nextCells = [...cells]
    const piece = nextCells[move.from]
    nextCells[move.from] = 0
    nextCells[move.to] = step.promotion ? (piece === 1 ? 2 : piece === 3 ? 4 : piece) : piece
    for (const captured of step.captures ?? []) {
      nextCells[captured] = 0
    }

    const seq = numeric(last?.seq) + 1
    return {
      ...match,
      board_state: {
        ...board,
        cells: nextCells,
        turn: step.continues ? move.playerIndex : 1 - move.playerIndex,
        turn_seq: numeric(board.turn_seq) + 1,
        move_count: numeric(board.move_count) + 1,
        must_continue_from: step.continues ? move.to : null,
        legal_moves: [],
        last_move: {
          from: move.from,
          to: move.to,
          ...(step.captures?.length ? { captures: step.captures } : {}),
          ...(step.chain_captures?.length ? { chain_captures: step.chain_captures } : {}),
          ...(step.promotion ? { promotion: true } : {}),
        },
        move_history: [
          ...history,
          {
            seq,
            player: move.playerIndex,
            from: move.from,
            to: move.to,
            ...(step.captures?.length ? { captures: step.captures } : {}),
            ...(step.chain_captures?.length ? { chain_captures: step.chain_captures } : {}),
            ...(step.promotion ? { promotion: true } : {}),
          },
        ],
      },
    }
  }

  const key = String(move.playerIndex)
  const choices =
    board.choices && typeof board.choices === 'object'
      ? (board.choices as Record<string, unknown>)
      : {}
  // Already locked in on the server, or this is a repeated click.
  if (choices[key] !== undefined) return match

  return {
    ...match,
    board_state: {
      ...board,
      choices: { ...choices, [key]: move.choice },
      move_count: numeric(board.move_count) + 1,
    },
  }
}

/** Builds the presentation overlay for an action, or null if it is not a move. */
export function buildOptimisticMove(
  match: P2PMatch | undefined,
  userID: string | undefined,
  action: Record<string, unknown>,
): OptimisticMove | null {
  if (!match || !userID || match.status !== 'in_progress') return null
  const player = match.players?.find((candidate) => candidate.user_id === userID)
  if (!player) return null
  const playerIndex = player.player_index

  switch (match.game_slug) {
    case 'tic-tac-toe': {
      const cell = Number(action.cell)
      return Number.isInteger(cell) && cell >= 0 && cell < 9
        ? { game: 'tic-tac-toe', playerIndex, cell }
        : null
    }
    case 'connect-four': {
      const column = Number(action.column)
      return Number.isInteger(column) && column >= 0
        ? {
            game: 'connect-four',
            playerIndex,
            column,
            baseMoveCount: numeric(match.board_state?.move_count),
          }
        : null
    }
    case 'ethiopian-dama': {
      const from = Number(action.from)
      const to = Number(action.to)
      return Number.isInteger(from) && Number.isInteger(to)
        ? { game: 'ethiopian-dama', playerIndex, from, to }
        : null
    }
    case 'rock-paper-scissors': {
      const choice = action.choice
      return choice === 'rock' || choice === 'paper' || choice === 'scissors'
        ? { game: 'rock-paper-scissors', playerIndex, choice }
        : null
    }
    default:
      return null
  }
}
