import { ConnectFourBall } from './GamePieces'
import type { ConnectFourSeat } from './connectFour'

/**
 * Connect Four tokens, rendered from the shared `ConnectFourBall` artwork: the
 * same glossy red and amber discs that appear on the game card. Seat one is
 * KWIK red, seat two is amber; the balls carry no rings, stripes or markings.
 */
export type TokenEmphasis = 'none' | 'last' | 'win'

interface ConnectFourTokenProps {
  seat: ConnectFourSeat
  /** Sizing utilities only, e.g. "h-10 w-10". */
  className?: string
  /** Winning tokens glow, the most recent drop gets a thin rim. */
  emphasis?: TokenEmphasis
  /** Placement preview: same token, see-through, never interactive. */
  ghost?: boolean
}

const SEAT_LABEL: Record<ConnectFourSeat, string> = {
  1: 'red token',
  2: 'amber token',
}

export function ConnectFourToken({ seat, className, emphasis = 'none', ghost = false }: ConnectFourTokenProps) {
  return (
    <span
      role="img"
      aria-label={SEAT_LABEL[seat]}
      className={[
        'relative inline-block aspect-square rounded-full',
        emphasis === 'win' ? 'c4-token-win' : '',
        ghost ? 'opacity-55' : '',
        className ?? 'h-10 w-10',
      ]
        .filter(Boolean)
        .join(' ')}
    >
      <ConnectFourBall seat={seat} className="absolute inset-0 h-full w-full" />
      {/* Last placed token: a thin rim so the newest move is traceable. */}
      {emphasis === 'last' && (
        <span aria-hidden="true" className="absolute inset-0 rounded-full ring-1 ring-inset ring-white/85" />
      )}
    </span>
  )
}

/** Compact both-seats badge, used wherever the game itself is illustrated. */
export function ConnectFourTokenPair({ className }: { className?: string }) {
  const size = className ?? 'h-5 w-5'
  return (
    <span className="inline-flex items-center gap-0.5">
      <ConnectFourToken seat={1} className={size} />
      <ConnectFourToken seat={2} className={size} />
    </span>
  )
}
