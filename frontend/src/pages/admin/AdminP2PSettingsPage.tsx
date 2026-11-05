import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  AlertTriangle,
  Coins,
  Loader2,
  RefreshCw,
  Save,
  ShieldAlert,
  ShieldCheck,
} from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { p2pApi } from '@/features/p2p/api'
import { usePageTitle } from '@/hooks/usePageTitle'
import { cn, formatCurrency } from '@/lib/utils'
import type { P2PAdminMatch } from '@/types/api'

function apiErrorMessage(err: unknown): string {
  if (typeof err === 'object' && err !== null) {
    const response = (err as { response?: { data?: { message?: string } } }).response
    if (response?.data?.message) return response.data.message
    const message = (err as { message?: string }).message
    if (message) return message
  }
  return 'Something went wrong'
}

const financialBadge: Record<P2PAdminMatch['financial_status'], string> = {
  settled: 'border-success/30 bg-success/10 text-success',
  refunded: 'border-border bg-muted text-foreground',
  reserved: 'border-warning/40 bg-warning/10 text-warning',
  pending: 'border-border bg-muted text-muted-foreground',
  needs_attention: 'border-destructive/40 bg-destructive/10 text-destructive',
}

type MatchFilter = 'all' | 'reserved' | 'settled' | 'refunded' | 'needs_attention'

export function Component() {
  usePageTitle('P2P Earnings')
  const queryClient = useQueryClient()

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['p2p-admin-settings'],
    queryFn: p2pApi.adminGetSettings,
  })

  const [filter, setFilter] = useState<MatchFilter>('all')
  const {
    data: matchData,
    isLoading: matchesLoading,
    refetch: refetchMatches,
  } = useQuery({
    queryKey: ['p2p-admin-matches', filter],
    queryFn: () =>
      p2pApi.adminListMatches({
        status: filter === 'all' ? undefined : filter,
        limit: 50,
      }),
  })

  const [paidEnabled, setPaidEnabled] = useState<boolean | null>(null)
  const [edits, setEdits] = useState<Record<string, string>>({})
  const [saved, setSaved] = useState(false)

  // Form starts from the persisted values; edits are local until Save.
  const effectivePaid = paidEnabled ?? data?.settings.paid_matches_enabled === 'true'
  const value = (key: string, fallback = '') =>
    edits[key] ??
    (() => {
      const raw = data?.settings[key]
      if (raw === undefined) return fallback
      // Display money configuration in ETB rather than minor units.
      if (['min_stake_cents', 'max_stake_cents', 'stake_increment_cents'].includes(key)) {
        return (Number(raw) / 100).toString()
      }
      if (key === 'default_service_fee_bp') return (Number(raw) / 100).toFixed(2)
      return raw
    })()

  const update = useMutation({
    mutationFn: () => {
      const toCents = (key: string) => String(Math.round(Number(value(key)) * 100))
      return p2pApi.adminUpdateSettings({
        paid_matches_enabled: String(effectivePaid),
        default_service_fee_bp: String(Math.round(Number(value('default_service_fee_bp', '5.00')) * 100)),
        min_stake_cents: toCents('min_stake_cents'),
        max_stake_cents: toCents('max_stake_cents'),
        stake_increment_cents: toCents('stake_increment_cents'),
      })
    },
    onSuccess: (updated) => {
      setEdits({})
      setPaidEnabled(updated?.settings.paid_matches_enabled === 'true')
      setSaved(true)
      setTimeout(() => setSaved(false), 2500)
      void queryClient.invalidateQueries({ queryKey: ['p2p-admin-settings'] })
      void queryClient.invalidateQueries({ queryKey: ['p2p-config'] })
    },
  })

  const matches = useMemo(() => matchData?.matches ?? [], [matchData])
  const attentionCount = useMemo(
    () => matches.filter((m) => m.needs_attention).length,
    [matches]
  )

  return (
    <div className="mx-auto max-w-6xl space-y-6 p-4 sm:p-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-bold">
            <Coins className="h-6 w-6 text-[#d99a06] dark:text-[#fbbf24]" />
            P2P Competition Settings
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Control Compete to Earn, stakes and the platform commission. Every change is
            permission-protected and audited.
          </p>
        </div>
        <Badge
          variant="outline"
          className={cn(
            'gap-1.5 px-3 py-1 text-xs font-semibold',
            data?.config.paid_matches_enabled
              ? 'border-success/40 bg-success/10 text-success'
              : 'border-border text-muted-foreground'
          )}
        >
          {data?.config.paid_matches_enabled ? (
            <ShieldCheck className="h-3.5 w-3.5" />
          ) : (
            <ShieldAlert className="h-3.5 w-3.5" />
          )}
          {data?.config.paid_matches_enabled ? 'Paid play enabled' : 'Paid play disabled'}
        </Badge>
      </div>

      {isError && (
        <Card className="border-destructive/40 bg-destructive/5">
          <CardContent className="p-4 text-sm text-destructive">
            {apiErrorMessage(error)}
          </CardContent>
        </Card>
      )}

      {!data?.operator_gate_open && !isLoading && (
        <Card className="border-warning/40 bg-warning/5">
          <CardContent className="flex items-start gap-2 p-4 text-xs text-warning">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
            <p>
              The deployment gate <code>PAID_MATCHES_ENABLED</code> is closed. Compete to Earn
              cannot be switched on from the admin panel until the operator clears it for this
              environment.
            </p>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Configuration</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {isLoading ? (
            <div className="grid gap-3 sm:grid-cols-2">
              {Array.from({ length: 4 }).map((_, i) => (
                <Skeleton key={i} className="h-20 w-full" />
              ))}
            </div>
          ) : (
            <>
              <button
                type="button"
                onClick={() => setPaidEnabled(!effectivePaid)}
                disabled={!data?.operator_gate_open && !effectivePaid}
                className={cn(
                  'flex w-full items-center justify-between gap-4 rounded-xl border p-4 text-left transition-colors',
                  effectivePaid
                    ? 'border-success/40 bg-success/5'
                    : 'border-border bg-card hover:bg-accent/40',
                  !data?.operator_gate_open && 'cursor-not-allowed opacity-70'
                )}
              >
                <div>
                  <p className="text-sm font-bold text-foreground">Compete to Earn</p>
                  <p className="mt-0.5 text-xs text-muted-foreground">
                    Allow players to reserve wallet stakes and compete for prize pools.
                  </p>
                </div>
                <span
                  className={cn(
                    'relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition-colors',
                    effectivePaid ? 'bg-success' : 'bg-muted-foreground/30'
                  )}
                >
                  <span
                    className={cn(
                      'inline-block h-5 w-5 transform rounded-full bg-white shadow transition-transform',
                      effectivePaid ? 'translate-x-5' : 'translate-x-0.5'
                    )}
                  />
                </span>
              </button>

              <div className="grid gap-3 sm:grid-cols-2">
                <Field
                  label="Platform commission"
                  unit="%"
                  value={value('default_service_fee_bp', '5.00')}
                  onChange={(v) => setEdits((prev) => ({ ...prev, default_service_fee_bp: v }))}
                  hint="Deducted from the winner payout."
                />
                <Field
                  label="Stake increment"
                  unit="ETB"
                  value={value('stake_increment_cents', '10')}
                  onChange={(v) => setEdits((prev) => ({ ...prev, stake_increment_cents: v }))}
                />
                <Field
                  label="Minimum stake"
                  unit="ETB"
                  value={value('min_stake_cents', '10')}
                  onChange={(v) => setEdits((prev) => ({ ...prev, min_stake_cents: v }))}
                />
                <Field
                  label="Maximum stake"
                  unit="ETB"
                  value={value('max_stake_cents', '5000')}
                  onChange={(v) => setEdits((prev) => ({ ...prev, max_stake_cents: v }))}
                />
              </div>

              <div className="flex flex-wrap items-center justify-between gap-2 border-t border-border/70 pt-3">
                <p className="text-[11px] text-muted-foreground">
                  Free play is always available and never requires a wallet balance.
                </p>
                <div className="flex items-center gap-2">
                  {saved && <span className="text-xs font-semibold text-success">Saved</span>}
                  {update.isError && (
                    <span className="text-xs text-destructive">{apiErrorMessage(update.error)}</span>
                  )}
                  <Button
                    onClick={() => update.mutate()}
                    disabled={update.isPending}
                    className="h-9 gap-1.5 text-xs font-bold"
                  >
                    {update.isPending ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <Save className="h-3.5 w-3.5" />
                    )}
                    Save settings
                  </Button>
                </div>
              </div>
            </>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-3 pb-3">
          <div className="flex items-center gap-2">
            <CardTitle className="text-base">Paid matches</CardTitle>
            {attentionCount > 0 && (
              <Badge variant="destructive" className="text-[10px]">
                {attentionCount} need attention
              </Badge>
            )}
          </div>
          <div className="flex items-center gap-2">
            <select
              value={filter}
              onChange={(event) => setFilter(event.target.value as MatchFilter)}
              className="h-8 rounded-lg border border-input bg-background px-2 text-xs"
            >
              <option value="all">All paid matches</option>
              <option value="reserved">Reserved / pending</option>
              <option value="settled">Settled</option>
              <option value="refunded">Refunded</option>
              <option value="needs_attention">Needs attention</option>
            </select>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => refetchMatches()}
              className="h-8 gap-1.5 text-xs"
            >
              <RefreshCw className="h-3.5 w-3.5" />
              Refresh
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {matchesLoading ? (
            <div className="space-y-2">
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className="h-12 w-full" />
              ))}
            </div>
          ) : matches.length === 0 ? (
            <p className="py-10 text-center text-sm text-muted-foreground">
              No matches in this view.
            </p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead>
                  <tr className="border-b text-[10px] uppercase tracking-[0.1em] text-muted-foreground">
                    <th className="pb-2 pr-3 font-semibold">Match</th>
                    <th className="pb-2 pr-3 font-semibold">Status</th>
                    <th className="pb-2 pr-3 font-semibold">Stake</th>
                    <th className="pb-2 pr-3 font-semibold">Pool</th>
                    <th className="pb-2 pr-3 font-semibold">Fee</th>
                    <th className="pb-2 pr-3 font-semibold">Ledger</th>
                    <th className="pb-2 font-semibold">Settlement</th>
                  </tr>
                </thead>
                <tbody>
                  {matches.map((entry) => {
                    const match = entry.match
                    const reserved = entry.ledger.filter((l) => l.status === 'reserved').length
                    const refunds = entry.ledger.filter((l) => l.type === 'p2p_stake_refund').length
                    const payouts = entry.ledger.filter((l) => l.type === 'p2p_payout').length
                    return (
                      <tr key={match.id} className="border-b align-top last:border-0">
                        <td className="py-3 pr-3">
                          <p className="font-semibold capitalize text-foreground">
                            {match.game_slug.replace(/-/g, ' ')}
                          </p>
                          <p className="font-mono text-[10px] text-muted-foreground">
                            {match.id.slice(0, 8)} · {new Date(match.created_at).toLocaleDateString()}
                          </p>
                        </td>
                        <td className="py-3 pr-3 capitalize text-muted-foreground">
                          {match.status.replace(/_/g, ' ')}
                        </td>
                        <td className="py-3 pr-3 font-semibold tabular-nums">
                          {formatCurrency(match.stake_amount)}
                        </td>
                        <td className="py-3 pr-3 tabular-nums">{formatCurrency(match.prize_pool)}</td>
                        <td className="py-3 pr-3 tabular-nums">{formatCurrency(match.total_fee)}</td>
                        <td className="py-3 pr-3 text-[10px] text-muted-foreground">
                          {reserved} reserved · {refunds} refunds · {payouts} payouts
                        </td>
                        <td className="py-3">
                          <span
                            className={cn(
                              'inline-flex rounded-full border px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide',
                              financialBadge[entry.financial_status]
                            )}
                          >
                            {entry.financial_status.replace(/_/g, ' ')}
                          </span>
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function Field({
  label,
  unit,
  value,
  onChange,
  hint,
}: {
  label: string
  unit: string
  value: string
  onChange: (value: string) => void
  hint?: string
}) {
  return (
    <label className="space-y-1.5 rounded-xl border border-border/70 bg-card p-3">
      <span className="text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">
        {label}
      </span>
      <span className="flex items-center gap-2">
        <Input
          type="number"
          min="0"
          step={unit === '%' ? '0.01' : '1'}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          className="h-9 font-semibold tabular-nums"
        />
        <span className="text-xs font-semibold text-muted-foreground">{unit}</span>
      </span>
      {hint && <span className="block text-[10px] text-muted-foreground">{hint}</span>}
    </label>
  )
}

export default Component
