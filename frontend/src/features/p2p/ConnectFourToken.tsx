import type { ConnectFourSeat } from './connectFour'

/**
 * Drawn Connect Four tokens.
 *
 * Both seats share the same geometry — a bevelled disc, one inner ring, one
 * centre mark and one specular highlight — and differ by hue and by the shape
 * of that centre mark (seat one: a ring, seat two: a diamond). Colour alone is
 * therefore never the only cue, which keeps the two sides distinguishable for
 * colour-blind players and on washed-out WebView screens.
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

const SEAT_STYLES: Record<
  ConnectFourSeat,
  { body: string; ring: string; mark: string; label: string }
> = {
  1: {
    body: 'border-teal-200/45 bg-[radial-gradient(circle_at_34%_27%,#8ff5e3_0%,#14b8a6_40%,#0f766e_76%,#0f4f4a_100%)] shadow-[0_3px_6px_rgba(0,0,0,0.45),inset_0_2px_3px_rgba(255,255,255,0.45),inset_0_-4px_8px_rgba(0,0,0,0.45)]',
    ring: 'border-white/30',
    mark: 'rounded-full border-2 border-white/55',
    label: 'teal token',
  },
  2: {
    body: 'border-amber-200/55 bg-[radial-gradient(circle_at_34%_27%,#fff0b8_0%,#fbbf24_40%,#d97706_76%,#8a4a03_100%)] shadow-[0_3px_6px_rgba(0,0,0,0.4),inset_0_2px_3px_rgba(255,255,255,0.55),inset_0_-4px_8px_rgba(120,53,15,0.45)]',
    ring: 'border-black/15',
    mark: 'rotate-45 rounded-[2px] border-2 border-white/60',
    label: 'amber token',
  },
}

export function ConnectFourToken({ seat, className, emphasis = 'none', ghost = false }: ConnectFourTokenProps) {
  const style = SEAT_STYLES[seat]

  return (
    <span
      role="img"
      aria-label={style.label}
      className={[
        'relative inline-block aspect-square rounded-full',
        emphasis === 'win' ? 'c4-token-win' : '',
        ghost ? 'opacity-55' : '',
        className ?? 'h-10 w-10',
      ]
        .filter(Boolean)
        .join(' ')}
    >
      {/* Disc with bevel and shading. */}
      <span aria-hidden="true" className={['absolute inset-0 rounded-full border', style.body].join(' ')} />
      {/* Inner engraved ring. */}
      <span aria-hidden="true" className={['absolute inset-[13%] rounded-full border', style.ring].join(' ')} />
      {/* Centre mark: ring for seat one, diamond for seat two. */}
      <span aria-hidden="true" className={['absolute inset-[37%]', style.mark].join(' ')} />
      {/* Restrained specular highlight, not a neon glow. */}
      <span
        aria-hidden="true"
        className="absolute left-[16%] top-[12%] h-[16%] w-[30%] -rotate-[18deg] rounded-full bg-white/35 blur-[0.5px]"
      />
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
