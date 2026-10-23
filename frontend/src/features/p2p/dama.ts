/**
 * Pure helpers for the Ethiopian Dama board.
 *
 * The server owns the rules. These helpers only translate the authoritative
 * board payload into display concepts (orientation, squares, notation) so they
 * can be unit-tested without a DOM or a network connection.
 */

export const DAMA_BOARD_SIZE = 8

export interface DamaStep {
  from: number
  to: number
  captures?: number[]
  promotion?: boolean
  continues?: boolean
}

export interface DamaLastMove {
  from: number
  to: number
  captures?: number[]
  promotion?: boolean
}

export interface DamaHistoryEntry {
  seq: number
  player: number
  from: number
  to: number
  captures?: number[]
  promotion?: boolean
}

export interface DamaResult {
  winner?: number | null
  draw: boolean
  reason: string
}

export interface DamaBoardState {
  variant?: string
  board_size?: number
  cells?: number[]
  turn?: number
  turn_seq?: number
  move_count?: number
  halfmove_clock?: number
  must_continue_from?: number | null
  legal_moves?: DamaStep[]
  last_move?: DamaLastMove | null
  move_history?: DamaHistoryEntry[]
  result?: DamaResult | null
  rules?: Record<string, unknown>
}

export type DamaSide = 0 | 1

export interface DamaDisplaySquare {
  index: number
  playable: boolean
  value: number
}

/** 0 empty, 1 light man, 2 light king, 3 dark man, 4 dark king. */
export function isPlayableSquare(index: number): boolean {
  if (!Number.isInteger(index) || index < 0 || index >= DAMA_BOARD_SIZE * DAMA_BOARD_SIZE) {
    return false
  }
  const row = Math.floor(index / DAMA_BOARD_SIZE)
  const col = index % DAMA_BOARD_SIZE
  return (row + col) % 2 === 1
}

/** Returns the owning side, or null for an empty square. */
export function pieceSide(value: number | undefined): DamaSide | null {
  if (value === 1 || value === 2) return 0
  if (value === 3 || value === 4) return 1
  return null
}

export function pieceIsKing(value: number | undefined): boolean {
  return value === 2 || value === 4
}

/**
 * Flips the board so the local player is always at the bottom. The same
 * function converts both directions (it is its own inverse).
 */
export function orientSquare(index: number, orientation: DamaSide): number {
  if (!Number.isInteger(index) || index < 0 || index >= DAMA_BOARD_SIZE * DAMA_BOARD_SIZE) {
    return index
  }
  return orientation === 1 ? DAMA_BOARD_SIZE * DAMA_BOARD_SIZE - 1 - index : index
}

/** Board orientation for a seat: dark sits at the bottom on a dark screen. */
export function orientationFor(playerIndex: number | undefined): DamaSide {
  return playerIndex === 1 ? 1 : 0
}

/**
 * Produces the 64 display cells in visual order for the given orientation.
 * Keeping this pure makes the board's mapping testable without a DOM.
 */
export function displaySquares(cells: number[] | undefined, orientation: DamaSide): DamaDisplaySquare[] {
  return Array.from({ length: DAMA_BOARD_SIZE * DAMA_BOARD_SIZE }, (_, displayIndex) => {
    const index = orientSquare(displayIndex, orientation)
    return {
      index,
      playable: isPlayableSquare(index),
      value: cells?.[index] ?? 0,
    }
  })
}

/** Algebraic square name, independent of the viewing orientation. */
export function squareName(index: number): string {
  const row = Math.floor(index / DAMA_BOARD_SIZE)
  const col = index % DAMA_BOARD_SIZE
  const file = String.fromCharCode('a'.charCodeAt(0) + col)
  return `${file}${DAMA_BOARD_SIZE - row}`
}

export function legalStepsFrom(boardState: DamaBoardState | undefined, from: number | null): DamaStep[] {
  if (!boardState || from === null || !Array.isArray(boardState.legal_moves)) return []
  return boardState.legal_moves.filter((step) => step && step.from === from)
}

export function legalTargetsFrom(boardState: DamaBoardState | undefined, from: number | null): number[] {
  return legalStepsFrom(boardState, from).map((step) => step.to)
}

export function findStep(
  boardState: DamaBoardState | undefined,
  from: number,
  to: number
): DamaStep | undefined {
  return legalStepsFrom(boardState, from).find((step) => step.to === to)
}

export function isCaptureStep(step: DamaStep | undefined): boolean {
  return !!step && Array.isArray(step.captures) && step.captures.length > 0
}

/** Compact algebraic notation for the move list. */
export function moveNotation(entry: Pick<DamaHistoryEntry, 'from' | 'to' | 'captures' | 'promotion'>): string {
  const separator = entry.captures && entry.captures.length > 0 ? '×' : '–'
  const promotion = entry.promotion ? '=K' : ''
  return `${squareName(entry.from)}${separator}${squareName(entry.to)}${promotion}`
}

/** A short, human-readable result line for the match page. */
export function resultLabel(result: DamaResult | null | undefined, myPlayerIndex: number | undefined): string {
  if (!result) return 'Match in progress'
  if (result.draw) {
    switch (result.reason) {
      case 'draw_repetition':
        return 'Draw — threefold repetition'
      case 'draw_halfmove':
        return 'Draw — 50-move rule'
      default:
        return 'Draw'
    }
  }
  if (result.winner === null || result.winner === undefined) return 'Match finished'
  if (myPlayerIndex !== undefined && result.winner === myPlayerIndex) return 'Victory'
  return 'Defeat'
}
