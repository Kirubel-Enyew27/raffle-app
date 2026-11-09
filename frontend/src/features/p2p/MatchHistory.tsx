import { useEffect, type ReactNode } from 'react'
import {
  ChevronRight,
  Clock,
  Coins,
  Gamepad2,
  ReceiptText,
  RefreshCw,
  ShieldCheck,
  Trophy,
  Users,
  X,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import type { TFunction } from 'i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { GameArtwork } from '@/features/p2p/GameArtwork'
import { GameIcon } from '@/features/p2p/GameIcon'
import { cn, formatCurrency } from '@/lib/utils'
import type { P2PMatch, P2PMatchPlayer } from '@/types/api'
import {
  compactReference,
  formatMatchDate,
  gameName,
  isTerminalMatch,
  matchOutcome,
} from '@/features/p2p/matchHistory'

export function PaidMetric({
  label,
  value,
  highlight = false,
}: {
  label: string
  value: string
  highlight?: boolean
}) {
  return (
    <div className="min-w-0 space-y-0.5">
      <p className="text-[10px] font-semibold uppercase tracking-[0.1em] text-muted-foreground">
        {label}
      </p>
      <p className={cn('truncate font-bold tabular-nums', highlight ? 'text-success' : 'text-foreground')}>
        {value}
      </p>
    </div>
  )
}

function settlementInfo(
  match: P2PMatch,
  myPlayer?: P2PMatchPlayer,
  t?: TFunction,
): { label: string; className: string; isRefund: boolean } | null {
  if (match.competition_mode !== 'earn') return null

  const isRefund = match.status === 'tied' || match.status === 'cancelled'
  const hasLedger = Boolean(myPlayer?.debit_tx_id || myPlayer?.credit_tx_id)
  const isTerminal = isTerminalMatch(match)

  if (!isTerminal) {
    return {
      label: t ? t('p2p.history.settlementPending', 'Settlement pending') : 'Settlement pending',
      className: 'border-warning/30 bg-warning/10 text-warning',
      isRefund,
    }
  }

  if (isRefund) {
    return hasLedger
      ? {
          label: t ? t('p2p.history.refunded', 'Refunded') : 'Refunded',
          className: 'border-border bg-muted/60 text-muted-foreground',
          isRefund,
        }
      : {
          label: t ? t('p2p.history.refundPending', 'Refund pending') : 'Refund pending',
          className: 'border-warning/30 bg-warning/10 text-warning',
          isRefund,
        }
  }

  return hasLedger
    ? {
        label: t ? t('p2p.history.settled', 'Settled') : 'Settled',
        className: 'border-success/30 bg-success/10 text-success',
        isRefund,
      }
    : {
        label: t ? t('p2p.history.settlementPending', 'Settlement pending') : 'Settlement pending',
        className: 'border-warning/30 bg-warning/10 text-warning',
        isRefund,
      }
}

function returnValue(match: P2PMatch, currentUserId?: string): string {
  if (match.competition_mode !== 'earn') return '—'
  if (match.status === 'tied' || match.status === 'cancelled') {
    return formatCurrency(match.stake_amount)
  }
  return match.winner_user_id === currentUserId
    ? formatCurrency(match.potential_payout)
    : '—'
}

function formatDuration(startedAt?: string, completedAt?: string): string {
  if (!startedAt || !completedAt) return '—'
  const started = new Date(startedAt).getTime()
  const completed = new Date(completedAt).getTime()
  if (Number.isNaN(started) || Number.isNaN(completed) || completed < started) return '—'

  const totalSeconds = Math.round((completed - started) / 1000)
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60

  if (hours > 0) return `${hours}h ${minutes}m`
  if (minutes > 0) return `${minutes}m ${seconds}s`
  return `${seconds}s`
}

function playerScore(match: P2PMatch, player: P2PMatchPlayer | null, seat: number): number {
  const score = match.scores?.[String(player?.player_index ?? seat)]
  if (typeof score === 'number') return score
  return player?.score ?? 0
}

function playerLabel(player: P2PMatchPlayer | null, seat: number): string {
  return player?.symbol || `P${seat + 1}`
}

function avatarInitial(name?: string): string {
  return (name || '?').trim().charAt(0).toUpperCase() || '?'
}

function Avatar({
  name,
  url,
  className,
}: {
  name?: string
  url?: string
  className?: string
}) {
  if (url) {
    return (
      <img
        src={url}
        alt=""
        className={cn('shrink-0 rounded-full object-cover', className)}
      />
    )
  }
  return (
    <span
      aria-hidden="true"
      className={cn(
        'flex shrink-0 items-center justify-center rounded-full bg-muted font-bold text-muted-foreground',
        className,
      )}
    >
      {avatarInitial(name)}
    </span>
  )
}

export function MatchHistoryList({
  matches,
  isLoading,
  isError,
  onRetry,
  currentUserId,
  onSelect,
}: {
  matches: P2PMatch[]
  isLoading: boolean
  isError: boolean
  onRetry: () => void
  currentUserId?: string
  onSelect?: (match: P2PMatch) => void
}) {
  const { t, i18n } = useTranslation()

  if (isLoading) {
    return (
      <div className="space-y-2" role="status" aria-label={t('p2p.history.title', 'Loading recent match history')}>
        {[0, 1, 2].map((item) => (
          <div
            key={item}
            className="h-[74px] animate-pulse rounded-xl border border-border/60 bg-muted/30"
          />
        ))}
      </div>
    )
  }

  if (isError) {
    return (
      <div
        role="alert"
        className="rounded-xl border border-destructive/30 bg-destructive/5 p-5 text-center"
      >
        <p className="text-sm font-bold text-foreground">
          {t('p2p.history.couldNotLoad', 'Could not load match history')}
        </p>
        <p className="mt-1 text-xs text-muted-foreground">
          {t('p2p.history.recordsUnchanged', 'Your match and wallet records are unchanged.')}
        </p>
        <Button
          variant="outline"
          size="sm"
          onClick={onRetry}
          className="mt-3 h-8 gap-1.5 text-xs"
        >
          <RefreshCw className="h-3.5 w-3.5" />
          {t('common.retry', 'Try again')}
        </Button>
      </div>
    )
  }

  if (matches.length === 0) {
    return (
      <div className="rounded-xl border border-dashed border-border bg-background/30 p-6 text-center">
        <Trophy className="mx-auto h-7 w-7 text-muted-foreground/50" />
        <p className="mt-2 text-sm font-semibold text-foreground">
          {t('p2p.history.noCompletedMatches', 'No completed matches yet')}
        </p>
        <p className="mt-1 text-xs text-muted-foreground">
          {t('p2p.history.finishedDuelsAppear', 'Your finished duels and transaction details will appear here.')}
        </p>
      </div>
    )
  }

  return (
    <div className="space-y-2">
      {matches.map((match) => {
        const isPaid = match.competition_mode === 'earn'
        const myPlayer = match.players?.find((player) => player.user_id === currentUserId)
        const opponent = match.players?.find((player) => player.user_id !== currentUserId)
        const outcome = matchOutcome(match, currentUserId, t)
        const settlement = settlementInfo(match, myPlayer, t)
        const isRefund = settlement?.isRefund ?? false
        const payoutValue = isRefund
          ? formatCurrency(match.stake_amount)
          : match.winner_user_id === currentUserId
            ? formatCurrency(match.potential_payout)
            : '—'
        const opponentInitial = avatarInitial(opponent?.display_name)

        const openDetails = onSelect ? () => onSelect(match) : undefined
        const detailsLabel = `View details for ${gameName(match.game_slug, t)} against ${
          opponent?.display_name || t('p2p.rps.opponent', 'opponent')
        }`

        return (
          <div
            key={match.id}
            role={onSelect ? 'button' : undefined}
            tabIndex={onSelect ? 0 : undefined}
            aria-label={onSelect ? detailsLabel : undefined}
            onClick={openDetails}
            onKeyDown={
              onSelect
                ? (event) => {
                    if (event.key === 'Enter' || event.key === ' ') {
                      event.preventDefault()
                      onSelect(match)
                    }
                  }
                : undefined
            }
            className={cn(
              'rounded-xl border p-3.5',
              isPaid
                ? 'border-[#fbbf24]/25 bg-[#fbbf24]/[0.035]'
                : 'border-border/70 bg-background/30',
              onSelect &&
                'cursor-pointer transition-colors hover:border-primary/40 hover:bg-accent/30 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background active:scale-[0.995]',
            )}
          >
            <div className="flex items-start gap-3">
              <div
                className={cn(
                  'flex h-11 w-11 shrink-0 items-center justify-center rounded-xl border',
                  isPaid
                    ? 'border-[#fbbf24]/25 bg-[#fbbf24]/10'
                    : 'border-border/70 bg-muted/40',
                )}
              >
                <GameIcon slug={match.game_slug} size="md" />
              </div>

              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-1.5">
                  <span className="truncate text-sm font-bold text-foreground">
                    {gameName(match.game_slug, t)}
                  </span>
                  <span
                    className={cn(
                      'rounded-full border px-2 py-0.5 text-[10px] font-bold',
                      outcome.className,
                    )}
                  >
                    {outcome.label}
                  </span>
                  <span
                    className={cn(
                      'rounded-full border px-2 py-0.5 text-[10px] font-semibold',
                      isPaid
                        ? 'border-[#fbbf24]/30 bg-[#fbbf24]/10 text-[#b8860b] dark:text-[#fbbf24]'
                        : 'border-border bg-muted/40 text-muted-foreground',
                    )}
                  >
                    {isPaid ? t('p2p.modeEarn', 'Compete to Earn') : t('p2p.modeFun', 'Compete for Fun')}
                  </span>
                </div>

                <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
                  <span className="inline-flex min-w-0 items-center gap-1.5">
                    {opponent?.avatar_url ? (
                      <img
                        src={opponent.avatar_url}
                        alt=""
                        className="h-5 w-5 rounded-full object-cover"
                      />
                    ) : (
                      <span className="flex h-5 w-5 items-center justify-center rounded-full bg-muted text-[9px] font-bold text-muted-foreground">
                        {opponentInitial}
                      </span>
                    )}
                    <span className="truncate">
                      vs {opponent?.display_name || t('p2p.rps.opponent', 'Opponent')}
                    </span>
                  </span>
                  <span className="inline-flex items-center gap-1">
                    <Clock className="h-3 w-3" />
                    {formatMatchDate(match.completed_at ?? match.created_at, i18n.language)}
                  </span>
                </div>
              </div>

              {onSelect && (
                <ChevronRight
                  aria-hidden="true"
                  className="mt-1 h-4 w-4 shrink-0 text-muted-foreground/70"
                />
              )}
            </div>

            {isPaid && (
              <div className="mt-3 space-y-2">
                <div className="grid grid-cols-2 gap-2 rounded-xl border border-[#fbbf24]/20 bg-[#fbbf24]/[0.04] p-2.5 sm:grid-cols-4">
                  <PaidMetric
                    label={t('p2p.history.stakeReserved', 'Stake reserved')}
                    value={formatCurrency(match.stake_amount)}
                  />
                  <PaidMetric
                    label={isRefund ? t('p2p.history.refund', 'Refund') : t('p2p.history.payout', 'Payout')}
                    value={payoutValue}
                    highlight={payoutValue !== '—'}
                  />
                  <PaidMetric
                    label={`${t('p2p.history.fee', 'Fee')} ${match.service_fee_percent.toFixed(2)}%`}
                    value={formatCurrency(isRefund ? 0 : match.total_fee)}
                  />
                  {settlement && (
                    <div className="min-w-0 space-y-0.5">
                      <p className="text-[10px] font-semibold uppercase tracking-[0.1em] text-muted-foreground">
                        {t('p2p.history.settlement', 'Settlement')}
                      </p>
                      <Badge
                        variant="outline"
                        className={cn('max-w-full text-[10px] font-bold', settlement.className)}
                      >
                        {settlement.label}
                      </Badge>
                    </div>
                  )}
                </div>

                <div className="flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-[#fbbf24]/15 pt-2 text-[10px] text-muted-foreground">
                  <span className="inline-flex items-center gap-1">
                    <ReceiptText className="h-3 w-3" />
                    {t('p2p.history.matchRef', 'Match ref')}{' '}
                    <span className="font-mono" title={match.id}>
                      {compactReference(match.id)}
                    </span>
                  </span>
                  {myPlayer?.debit_tx_id && (
                    <span className="font-mono" title={myPlayer.debit_tx_id}>
                      {t('p2p.history.stake', 'Stake')} {compactReference(myPlayer.debit_tx_id)}
                    </span>
                  )}
                  {myPlayer?.credit_tx_id && (
                    <span className="font-mono" title={myPlayer.credit_tx_id}>
                      {isRefund ? t('p2p.history.refund', 'Refund') : t('p2p.history.payout', 'Payout')}{' '}
                      {compactReference(myPlayer.credit_tx_id)}
                    </span>
                  )}
                  {myPlayer?.fee_tx_id && (
                    <span className="font-mono" title={myPlayer.fee_tx_id}>
                      {t('p2p.history.fee', 'Fee')} {compactReference(myPlayer.fee_tx_id)}
                    </span>
                  )}
                </div>
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}

function DetailSection({
  title,
  icon,
  children,
}: {
  title: string
  icon: ReactNode
  children: ReactNode
}) {
  return (
    <section className="rounded-xl border border-border/70 bg-background/30 p-3.5 sm:p-4">
      <h3 className="flex items-center gap-2 text-xs font-bold uppercase tracking-[0.12em] text-muted-foreground">
        {icon}
        {title}
      </h3>
      <div className="mt-3">{children}</div>
    </section>
  )
}

function DetailField({
  label,
  value,
  mono = false,
  className,
}: {
  label: string
  value: ReactNode
  mono?: boolean
  className?: string
}) {
  return (
    <div className={cn('min-w-0', className)}>
      <dt className="text-[10px] font-semibold uppercase tracking-[0.1em] text-muted-foreground">
        {label}
      </dt>
      <dd
        className={cn(
          'mt-0.5 break-words text-sm font-semibold text-foreground',
          mono && 'break-all font-mono text-[11px] font-medium',
        )}
      >
        {value}
      </dd>
    </div>
  )
}

function modeLabel(match: P2PMatch, t?: TFunction): string {
  if (match.competition_mode === 'earn') {
    return t ? t('p2p.modeEarn', 'Compete to Earn') : 'Compete to Earn'
  }
  return t ? t('p2p.modeFun', 'Compete for Fun') : 'Compete for Fun'
}

function statusLabel(status: P2PMatch['status'], t?: TFunction): string {
  switch (status) {
    case 'completed':
      return t ? t('p2p.history.completed', 'Completed') : 'Completed'
    case 'tied':
      return t ? t('p2p.history.draw', 'Draw') : 'Draw'
    case 'forfeited':
      return t ? t('p2p.history.forfeit', 'Forfeit') : 'Forfeit'
    case 'cancelled':
      return t ? t('p2p.history.cancelled', 'Cancelled') : 'Cancelled'
    case 'in_progress':
      return t ? t('p2p.history.inProgress', 'In progress') : 'In progress'
    case 'ready':
      return t ? t('p2p.history.ready', 'Ready') : 'Ready'
    default:
      return t ? t('p2p.history.waiting', 'Waiting') : 'Waiting'
  }
}

function PlayerCard({
  player,
  seat,
  match,
  currentUserId,
  viewer,
}: {
  player: P2PMatchPlayer | null
  seat: number
  match: P2PMatch
  currentUserId?: string
  viewer?: { full_name?: string; email?: string; avatar_url?: string }
}) {
  const { t } = useTranslation()
  const isViewer = Boolean(player && player.user_id === currentUserId)
  const name = isViewer
    ? viewer?.full_name || player?.display_name || viewer?.email || t('p2p.rps.you', 'You')
    : player?.display_name || t('p2p.history.waitingForOpponent', 'Waiting for opponent')
  const avatarUrl = (isViewer ? viewer?.avatar_url : undefined) || player?.avatar_url
  const isWinner = Boolean(player && match.winner_user_id === player.user_id)
  const score = playerScore(match, player, seat)

  return (
    <div className="flex min-w-0 items-center gap-3 rounded-xl border border-border/70 bg-card/70 p-3">
      <Avatar name={name} url={avatarUrl} className="h-10 w-10 text-sm" />
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-1.5">
          <p className="truncate text-sm font-bold text-foreground">{name}</p>
          {isWinner && <Trophy className="h-3.5 w-3.5 shrink-0 text-[#fbbf24]" />}
        </div>
        <div className="mt-1 flex flex-wrap items-center gap-1.5 text-[10px]">
          <span className="rounded-full border border-border px-2 py-0.5 font-semibold text-muted-foreground">
            {playerLabel(player, seat)}
          </span>
          <span className="rounded-full border border-border px-2 py-0.5 text-muted-foreground">
            {isViewer ? t('p2p.rps.you', 'You') : player ? t('p2p.rps.opponent', 'Opponent') : t('p2p.history.openSeat', 'Open seat')}
          </span>
          <span className="font-semibold tabular-nums text-foreground">
            {t('p2p.history.score', { score: player ? score : '—', defaultValue: `Score ${player ? score : '—'}` })}
          </span>
        </div>
      </div>
    </div>
  )
}

export function MatchDetailsModal({
  match,
  currentUserId,
  viewer,
  onClose,
}: {
  match: P2PMatch
  currentUserId?: string
  viewer?: { full_name?: string; email?: string; avatar_url?: string }
  onClose: () => void
}) {
  const { t, i18n } = useTranslation()
  const myPlayer = match.players?.find((player) => player.user_id === currentUserId)
  const outcome = matchOutcome(match, currentUserId, t)
  const settlement = settlementInfo(match, myPlayer, t)
  const isPaid = match.competition_mode === 'earn'
  const isRefund = settlement?.isRefund ?? false
  const winner = match.players?.find((player) => player.user_id === match.winner_user_id)
  const winnerName = winner?.display_name || '—'
  const seats: (P2PMatchPlayer | null)[] = [0, 1].map(
    (seat) => match.players?.find((player) => player.player_index === seat) ?? null,
  )
  const completedAt = match.completed_at
  const duration = formatDuration(match.started_at, completedAt)

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [onClose])

  useEffect(() => {
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = previousOverflow
    }
  }, [])

  return (
    <div
      className="fixed inset-0 z-50 flex items-stretch justify-center bg-background/80 backdrop-blur-sm sm:items-center sm:p-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="match-details-title"
      onClick={onClose}
    >
      <Card
        className="flex h-screen h-[100dvh] w-full max-w-3xl flex-col overflow-hidden rounded-none border-0 bg-card pb-[env(safe-area-inset-bottom)] pt-[env(safe-area-inset-top)] shadow-2xl sm:h-auto sm:max-h-[90vh] sm:rounded-2xl sm:border sm:border-border/80 sm:pb-0 sm:pt-0"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex shrink-0 items-start justify-between gap-3 border-b border-border/70 px-4 py-3.5 sm:px-5">
          <div className="flex min-w-0 items-center gap-3">
            <div className="h-12 w-20 shrink-0 overflow-hidden rounded-xl border border-border/70 bg-surface-2 dark:bg-[#0c0d10] sm:h-16 sm:w-28">
              <GameArtwork slug={match.game_slug} />
            </div>
            <div className="min-w-0">
              <p className="text-[10px] font-bold uppercase tracking-[0.14em] text-muted-foreground">
                {t('p2p.history.title', 'Match details')}
              </p>
              <h2
                id="match-details-title"
                className="mt-0.5 truncate text-base font-bold text-foreground sm:text-lg"
              >
                {gameName(match.game_slug, t)}
              </h2>
              <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
                <span
                  className={cn(
                    'rounded-full border px-2 py-0.5 text-[10px] font-bold',
                    outcome.className,
                  )}
                >
                  {outcome.label}
                </span>
                <span
                  className={cn(
                    'rounded-full border px-2 py-0.5 text-[10px] font-semibold',
                    isPaid
                      ? 'border-[#fbbf24]/30 bg-[#fbbf24]/10 text-[#b8860b] dark:text-[#fbbf24]'
                      : 'border-border bg-muted/40 text-muted-foreground',
                  )}
                >
                  {modeLabel(match, t)}
                </span>
              </div>
            </div>
          </div>

          <Button
            variant="ghost"
            size="icon"
            onClick={onClose}
            autoFocus
            aria-label={t('common.close', 'Close match details')}
            className="h-8 w-8 shrink-0 text-muted-foreground hover:text-foreground"
          >
            <X className="h-4 w-4" />
          </Button>
        </div>

        <div className="flex-1 space-y-4 overflow-y-auto overscroll-contain p-4 sm:p-5">
          <DetailSection title={t('p2p.history.matchInformation', 'Match information')} icon={<Gamepad2 className="h-3.5 w-3.5" />}>
            <dl className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <DetailField label={t('p2p.history.game', 'Game')} value={gameName(match.game_slug, t)} />
              <DetailField label={t('p2p.history.matchReference', 'Match reference')} value={match.id} mono />
              <DetailField label={t('p2p.history.matchMode', 'Match mode')} value={modeLabel(match, t)} />
              <DetailField
                label={t('p2p.history.matchType', 'Match type')}
                value={match.match_type === 'private_room' ? t('p2p.history.privateRoom', 'Private room') : t('p2p.history.publicQueue', 'Public queue')}
              />
              <DetailField label={t('p2p.history.matchDate', 'Match date')} value={formatMatchDate(match.created_at, i18n.language)} />
              <DetailField
                label={t('p2p.history.started', 'Started')}
                value={formatMatchDate(match.started_at, i18n.language)}
              />
              <DetailField
                label={t('p2p.history.completedDate', 'Completed')}
                value={formatMatchDate(completedAt, i18n.language)}
              />
              <DetailField label={t('p2p.history.duration', 'Duration')} value={duration} />
              <DetailField
                label={t('p2p.history.rounds', 'Rounds')}
                value={
                  match.max_rounds > 1
                    ? t('p2p.history.bestOfRounds', { rounds: match.max_rounds, defaultValue: `Best of ${match.max_rounds}` })
                    : t('p2p.history.singleRound', 'Single round')
                }
              />
            </dl>
          </DetailSection>

          <DetailSection title={t('p2p.history.players', 'Players')} icon={<Users className="h-3.5 w-3.5" />}>
            <div className="grid gap-2 sm:grid-cols-2">
              {seats.map((player, seat) => (
                <PlayerCard
                  key={player?.id ?? `empty-${seat}`}
                  player={player}
                  seat={seat}
                  match={match}
                  currentUserId={currentUserId}
                  viewer={viewer}
                />
              ))}
            </div>
          </DetailSection>

          <DetailSection title={t('p2p.history.result', 'Result')} icon={<Trophy className="h-3.5 w-3.5" />}>
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <p className="text-lg font-black text-foreground">{outcome.label}</p>
                <p className="mt-0.5 text-xs text-muted-foreground">
                  {match.status === 'tied'
                    ? t('p2p.history.endedInDraw', 'The match ended in a draw.')
                    : match.status === 'cancelled'
                      ? t('p2p.history.cancelledBeforeResult', 'The match was cancelled before a result.')
                      : match.winner_user_id
                        ? t('p2p.history.winnerRecorded', { name: winnerName, defaultValue: `Winner: ${winnerName}` })
                        : t('p2p.history.noWinnerRecorded', 'No winner was recorded.')}
                </p>
              </div>
              <Badge variant="outline" className="text-[10px] font-bold">
                {statusLabel(match.status, t)}
              </Badge>
            </div>

            <div className="mt-3 grid grid-cols-2 gap-2">
              {seats.map((player, seat) => (
                <div
                  key={player?.id ?? `score-${seat}`}
                  className="flex min-w-0 items-center justify-between gap-2 rounded-lg border border-border/70 bg-card/60 px-2.5 py-2"
                >
                  <div className="min-w-0">
                    <p className="truncate text-[10px] font-bold uppercase tracking-[0.1em] text-muted-foreground">
                      {playerLabel(player, seat)}
                    </p>
                    <p className="truncate text-xs font-semibold text-foreground">
                      {player?.display_name || '—'}
                    </p>
                  </div>
                  <span className="text-lg font-black tabular-nums text-foreground">
                    {player ? playerScore(match, player, seat) : '—'}
                  </span>
                </div>
              ))}
            </div>
          </DetailSection>

          {isPaid && settlement && (
            <DetailSection title={t('p2p.history.financialInformation', 'Financial information')} icon={<Coins className="h-3.5 w-3.5" />}>
              <dl className="grid grid-cols-2 gap-3 sm:grid-cols-3">
                <DetailField
                  label={t('p2p.history.originalStake', 'Original stake')}
                  value={formatCurrency(match.stake_amount)}
                />
                <DetailField label={t('p2p.history.prizePool', 'Prize pool')} value={formatCurrency(match.prize_pool)} />
                <DetailField
                  label={t('p2p.stakeSetup.platformFee', {
                    percent: match.service_fee_percent.toFixed(2),
                    defaultValue: `Platform fee (${match.service_fee_percent.toFixed(2)}%)`,
                  })}
                  value={formatCurrency(match.total_fee)}
                />
                <DetailField
                  label={t('p2p.history.winnerPayout', 'Winner payout')}
                  value={isRefund ? '—' : formatCurrency(match.potential_payout)}
                />
                <DetailField
                  label={t('p2p.history.refundAmount', 'Refund amount')}
                  value={isRefund ? formatCurrency(match.stake_amount) : '—'}
                />
                <DetailField
                  label={t('p2p.history.yourReturn', 'Your return')}
                  value={returnValue(match, currentUserId)}
                />
                <div className="min-w-0">
                  <dt className="text-[10px] font-semibold uppercase tracking-[0.1em] text-muted-foreground">
                    {t('p2p.history.settlement', 'Settlement')}
                  </dt>
                  <dd className="mt-1">
                    <Badge
                      variant="outline"
                      className={cn('max-w-full text-[10px] font-bold', settlement.className)}
                    >
                      <ShieldCheck className="mr-1 h-3 w-3" />
                      {settlement.label}
                    </Badge>
                  </dd>
                </div>
              </dl>

              <div className="mt-3 space-y-2 border-t border-border/70 pt-3">
                <p className="flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-[0.1em] text-muted-foreground">
                  <ReceiptText className="h-3 w-3" />
                  {t('p2p.history.walletReferences', 'Wallet references')}
                </p>
                <dl className="space-y-1.5">
                  <DetailField label={t('p2p.history.matchReference', 'Match reference')} value={match.id} mono />
                  {myPlayer?.debit_tx_id && (
                    <DetailField label={t('p2p.history.stakeTransaction', 'Stake transaction')} value={myPlayer.debit_tx_id} mono />
                  )}
                  {myPlayer?.credit_tx_id && (
                    <DetailField
                      label={isRefund ? t('p2p.history.refundTransaction', 'Refund transaction') : t('p2p.history.payoutTransaction', 'Payout transaction')}
                      value={myPlayer.credit_tx_id}
                      mono
                    />
                  )}
                  {myPlayer?.fee_tx_id && (
                    <DetailField label={t('p2p.history.feeTransaction', 'Fee transaction')} value={myPlayer.fee_tx_id} mono />
                  )}
                  {!myPlayer?.debit_tx_id && !myPlayer?.credit_tx_id && !myPlayer?.fee_tx_id && (
                    <p className="text-xs text-muted-foreground">
                      {t('p2p.history.noWalletTransactions', 'No wallet transaction references are recorded for this match.')}
                    </p>
                  )}
                </dl>
              </div>
            </DetailSection>
          )}
        </div>
      </Card>
    </div>
  )
}
