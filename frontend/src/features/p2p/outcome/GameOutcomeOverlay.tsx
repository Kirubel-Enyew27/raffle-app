import { Trophy, Frown, Handshake, Eye, ArrowLeft, Loader2, RotateCcw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import type { GameOutcome } from './outcomeTypes'

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
  const isWin = outcome.type === 'win'
  const isLoss = outcome.type === 'loss'

  // ── Review mode: minimal floating bar that doesn't cover the board ─────────
  // Kept as a fixed bar at the bottom so the player can inspect the board freely.
  if (isReviewingBoard) {
    return (
      <aside
        aria-label="Match outcome controls"
        className="fixed bottom-4 left-1/2 z-40 -translate-x-1/2 flex items-center gap-3 rounded-full border border-border/80 bg-background/95 px-5 py-2.5 shadow-2xl backdrop-blur-md animate-in slide-in-from-bottom-3"
      >
        <div className="flex items-center gap-2 font-bold text-xs">
          <span>{isWin ? '🏆' : isLoss ? '💀' : '🤝'}</span>
          <span className={isWin ? 'text-emerald-500' : isLoss ? 'text-rose-500' : 'text-amber-500'}>
            {outcome.title}
          </span>
          <span className="text-muted-foreground hidden sm:inline">· {outcome.subtitle}</span>
        </div>

        <div className="h-4 w-px bg-border/80" />

        <Button
          size="sm"
          variant="secondary"
          onClick={onToggleReviewBoard}
          className="h-7 text-xs font-semibold gap-1.5 rounded-full"
        >
          <Trophy className="h-3.5 w-3.5" />
          <span>Show Summary</span>
        </Button>

        {canRematch && (
          <Button
            size="sm"
            onClick={onRematch}
            disabled={isRematchPending || iRequestedRematch}
            className="h-7 text-xs font-semibold rounded-full gap-1"
          >
            {iRequestedRematch ? (
              <>
                <Loader2 className="h-3 w-3 animate-spin" />
                <span className="hidden sm:inline">Waiting…</span>
              </>
            ) : opponentRequestedRematch ? (
              'Accept Rematch'
            ) : (
              'Rematch'
            )}
          </Button>
        )}
      </aside>
    )
  }

  // ── Full result card — absolutely positioned over the board ────────────────
  //
  // The parent in P2PMatchPage has `position: relative` (via the
  // `relative w-full flex flex-col items-center` wrapper), so this element
  // sits on top of the board while the board itself remains fully visible
  // through the semi-transparent backdrop.
  //
  // pointer-events on the backdrop are none so clicks pass through to the
  // board for board review; the card itself restores pointer-events.
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
              ? 'border-emerald-500/40 shadow-emerald-500/10'
              : isLoss
                ? 'border-rose-500/40 shadow-rose-500/10'
                : 'border-amber-500/40 shadow-amber-500/10',
          ].join(' ')}
        >
          <div className="flex flex-col items-center text-center space-y-3">
            {/* Outcome Emblem */}
            <div
              className={[
                'relative flex h-14 w-14 items-center justify-center rounded-2xl shadow-lg border-2',
                isWin
                  ? 'bg-emerald-500/15 border-emerald-500/40 text-emerald-500 shadow-emerald-500/20'
                  : isLoss
                    ? 'bg-rose-500/15 border-rose-500/40 text-rose-500 shadow-rose-500/20'
                    : 'bg-amber-500/15 border-amber-500/40 text-amber-500 shadow-amber-500/20',
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
                <span className="absolute -top-1.5 -right-1.5 flex h-5 w-5 items-center justify-center rounded-full bg-emerald-500 text-white shadow">
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
                    ? 'border-emerald-500/40 text-emerald-600 dark:text-emerald-400 bg-emerald-500/10'
                    : isLoss
                      ? 'border-rose-500/40 text-rose-600 dark:text-rose-400 bg-rose-500/10'
                      : 'border-amber-500/40 text-amber-600 dark:text-amber-400 bg-amber-500/10',
                ].join(' ')}
              >
                {isWin ? 'Match Won' : isLoss ? 'Match Lost' : 'Match Tied'}
              </Badge>
              <h2 id="outcome-title" className="text-xl sm:text-2xl font-black tracking-tight">
                {outcome.title}
              </h2>
              <p className="text-xs font-semibold text-foreground/90 max-w-[220px] mx-auto">
                {outcome.subtitle}
              </p>
              <p className="text-[11px] text-muted-foreground">
                {outcome.detail} · vs{' '}
                <span className="font-semibold text-foreground">{outcome.opponentName}</span>
              </p>
            </div>

            {/* Opponent Rematch Notice */}
            {opponentRequestedRematch && (
              <div className="w-full rounded-xl border border-emerald-500/30 bg-emerald-500/10 p-2 text-[11px] font-semibold text-emerald-700 dark:text-emerald-300 animate-pulse">
                Your opponent wants a rematch! Accept to play immediately.
              </div>
            )}

            {/* Action Controls */}
            <div className="w-full pt-1 flex flex-col gap-2">
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={onToggleReviewBoard}
                  className="flex-1 text-xs font-semibold h-8 gap-1.5"
                >
                  <Eye className="h-3.5 w-3.5" />
                  <span>Review Board</span>
                </Button>

                {canRematch ? (
                  <Button
                    size="sm"
                    onClick={onRematch}
                    disabled={isRematchPending || iRequestedRematch}
                    className="flex-1 text-xs font-bold h-8 gap-1.5 shadow-md shadow-primary/20"
                  >
                    {iRequestedRematch ? (
                      <>
                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                        <span>Waiting…</span>
                      </>
                    ) : opponentRequestedRematch ? (
                      <>
                        <RotateCcw className="h-3.5 w-3.5" />
                        <span>Accept</span>
                      </>
                    ) : (
                      <>
                        <RotateCcw className="h-3.5 w-3.5" />
                        <span>Play Again</span>
                      </>
                    )}
                  </Button>
                ) : null}
              </div>

              <Button
                variant="ghost"
                size="sm"
                onClick={onBackToLobby}
                className="text-xs text-muted-foreground hover:text-foreground h-7 gap-1.5"
              >
                <ArrowLeft className="h-3.5 w-3.5" />
                <span>Return to Lobby</span>
              </Button>
            </div>
          </div>
        </div>
      </div>
    </>
  )
}
