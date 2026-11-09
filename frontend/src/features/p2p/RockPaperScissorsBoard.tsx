import { useState } from 'react'
import { CheckCircle2, Trophy, Swords } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import type { TFunction } from 'i18next'

import { Button } from '@/components/ui/button'
import { RpsChoiceIcon } from './RockPaperScissorsIcons'
import {
  RPS_CHOICES,
  RPS_CHOICE_MAP,
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
  isGameOver?: boolean
}

const VERDICT_TONE: Record<RpsVerdict, { box: string; text: string }> = {
  win: { box: 'border-emerald-500/40 bg-emerald-500/15 ring-2 ring-emerald-500/20', text: 'text-success' },
  lose: { box: 'border-destructive/40 bg-destructive/15 ring-2 ring-destructive/20', text: 'text-destructive' },
  draw: { box: 'border-warning/40 bg-warning/15 ring-2 ring-warning/20', text: 'text-warning' },
}

function getChoiceLabel(choice: RpsChoice, t: TFunction): string {
  switch (choice) {
    case 'rock':
      return t('p2p.rps.rock', 'Rock')
    case 'paper':
      return t('p2p.rps.paper', 'Paper')
    case 'scissors':
      return t('p2p.rps.scissors', 'Scissors')
    default:
      return choice
  }
}

function rpsExplanation(c1: RpsChoice, c2: RpsChoice, t: TFunction): string {
  const c1Label = getChoiceLabel(c1, t)
  const c2Label = getChoiceLabel(c2, t)
  if (c1 === c2) {
    return t('p2p.outcome.rpsBothPicked', { choice: c1Label, defaultValue: `Both picked ${c1Label} — Tied round` })
  }
  if ((c1 === 'rock' && c2 === 'scissors') || (c2 === 'rock' && c1 === 'scissors')) {
    return t('p2p.outcome.rpsRockCrushesScissors', 'Rock crushes Scissors')
  }
  if ((c1 === 'scissors' && c2 === 'paper') || (c2 === 'scissors' && c1 === 'paper')) {
    return t('p2p.outcome.rpsScissorsCutPaper', 'Scissors cut Paper')
  }
  if ((c1 === 'paper' && c2 === 'rock') || (c2 === 'paper' && c1 === 'rock')) {
    return t('p2p.outcome.rpsPaperCoversRock', 'Paper covers Rock')
  }
  return `${c1Label} vs ${c2Label}`
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
            'h-2 w-4 rounded-full transition-all duration-300',
            index < wins ? 'bg-emerald-500 shadow-sm shadow-emerald-500/40' : 'bg-muted-foreground/25',
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
      <span className="truncate text-[10px] font-bold uppercase tracking-wider text-muted-foreground">{name}</span>
      <div className={['flex items-center gap-2', align === 'right' ? 'flex-row-reverse' : ''].join(' ')}>
        <span className="text-xl font-black tabular-nums leading-none">{wins}</span>
        <WinDots wins={wins} target={target} label={name} />
      </div>
    </div>
  )
}

function RevealTile({
  caption,
  choice,
  isWinningSide,
  isLosingSide,
  t,
}: {
  caption: string
  choice: RpsChoice
  isWinningSide: boolean
  isLosingSide: boolean
  t: TFunction
}) {
  const meta = RPS_CHOICE_MAP[choice]
  const choiceText = getChoiceLabel(choice, t)
  return (
    <div
      className={[
        'flex min-w-0 flex-1 flex-col items-center gap-1.5 rounded-2xl border-2 px-3 py-3 transition-all duration-300',
        isWinningSide
          ? 'border-emerald-500/60 bg-emerald-500/20 shadow-lg shadow-emerald-500/25 scale-105 z-10'
          : isLosingSide
            ? 'border-rose-500/30 bg-rose-500/5 opacity-60 scale-95'
            : 'border-border/70 bg-background/80',
      ].join(' ')}
    >
      <div className="relative">
        <RpsChoiceIcon choice={choice} className={['h-9 w-9 sm:h-10 sm:w-10', meta.accentClass].join(' ')} />
        {isWinningSide && (
          <span className="absolute -top-1 -right-1 text-success animate-pulse">
            <Trophy className="h-4 w-4" />
          </span>
        )}
      </div>
      <span className="truncate text-xs sm:text-sm font-extrabold">{choiceText}</span>
      <span className="text-[10px] font-bold uppercase tracking-wide text-muted-foreground">{caption}</span>
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
  isGameOver = false,
}: RockPaperScissorsBoardProps) {
  const { t } = useTranslation()
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
  const canSelect = isMyTurn && !hasSubmitted && !disabled && !isGameOver

  const lastResult = boardState?.last_round_result ?? null
  const verdict = roundVerdict(lastResult, myPlayerIndex)
  const myRevealed = parseRpsChoice(myPlayerIndex === 1 ? lastResult?.p1_choice : lastResult?.p0_choice)
  const theirRevealed = parseRpsChoice(myPlayerIndex === 1 ? lastResult?.p0_choice : lastResult?.p1_choice)

  const handlePick = (choice: RpsChoice) => {
    if (!canSelect) return
    setDraft({ round: roundNumber, choice })
  }

  const handleConfirm = () => {
    if (!selected || hasSubmitted || disabled || !isMyTurn || isGameOver) return
    onMove(selected)
  }

  return (
    <div className="mx-auto flex w-full max-w-lg select-none flex-col items-center justify-center p-4">
      {/* Score, round progress and target wins for this best-of-three */}
      {series && (
        <div className="mb-4 grid w-full grid-cols-[1fr_auto_1fr] items-center gap-3 rounded-2xl border border-border/80 bg-muted/40 px-4 py-3 shadow-sm">
          <ScoreColumn name={t('p2p.rps.you', 'You')} wins={series.myWins} target={series.winsNeeded} align="left" />
          <div className="text-center px-2">
            <div className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">{t('common.round', 'Round')}</div>
            <div className="text-sm font-black tabular-nums">
              {series.roundsPlayed} <span className="font-semibold text-muted-foreground">{t('common.of', 'of')} {maxRounds ?? 3}</span>
            </div>
            <div className="text-[10px] text-muted-foreground">
              {t('p2p.rps.firstToWins', { count: series.winsNeeded, defaultValue: `First to ${series.winsNeeded} wins` })}
            </div>
          </div>
          <ScoreColumn name={t('p2p.rps.opponent', 'Opponent')} wins={series.opponentWins} target={series.winsNeeded} align="right" />
        </div>
      )}

      {/* Simultaneous reveal clash & interaction presentation */}
      {lastResult && myRevealed && theirRevealed && verdict && (
        <div
          role="status"
          aria-live="polite"
          className={['rps-reveal mb-4 w-full rounded-2xl border p-4 shadow-md', VERDICT_TONE[verdict].box].join(' ')}
        >
          <div className="flex items-center justify-center gap-2 mb-1">
            <Swords className="h-4 w-4 shrink-0 text-muted-foreground" />
            <div className={['text-center text-sm font-black uppercase tracking-wider', VERDICT_TONE[verdict].text].join(' ')}>
              {verdict === 'draw'
                ? t('p2p.rps.tiedRound', 'Round Drawn — Tied Move!')
                : verdict === 'win'
                  ? t('p2p.rps.youWonRound', 'You Won the Round!')
                  : t('p2p.rps.opponentWonRound', 'Opponent Won the Round!')}
            </div>
          </div>

          <p className="text-center text-xs font-bold text-foreground/90 mb-3">
            {rpsExplanation(myRevealed, theirRevealed, t)}
          </p>

          <div className="flex items-center justify-center gap-3">
            <RevealTile
              caption={t('p2p.rps.you', 'You')}
              choice={myRevealed}
              isWinningSide={verdict === 'win'}
              isLosingSide={verdict === 'lose'}
              t={t}
            />
            <span className="text-xs font-extrabold uppercase tracking-wide text-muted-foreground">VS</span>
            <RevealTile
              caption={t('p2p.rps.opponent', 'Opponent')}
              choice={theirRevealed}
              isWinningSide={verdict === 'lose'}
              isLosingSide={verdict === 'win'}
              t={t}
            />
          </div>

          {verdict === 'draw' && series && (
            <p className="mt-2 text-center text-[11px] text-muted-foreground">
              {t('p2p.rps.drawnNotice', 'A drawn round yields no point for either player.')}
            </p>
          )}
        </div>
      )}

      {/* Opponent readiness indicator (when match still active) */}
      {!isGameOver && (
        <div className="mb-4 flex w-full items-center justify-between rounded-xl border border-border/80 bg-muted/40 px-4 py-2.5">
          <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            {t('p2p.rps.opponentStatus', 'Opponent Status')}
          </span>
          <span
            className={[
              'inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-semibold',
              opponentPicked
                ? 'bg-success/15 text-success'
                : 'bg-warning/15 text-warning',
            ].join(' ')}
          >
            {opponentPicked ? t('p2p.rps.choiceLockedIn', 'Choice Locked In') : t('p2p.rps.choosingMove', 'Choosing move…')}
          </span>
        </div>
      )}

      {/* Move selection tiles */}
      <div className="mb-5 grid w-full grid-cols-3 gap-2.5 sm:gap-4" role="group" aria-label={t('p2p.rps.makeYourChoice', 'Choose your move')}>
        {RPS_CHOICES.map((meta) => {
          const isChosen = hasSubmitted ? myChoice === meta.id : selected === meta.id
          const accent = RPS_CHOICE_MAP[meta.id]
          const label = getChoiceLabel(meta.id, t)
          const beats = getChoiceLabel(meta.beats, t)

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
                    : 'cursor-not-allowed border-border/40 bg-muted/30 opacity-50',
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
              <span className="text-sm font-bold sm:text-base">{label}</span>
              <span className="text-[10px] text-muted-foreground">
                {t('p2p.rps.beats', { move: beats, defaultValue: `Beats ${beats}` })}
              </span>

              {isChosen && (
                <span className="absolute right-2 top-2 text-primary" aria-hidden="true">
                  <CheckCircle2 className="h-4 w-4" />
                </span>
              )}
            </button>
          )
        })}
      </div>

      {/* Action / Selection Confirmation */}
      {isGameOver ? (
        <div className="w-full text-center py-2 text-xs font-semibold text-muted-foreground">
          {t('p2p.outcome.gameCompleted', 'Match completed · Final decisive move revealed above')}
        </div>
      ) : hasSubmitted && myChoice ? (
        <div className="flex flex-col items-center gap-1 text-center">
          <div className="inline-flex items-center gap-2 rounded-full border border-success/20 bg-success/10 px-4 py-2 text-sm font-semibold text-success">
            <RpsChoiceIcon choice={myChoice} className={['h-4 w-4', RPS_CHOICE_MAP[myChoice].accentClass].join(' ')} />
            <span>
              {t('p2p.rps.lockedInChoice', { choice: getChoiceLabel(myChoice, t), defaultValue: `Locked in: ${getChoiceLabel(myChoice, t)}` })}
            </span>
          </div>
          <p className="text-[11px] text-muted-foreground">
            {t('p2p.rps.simultaneousNotice', 'Both moves will reveal simultaneously once your opponent locks in.')}
          </p>
        </div>
      ) : !isMyTurn ? (
        <p className="py-2 text-center text-xs text-muted-foreground">
          {t('p2p.rps.waitingOpponentLockIn', 'Waiting for your opponent to lock in…')}
        </p>
      ) : (
        <Button
          onClick={handleConfirm}
          disabled={!selected || disabled}
          className="h-11 w-full max-w-xs text-base font-semibold shadow-md"
        >
          {selected ? (
            <span className="inline-flex items-center gap-2">
              {t('p2p.rps.lockInBtn', 'Lock in')}
              <RpsChoiceIcon choice={selected} className="h-4 w-4" />
              {getChoiceLabel(selected, t)}
            </span>
          ) : (
            t('p2p.rps.selectMove', 'Select a move')
          )}
        </Button>
      )}
    </div>
  )
}
