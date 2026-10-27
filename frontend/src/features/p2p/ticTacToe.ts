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
  /** Shape name, also used as the accessible label. */
  readonly shape: 'diamond' | 'square'
  /** Text colour driving the SVG's currentColor. Tuned for light and dark. */
  readonly colorClass: string
  /** Soft halo applied to a marker once it is placed on the board. */
  readonly glowClass: string
}

export const MARKER_THEME: Record<TicTacToeSeat, MarkerTheme> = {
  1: {
    shape: 'diamond',
    colorClass: 'text-sky-600 dark:text-sky-400',
    glowClass: 'drop-shadow-[0_2px_7px_rgba(2,132,199,0.45)]',
  },
  2: {
    shape: 'square',
    colorClass: 'text-amber-600 dark:text-amber-400',
    glowClass: 'drop-shadow-[0_2px_7px_rgba(180,83,9,0.45)]',
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
  return MARKER_THEME[seat].shape === 'diamond' ? 'Diamond' : 'Square'
}
