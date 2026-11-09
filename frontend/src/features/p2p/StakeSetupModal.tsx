import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { AlertTriangle, Coins, Loader2, ShieldCheck, Swords, Users, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import type { TFunction } from 'i18next'

import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { p2pApi } from '@/features/p2p/api'
import { GameIcon } from '@/features/p2p/GameIcon'
import { formatCurrency } from '@/lib/utils'
import { gameName } from '@/features/p2p/matchHistory'
import type { P2PConfig, P2PGame } from '@/types/api'

function apiErrorMessage(err: unknown, t: TFunction): string {
  if (typeof err === 'object' && err !== null) {
    const response = (err as { response?: { data?: { message?: string } } }).response
    if (response?.data?.message) return response.data.message
    const message = (err as { message?: string }).message
    if (message) return message
  }
  return t('p2p.stakeSetup.couldNotCalculate', 'Could not calculate this stake.')
}

interface StakeSetupModalProps {
  open: boolean
  game: P2PGame | null
  config?: P2PConfig
  pending?: boolean
  onClose: () => void
  onCreate: (payload: {
    game_slug: string
    match_type: 'public_queue' | 'private_room'
    competition_mode: 'earn'
    stake_amount: number
    auto_match: boolean
  }) => void
}

/**
 * Compete to Earn setup. Every number shown here (limits, fee, pool, payout)
 * comes from the backend quote; the client only collects the proposed stake.
 */
export function StakeSetupModal({
  open,
  game,
  config,
  pending = false,
  onClose,
  onCreate,
}: StakeSetupModalProps) {
  const { t } = useTranslation()
  const initialStake = useMemo(() => {
    if (!config) return '10'
    const step = config.stake_increment_cents / 100
    const min = config.min_stake_cents / 100
    const firstStep = Math.ceil(min / step) * step
    return String(Math.min(firstStep, config.max_stake_cents / 100))
  }, [config])

  const [stake, setStake] = useState(initialStake)

  const numericStake = Number(stake)
  const validNumber = Number.isFinite(numericStake) && numericStake > 0

  const {
    data: quote,
    isLoading: quoteLoading,
    error: quoteError,
  } = useQuery({
    queryKey: ['p2p-stake-quote', game?.slug, numericStake],
    queryFn: () => p2pApi.quoteStake(game!.slug, numericStake),
    enabled: open && !!game && validNumber,
    retry: false,
  })

  const presets = useMemo(() => {
    if (!config) return []
    const step = config.stake_increment_cents / 100
    return [1, 2, 5, 10]
      .map((multiple) => step * multiple)
      .filter((value) => value >= config.min_stake_cents / 100 && value <= config.max_stake_cents / 100)
  }, [config])

  if (!open || !game) return null

  const insufficient = !!config && validNumber && numericStake > config.available_balance
  const canSubmit = !!quote && !quoteError && !insufficient && !pending

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-background/80 p-4 backdrop-blur-sm animate-in fade-in duration-200"
      role="dialog"
      aria-modal="true"
      aria-labelledby="stake-modal-title"
    >
      <Card className="my-auto w-full max-w-lg overflow-hidden border-border/80 bg-card/95 shadow-2xl backdrop-blur">
        <div className="flex items-center justify-between border-b border-border/70 px-5 py-4">
          <div className="flex items-center gap-2.5">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-[#fbbf24]/10 text-[#fbbf24]">
              <Coins className="h-5 w-5" />
            </div>
            <div>
              <h2 id="stake-modal-title" className="text-base font-bold text-foreground">
                {t('p2p.stakeSetup.title', 'Compete to Earn')}
              </h2>
              <p className="text-xs text-muted-foreground">{t('p2p.stakeSetup.subtitle', 'Choose a stake for both players')}</p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg p-1.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            aria-label={t('common.close', 'Close')}
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <CardContent className="space-y-4 p-5">
          <div className="flex items-center gap-3 rounded-2xl border border-border/80 bg-muted/30 p-3.5">
            <div className="rounded-xl bg-muted p-2.5">
              <GameIcon slug={game.slug} size="lg" />
            </div>
            <div className="min-w-0">
              <h3 className="truncate text-sm font-bold text-foreground">
                {gameName(game.slug, t) || game.title}
              </h3>
              <p className="text-xs text-muted-foreground">
                {t('p2p.stakeSetup.winnerTakesPool', 'Winner takes the pool minus the platform fee')}
              </p>
            </div>
          </div>

          <div className="flex items-center justify-between rounded-xl border border-border/70 bg-card px-3.5 py-2.5 text-sm">
            <span className="flex items-center gap-1.5 text-muted-foreground">
              <ShieldCheck className="h-4 w-4 text-success" />
              {t('p2p.stakeSetup.availableBalance', 'Available balance')}
            </span>
            <span className="font-bold tabular-nums text-foreground">
              {config ? formatCurrency(config.available_balance) : '—'}
            </span>
          </div>

          <div className="space-y-2">
            <label htmlFor="stake-amount" className="text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-foreground">
              {t('p2p.stakeSetup.yourStake', 'Your stake (ETB)')}
            </label>
            <Input
              id="stake-amount"
              type="number"
              inputMode="decimal"
              min={config ? config.min_stake_cents / 100 : undefined}
              max={config ? config.max_stake_cents / 100 : undefined}
              step={config ? config.stake_increment_cents / 100 : 10}
              value={stake}
              onChange={(event) => setStake(event.target.value)}
              className="h-11 text-lg font-bold tabular-nums"
            />
            {presets.length > 0 && (
              <div className="flex flex-wrap gap-2">
                {presets.map((value) => (
                  <button
                    key={value}
                    type="button"
                    onClick={() => setStake(String(value))}
                    className={`rounded-lg border px-3 py-1.5 text-xs font-semibold transition-colors ${
                      Number(stake) === value
                        ? 'border-primary/60 bg-primary/10 text-primary'
                        : 'border-border text-muted-foreground hover:bg-accent hover:text-foreground'
                    }`}
                  >
                    {formatCurrency(value)}
                  </button>
                ))}
              </div>
            )}
            {config && (
              <p className="text-[11px] text-muted-foreground">
                {t('p2p.stakeSetup.minMaxSteps', {
                  min: formatCurrency(config.min_stake_cents / 100),
                  max: formatCurrency(config.max_stake_cents / 100),
                  step: formatCurrency(config.stake_increment_cents / 100),
                  defaultValue: `${formatCurrency(config.min_stake_cents / 100)} – ${formatCurrency(config.max_stake_cents / 100)} in ${formatCurrency(config.stake_increment_cents / 100)} steps`,
                })}
              </p>
            )}
          </div>

          <div className="space-y-2 rounded-2xl border border-[#fbbf24]/25 bg-[#fbbf24]/5 p-4">
            {quoteLoading && !quote ? (
              <div className="flex items-center justify-center gap-2 py-4 text-xs text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin" />
                {t('p2p.stakeSetup.calculatingPrize', 'Calculating your prize…')}
              </div>
            ) : quoteError ? (
              <div className="flex items-start gap-2 text-xs text-destructive">
                <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                <span>{apiErrorMessage(quoteError, t)}</span>
              </div>
            ) : quote ? (
              <>
                <Row label={t('p2p.stakeSetup.yourStakeRow', 'Your stake')} value={formatCurrency(quote.stake_amount)} />
                <Row label={t('p2p.stakeSetup.opponentMatchingStake', "Opponent's matching stake")} value={formatCurrency(quote.opponent_stake_amount)} />
                <Row label={t('p2p.stakeSetup.totalPrizePool', 'Total prize pool')} value={formatCurrency(quote.prize_pool)} strong />
                <Row
                  label={t('p2p.stakeSetup.platformFee', {
                    percent: quote.service_fee_percent.toFixed(2),
                    defaultValue: `Platform fee (${quote.service_fee_percent.toFixed(2)}%)`,
                  })}
                  value={formatCurrency(quote.fee)}
                />
                <div className="border-t border-[#fbbf24]/20 pt-2">
                  <Row
                    label={t('p2p.stakeSetup.potentialWinnerPayout', 'Potential winner payout')}
                    value={formatCurrency(quote.potential_payout)}
                    strong
                    highlight
                  />
                </div>
              </>
            ) : (
              <p className="py-4 text-center text-xs text-muted-foreground">
                {t('p2p.stakeSetup.enterPermittedStake', 'Enter a permitted stake to see the prize breakdown.')}
              </p>
            )}
          </div>

          {insufficient && (
            <p className="flex items-center gap-1.5 text-xs font-medium text-destructive">
              <AlertTriangle className="h-3.5 w-3.5" />
              {t('p2p.stakeSetup.balanceTooLow', 'Your available balance is too low for this stake.')}
            </p>
          )}

          <div className="flex flex-col gap-2 pt-1 sm:flex-row">
            <Button
              className="h-11 flex-1 gap-2 font-bold"
              disabled={!canSubmit}
              onClick={() =>
                onCreate({
                  game_slug: game.slug,
                  match_type: 'public_queue',
                  competition_mode: 'earn',
                  stake_amount: numericStake,
                  auto_match: true,
                })
              }
            >
              {pending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Swords className="h-4 w-4" />}
              {t('p2p.stakeSetup.findOpponent', 'Find Opponent')}
            </Button>
            <Button
              variant="outline"
              className="h-11 flex-1 gap-2 font-semibold"
              disabled={!canSubmit}
              onClick={() =>
                onCreate({
                  game_slug: game.slug,
                  match_type: 'private_room',
                  competition_mode: 'earn',
                  stake_amount: numericStake,
                  auto_match: false,
                })
              }
            >
              <Users className="h-4 w-4" />
              {t('p2p.stakeSetup.challengeFriend', 'Challenge a Friend')}
            </Button>
          </div>

          <div className="flex items-start gap-2 rounded-xl border border-border/70 bg-muted/30 p-3 text-[11px] leading-relaxed text-muted-foreground">
            <ShieldCheck className="mt-0.5 h-3.5 w-3.5 shrink-0 text-success" />
            <p>
              {t('p2p.stakeSetup.stakeReservedNotice', 'Your stake is reserved when you accept and settles only when the match ends. If the match is drawn or cancelled both stakes are refunded in full.')}
            </p>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

function Row({
  label,
  value,
  strong = false,
  highlight = false,
}: {
  label: string
  value: string
  strong?: boolean
  highlight?: boolean
}) {
  return (
    <div className="flex items-center justify-between gap-3 text-xs">
      <span className="text-muted-foreground">{label}</span>
      <span
        className={`tabular-nums ${strong ? 'font-bold' : 'font-semibold'} ${
          highlight ? 'text-success' : 'text-foreground'
        }`}
      >
        {value}
      </span>
    </div>
  )
}
