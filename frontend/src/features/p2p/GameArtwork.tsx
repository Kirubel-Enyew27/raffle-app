import { useId } from 'react'

import { ConnectFourBall, DamaDisc, RpsGlyph, TicTacToeGlyph } from './GamePieces'

/**
 * Custom, self-contained SVG artwork for the KWIK game cards.
 *
 * Each illustration has its own lighting and material language while sharing a
 * 240×128 canvas, a dark backing and a controlled accent glow, so the cards read
 * as one family. The pieces themselves come from `GamePieces`, the same
 * components the playable boards render, so a card can never drift from
 * gameplay. Purely decorative: `aria-hidden` + `role="presentation"`.
 */

const FRAME = 'h-full w-full'

function Backdrop({ glow = '#ff5b55', strength = 0.28 }: { glow?: string; strength?: number }) {
  return (
    <>
      <ellipse cx="120" cy="122" rx="96" ry="34" fill={glow} opacity={strength * 0.5} />
      <ellipse cx="120" cy="70" rx="104" ry="54" fill={glow} opacity={strength * 0.16} />
    </>
  )
}

/* ── Ethiopian Dama ──────────────────────────────────────────────────────── */

function DamaArtwork() {
  const id = useId().replace(/:/g, '')

  return (
    <svg viewBox="0 0 240 128" className={FRAME} role="presentation" aria-hidden="true">
      <defs>
        <pattern id={`${id}-board`} width="24" height="24" patternUnits="userSpaceOnUse">
          <rect width="24" height="24" fill="var(--art-board)" />
          <rect width="12" height="12" fill="var(--art-board-alt)" />
          <rect x="12" y="12" width="12" height="12" fill="var(--art-board-alt)" />
        </pattern>
        <linearGradient id={`${id}-rim`} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#fbbf24" stopOpacity="0.55" />
          <stop offset="100%" stopColor="#fbbf24" stopOpacity="0" />
        </linearGradient>
      </defs>

      <Backdrop glow="#fbbf24" strength={0.34} />

      <g transform="rotate(-7 120 68)">
        <rect x="66" y="16" width="108" height="108" rx="12" fill="var(--art-frame-2)" stroke="var(--art-frame)" strokeWidth="1.5" />
        <rect x="72" y="22" width="96" height="96" rx="7" fill={`url(#${id}-board)`} />
        <rect x="72" y="22" width="96" height="18" rx="7" fill={`url(#${id}-rim)`} />
        <rect x="66" y="16" width="108" height="108" rx="12" fill="none" stroke="#fbbf24" strokeOpacity="0.18" />
        <DamaDisc side={0} x={87} y={37} width={34} height={34} />
        <DamaDisc side={1} x={123} y={67} width={34} height={34} />
      </g>
    </svg>
  )
}

/* ── Tic-Tac-Toe ─────────────────────────────────────────────────────────── */

function TicTacToeArtwork() {
  const id = useId().replace(/:/g, '')
  return (
    <svg viewBox="0 0 240 128" className={FRAME} role="presentation" aria-hidden="true">
      <defs>
        <filter id={`${id}-soft`} x="-40%" y="-40%" width="180%" height="180%">
          <feDropShadow dx="0" dy="6" stdDeviation="4" floodColor="#000" floodOpacity="0.55" />
        </filter>
      </defs>

      <Backdrop glow="#ff5b55" strength={0.4} />

      <g stroke="var(--art-line)" strokeWidth="2.5" strokeLinecap="round">
        <line x1="114" y1="22" x2="114" y2="106" />
        <line x1="152" y1="22" x2="152" y2="106" />
        <line x1="76" y1="50" x2="190" y2="50" />
        <line x1="76" y1="80" x2="190" y2="80" />
      </g>

      <g filter={`url(#${id}-soft)`}>
        <TicTacToeGlyph seat={1} x={73} y={43} width={44} height={44} />
        <TicTacToeGlyph seat={2} x={151} y={45} width={40} height={40} />
      </g>
    </svg>
  )
}

/* ── Rock-Paper-Scissors ─────────────────────────────────────────────────── */

function RockPaperScissorsArtwork() {
  const medallion = (cx: number, ring: string) => (
    <>
      <circle cx={cx} cy="70" r="29" fill="var(--art-medallion)" stroke={ring} strokeWidth="1.2" />
      <circle cx={cx} cy="70" r="29" fill="#000" opacity="0.08" />
    </>
  )

  return (
    <svg viewBox="0 0 240 128" className={FRAME} role="presentation" aria-hidden="true">
      <Backdrop glow="#a78bfa" strength={0.3} />

      <g>
        {medallion(62, 'rgba(167,139,250,0.35)')}
        <RpsGlyph choice="rock" x={39} y={47} width={46} height={46} />
      </g>

      <g>
        {medallion(120, 'rgba(255,91,85,0.35)')}
        <RpsGlyph choice="paper" x={97} y={47} width={46} height={46} />
      </g>

      <g>
        {medallion(178, 'rgba(255,91,85,0.35)')}
        <RpsGlyph choice="scissors" x={155} y={47} width={46} height={46} />
      </g>
    </svg>
  )
}

/* ── Connect Four ────────────────────────────────────────────────────────── */

function ConnectFourArtwork() {
  const id = useId().replace(/:/g, '')
  const cols = [66, 87, 108, 129, 150, 171]
  const rows = [44, 66, 88]
  const filled: Record<string, 1 | 2> = {
    '66,44': 1,
    '87,66': 2,
    '108,44': 2,
    '108,88': 1,
    '129,66': 1,
    '150,44': 1,
    '150,88': 2,
    '171,66': 1,
  }

  return (
    <svg viewBox="0 0 240 128" className={FRAME} role="presentation" aria-hidden="true">
      <defs>
        <linearGradient id={`${id}-frame`} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="var(--art-frame)" />
          <stop offset="100%" stopColor="var(--art-frame-2)" />
        </linearGradient>
      </defs>

      <Backdrop glow="#34d399" strength={0.22} />

      <rect x="48" y="24" width="144" height="82" rx="14" fill="#000" opacity="0.16" />
      <rect x="48" y="22" width="144" height="82" rx="14" fill={`url(#${id}-frame)`} stroke="var(--art-hole-stroke)" strokeWidth="1.2" />
      <rect x="51" y="25" width="138" height="12" rx="6" fill="#fff" opacity="0.04" />

      {rows.map((y) =>
        cols.map((x) => {
          const disc = filled[`${x},${y}`]
          return (
            <g key={`${x}-${y}`}>
              <circle cx={x} cy={y} r="8.2" fill="var(--art-hole)" stroke="var(--art-hole-stroke)" strokeWidth="1" />
              {disc && (
                <>
                  <circle cx={x} cy={y} r="10" fill={disc === 1 ? '#ff5b55' : '#fbbf24'} opacity="0.18" />
                  <ConnectFourBall seat={disc} x={x - 7.6} y={y - 7.6} width={15.2} height={15.2} />
                </>
              )}
            </g>
          )
        })
      )}
    </svg>
  )
}

/* ── Crash ───────────────────────────────────────────────────────────────── */

export function CrashArtwork({ className = FRAME }: { className?: string }) {
  const id = useId().replace(/:/g, '')
  return (
    <svg viewBox="0 0 240 128" className={className} role="presentation" aria-hidden="true">
      <defs>
        <linearGradient id={`${id}-curve`} x1="0" y1="1" x2="1" y2="0">
          <stop offset="0%" stopColor="#7d1418" />
          <stop offset="45%" stopColor="#ff5b55" />
          <stop offset="100%" stopColor="#ffb3a3" />
        </linearGradient>
        <linearGradient id={`${id}-fill`} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#ff5b55" stopOpacity="0.35" />
          <stop offset="100%" stopColor="#ff5b55" stopOpacity="0" />
        </linearGradient>
        <filter id={`${id}-glow`} x="-30%" y="-30%" width="160%" height="160%">
          <feGaussianBlur stdDeviation="3.2" result="blur" />
          <feMerge>
            <feMergeNode in="blur" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>

      <g stroke="#ffffff" strokeOpacity="0.06" strokeWidth="1">
        <line x1="18" y1="34" x2="222" y2="34" />
        <line x1="18" y1="62" x2="222" y2="62" />
        <line x1="18" y1="90" x2="222" y2="90" />
        <line x1="60" y1="16" x2="60" y2="112" />
        <line x1="120" y1="16" x2="120" y2="112" />
        <line x1="180" y1="16" x2="180" y2="112" />
      </g>

      <path d="M24 110 H216 V34 C176 40 150 58 128 76 C108 92 78 104 24 110 Z" fill={`url(#${id}-fill)`} />
      <path
        d="M24 110 C74 106 108 96 132 74 C156 52 176 34 212 26"
        fill="none"
        stroke={`url(#${id}-curve)`}
        strokeWidth="4"
        strokeLinecap="round"
        filter={`url(#${id}-glow)`}
      />
      <circle cx="212" cy="26" r="4.5" fill="#fff" />
      <circle cx="212" cy="26" r="11" fill="#ff5b55" opacity="0.25" />
      <text x="132" y="46" fill="#fff" fontSize="20" fontWeight="800" fontFamily="ui-monospace, monospace">
        3.42×
      </text>
    </svg>
  )
}

/* ── Public switch ───────────────────────────────────────────────────────── */

export function GameArtwork({ slug, className = FRAME }: { slug: string; className?: string }) {
  switch (slug) {
    case 'ethiopian-dama':
      return <DamaArtwork />
    case 'tic-tac-toe':
      return <TicTacToeArtwork />
    case 'rock-paper-scissors':
      return <RockPaperScissorsArtwork />
    case 'connect-four':
      return <ConnectFourArtwork />
    default:
      return (
        <svg viewBox="0 0 240 128" className={className} role="presentation" aria-hidden="true">
          <Backdrop />
          <circle cx="120" cy="64" r="24" fill="var(--art-medallion)" stroke="var(--art-frame)" strokeWidth="1.5" />
          <path d="M112 56h16v6h-16zM112 66h16v6h-16z" fill="#ff5b55" opacity="0.7" />
        </svg>
      )
  }
}
