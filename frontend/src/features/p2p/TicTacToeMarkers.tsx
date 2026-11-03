import { TicTacToeGlyph } from './GamePieces'
import { MARKER_THEME, markerLabel, type TicTacToeSeat } from './ticTacToe'

/**
 * Tic-Tac-Toe seat marks, rendered from the shared `TicTacToeGlyph` artwork:
 * the dimensional red X and cream O that also appear on the game card.
 *
 * `variant` separates the three contexts the marks appear in:
 *  - placed:  on the board for a played move, with a halo and a scale-in,
 *  - preview: the dimmed ghost shown under the pointer for the current player,
 *  - plain:   in headers, legends and lobby icons, with no motion.
 */
export type MarkerVariant = 'placed' | 'preview' | 'plain'

interface MarkerProps {
  /** Sizing utilities only, e.g. "h-7 w-7". */
  className?: string
  variant?: MarkerVariant
}

const DEFAULT_SIZE = 'h-7 w-7'

function markerClassName(seat: TicTacToeSeat, variant: MarkerVariant, className?: string) {
  const theme = MARKER_THEME[seat]
  return [
    'inline-block shrink-0 select-none',
    variant === 'placed' ? theme.glowClass : '',
    variant === 'placed' ? 'ttt-marker-place' : '',
    variant === 'preview' ? 'opacity-40' : '',
    className ?? DEFAULT_SIZE,
  ]
    .filter(Boolean)
    .join(' ')
}

export function TicTacToeMarker({
  seat,
  className,
  variant = 'placed',
}: MarkerProps & { seat: TicTacToeSeat }) {
  const classes = markerClassName(seat, variant, className)
  return (
    <span role="img" aria-label={markerLabel(seat)} className={classes}>
      <TicTacToeGlyph seat={seat} className="h-full w-full" />
    </span>
  )
}

/** Seat one: the room creator and opening mover. */
export function PlayerOneMarker(props: MarkerProps) {
  return <TicTacToeMarker seat={1} {...props} />
}

/** Seat two: the player who joined the room. */
export function PlayerTwoMarker(props: MarkerProps) {
  return <TicTacToeMarker seat={2} {...props} />
}

/** Compact both-seats badge, used wherever the game itself is illustrated. */
export function TicTacToeMarkerPair({ className }: { className?: string }) {
  return (
    <span className="inline-flex items-center gap-1" aria-hidden="true">
      <PlayerOneMarker variant="plain" className={className ?? 'h-5 w-5'} />
      <PlayerTwoMarker variant="plain" className={className ?? 'h-5 w-5'} />
    </span>
  )
}
