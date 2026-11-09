import { createPortal } from 'react-dom'
import { Trophy, Frown, Handshake, Eye, ArrowLeft, Loader2, RotateCcw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import type { GameOutcome } from './outcomeTypes'
import { getOutcomeTitle, getOutcomeSubtitle, getOutcomeDetail } from './outcomeExtractor'

interface GameOutcomeOverlayProps {
  outcome: GameOutcome
  isReviewingBoard: boolean
  onToggleReviewBoard: () => void
  onBackToLobby: () => void
  canRematch: boolean
  onRematch: () => void
  isRematchPending: boolean
  iRequestedRematch: boolean
  opponentRequestedRematch: boolean
}

export function GameOutcomeOverlay({
  outcome,
  isReviewingBoard,
  onToggleReviewBoard,
  onBackToLobby,
  canRematch,
  onRematch,
  isRematchPending,
  iRequestedRematch,
  opponentRequestedRematch,
}: GameOutcomeOverlayProps) {
  const { t } = useTranslation()
  const isWin = outcome.type === 'win'
  const isLoss = outcome.type === 'loss'

  const title = getOutcomeTitle(outcome, t)
  const subtitle = getOutcomeSubtitle(outcome, t)
  const detail = getOutcomeDetail(outcome, t)

  // ── Review mode: touch-friendly floating bar that doesn't obstruct the board ─
  if (isReviewingBoard) {
    const reviewBarContent = (
      <aside
        aria-label={t('p2p.outcome.matchOutcomeControls', 'Match outcome controls')}
        className="fixed bottom-[calc(env(safe-area-inset-bottom,0px)+4.25rem)] sm:bottom-6 left-1/2 z-50 -translate-x-1/2 flex items-center justify-between sm:justify-center gap-2 sm:gap-3 rounded-full border border-border/80 bg-background/95 px-3 py-1.5 sm:px-5 sm:py-2.5 shadow-2xl backdrop-blur-md animate-in slide-in-from-bottom-3 w-[calc(100%-1.5rem)] max-w-md sm:w-auto"
      >
        <div className="flex items-center gap-1.5 sm:gap-2 font-bold text-xs shrink-0">
          <span className="text-sm sm:text-base">{isWin ? '🏆' : isLoss ? '💀' : '🤝'}</span>
          <span className={isWin ? 'text-success' : isLoss ? 'text-destructive' : 'text-warning'}>
            {title}
          </span>
          <span className="text-muted-foreground hidden md:inline">· {subtitle}</span>
        </div>

        <div className="h-5 w-px bg-border/80 shrink-0 hidden sm:block" />

        <div className="flex items-center gap-1.5 sm:gap-2 shrink-0">
          <Button
            size="sm"
            variant="secondary"
            onClick={onToggleReviewBoard}
            aria-label={t('p2p.outcome.viewResults', 'View Results')}
            className="h-11 min-h-[44px] min-w-[44px] text-xs font-semibold gap-1.5 rounded-full px-3.5 sm:px-4 shadow-sm active:scale-95 transition-transform"
          >
            <Trophy className="h-4 w-4 text-primary" />
            <span>{t('p2p.outcome.viewResults', 'View Results')}</span>
          </Button>

          {canRematch && (
            <Button
              size="sm"
              onClick={onRematch}
              disabled={isRematchPending || iRequestedRematch}
              aria-label={t('p2p.outcome.rematch', 'Rematch')}
              className="h-11 min-h-[44px] min-w-[44px] text-xs font-bold rounded-full gap-1.5 px-3.5 sm:px-4 shadow-md shadow-primary/20 active:scale-95 transition-transform"
            >
              {iRequestedRematch ? (
                <>
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  <span className="hidden sm:inline">{t('p2p.outcome.waitingForRematch', 'Waiting…')}</span>
                </>
              ) : opponentRequestedRematch ? (
                <>
                  <RotateCcw className="h-3.5 w-3.5" />
                  <span>{t('p2p.outcome.acceptRematch', 'Accept')}</span>
                </>
              ) : (
                <>
                  <RotateCcw className="h-3.5 w-3.5" />
                  <span>{t('p2p.outcome.rematch', 'Rematch')}</span>
                </>
              )}
            </Button>
          )}
        </div>
      </aside>
    )

    if (typeof document !== 'undefined') {
      return createPortal(reviewBarContent, document.body)
    }
    return reviewBarContent
  }

  // ── Full result card — absolutely positioned over the board ────────────────
  return (
    <>
      {/* Translucent backdrop — covers the board but lets it show through */}
      <div
        aria-hidden="true"
        className="absolute inset-0 z-10 rounded-xl bg-background/60 backdrop-blur-[2px] pointer-events-none"
      />

      {/* Centered card — pointer-events restored so buttons work */}
      <div
        role="dialog"
        aria-modal="false"
        aria-labelledby="outcome-title"
        className="absolute inset-0 z-20 flex items-center justify-center p-4 pointer-events-none"
      >
        <div
          className={[
            'pointer-events-auto w-full max-w-xs rounded-2xl border bg-card/90 p-5 shadow-2xl backdrop-blur-md',
            'animate-in zoom-in-95 duration-200',
            isWin
              ? 'border-success/40 shadow-success/10'
              : isLoss
                ? 'border-destructive/40 shadow-destructive/10'
                : 'border-warning/40 shadow-warning/10',
          ].join(' ')}
        >
          <div className="flex flex-col items-center text-center space-y-3">
            {/* Outcome Emblem */}
            <div
              className={[
                'relative flex h-14 w-14 items-center justify-center rounded-2xl shadow-lg border-2',
                isWin
                  ? 'border-success/40 bg-success/15 text-success shadow-success/20'
                  : isLoss
                    ? 'border-destructive/40 bg-destructive/15 text-destructive shadow-destructive/20'
                    : 'bg-warning/15 border-warning/40 text-warning shadow-warning/20',
              ].join(' ')}
            >
              {isWin ? (
                <Trophy className="h-7 w-7 animate-bounce" />
              ) : isLoss ? (
                <Frown className="h-7 w-7" />
              ) : (
                <Handshake className="h-7 w-7" />
              )}
              {isWin && (
                <span className="absolute -top-1.5 -right-1.5 flex h-5 w-5 items-center justify-center rounded-full bg-success text-white shadow">
                </span>
              )}
            </div>

            {/* Title & Explanation */}
            <div className="space-y-1">
              <Badge
                variant="outline"
                className={[
                  'text-[11px] font-extrabold uppercase tracking-widest px-2.5 py-0.5',
                  isWin
                    ? 'border-success/40 text-success bg-success/10'
                    : isLoss
                      ? 'border-destructive/40 text-destructive bg-destructive/10'
                      : 'border-warning/40 text-warning bg-warning/10',
                ].join(' ')}
              >
                {isWin
                  ? t('p2p.outcome.matchWon', 'Match Won')
                  : isLoss
                    ? t('p2p.outcome.matchLost', 'Match Lost')
                    : t('p2p.outcome.matchTied', 'Match Tied')}
              </Badge>
              <h2 id="outcome-title" className="text-xl sm:text-2xl font-black tracking-tight">
                {title}
              </h2>
              <p className="text-xs font-semibold text-foreground/90 max-w-[220px] mx-auto">
                {subtitle}
              </p>
              <p className="text-[11px] text-muted-foreground">
                {detail} ·{' '}
                <span className="font-semibold text-foreground">
                  {t('p2p.outcome.vsOpponent', { name: outcome.opponentName, defaultValue: `vs ${outcome.opponentName}` })}
                </span>
              </p>
            </div>

            {/* Opponent Rematch Notice */}
            {opponentRequestedRematch && (
              <div className="w-full rounded-xl border border-success/30 bg-success/10 p-2 text-[11px] font-semibold text-success animate-pulse">
                {t('p2p.outcome.opponentRematchPrompt', 'Your opponent wants a rematch! Accept to play immediately.')}
              </div>
            )}

            {/* Action Controls */}
            <div className="w-full pt-1 flex flex-col gap-2">
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={onToggleReviewBoard}
                  className="flex-1 text-xs font-semibold min-h-[44px] h-10 gap-1.5 active:scale-95 transition-transform"
                >
                  <Eye className="h-4 w-4" />
                  <span>{t('p2p.outcome.reviewBoard', 'Review Board')}</span>
                </Button>

                {canRematch ? (
                  <Button
                    size="sm"
                    onClick={onRematch}
                    disabled={isRematchPending || iRequestedRematch}
                    className="flex-1 text-xs font-bold min-h-[44px] h-10 gap-1.5 shadow-md shadow-primary/20 active:scale-95 transition-transform"
                  >
                    {iRequestedRematch ? (
                      <>
                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                        <span>{t('p2p.outcome.waitingForRematch', 'Waiting…')}</span>
                      </>
                    ) : opponentRequestedRematch ? (
                      <>
                        <RotateCcw className="h-3.5 w-3.5" />
                        <span>{t('p2p.outcome.acceptRematch', 'Accept')}</span>
                      </>
                    ) : (
                      <>
                        <RotateCcw className="h-3.5 w-3.5" />
                        <span>{t('p2p.outcome.playAgain', 'Play Again')}</span>
                      </>
                    )}
                  </Button>
                ) : null}
              </div>

              <Button
                variant="ghost"
                size="sm"
                onClick={onBackToLobby}
                className="text-xs text-muted-foreground hover:text-foreground min-h-[44px] h-10 gap-1.5 active:scale-95 transition-transform"
              >
                <ArrowLeft className="h-4 w-4" />
                <span>{t('p2p.outcome.returnToLobby', 'Return to Lobby')}</span>
              </Button>
            </div>
          </div>
        </div>
      </div>
    </>
  )
}
