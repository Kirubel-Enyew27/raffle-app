import { useId, type ComponentProps } from 'react'

/**
 * Shared KWIK game-piece artwork.
 *
 * These are the single source of truth for how a Dama piece, a Tic-Tac-Toe
 * mark, a Rock-Paper-Scissors hand and a Connect Four ball look. The lobby
 * cards (`GameArtwork`) and the playable boards both render these components,
 * so the artwork on a card can never drift from the artwork in a match.
 *
 * Every component draws its own gradients with an id from `useId`, renders a
 * square viewBox and sizes itself from the `className` it is given (or from
 * nested-SVG x/y/width/height when composed inside card artwork).
 */
type SvgProps = Omit<ComponentProps<'svg'>, 'viewBox' | 'children'>

const DEFAULT_SIZE = 'h-full w-full'

function useArtId() {
  return useId().replace(/[^a-zA-Z0-9]/g, '')
}

/* ── Ethiopian Dama ──────────────────────────────────────────────────────── */

export type DamaArtSide = 0 | 1

const DAMA_MATERIAL: Record<
  DamaArtSide,
  { light: string; mid: string; deep: string; edge: string; detail: string; ink: string }
> = {
  // Seat 0 — ivory.
  0: { light: '#fffaf0', mid: '#e8dcc0', deep: '#9b8a66', edge: '#c9b891', detail: '#8a7550', ink: '#7a4a20' },
  // Seat 1 — crimson.
  1: { light: '#ff8a7a', mid: '#e0363c', deep: '#7d1418', edge: '#ff9a8e', detail: '#ffd0c4', ink: '#ffe6df' },
}

/**
 * A single Dama disc: bevelled face, engraved rim, inner ring, specular
 * highlight and — for a king — a small engraved crown.
 */
export function DamaDisc({
  side,
  king = false,
  className,
  ...rest
}: SvgProps & { side: DamaArtSide; king?: boolean }) {
  const id = useArtId()
  const material = DAMA_MATERIAL[side]

  return (
    <svg
      viewBox="0 0 32 32"
      className={className ?? DEFAULT_SIZE}
      role="presentation"
      aria-hidden="true"
      {...rest}
    >
      <defs>
        <radialGradient id={`${id}-face`} cx="35%" cy="28%" r="78%">
          <stop offset="0%" stopColor={material.light} />
          <stop offset="55%" stopColor={material.mid} />
          <stop offset="100%" stopColor={material.deep} />
        </radialGradient>
      </defs>

      <circle cx="16" cy="16" r="15" fill={`url(#${id}-face)`} stroke={material.edge} strokeWidth="1.3" />
      <circle cx="16" cy="16" r="9.7" fill="none" stroke={material.detail} strokeWidth="1.1" opacity="0.55" />
      <circle cx="16" cy="16" r="4.8" fill="none" stroke={material.detail} strokeWidth="0.95" opacity="0.4" />
      <ellipse cx="11.6" cy="9.6" rx="5.1" ry="2.9" fill="#fff" opacity="0.32" />

      {king && (
        <g stroke={material.ink} strokeWidth="1.35" strokeLinejoin="round" strokeLinecap="round" fill="none" opacity="0.9">
          <path d="M10.6 22.1 12.4 13.2 16 17.7 19.6 13.2 21.4 22.1Z" />
          <path d="M11.2 24.3h9.6" />
        </g>
      )}
    </svg>
  )
}

/* ── Tic-Tac-Toe ─────────────────────────────────────────────────────────── */

export type TicTacToeArtSeat = 1 | 2

/**
 * Seat one is the dimensional red X, seat two the cream O — the same pair the
 * Tic-Tac-Toe card is built from.
 */
export function TicTacToeGlyph({
  seat,
  className,
  ...rest
}: SvgProps & { seat: TicTacToeArtSeat }) {
  const id = useArtId()

  if (seat === 2) {
    return (
      <svg
        viewBox="0 0 24 24"
        className={className ?? DEFAULT_SIZE}
        role="presentation"
        aria-hidden="true"
        {...rest}
      >
        <defs>
          <linearGradient id={`${id}-o`} x1="0" y1="0" x2="1" y2="1">
            <stop offset="0%" stopColor="#ffffff" />
            <stop offset="60%" stopColor="#efe9dc" />
            <stop offset="100%" stopColor="#a99f8b" />
          </linearGradient>
        </defs>
        {/* Thin dark rim keeps the cream ring legible on light board cells. */}
        <circle cx="12" cy="12" r="8.6" fill="none" stroke="#6f6754" strokeOpacity="0.45" strokeWidth="5.4" />
        <circle cx="12" cy="12" r="8.6" fill="none" stroke={`url(#${id}-o)`} strokeWidth="4.6" />
        <circle cx="8.9" cy="8.4" r="3.1" fill="none" stroke="#ffffff" strokeOpacity="0.55" strokeWidth="0.9" />
      </svg>
    )
  }

  return (
    <svg viewBox="0 0 24 24" className={className ?? DEFAULT_SIZE} role="presentation" aria-hidden="true" {...rest}>
      <defs>
        <linearGradient id={`${id}-x`} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="#ffb3a8" />
          <stop offset="45%" stopColor="#ff5b55" />
          <stop offset="100%" stopColor="#b31d20" />
        </linearGradient>
      </defs>
      <g transform="rotate(45 12 12)">
        <rect x="9.4" y="2.6" width="5.2" height="18.8" rx="2.6" fill={`url(#${id}-x)`} />
        <rect x="9.4" y="2.6" width="1.9" height="18.8" rx="0.95" fill="#fff" opacity="0.22" />
      </g>
      <g transform="rotate(-45 12 12)">
        <rect x="9.4" y="2.6" width="5.2" height="18.8" rx="2.6" fill={`url(#${id}-x)`} />
        <rect x="9.4" y="2.6" width="1.9" height="18.8" rx="0.95" fill="#fff" opacity="0.18" />
      </g>
    </svg>
  )
}

/* ── Rock-Paper-Scissors ─────────────────────────────────────────────────── */

export type RpsArtChoice = 'rock' | 'paper' | 'scissors'

/**
 * The three RPS hands, drawn as small objects rather than line icons: a faceted
 * steel stone, a folded sheet of paper and a pair of red steel scissors. These
 * same three illustrations appear on the game card.
 */
export function RpsGlyph({
  choice,
  className,
  ...rest
}: SvgProps & { choice: RpsArtChoice }) {
  const id = useArtId()

  if (choice === 'paper') {
    return (
      <svg
        viewBox="0 0 32 32"
        className={className ?? DEFAULT_SIZE}
        role="presentation"
        aria-hidden="true"
        {...rest}
      >
        <defs>
          <linearGradient id={`${id}-paper`} x1="0" y1="0" x2="1" y2="1">
            <stop offset="0%" stopColor="#fffdf6" />
            <stop offset="70%" stopColor="#e6dcc4" />
            <stop offset="100%" stopColor="#b1a487" />
          </linearGradient>
        </defs>
        <g transform="rotate(-6 16 16)">
          <rect x="8.6" y="4.6" width="14.8" height="22.4" rx="2.2" fill={`url(#${id}-paper)`} stroke="#8f8268" strokeOpacity="0.45" strokeWidth="0.8" />
          <path d="M18.4 4.6 23.4 9.9 18.4 9.9Z" fill="#b1a487" opacity="0.75" />
          <g stroke="#8f8268" strokeOpacity="0.55" strokeWidth="0.9" strokeLinecap="round">
            <line x1="11.6" y1="14.2" x2="20.4" y2="14.2" />
            <line x1="11.6" y1="17.8" x2="20.4" y2="17.8" />
            <line x1="11.6" y1="21.4" x2="17.6" y2="21.4" />
          </g>
        </g>
      </svg>
    )
  }

  if (choice === 'scissors') {
    return (
      <svg
        viewBox="0 0 32 32"
        className={className ?? DEFAULT_SIZE}
        role="presentation"
        aria-hidden="true"
        {...rest}
      >
        <defs>
          <linearGradient id={`${id}-steel`} x1="0" y1="0" x2="1" y2="1">
            <stop offset="0%" stopColor="#ffb0a6" />
            <stop offset="55%" stopColor="#ff5b55" />
            <stop offset="100%" stopColor="#c0272b" />
          </linearGradient>
        </defs>
        <g stroke={`url(#${id}-steel)`} strokeWidth="3.2" strokeLinecap="round">
          <line x1="9.4" y1="25.4" x2="22.4" y2="6.6" />
          <line x1="22.6" y1="25.4" x2="9.6" y2="6.6" />
        </g>
        <circle cx="9" cy="26.2" r="3.5" fill="none" stroke={`url(#${id}-steel)`} strokeWidth="2.4" />
        <circle cx="23" cy="26.2" r="3.5" fill="none" stroke={`url(#${id}-steel)`} strokeWidth="2.4" />
        <circle cx="16" cy="16" r="1.5" fill="#fff" opacity="0.8" />
      </svg>
    )
  }

  return (
    <svg viewBox="0 0 32 32" className={className ?? DEFAULT_SIZE} role="presentation" aria-hidden="true" {...rest}>
      <defs>
        <linearGradient id={`${id}-rock`} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="#c9cfd8" />
          <stop offset="55%" stopColor="#7c8592" />
          <stop offset="100%" stopColor="#3a4048" />
        </linearGradient>
      </defs>
      <path
        d="M16 3.8 24.8 8.9 27.6 18.6 20.9 27.4 11.1 27.4 4.4 18.6 7.2 8.9Z"
        fill={`url(#${id}-rock)`}
        stroke="#e2e8f0"
        strokeOpacity="0.35"
        strokeWidth="0.9"
      />
      <g fill="none" stroke="#ffffff" strokeOpacity="0.28" strokeWidth="0.8">
        <path d="M7.2 8.9 16 14.3 24.8 8.9" />
        <path d="M16 14.3 11.1 27.4" />
        <path d="M16 14.3 20.9 27.4" />
      </g>
      <ellipse cx="12.4" cy="9.4" rx="3.1" ry="1.7" fill="#fff" opacity="0.22" transform="rotate(-18 12.4 9.4)" />
    </svg>
  )
}

/* ── Connect Four ────────────────────────────────────────────────────────── */

export type ConnectFourArtSeat = 1 | 2

const C4_MATERIAL: Record<ConnectFourArtSeat, { light: string; mid: string; deep: string; rim: string }> = {
  // Seat one — KWIK red.
  1: { light: '#ffb4a8', mid: '#ff5b55', deep: '#a51c20', rim: '#ffb4a8' },
  // Seat two — amber.
  2: { light: '#ffe9a8', mid: '#fbbf24', deep: '#a76a08', rim: '#ffe9a8' },
}

/** A glossy, unmarked Connect Four ball — the exact disc used on the card. */
export function ConnectFourBall({
  seat,
  className,
  ...rest
}: SvgProps & { seat: ConnectFourArtSeat }) {
  const id = useArtId()
  const material = C4_MATERIAL[seat]

  return (
    <svg
      viewBox="0 0 24 24"
      className={className ?? DEFAULT_SIZE}
      role="presentation"
      aria-hidden="true"
      {...rest}
    >
      <defs>
        <radialGradient id={`${id}-ball`} cx="35%" cy="30%" r="75%">
          <stop offset="0%" stopColor={material.light} />
          <stop offset="55%" stopColor={material.mid} />
          <stop offset="100%" stopColor={material.deep} />
        </radialGradient>
      </defs>
      <circle cx="12" cy="12" r="11.5" fill={`url(#${id}-ball)`} stroke={material.rim} strokeOpacity="0.45" strokeWidth="0.8" />
      <ellipse cx="8.4" cy="7.4" rx="3.4" ry="2" fill="#fff" opacity="0.5" transform="rotate(-25 8.4 7.4)" />
    </svg>
  )
}
