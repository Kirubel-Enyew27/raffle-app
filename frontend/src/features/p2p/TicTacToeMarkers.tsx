import { MARKER_THEME, markerLabel, type TicTacToeSeat } from './ticTacToe'

/**
 * Hand-drawn seat markers: a solid faceted diamond for seat one and a hollow
 * rounded square for seat two. Both live on the same 24x24 grid, share one
 * stroke weight and one bounding box, so they read as a matched pair while
 * staying distinguishable by fill and silhouette alone (no colour needed).
 *
 * `variant` separates the three contexts the markers appear in:
 *  - placed:  on the board for a played move, with a halo and a scale-in,
 *  - preview: the dimmed ghost shown under the pointer for the current player,
 *  - plain:   in headers, legends and lobby icons, with no motion.
 */
export type MarkerVariant = 'placed' | 'preview' | 'plain'

interface MarkerProps {
  /** Sizing utilities only, e.g. "h-7 w-7". Colour always comes from the seat. */
  className?: string
  variant?: MarkerVariant
}

const DEFAULT_SIZE = 'h-7 w-7'

function markerClassName(seat: TicTacToeSeat, variant: MarkerVariant, className?: string) {
  const theme = MARKER_THEME[seat]
  return [
    'inline-block shrink-0 select-none',
    theme.colorClass,
    variant === 'placed' ? theme.glowClass : '',
    variant === 'placed' ? 'ttt-marker-place' : '',
    variant === 'preview' ? 'opacity-40' : '',
    className ?? DEFAULT_SIZE,
  ]
    .filter(Boolean)
    .join(' ')
}

function DiamondGlyph({ className, seat }: { className: string; seat: TicTacToeSeat }) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={className}
      role="img"
      aria-label={markerLabel(seat)}
      focusable="false"
    >
      <path
        d="M12 2.75 21.25 12 12 21.25 2.75 12Z"
        fill="currentColor"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinejoin="round"
      />
      <path
        d="M12 7.5 16.5 12 12 16.5 7.5 12Z"
        fill="none"
        stroke="#ffffff"
        strokeOpacity="0.4"
        strokeWidth="1.25"
        strokeLinejoin="round"
      />
    </svg>
  )
}

function SquareGlyph({ className, seat }: { className: string; seat: TicTacToeSeat }) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={className}
      role="img"
      aria-label={markerLabel(seat)}
      focusable="false"
    >
      <rect
        x="3.75"
        y="3.75"
        width="16.5"
        height="16.5"
        rx="4.75"
        fill="none"
        stroke="currentColor"
        strokeWidth="2.5"
      />
    </svg>
  )
}

export function TicTacToeMarker({
  seat,
  className,
  variant = 'placed',
}: MarkerProps & { seat: TicTacToeSeat }) {
  const classes = markerClassName(seat, variant, className)
  return seat === 1 ? <DiamondGlyph className={classes} seat={seat} /> : <SquareGlyph className={classes} seat={seat} />
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
