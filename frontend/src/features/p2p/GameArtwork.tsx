import { useId } from 'react'

/**
 * Custom, self-contained SVG artwork for the KWIK game cards.
 *
 * Each illustration has its own lighting and material language while sharing a
 * 240×128 canvas, a dark backing and a controlled accent glow, so the cards read
 * as one family. Purely decorative: `aria-hidden` + `role="presentation"`.
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
  const piece = (cx: number, cy: number, face: string, edge: string, highlight: string) => (
    <g>
      <ellipse cx={cx} cy={cy + 15} rx="18" ry="5" fill="#000" opacity="0.55" />
      <circle cx={cx} cy={cy} r="17" fill={`url(#${id}-${face})`} stroke={edge} strokeWidth="1.5" />
      <circle cx={cx} cy={cy} r="11" fill="none" stroke={highlight} strokeWidth="1.2" opacity="0.55" />
      <circle cx={cx} cy={cy} r="5.5" fill="none" stroke={highlight} strokeWidth="1" opacity="0.4" />
      <ellipse cx={cx - 5} cy={cy - 7} rx="6" ry="3.4" fill="#fff" opacity="0.35" />
    </g>
  )

  return (
    <svg viewBox="0 0 240 128" className={FRAME} role="presentation" aria-hidden="true">
      <defs>
        <pattern id={`${id}-board`} width="24" height="24" patternUnits="userSpaceOnUse">
          <rect width="24" height="24" fill="#17191d" />
          <rect width="12" height="12" fill="#232630" />
          <rect x="12" y="12" width="12" height="12" fill="#232630" />
        </pattern>
        <radialGradient id={`${id}-ivory`} cx="35%" cy="28%" r="78%">
          <stop offset="0%" stopColor="#fffaf0" />
          <stop offset="55%" stopColor="#e8dcc0" />
          <stop offset="100%" stopColor="#9b8a66" />
        </radialGradient>
        <radialGradient id={`${id}-crimson`} cx="35%" cy="28%" r="78%">
          <stop offset="0%" stopColor="#ff8a7a" />
          <stop offset="55%" stopColor="#e0363c" />
          <stop offset="100%" stopColor="#7d1418" />
        </radialGradient>
        <linearGradient id={`${id}-rim`} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#fbbf24" stopOpacity="0.55" />
          <stop offset="100%" stopColor="#fbbf24" stopOpacity="0" />
        </linearGradient>
      </defs>

      <Backdrop glow="#fbbf24" strength={0.34} />

      <g transform="rotate(-7 120 68)">
        <rect x="66" y="16" width="108" height="108" rx="12" fill="#0f1013" stroke="#3a3226" strokeWidth="1.5" />
        <rect x="72" y="22" width="96" height="96" rx="7" fill={`url(#${id}-board)`} />
        <rect x="72" y="22" width="96" height="18" rx="7" fill={`url(#${id}-rim)`} />
        <rect x="66" y="16" width="108" height="108" rx="12" fill="none" stroke="#fbbf24" strokeOpacity="0.18" />
        {piece(104, 54, 'ivory', '#c9b891', '#8a7550')}
        {piece(140, 84, 'crimson', '#ff8f83', '#ffd0c4')}
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
        <linearGradient id={`${id}-x`} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="#ffb3a8" />
          <stop offset="45%" stopColor="#ff5b55" />
          <stop offset="100%" stopColor="#b31d20" />
        </linearGradient>
        <linearGradient id={`${id}-o`} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="#ffffff" />
          <stop offset="60%" stopColor="#efe9dc" />
          <stop offset="100%" stopColor="#a99f8b" />
        </linearGradient>
        <filter id={`${id}-soft`} x="-40%" y="-40%" width="180%" height="180%">
          <feDropShadow dx="0" dy="6" stdDeviation="4" floodColor="#000" floodOpacity="0.55" />
        </filter>
      </defs>

      <Backdrop glow="#ff5b55" strength={0.4} />

      <g stroke="#ffffff" strokeOpacity="0.09" strokeWidth="2.5" strokeLinecap="round">
        <line x1="114" y1="22" x2="114" y2="106" />
        <line x1="152" y1="22" x2="152" y2="106" />
        <line x1="76" y1="50" x2="190" y2="50" />
        <line x1="76" y1="80" x2="190" y2="80" />
      </g>

      <g filter={`url(#${id}-soft)`}>
        <g transform="rotate(45 95 65)">
          <rect x="81" y="38" width="17" height="54" rx="8.5" fill={`url(#${id}-x)`} />
          <rect x="81" y="38" width="6" height="54" rx="3" fill="#fff" opacity="0.22" />
        </g>
        <g transform="rotate(-45 95 65)">
          <rect x="81" y="38" width="17" height="54" rx="8.5" fill={`url(#${id}-x)`} />
          <rect x="81" y="38" width="6" height="54" rx="3" fill="#fff" opacity="0.18" />
        </g>

        <circle cx="171" cy="65" r="24" fill="none" stroke={`url(#${id}-o)`} strokeWidth="14" />
        <circle cx="171" cy="65" r="24" fill="none" stroke="#fff" strokeOpacity="0.35" strokeWidth="2" />
      </g>
    </svg>
  )
}

/* ── Rock-Paper-Scissors ─────────────────────────────────────────────────── */

function RockPaperScissorsArtwork() {
  const id = useId().replace(/:/g, '')
  const medallion = (cx: number, ring: string) => (
    <>
      <circle cx={cx} cy="70" r="29" fill="#121318" stroke={ring} strokeWidth="1.2" />
      <circle cx={cx} cy="70" r="29" fill="#000" opacity="0.18" />
    </>
  )

  return (
    <svg viewBox="0 0 240 128" className={FRAME} role="presentation" aria-hidden="true">
      <defs>
        <linearGradient id={`${id}-rock`} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="#c9cfd8" />
          <stop offset="55%" stopColor="#7c8592" />
          <stop offset="100%" stopColor="#3a4048" />
        </linearGradient>
        <linearGradient id={`${id}-paper`} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="#fffdf6" />
          <stop offset="70%" stopColor="#e6dcc4" />
          <stop offset="100%" stopColor="#b1a487" />
        </linearGradient>
        <linearGradient id={`${id}-steel`} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="#ffb0a6" />
          <stop offset="100%" stopColor="#e0363c" />
        </linearGradient>
      </defs>

      <Backdrop glow="#a78bfa" strength={0.3} />

      <g>
        {medallion(62, 'rgba(167,139,250,0.35)')}
        <path
          d="M48 78 L53 58 68 50 82 58 78 78 64 86 Z"
          fill={`url(#${id}-rock)`}
          stroke="#e2e8f0"
          strokeOpacity="0.35"
          strokeWidth="1"
        />
        <path d="M53 58 L64 70 68 50" fill="none" stroke="#fff" strokeOpacity="0.25" strokeWidth="1" />
        <path d="M64 70 L78 78" fill="none" stroke="#000" strokeOpacity="0.25" strokeWidth="1" />
      </g>

      <g>
        {medallion(120, 'rgba(255,91,85,0.35)')}
        <rect
          x="108"
          y="50"
          width="25"
          height="35"
          rx="4"
          fill={`url(#${id}-paper)`}
          transform="rotate(-6 120 67)"
        />
        <path d="M126 50 L133 57 126 57 Z" fill="#b1a487" opacity="0.7" transform="rotate(-6 120 67)" />
        <g stroke="#8f8268" strokeOpacity="0.5" strokeWidth="1" transform="rotate(-6 120 67)">
          <line x1="113" y1="62" x2="128" y2="62" />
          <line x1="113" y1="68" x2="128" y2="68" />
          <line x1="113" y1="74" x2="123" y2="74" />
        </g>
      </g>

      <g>
        {medallion(178, 'rgba(255,91,85,0.35)')}
        <g stroke={`url(#${id}-steel)`} strokeWidth="7" strokeLinecap="round">
          <line x1="164" y1="88" x2="184" y2="54" />
          <line x1="192" y1="88" x2="172" y2="54" />
        </g>
        <circle cx="163" cy="90" r="7" fill="none" stroke="#ff8a80" strokeWidth="4" />
        <circle cx="193" cy="90" r="7" fill="none" stroke="#ff8a80" strokeWidth="4" />
        <circle cx="178" cy="71" r="3" fill="#fff" opacity="0.7" />
      </g>
    </svg>
  )
}

/* ── Connect Four ────────────────────────────────────────────────────────── */

function ConnectFourArtwork() {
  const id = useId().replace(/:/g, '')
  const cols = [66, 87, 108, 129, 150, 171]
  const rows = [44, 66, 88]
  const filled: Record<string, 'red' | 'amber'> = {
    '66,44': 'red',
    '87,66': 'amber',
    '108,44': 'amber',
    '108,88': 'red',
    '129,66': 'red',
    '150,44': 'red',
    '150,88': 'amber',
    '171,66': 'red',
  }

  return (
    <svg viewBox="0 0 240 128" className={FRAME} role="presentation" aria-hidden="true">
      <defs>
        <linearGradient id={`${id}-frame`} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#2a2d35" />
          <stop offset="100%" stopColor="#111318" />
        </linearGradient>
        <radialGradient id={`${id}-red`} cx="35%" cy="30%" r="75%">
          <stop offset="0%" stopColor="#ffb4a8" />
          <stop offset="55%" stopColor="#ff5b55" />
          <stop offset="100%" stopColor="#a51c20" />
        </radialGradient>
        <radialGradient id={`${id}-amber`} cx="35%" cy="30%" r="75%">
          <stop offset="0%" stopColor="#ffe9a8" />
          <stop offset="55%" stopColor="#fbbf24" />
          <stop offset="100%" stopColor="#a76a08" />
        </radialGradient>
      </defs>

      <Backdrop glow="#34d399" strength={0.22} />

      <rect x="48" y="24" width="144" height="82" rx="14" fill="#000" opacity="0.5" />
      <rect x="48" y="22" width="144" height="82" rx="14" fill={`url(#${id}-frame)`} stroke="#3a3e46" strokeWidth="1.2" />
      <rect x="51" y="25" width="138" height="12" rx="6" fill="#fff" opacity="0.04" />

      {rows.map((y) =>
        cols.map((x) => {
          const disc = filled[`${x},${y}`]
          return (
            <g key={`${x}-${y}`}>
              <circle cx={x} cy={y} r="8.2" fill="#0a0b0d" stroke="#2c3037" strokeWidth="1" />
              {disc && (
                <>
                  <circle cx={x} cy={y} r="10" fill={disc === 'red' ? '#ff5b55' : '#fbbf24'} opacity="0.18" />
                  <circle
                    cx={x}
                    cy={y}
                    r="7.6"
                    fill={`url(#${id}-${disc})`}
                    stroke={disc === 'red' ? '#ffb4a8' : '#ffe9a8'}
                    strokeOpacity="0.45"
                    strokeWidth="0.8"
                  />
                  <ellipse cx={x - 2.4} cy={y - 2.8} rx="2.6" ry="1.5" fill="#fff" opacity="0.5" />
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
          <circle cx="120" cy="64" r="24" fill="#1a1c21" stroke="#3a3e46" strokeWidth="1.5" />
          <path d="M112 56h16v6h-16zM112 66h16v6h-16z" fill="#ff5b55" opacity="0.7" />
        </svg>
      )
  }
}
