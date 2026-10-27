import { useState } from 'react'
import { CheckCircle2 } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { RpsChoiceIcon } from './RockPaperScissorsIcons'
import {
  RPS_CHOICES,
  RPS_CHOICE_MAP,
  choiceLabel,
  parseRpsChoice,
  roundVerdict,
  seriesProgress,
  type RpsChoice,
  type RpsRoundResult,
  type RpsVerdict,
} from './rockPaperScissors'

interface RockPaperScissorsBoardProps {
  boardState: {
    choices?: Record<string, string>
    last_round_result?: RpsRoundResult | null
  }
  isMyTurn: boolean
  myPlayerIndex?: number
  scores?: Record<string, number>
  currentRound?: number
  maxRounds?: number
  onMove: (choice: RpsChoice) => void
  disabled?: boolean
}

const VERDICT_TONE: Record<RpsVerdict, { box: string; text: string }> = {
  win: { box: 'border-emerald-500/30 bg-emerald-500/10', text: 'text-emerald-600 dark:text-emerald-400' },
  lose: { box: 'border-rose-500/30 bg-rose-500/10', text: 'text-rose-600 dark:text-rose-400' },
  draw: { box: 'border-amber-500/30 bg-amber-500/10', text: 'text-amber-600 dark:text-amber-400' },
}

/** Best-of-three progress: filled segments are the rounds a side has taken. */
function WinDots({ wins, target, label }: { wins: number; target: number; label: string }) {
  return (
    <span className="flex items-center gap-1" role="img" aria-label={`${label}: ${wins} of ${target} round wins`}>
      {Array.from({ length: target }, (_, index) => (
        <span
          key={index}
          aria-hidden="true"
          className={[
            'h-1.5 w-4 rounded-full transition-colors duration-200',
            index < wins ? 'bg-emerald-500' : 'bg-muted-foreground/25',
          ].join(' ')}
        />
      ))}
    </span>
  )
}

function ScoreColumn({
  name,
  wins,
  target,
  align,
}: {
  name: string
  wins: number
  target: number
  align: 'left' | 'right'
}) {
  return (
    <div className={['flex min-w-0 flex-col gap-1', align === 'right' ? 'items-end text-right' : 'items-start'].join(' ')}>
      <span className="truncate text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">{name}</span>
      <div className={['flex items-center gap-2', align === 'right' ? 'flex-row-reverse' : ''].join(' ')}>
        <span className="text-lg font-black tabular-nums leading-none">{wins}</span>
        <WinDots wins={wins} target={target} label={name} />
      </div>
    </div>
  )
}

function RevealTile({
  caption,
  choice,
  highlight,
}: {
  caption: string
  choice: RpsChoice
  highlight: boolean
}) {
  const meta = RPS_CHOICE_MAP[choice]
  return (
    <div
      className={[
        'flex min-w-0 flex-1 flex-col items-center gap-1 rounded-xl border px-3 py-2 transition-colors',
        highlight ? 'border-emerald-500/50 bg-emerald-500/10' : 'border-border/70 bg-background/60',
      ].join(' ')}
    >
      <RpsChoiceIcon choice={choice} className={['h-7 w-7', meta.accentClass].join(' ')} />
      <span className="truncate text-xs font-bold">{meta.label}</span>
      <span className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">{caption}</span>
    </div>
  )
}

export function RockPaperScissorsBoard({
  boardState,
  isMyTurn,
  myPlayerIndex,
  scores,
  currentRound,
  maxRounds,
  onMove,
  disabled = false,
}: RockPaperScissorsBoardProps) {
  // A pick belongs to one round only. The server clears `choices` when a round
  // resolves, so tagging the draft with its round keeps a previous round's
  // selection from reappearing as a pre-selected move.
  const [draft, setDraft] = useState<{ round: number; choice: RpsChoice } | null>(null)

  const choices = boardState?.choices || {}
  const myKey = myPlayerIndex !== undefined ? String(myPlayerIndex) : ''
  const opponentKey = myPlayerIndex !== undefined ? String(1 - myPlayerIndex) : ''

  const myChoice = myKey ? parseRpsChoice(choices[myKey]) : null
  const hasSubmitted = !!myChoice
  const opponentPicked = opponentKey ? !!choices[opponentKey] : false

  const roundNumber = currentRound && currentRound > 0 ? currentRound : 1
  const series = seriesProgress(scores, myPlayerIndex, roundNumber, maxRounds)
  const selected = draft && draft.round === roundNumber ? draft.choice : null
  const canSelect = isMyTurn && !hasSubmitted && !disabled

  const lastResult = boardState?.last_round_result ?? null
  const verdict = roundVerdict(lastResult, myPlayerIndex)
  const myRevealed = parseRpsChoice(myPlayerIndex === 1 ? lastResult?.p1_choice : lastResult?.p0_choice)
  const theirRevealed = parseRpsChoice(myPlayerIndex === 1 ? lastResult?.p0_choice : lastResult?.p1_choice)

  const handlePick = (choice: RpsChoice) => {
    if (!canSelect) return
    setDraft({ round: roundNumber, choice })
  }

  const handleConfirm = () => {
    if (!selected || hasSubmitted || disabled || !isMyTurn) return
    onMove(selected)
  }

  return (
    <div className="mx-auto flex w-full max-w-lg select-none flex-col items-center justify-center p-4">
      {/* Score, round progress and the target wins for this best-of-three. */}
      {series && (
        <div className="mb-4 grid w-full grid-cols-[1fr_auto_1fr] items-center gap-3 rounded-xl border border-border/80 bg-muted/30 px-4 py-3">
          <ScoreColumn name="You" wins={series.myWins} target={series.winsNeeded} align="left" />
          <div className="text-center">
            <div className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Round</div>
            <div className="text-sm font-black tabular-nums">
              {series.roundsPlayed} <span className="font-semibold text-muted-foreground">of {maxRounds ?? 3}</span>
            </div>
            <div className="text-[10px] text-muted-foreground">First to {series.winsNeeded}</div>
          </div>
          <ScoreColumn name="Opponent" wins={series.opponentWins} target={series.winsNeeded} align="right" />
        </div>
      )}

      {/* Both moves are revealed together, as the engine resolves the round. */}
      {lastResult && myRevealed && theirRevealed && verdict && (
        <div
          role="status"
          aria-live="polite"
          className={['rps-reveal mb-4 w-full rounded-2xl border p-4', VERDICT_TONE[verdict].box].join(' ')}
        >
          <div className={['text-center text-sm font-bold uppercase tracking-wider', VERDICT_TONE[verdict].text].join(' ')}>
            {verdict === 'draw' ? 'Round drawn — no point scored' : verdict === 'win' ? 'You won the round' : 'Opponent won the round'}
          </div>
          <div className="mt-3 flex items-center justify-center gap-3">
            <RevealTile caption="You" choice={myRevealed} highlight={verdict === 'win'} />
            <span className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">vs</span>
            <RevealTile caption="Opponent" choice={theirRevealed} highlight={verdict === 'lose'} />
          </div>
          {verdict === 'draw' && series && (
            <p className="mt-2 text-center text-[11px] text-muted-foreground">
              A draw uses up the round and scores for neither side.
            </p>
          )}
        </div>
      )}

      {/* Opponent readiness: the opponent's move stays masked until both pick. */}
      <div className="mb-4 flex w-full items-center justify-between rounded-xl border border-border/80 bg-muted/40 px-4 py-2.5">
        <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Opponent</span>
        <span
          className={[
            'inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-semibold',
            opponentPicked
              ? 'bg-emerald-500/15 text-emerald-600 dark:text-emerald-400'
              : 'bg-amber-500/15 text-amber-600 dark:text-amber-400',
          ].join(' ')}
        >
          {opponentPicked ? 'Locked in' : 'Choosing…'}
        </span>
      </div>

      {/* The three moves, presented as game pieces rather than buttons. */}
      <div className="mb-5 grid w-full grid-cols-3 gap-2.5 sm:gap-4" role="group" aria-label="Choose your move">
        {RPS_CHOICES.map((meta) => {
          const isChosen = hasSubmitted ? myChoice === meta.id : selected === meta.id
          const accent = RPS_CHOICE_MAP[meta.id]

          return (
            <button
              key={meta.id}
              type="button"
              onClick={() => handlePick(meta.id)}
              disabled={!canSelect}
              aria-pressed={isChosen}
              className={[
                'group relative flex flex-col items-center justify-center gap-2 rounded-2xl border-2 p-3 sm:p-4',
                'outline-none transition-all duration-150',
                'focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background',
                isChosen
                  ? 'rps-choice-select border-primary bg-primary/10 shadow-sm'
                  : canSelect
                    ? 'cursor-pointer border-border/80 bg-card hover:border-primary/40 hover:bg-accent/40 active:scale-[0.98]'
                    : 'cursor-not-allowed border-border/40 bg-muted/30 opacity-60',
              ].join(' ')}
            >
              <span
                className={[
                  'flex h-12 w-12 items-center justify-center rounded-xl border transition-transform duration-150 sm:h-14 sm:w-14',
                  accent.tileClass,
                  isChosen ? 'scale-105' : canSelect ? 'group-hover:scale-105' : '',
                ].join(' ')}
              >
                <RpsChoiceIcon
                  choice={meta.id}
                  className={['h-7 w-7 sm:h-8 sm:w-8', accent.accentClass].join(' ')}
                />
              </span>
              <span className="text-sm font-bold sm:text-base">{meta.label}</span>
              <span className="text-[10px] text-muted-foreground">Beats {meta.beatsLabel}</span>

              {isChosen && (
                <span className="absolute right-2 top-2 text-primary" aria-hidden="true">
                  <CheckCircle2 className="h-4 w-4" />
                </span>
              )}
            </button>
          )
        })}
      </div>

      {/* Selection is confirmed explicitly, then locked until the round resolves. */}
      {hasSubmitted && myChoice ? (
        <div className="flex flex-col items-center gap-1 text-center">
          <div className="inline-flex items-center gap-2 rounded-full border border-emerald-500/20 bg-emerald-500/10 px-4 py-2 text-sm font-semibold text-emerald-600 dark:text-emerald-400">
            <RpsChoiceIcon choice={myChoice} className={['h-4 w-4', RPS_CHOICE_MAP[myChoice].accentClass].join(' ')} />
            <span>Locked in: {choiceLabel(myChoice)}</span>
          </div>
          <p className="text-[11px] text-muted-foreground">
            Both moves reveal together once your opponent picks.
          </p>
        </div>
      ) : !isMyTurn ? (
        <p className="py-2 text-center text-xs text-muted-foreground">
          Waiting for your opponent to lock in…
        </p>
      ) : (
        <Button
          onClick={handleConfirm}
          disabled={!selected || disabled}
          className="h-11 w-full max-w-xs text-base font-semibold shadow-md"
        >
          {selected ? (
            <span className="inline-flex items-center gap-2">
              Lock in
              <RpsChoiceIcon choice={selected} className="h-4 w-4" />
              {choiceLabel(selected)}
            </span>
          ) : (
            'Select a move'
          )}
        </Button>
      )}
    </div>
  )
}
