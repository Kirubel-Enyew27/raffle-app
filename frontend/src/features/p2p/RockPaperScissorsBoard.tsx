import { useState } from 'react'
import { CheckCircle2 } from 'lucide-react'
import { Button } from '@/components/ui/button'

type Choice = 'rock' | 'paper' | 'scissors'

interface RockPaperScissorsBoardProps {
  boardState: {
    choices?: Record<string, string>
    last_round_result?: {
      p0_choice?: string
      p1_choice?: string
      is_tie?: boolean
      winner?: number | null
    } | null
  }
  isMyTurn: boolean
  myPlayerIndex?: number
  onMove: (choice: Choice) => void
  disabled?: boolean
}

const choicesConfig: { id: Choice; label: string; icon: string; beats: string }[] = [
  { id: 'rock', label: 'Rock', icon: '🪨', beats: 'Scissors' },
  { id: 'paper', label: 'Paper', icon: '📄', beats: 'Rock' },
  { id: 'scissors', label: 'Scissors', icon: '✂️', beats: 'Paper' },
]

export function RockPaperScissorsBoard({
  boardState,
  isMyTurn,
  myPlayerIndex,
  onMove,
  disabled = false,
}: RockPaperScissorsBoardProps) {
  const [selected, setSelected] = useState<Choice | null>(null)

  const choices = boardState?.choices || {}
  const myKey = myPlayerIndex !== undefined ? String(myPlayerIndex) : ''
  const opponentKey = myPlayerIndex !== undefined ? String(1 - myPlayerIndex) : ''

  const myChoice = myKey ? choices[myKey] : undefined
  const opponentPicked = opponentKey ? !!choices[opponentKey] : false

  const hasSubmitted = !!myChoice && myChoice !== 'hidden'
  const lastResult = boardState?.last_round_result

  const handlePick = (choice: Choice) => {
    if (disabled || hasSubmitted || !isMyTurn) return
    setSelected(choice)
  }

  const handleConfirm = () => {
    if (!selected || hasSubmitted || disabled || !isMyTurn) return
    onMove(selected)
  }

  return (
    <div className="flex flex-col items-center justify-center p-4 max-w-lg w-full mx-auto select-none">
      {/* Last Round Reveal Banner */}
      {lastResult && (
        <div
          className={[
            'w-full mb-6 p-4 rounded-2xl border text-center transition-all animate-in fade-in slide-in-from-top-2',
            lastResult.is_tie
              ? 'bg-amber-500/10 border-amber-500/30 text-amber-600 dark:text-amber-400'
              : lastResult.winner === myPlayerIndex
                ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-600 dark:text-emerald-400'
                : 'bg-rose-500/10 border-rose-500/30 text-rose-600 dark:text-rose-400',
          ].join(' ')}
        >
          <div className="text-sm font-semibold uppercase tracking-wider mb-1">
            {lastResult.is_tie ? '🤝 Round Tied' : lastResult.winner === myPlayerIndex ? '🎉 You Won the Round!' : '💥 Opponent Won the Round'}
          </div>
          <div className="flex items-center justify-center gap-6 text-xl font-bold">
            <span className="flex items-center gap-1.5">
              Player 1: {lastResult.p0_choice?.toUpperCase()}
            </span>
            <span className="text-muted-foreground font-normal">vs</span>
            <span className="flex items-center gap-1.5">
              Player 2: {lastResult.p1_choice?.toUpperCase()}
            </span>
          </div>
        </div>
      )}

      {/* Opponent Status Indicator */}
      <div className="w-full flex items-center justify-between bg-muted/40 border border-border/80 px-4 py-2.5 rounded-xl mb-6">
        <span className="text-sm text-muted-foreground font-medium">Opponent Status</span>
        <span
          className={[
            'inline-flex items-center gap-1.5 text-xs font-semibold px-2.5 py-1 rounded-full',
            opponentPicked
              ? 'bg-emerald-500/15 text-emerald-600 dark:text-emerald-400'
              : 'bg-amber-500/15 text-amber-600 dark:text-amber-400 animate-pulse',
          ].join(' ')}
        >
          {opponentPicked ? '✓ Ready (Secret Choice Locked)' : 'Thinking…'}
        </span>
      </div>

      {/* 3 Choice Cards */}
      <div className="grid grid-cols-3 gap-3 sm:gap-4 w-full mb-6">
        {choicesConfig.map((item) => {
          const isChosen = (hasSubmitted && myChoice === item.id) || (!hasSubmitted && selected === item.id)
          const canSelect = isMyTurn && !hasSubmitted && !disabled

          return (
            <button
              key={item.id}
              type="button"
              onClick={() => handlePick(item.id)}
              disabled={!canSelect}
              className={[
                'flex flex-col items-center justify-center p-4 sm:p-5 rounded-2xl border-2 transition-all relative',
                isChosen
                  ? 'border-primary bg-primary/10 shadow-lg shadow-primary/20 scale-105'
                  : canSelect
                    ? 'border-border/80 bg-card hover:bg-accent/40 hover:scale-[1.02] cursor-pointer'
                    : 'border-border/40 bg-muted/30 opacity-60 cursor-not-allowed',
              ].join(' ')}
            >
              <span className="text-4xl sm:text-5xl mb-2">{item.icon}</span>
              <span className="font-bold text-sm sm:text-base">{item.label}</span>
              <span className="text-[10px] text-muted-foreground mt-0.5">Beats {item.beats}</span>

              {isChosen && (
                <div className="absolute top-2 right-2 text-primary">
                  <CheckCircle2 className="w-4 h-4 fill-primary text-background" />
                </div>
              )}
            </button>
          )
        })}
      </div>

      {/* Confirmation Button or Status */}
      {hasSubmitted ? (
        <div className="text-center py-2">
          <div className="inline-flex items-center gap-2 text-sm font-medium text-emerald-600 dark:text-emerald-400 bg-emerald-500/10 px-4 py-2 rounded-full border border-emerald-500/20">
            <CheckCircle2 className="w-4 h-4" />
            <span>Choice submitted! Waiting for opponent to reveal...</span>
          </div>
        </div>
      ) : (
        <Button
          onClick={handleConfirm}
          disabled={!selected || !isMyTurn || disabled}
          className="w-full max-w-xs h-11 text-base font-semibold shadow-md"
        >
          {selected ? `Lock in ${selected.toUpperCase()}` : 'Select a Hand'}
        </Button>
      )}
    </div>
  )
}
