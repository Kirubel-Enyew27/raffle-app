import { Sparkles, FastForward } from 'lucide-react'
import type { GameOutcome, OutcomePhase } from './outcomeTypes'
import { Button } from '@/components/ui/button'

interface DecisiveBannerProps {
  outcome: GameOutcome
  phase: OutcomePhase
  onSkip?: () => void
}

export function DecisiveBanner({ outcome, phase, onSkip }: DecisiveBannerProps) {
  if (phase === 'idle' || phase === 'announced') return null

  const isWin = outcome.type === 'win'
  const isLoss = outcome.type === 'loss'

  return (
    <div
      role="status"
      aria-live="polite"
      className={[
        'w-full max-w-md mx-auto mb-3 flex items-center justify-between rounded-xl border px-3.5 py-2 shadow-lg backdrop-blur-sm transition-all duration-300',
        isWin
          ? 'border-emerald-500/50 bg-emerald-500/15 text-emerald-900 dark:text-emerald-100 ring-1 ring-emerald-500/30'
          : isLoss
            ? 'border-rose-500/50 bg-rose-500/15 text-rose-900 dark:text-rose-100 ring-1 ring-rose-500/30'
            : 'border-amber-500/50 bg-amber-500/15 text-amber-900 dark:text-amber-100 ring-1 ring-amber-500/30',
      ].join(' ')}
    >
      <div className="flex items-center gap-2 text-xs font-bold">
        <Sparkles className="h-4 w-4 animate-spin text-primary shrink-0" />
        <span className="truncate">{outcome.subtitle}</span>
      </div>

      {onSkip && (
        <Button
          size="sm"
          variant="ghost"
          onClick={onSkip}
          className="h-6 px-2 text-[11px] font-semibold text-muted-foreground hover:text-foreground shrink-0 gap-1"
        >
          <span>Skip</span>
          <FastForward className="h-3 w-3" />
        </Button>
      )}
    </div>
  )
}
