import { DamaDisc } from './GamePieces'
import type { DamaSide } from './dama'

/**
 * Ethiopian Dama checkers, rendered from the shared `DamaDisc` artwork so the
 * board, move history, seat badges and lobby card all show the same pieces.
 *
 * Seat 0 is warm ivory, seat 1 is crimson; kings add an engraved crown that
 * stays readable at 14px.
 */
interface DamaPieceProps {
  side: DamaSide
  king?: boolean
  /** Sizing utilities only, e.g. "h-full w-full". */
  className?: string
  /** Short arrival animation for the piece that just landed. */
  arriving?: boolean
}

const SIDE_LABEL: Record<DamaSide, string> = { 0: 'ivory', 1: 'crimson' }

export function DamaPiece({ side, king = false, className, arriving = false }: DamaPieceProps) {
  return (
    <span
      role="img"
      aria-label={`${SIDE_LABEL[side]} dama ${king ? 'king' : 'checker'}`}
      className={[
        'relative inline-block aspect-square rounded-full',
        arriving ? 'dama-piece-arrive' : '',
        className ?? 'h-full w-full',
      ]
        .filter(Boolean)
        .join(' ')}
    >
      <DamaDisc side={side} king={king} className="absolute inset-0 h-full w-full" />
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
      <DamaPiece side={0} className={size} />
      <DamaPiece side={1} className={size} />
    </span>
  )
}
