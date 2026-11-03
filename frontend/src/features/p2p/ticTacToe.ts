/**
 * Presentation model for the Tic-Tac-Toe seat markers.
 *
 * The server stays the source of truth for seats: players carry the symbols
 * "X" and "O" (symbolFor in backend/internal/p2p/application/service.go) and
 * the board is written as 0 / 1 / 2 cells. Everything below only translates
 * those numbers into a shape, so gameplay never depends on how a marker looks.
 */
export type TicTacToeSeat = 1 | 2

export interface MarkerTheme {
  /** Glyph name, also used as the accessible label. */
  readonly shape: 'x' | 'o'
  /** Soft halo applied to a marker once it is placed on the board. */
  readonly glowClass: string
  /** Short description of the mark's material, for accessible labels. */
  readonly solidLabel: string
}

export const MARKER_THEME: Record<TicTacToeSeat, MarkerTheme> = {
  1: {
    shape: 'x',
    glowClass: 'drop-shadow-[0_2px_8px_rgba(255,91,85,0.45)]',
    solidLabel: 'crimson X',
  },
  2: {
    shape: 'o',
    glowClass: 'drop-shadow-[0_2px_8px_rgba(120,110,90,0.45)]',
    solidLabel: 'ivory O',
  },
}

/** An empty board cell. Occupied cells hold the seat number itself. */
export const EMPTY_CELL = 0

/** Board cells are 0 for empty, 1 for seat one and 2 for seat two. */
export function seatForCell(cell: number | null | undefined): TicTacToeSeat | null {
  if (cell === 1) return 1
  if (cell === 2) return 2
  return null
}

/** Seats are addressed by the server-assigned symbol in the player list. */
export function seatForSymbol(symbol: string | null | undefined): TicTacToeSeat | null {
  switch (symbol?.trim().toUpperCase()) {
    case 'X':
      return 1
    case 'O':
      return 2
    default:
      return null
  }
}

/** Seat for a 0-based player index, which is how the API numbers participants. */
export function seatForPlayerIndex(index: number | null | undefined): TicTacToeSeat | null {
  if (index === 0) return 1
  if (index === 1) return 2
  return null
}

export function markerLabel(seat: TicTacToeSeat): string {
  return MARKER_THEME[seat].shape === 'x' ? 'X' : 'O'
}
