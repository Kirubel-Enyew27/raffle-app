import { Plane, TrendingUp } from 'lucide-react'
import type { CrashRoundState } from './api'

interface FlightStageProps {
  state: CrashRoundState | 'unknown'
  /** Multiplier currently displayed (driven by the server, interpolated between ticks). */
  multiplier: number
  /** Final crash multiplier, shown once the round has crashed. */
  crashMultiplier?: number
  /** Milliseconds left in the waiting/betting countdown. */
  countdownMs?: number
  /** 0..1 position along the flight curve. Display only. */
  progress: number
  label: string
  countdownLabel: string
}

const padded = (ms: number) =>
  `${Math.floor(Math.max(0, ms) / 1000)}`.padStart(2, '0')

/**
 * The central game area: a rising-flight animation plus the big multiplier.
 *
 * Everything here is presentation. The values come from the server (ticks,
 * crash event) and the animation timing is amortised in a requestAnimationFrame
 * loop, so a slow frame can never change what a player is paid or when.
 */
export function FlightStage({
  state,
  multiplier,
  crashMultiplier,
  countdownMs = 0,
  progress,
  label,
  countdownLabel,
}: FlightStageProps) {
  const running = state === 'RUNNING'
  const crashed = state === 'CRASHED' || state === 'SETTLEMENT' || state === 'NEXT_ROUND'
  const waiting = state === 'WAITING' || state === 'BETTING' || state === 'unknown'

  const p = Math.max(0, Math.min(1, progress))
  const planeLeft = 6 + p * 78
  const planeBottom = 10 + p * 68
  const rotation = -18 - p * 34

  const accent = crashed ? 'text-destructive' : running ? 'text-green-500' : 'text-muted-foreground'
  const shownMultiplier = crashed && crashMultiplier ? crashMultiplier : multiplier

  return (
    <div
      className="relative h-[300px] w-full overflow-hidden rounded-xl border bg-card sm:h-[380px] lg:h-[440px]"
      role="img"
      aria-label={`${label} ${shownMultiplier.toFixed(2)}x`}
    >
      {/* Grid backdrop */}
      <div
        className="absolute inset-0 opacity-[0.35]"
        style={{
          backgroundImage:
            'linear-gradient(to right, hsl(var(--border)) 1px, transparent 1px), linear-gradient(to bottom, hsl(var(--border)) 1px, transparent 1px)',
          backgroundSize: '48px 48px',
        }}
        aria-hidden
      />

      {/* Rising flight path */}
      <svg
        className="absolute inset-0 h-full w-full"
        viewBox="0 0 100 100"
        preserveAspectRatio="none"
        aria-hidden
      >
        <defs>
          <linearGradient id="crash-fill" x1="0" y1="1" x2="0" y2="0">
            <stop offset="0%" stopColor={crashed ? 'rgb(239 68 68 / 0.05)' : 'rgb(34 197 94 / 0.05)'} />
            <stop offset="100%" stopColor={crashed ? 'rgb(239 68 68 / 0.35)' : 'rgb(34 197 94 / 0.35)'} />
          </linearGradient>
        </defs>
        <path
          d={`M0,100 C 35,${100 - p * 45} ${60 - p * 10},${100 - p * 60} 100,${100 - p * 82} L100,100 Z`}
          fill="url(#crash-fill)"
          className="transition-[d] duration-200 ease-linear"
        />
        <path
          d={`M0,100 C 35,${100 - p * 45} ${60 - p * 10},${100 - p * 60} 100,${100 - p * 82}`}
          fill="none"
          stroke={crashed ? 'rgb(239 68 68 / 0.8)' : 'rgb(34 197 94 / 0.8)'}
          strokeWidth="0.6"
          vectorEffect="non-scaling-stroke"
        />
      </svg>

      {/* Aircraft */}
      {(running || crashed) && (
        <div
          className="absolute transition-all duration-200 ease-linear"
          style={{
            left: `${planeLeft}%`,
            bottom: `${planeBottom}%`,
            transform: `rotate(${crashed ? 55 : rotation}deg)`,
          }}
          aria-hidden
        >
          <Plane
            className={`h-8 w-8 ${crashed ? 'text-destructive' : 'text-green-500 drop-shadow'}`}
            strokeWidth={2.5}
          />
        </div>
      )}

      {/* Centre readout */}
      <div className="relative flex h-full flex-col items-center justify-center gap-2 px-4 text-center">
        {waiting ? (
          <>
            <p className="text-xs font-medium uppercase tracking-[0.2em] text-muted-foreground">
              {countdownLabel}
            </p>
            <p className="font-mono text-6xl font-bold tabular-nums sm:text-7xl">
              00:{padded(countdownMs)}
            </p>
            <p className="flex items-center gap-1 text-xs text-muted-foreground">
              <TrendingUp className="h-3.5 w-3.5" />
              {label}
            </p>
          </>
        ) : (
          <>
            <p
              className={`font-mono text-6xl font-extrabold tabular-nums sm:text-7xl lg:text-8xl ${accent}`}
            >
              {shownMultiplier.toFixed(2)}x
            </p>
            {crashed ? (
              <p className="rounded-full bg-destructive/10 px-3 py-1 text-xs font-semibold uppercase tracking-wider text-destructive">
                {label}
              </p>
            ) : (
              <p className="text-xs uppercase tracking-[0.2em] text-muted-foreground">
                {label}
              </p>
            )}
          </>
        )}
      </div>
    </div>
  )
}
