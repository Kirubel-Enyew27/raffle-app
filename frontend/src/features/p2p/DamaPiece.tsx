import type { DamaSide } from './dama'

/**
 * Drawn Ethiopian Dama checkers.
 *
 * Both seats share identical geometry and weight — a bevelled disc, one
 * engraved rim ring and one Ethiopian-inspired lozenge motif on a 40x40 grid —
 * so neither side reads as secondary. Seat one is warm ivory, seat two is
 * obsidian; kings add a raised top surface with a small engraved crown so they
 * stay recognisable at 20px.
 */
interface DamaPieceProps {
  side: DamaSide
  king?: boolean
  /** Sizing utilities only, e.g. "h-full w-full". */
  className?: string
  /** Short arrival animation for the piece that just landed. */
  arriving?: boolean
}

const IVORY = {
  body: 'border-[#b98b52] bg-[radial-gradient(circle_at_32%_26%,#fffdf7_0%,#f3e3c4_55%,#cba876_100%)] shadow-[0_2px_5px_rgba(0,0,0,0.45),inset_0_2px_2px_rgba(255,255,255,0.65),inset_0_-4px_8px_rgba(122,82,40,0.35)]',
  rim: 'border-[#8a5a2b]/35',
  top: 'border-[#c9a878] bg-[radial-gradient(circle_at_34%_28%,#fffaf0_0%,#edd9b4_70%,#c9a878_100%)] shadow-[inset_0_2px_3px_rgba(255,255,255,0.6),inset_0_-2px_4px_rgba(122,82,40,0.35)]',
  ink: 'text-[#7a4a20]',
  label: 'ivory',
}

const OBSIDIAN = {
  body: 'border-black/70 bg-[radial-gradient(circle_at_32%_26%,#5c5c64_0%,#2a2a2f_52%,#08080b_100%)] shadow-[0_2px_6px_rgba(0,0,0,0.6),inset_0_2px_2px_rgba(255,255,255,0.22),inset_0_-4px_8px_rgba(0,0,0,0.6)]',
  rim: 'border-white/15',
  top: 'border-[#1b1b1f] bg-[radial-gradient(circle_at_34%_28%,#4e4e57_0%,#232327_70%,#101013_100%)] shadow-[inset_0_2px_3px_rgba(255,255,255,0.18),inset_0_-2px_5px_rgba(0,0,0,0.6)]',
  ink: 'text-[#f1ddb8]',
  label: 'obsidian',
}

export function DamaPiece({ side, king = false, className, arriving = false }: DamaPieceProps) {
  const theme = side === 0 ? IVORY : OBSIDIAN

  return (
    <span
      role="img"
      aria-label={`${theme.label} dama ${king ? 'king' : 'checker'}`}
      className={[
        'relative inline-block aspect-square rounded-full',
        arriving ? 'dama-piece-arrive' : '',
        className ?? 'h-full w-full',
      ]
        .filter(Boolean)
        .join(' ')}
    >
      {/* Engraved disc with bevel and soft depth. */}
      <span aria-hidden="true" className={['absolute inset-0 rounded-full border', theme.body].join(' ')} />
      {/* Rim ring, identical on both sides. */}
      <span aria-hidden="true" className={['absolute inset-[12%] rounded-full border', theme.rim].join(' ')} />

      {king ? (
        <>
          {/* Layered top surface marks a crowned piece. */}
          <span aria-hidden="true" className={['absolute inset-[24%] rounded-full border', theme.top].join(' ')} />
          <svg viewBox="0 0 40 40" className={['absolute inset-0 h-full w-full', theme.ink].join(' ')} aria-hidden="true">
            <path
              d="M14 25.2 15.9 16.8 20 21.1 24.1 16.8 26 25.2Z"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinejoin="round"
              opacity="0.85"
            />
            <path d="M14.6 27.1h10.8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" opacity="0.85" />
          </svg>
        </>
      ) : (
        <svg viewBox="0 0 40 40" className={['absolute inset-0 h-full w-full', theme.ink].join(' ')} aria-hidden="true">
          {/* Ethiopian-inspired lozenge motif: diamond, centre dot, four points. */}
          <path
            d="M20 10.8 29.2 20 20 29.2 10.8 20Z"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.4"
            strokeLinejoin="round"
            opacity="0.55"
          />
          <circle cx="20" cy="20" r="2" fill="currentColor" opacity="0.45" />
          <circle cx="20" cy="6.9" r="1.05" fill="currentColor" opacity="0.4" />
          <circle cx="33.1" cy="20" r="1.05" fill="currentColor" opacity="0.4" />
          <circle cx="20" cy="33.1" r="1.05" fill="currentColor" opacity="0.4" />
          <circle cx="6.9" cy="20" r="1.05" fill="currentColor" opacity="0.4" />
        </svg>
      )}
    </span>
  )
}

export function DamaKingPiece(props: Omit<DamaPieceProps, 'king'>) {
  return <DamaPiece {...props} king />
}

/** Compact both-seats badge, used wherever the game itself is illustrated. */
export function DamaPiecePair({ className }: { className?: string }) {
  const size = className ?? 'h-5 w-5'
  return (
    <span className="inline-flex items-center gap-0.5">
      <DamaPiece side={1} className={size} />
      <DamaPiece side={0} className={size} />
    </span>
  )
}
