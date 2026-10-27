/**
 * Pure helpers for the Connect Four board UI.
 *
 * The server owns drops, gravity, win detection and the draw check
 * (backend/internal/p2p/domain/rules.go). These helpers only translate the
 * authoritative cells into display concepts so the board can be tested without
 * a DOM, and so styling never has to re-implement a rule.
 */

export type ConnectFourSeat = 1 | 2

export const EMPTY_SLOT = 0

/** Board cells are 0 for empty, 1 for seat one, 2 for seat two. */
export function tokenSeat(cell: number | null | undefined): ConnectFourSeat | null {
  if (cell === 1) return 1
  if (cell === 2) return 2
  return null
}

/** Seat one is the server's "RED" player, seat two its "YELLOW" player. */
export function seatForSymbol(symbol: string | null | undefined): ConnectFourSeat | null {
  switch (symbol?.trim().toUpperCase()) {
    case 'RED':
      return 1
    case 'YELLOW':
      return 2
    default:
      return null
  }
}

/** A column is full when its top slot (row 0) is taken. */
export function isColumnFull(cells: number[] | undefined, column: number, columns: number): boolean {
  if (!cells || !Number.isInteger(column) || column < 0 || column >= columns) return false
  return cells[column] !== EMPTY_SLOT
}

/** Index the next token will occupy, or null when the column is full. */
export function landingIndex(
  cells: number[] | undefined,
  column: number,
  columns: number,
  rows: number
): number | null {
  if (!cells || !Number.isInteger(column) || column < 0 || column >= columns) return null
  for (let row = rows - 1; row >= 0; row -= 1) {
    const index = row * columns + column
    if (cells[index] === EMPTY_SLOT) return index
  }
  return null
}

export function isWinningIndex(winningLine: number[] | null | undefined, index: number): boolean {
  return Array.isArray(winningLine) && winningLine.includes(index)
}

/**
 * How far above its final slot a token starts falling from, in token heights.
 * The bottom row travels the full board, the top row one slot.
 */
export function dropFallOffset(row: number, rows: number): string {
  const clampedRow = Math.min(Math.max(Math.trunc(row), 0), Math.max(rows - 1, 0))
  return `${-(clampedRow + 1) * 108}%`
}
