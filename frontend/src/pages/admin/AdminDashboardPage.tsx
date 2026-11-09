import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import {
  Area, AreaChart, Bar, BarChart, CartesianGrid, Cell, Legend, Pie, PieChart,
  ResponsiveContainer, Tooltip, XAxis, YAxis,
} from 'recharts'
import {
  Activity, ArrowRight, CircleDollarSign, Clock3, Gamepad2, Swords,
  TrendingUp, UserPlus, Users, WalletCards,
} from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Select } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { usePageTitle } from '@/hooks/usePageTitle'
import { adminApi, defaultRange } from '@/features/admin/api'
import { DateRangeBar, EmptyState, ErrorState, KpiCard, Panel, StatusBadge } from '@/features/admin/AdminUi'
import { chartTooltipStyle, formatDate, formatMoney, formatNumber, relativeTime } from '@/features/admin/AdminUtils'
import { GameArtwork, CrashArtwork } from '@/features/p2p/GameArtwork'
import { cn } from '@/lib/utils'

const CHART = {
  brand: '#ff5b55',
  green: '#34d399',
  amber: '#fbbf24',
  blue: '#60a5fa',
  purple: '#a78bfa',
}

function ChartEmpty({ label }: { label?: string }) {
  const { t } = useTranslation()
  const displayLabel = label ?? t('admin.noActivityPeriod', 'No activity in this period')
  return (
    <div className="flex h-[230px] items-center justify-center text-xs text-muted-foreground">{displayLabel}</div>
  )
}

function GrowthChart({ data }: { data: { period: string; count: number }[] }) {
  const { t } = useTranslation()
  return (
    <ResponsiveContainer width="100%" height={240}>
      <AreaChart data={data} margin={{ top: 8, right: 8, left: -20, bottom: 0 }}>
        <defs>
          <linearGradient id="adminGrowth" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={CHART.brand} stopOpacity={0.32} />
            <stop offset="100%" stopColor={CHART.brand} stopOpacity={0} />
          </linearGradient>
        </defs>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" vertical={false} />
        <XAxis dataKey="period" tick={{ fontSize: 10, fill: 'var(--color-muted-foreground)' }} tickLine={false} axisLine={false} minTickGap={22} />
        <YAxis tick={{ fontSize: 10, fill: 'var(--color-muted-foreground)' }} tickLine={false} axisLine={false} allowDecimals={false} />
        <Tooltip contentStyle={chartTooltipStyle()} formatter={(value) => [formatNumber(Number(value)), t('admin.registrations', 'Registrations')]} />
        <Area type="monotone" dataKey="count" stroke={CHART.brand} fill="url(#adminGrowth)" strokeWidth={2.2} dot={false} />
      </AreaChart>
    </ResponsiveContainer>
  )
}

function MatchChart({ data }: { data: { period: string; total: number; completed: number; paid: number }[] }) {
  const { t } = useTranslation()
  return (
    <ResponsiveContainer width="100%" height={240}>
      <BarChart data={data} margin={{ top: 8, right: 8, left: -20, bottom: 0 }} barGap={2}>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" vertical={false} />
        <XAxis dataKey="period" tick={{ fontSize: 10, fill: 'var(--color-muted-foreground)' }} tickLine={false} axisLine={false} minTickGap={22} />
        <YAxis tick={{ fontSize: 10, fill: 'var(--color-muted-foreground)' }} tickLine={false} axisLine={false} allowDecimals={false} />
        <Tooltip contentStyle={chartTooltipStyle()} />
        <Legend wrapperStyle={{ fontSize: 11 }} />
        <Bar dataKey="total" name={t('admin.created', 'Created')} fill={CHART.brand} radius={[3, 3, 0, 0]} />
        <Bar dataKey="completed" name={t('admin.statusCompleted', 'Completed')} fill={CHART.green} radius={[3, 3, 0, 0]} />
        <Bar dataKey="paid" name={t('admin.paidOnly', 'Paid')} fill={CHART.amber} radius={[3, 3, 0, 0]} />
      </BarChart>
    </ResponsiveContainer>
  )
}

function RevenueChart({ data }: { data: { period: string; p2p_fees: number; crash_fees: number; raffle_profit: number; total: number }[] }) {
  const { t } = useTranslation()
  return (
    <ResponsiveContainer width="100%" height={240}>
      <BarChart data={data} margin={{ top: 8, right: 8, left: -10, bottom: 0 }} barGap={2}>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" vertical={false} />
        <XAxis dataKey="period" tick={{ fontSize: 10, fill: 'var(--color-muted-foreground)' }} tickLine={false} axisLine={false} minTickGap={22} />
        <YAxis tick={{ fontSize: 10, fill: 'var(--color-muted-foreground)' }} tickLine={false} axisLine={false} tickFormatter={(value) => `${Math.round(Number(value) / 1000)}k`} />
        <Tooltip contentStyle={chartTooltipStyle()} formatter={(value) => formatMoney(Number(value))} />
        <Legend wrapperStyle={{ fontSize: 11 }} />
        <Bar dataKey="p2p_fees" name={t('admin.p2pFees', 'P2P fees')} stackId="revenue" fill={CHART.brand} />
        <Bar dataKey="crash_fees" name={t('admin.crashFees', 'Crash fees')} stackId="revenue" fill={CHART.amber} />
        <Bar dataKey="raffle_profit" name={t('admin.raffleProfit', 'Raffle profit')} stackId="revenue" fill={CHART.blue} radius={[3, 3, 0, 0]} />
      </BarChart>
    </ResponsiveContainer>
  )
}

function FinanceChart({ data }: { data: { period: string; deposits: number; withdrawals: number }[] }) {
  const { t } = useTranslation()
  return (
    <ResponsiveContainer width="100%" height={240}>
      <AreaChart data={data} margin={{ top: 8, right: 8, left: -10, bottom: 0 }}>
        <defs>
          <linearGradient id="adminDeposits" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={CHART.green} stopOpacity={0.28} />
            <stop offset="100%" stopColor={CHART.green} stopOpacity={0} />
          </linearGradient>
          <linearGradient id="adminWithdrawals" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={CHART.amber} stopOpacity={0.22} />
            <stop offset="100%" stopColor={CHART.amber} stopOpacity={0} />
          </linearGradient>
        </defs>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" vertical={false} />
        <XAxis dataKey="period" tick={{ fontSize: 10, fill: 'var(--color-muted-foreground)' }} tickLine={false} axisLine={false} minTickGap={22} />
        <YAxis tick={{ fontSize: 10, fill: 'var(--color-muted-foreground)' }} tickLine={false} axisLine={false} tickFormatter={(value) => `${Math.round(Number(value) / 1000)}k`} />
        <Tooltip contentStyle={chartTooltipStyle()} formatter={(value) => formatMoney(Number(value))} />
        <Legend wrapperStyle={{ fontSize: 11 }} />
        <Area type="monotone" dataKey="deposits" name={t('admin.deposits', 'Deposits')} stroke={CHART.green} fill="url(#adminDeposits)" strokeWidth={2} dot={false} />
        <Area type="monotone" dataKey="withdrawals" name={t('admin.withdrawals', 'Withdrawals')} stroke={CHART.amber} fill="url(#adminWithdrawals)" strokeWidth={2} dot={false} />
      </AreaChart>
    </ResponsiveContainer>
  )
}

export function Component() {
  const { t } = useTranslation()
  usePageTitle(t('admin.overviewTitle', 'Admin Overview'))
  const [range, setRange] = useState(defaultRange)
  const [period, setPeriod] = useState('day')
  const { data, isLoading, isError, error, refetch, isFetching } = useQuery({
    queryKey: ['admin', 'overview', range, period],
    queryFn: () => adminApi.overview(range.from, range.to, period),
  })
  const { data: live } = useQuery({
    queryKey: ['admin', 'live', 'overview'],
    queryFn: adminApi.live,
    refetchInterval: 20_000,
  })

  const revenueMix = data ? [
    { name: t('admin.p2pFees', 'P2P fees'), value: data.p2p_service_fees, color: CHART.brand },
    { name: t('admin.crashFees', 'Crash fees'), value: data.crash_service_fees, color: CHART.amber },
    { name: t('admin.raffleProfit', 'Raffle profit'), value: Math.max(0, data.raffle_profit), color: CHART.blue },
  ].filter(item => item.value > 0) : []

  return (
    <div className="mx-auto w-full max-w-[1600px] space-y-5 p-3 sm:p-5 lg:p-7">
      <div className="relative overflow-hidden rounded-2xl border border-border/70 bg-card p-4 sm:p-6">
        <div className="pointer-events-none absolute -right-16 -top-24 h-64 w-64 rounded-full bg-primary/10 blur-3xl" />
        <div className="relative flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
          <div>
            <p className="text-[10px] font-black uppercase tracking-[0.25em] text-primary">{t('admin.playCompeteEarn', 'Play. Compete. Earn.')}</p>
            <h1 className="mt-1 text-2xl font-black tracking-tight sm:text-3xl">{t('admin.commandCenter', 'Platform command center')}</h1>
            <p className="mt-1 max-w-xl text-sm text-muted-foreground">
              {t('admin.commandCenterDesc', 'Live activity, player growth, competition, and money movement — from persisted KWIK records.')}
            </p>
          </div>
          <DateRangeBar from={range.from} to={range.to} onChange={(from, to) => setRange({ from, to })}>
            <Select value={period} onChange={event => setPeriod(event.target.value)} className="h-9 w-24 text-xs">
              <option value="day">{t('admin.daily', 'Daily')}</option>
              <option value="week">{t('admin.weekly', 'Weekly')}</option>
              <option value="month">{t('admin.monthly', 'Monthly')}</option>
            </Select>
          </DateRangeBar>
        </div>
        <div className="relative mt-4 flex flex-wrap items-center gap-x-4 gap-y-1 text-[11px] text-muted-foreground">
          <span>{formatDate(`${data?.from ?? range.from}T00:00:00`)} – {formatDate(`${data?.to ?? range.to}T00:00:00`)}                    {t('admin.utcDates')}</span>
          <span className="flex items-center gap-1.5">
            <span className="h-1.5 w-1.5 rounded-full bg-success" />
            {live ? t('admin.playersInMatches', { count: live.live_players, defaultValue: `${live.live_players} players in matches` }) : t('admin.liveSnapshotLoading', 'Live snapshot loading')}
          </span>
          {isFetching && <span>{t('admin.refreshing', 'Refreshing…')}</span>}
        </div>
      </div>

      {isError ? (
        <Card><ErrorState message={(error as Error)?.message} onRetry={() => refetch()} /></Card>
      ) : (
        <>
          <div className="grid grid-cols-2 gap-2.5 sm:gap-3 md:grid-cols-3 xl:grid-cols-5">
            <KpiCard label={t('admin.registeredUsers', 'Registered users')} value={formatNumber(data?.total_users ?? 0)} icon={Users}
              detail={t('admin.inPeriod', { count: formatNumber(data?.new_users ?? 0), defaultValue: `+${formatNumber(data?.new_users ?? 0)} in period` })} change={data?.new_users_change} loading={isLoading} />
            <KpiCard label={t('admin.activeUsers', 'Active users')} value={formatNumber(data?.active_users ?? 0)} icon={Activity}
              detail={t('admin.usersWithWalletActivity', 'Users with wallet activity')} change={data?.active_users_change} accent="blue" loading={isLoading} />
            <KpiCard label={t('admin.livePlayers', 'Live players')} value={formatNumber(live?.live_players ?? data?.live_players ?? 0)} icon={Gamepad2}
              detail={t('admin.activeMatchesDetail', { count: formatNumber(live?.active_matches ?? data?.live_matches ?? 0), defaultValue: `${formatNumber(live?.active_matches ?? data?.live_matches ?? 0)} active matches` })} accent="green" loading={isLoading} />
            <KpiCard label={t('admin.matches', 'Matches')} value={formatNumber(data?.total_matches ?? 0)} icon={Swords}
              detail={t('admin.createdToday', { count: formatNumber(data?.matches_today ?? 0), defaultValue: `${formatNumber(data?.matches_today ?? 0)} created today` })} change={data?.total_matches_change} loading={isLoading} />
            <KpiCard label={t('admin.completedMatches', 'Completed matches')} value={formatNumber(data?.completed_matches ?? 0)} icon={TrendingUp}
              detail={t('admin.drawsForfeits', { draws: formatNumber(data?.tied_matches ?? 0), forfeits: formatNumber(data?.forfeited_matches ?? 0), defaultValue: `${formatNumber(data?.tied_matches ?? 0)} draws · ${formatNumber(data?.forfeited_matches ?? 0)} forfeits` })} accent="green" loading={isLoading} />
            <KpiCard label={t('admin.deposits', 'Deposits')} value={formatMoney(data?.total_deposits ?? 0)} icon={WalletCards}
              detail={t('admin.awaitingReview', { count: formatNumber(data?.pending_deposits ?? 0), defaultValue: `${formatNumber(data?.pending_deposits ?? 0)} awaiting review` })} change={data?.deposits_change} accent="green" loading={isLoading} />
            <KpiCard label={t('admin.withdrawals', 'Withdrawals')} value={formatMoney(data?.total_withdrawals ?? 0)} icon={CircleDollarSign}
              detail={t('admin.pendingRequests', { count: formatNumber(data?.pending_withdrawals ?? 0), defaultValue: `${formatNumber(data?.pending_withdrawals ?? 0)} pending requests` })} change={data?.withdrawals_change} accent="amber" loading={isLoading} />
            <KpiCard label={t('admin.platformRevenue', 'Platform revenue')} value={formatMoney(data?.platform_revenue ?? 0)} icon={TrendingUp}
              detail={t('admin.platformRevenueDesc', 'P2P + Crash fees + raffle profit')} change={data?.platform_revenue_change} accent="brand" loading={isLoading} />
            <KpiCard label={t('admin.newUsers', 'New users')} value={formatNumber(data?.new_users ?? 0)} icon={UserPlus}
              detail={t('admin.registrationsInPeriod', 'Registrations in period')} accent="blue" loading={isLoading} />
            <KpiCard label={t('admin.pendingActions', 'Pending actions')} value={formatNumber((data?.pending_deposits ?? 0) + (data?.pending_withdrawals ?? 0))} icon={Clock3}
              detail={t('admin.depositsAndWithdrawals', 'Deposits and withdrawals')} accent="amber" loading={isLoading} />
          </div>

          <div className="grid gap-4 xl:grid-cols-3">
            <Panel title={t('admin.playerGrowth', 'Player growth')} description={t('admin.playerGrowthDesc', 'New registrations over the selected period')}>
              {isLoading ? <Skeleton className="m-4 h-[240px]" /> : data?.registration_trend.length ? <div className="p-2"><GrowthChart data={data.registration_trend} /></div> : <ChartEmpty />}
            </Panel>
            <Panel title={t('admin.matchActivity', 'Match activity')} description={t('admin.matchActivityDesc', 'Created, completed, and paid competition volume')}>
              {isLoading ? <Skeleton className="m-4 h-[240px]" /> : data?.match_trend.length ? <div className="p-2"><MatchChart data={data.match_trend} /></div> : <ChartEmpty />}
            </Panel>
            <Panel title={t('admin.moneyMovement', 'Money movement')} description={t('admin.moneyMovementDesc', 'Completed deposits and withdrawals')}>
              {isLoading ? <Skeleton className="m-4 h-[240px]" /> : data?.finance_trend.length ? <div className="p-2"><FinanceChart data={data.finance_trend} /></div> : <ChartEmpty />}
            </Panel>
            <Panel title={t('admin.platformRevenue', 'Platform revenue')} description={t('admin.platformRevenuePanelDesc', 'Collected P2P fees, settled Crash fees, and fully paid raffle margin')} className="xl:col-span-2">
              {isLoading ? <Skeleton className="m-4 h-[240px]" /> : data?.revenue_trend.length ? <div className="p-2"><RevenueChart data={data.revenue_trend} /></div> : <ChartEmpty label={t('admin.noRealizedRevenue', 'No realized revenue in this period')} />}
            </Panel>
            <Panel title={t('admin.revenueMix', 'Revenue mix')} description={t('admin.revenueMixDesc', 'Service fees and raffle profit')}>
              {isLoading ? <Skeleton className="m-4 h-[240px]" /> : revenueMix.length ? (
                <div className="grid items-center gap-2 p-3 sm:grid-cols-[1fr_1.1fr]">
                  <ResponsiveContainer width="100%" height={230}>
                    <PieChart>
                      <Pie data={revenueMix} dataKey="value" nameKey="name" innerRadius={54} outerRadius={82} paddingAngle={3} stroke="none">
                        {revenueMix.map(item => <Cell key={item.name} fill={item.color} />)}
                      </Pie>
                      <Tooltip contentStyle={chartTooltipStyle()} formatter={(value) => formatMoney(Number(value))} />
                    </PieChart>
                  </ResponsiveContainer>
                  <div className="space-y-2.5">
                    {revenueMix.map(item => (
                      <div key={item.name} className="flex items-center justify-between gap-3 rounded-lg border border-border/60 px-3 py-2">
                        <span className="flex items-center gap-2 text-xs font-medium">
                          <span className="h-2 w-2 rounded-full" style={{ backgroundColor: item.color }} />
                          {item.name}
                        </span>
                        <span className="text-xs font-bold tabular-nums">{formatMoney(item.value)}</span>
                      </div>
                    ))}
                    <div className="flex items-center justify-between border-t border-border/60 pt-2 text-xs">
                      <span className="font-semibold text-muted-foreground">{t('admin.totalRevenueLabel', 'Total revenue')}</span>
                      <span className="font-black text-primary">{formatMoney(data?.platform_revenue ?? 0)}</span>
                    </div>
                  </div>
                </div>
              ) : <ChartEmpty label={t('admin.noRealizedPlatformRevenue', 'No realized platform revenue in this period')} />}
            </Panel>
          </div>

          <div className="grid gap-4 xl:grid-cols-[1.35fr_1fr]">
            <Panel
              title={t('admin.liveMatches', 'Live matches')}
              description={t('admin.liveMatchesDesc', 'Matches with players currently waiting or playing')}
              action={<Link to="/admin/live" className="inline-flex items-center gap-1 text-xs font-semibold text-primary hover:underline">{t('admin.openLiveActivity', 'Open live activity')} <ArrowRight className="h-3.5 w-3.5" /></Link>}
            >
              {!live ? <Skeleton className="m-4 h-32" /> : live.matches.length === 0 ? (
                <EmptyState title={t('admin.noLiveMatches', 'No live matches right now')} description={t('admin.noLiveMatchesDesc', 'Active P2P matches will appear here as soon as players join.')} />
              ) : (
                <div className="divide-y divide-border/60">
                  {live.matches.slice(0, 5).map(match => (
                    <div key={match.id} className="flex flex-wrap items-center gap-3 px-4 py-3 sm:px-5">
                      <div className="flex -space-x-2">
                        {match.players.slice(0, 2).map(player => (
                          <span key={player.user_id} className="flex h-8 w-8 items-center justify-center overflow-hidden rounded-full border-2 border-card bg-muted text-[10px] font-bold">
                            {player.avatar_url ? <img src={player.avatar_url} alt="" className="h-full w-full object-cover" /> : player.display_name.charAt(0)}
                          </span>
                        ))}
                      </div>
                      <div className="min-w-0 flex-1">
                        <p className="truncate text-xs font-semibold">
                          {match.players.map(player => player.display_name).join(' vs ') || match.game_title}
                        </p>
                        <p className="text-[10px] text-muted-foreground">
                          {match.game_title} · {match.competition_mode === 'earn' ? formatMoney(match.stake_amount) + ' ' + t('admin.stake', 'stake') : t('admin.freePlay', 'Free play')} · {relativeTime(match.created_at)}
                        </p>
                      </div>
                      <StatusBadge status={match.status} />
                    </div>
                  ))}
                </div>
              )}
            </Panel>

            <Panel
              title={t('admin.mostPlayedGames', 'Most played games')}
              description={t('admin.mostPlayedGamesDesc', 'Share of matches in the selected period')}
              action={<Link to="/admin/games" className="inline-flex items-center gap-1 text-xs font-semibold text-primary hover:underline">{t('admin.gameAnalyticsLink', 'Game analytics')} <ArrowRight className="h-3.5 w-3.5" /></Link>}
            >
              {isLoading ? <Skeleton className="m-4 h-32" /> : (data?.games.length ?? 0) === 0 ? <EmptyState /> : (
                <div className="space-y-3 p-4 sm:p-5">
                  {data!.games.slice(0, 5).map((game, index) => {
                    const max = Math.max(...data!.games.map(item => item.total_matches), 1)
                    return (
                      <div key={game.slug} className="flex items-center gap-3">
                        <span className={cn('flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-[10px] font-black',
                          index === 0 ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground')}>
                          {index + 1}
                        </span>
                        <div className="h-9 w-16 shrink-0 overflow-hidden rounded-md border border-border/60 bg-muted/30">
                          {game.slug === 'crash' ? <CrashArtwork /> : <GameArtwork slug={game.slug} />}
                        </div>
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center justify-between gap-2 text-xs">
                            <span className="truncate font-semibold">{game.title}</span>
                            <span className="shrink-0 tabular-nums text-muted-foreground">{formatNumber(game.total_matches)}</span>
                          </div>
                          <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-muted">
                            <div className="h-full rounded-full bg-primary/80" style={{ width: `${Math.max(3, (game.total_matches / max) * 100)}%` }} />
                          </div>
                        </div>
                      </div>
                    )
                  })}
                </div>
              )}
            </Panel>
          </div>

          <Card className="overflow-hidden border-border/70">
            <CardContent className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between sm:p-5">
              <div>
                <p className="text-sm font-bold">{t('admin.deeperViewTitle', 'Need a deeper operational view?')}</p>
                <p className="text-xs text-muted-foreground">{t('admin.deeperViewDesc', 'Inspect player records, match settlements, or transaction history.')}</p>
              </div>
              <div className="flex flex-wrap gap-2">
                <Link to="/admin/users" className="rounded-lg border border-border bg-background px-3 py-2 text-xs font-semibold hover:bg-accent">{t('admin.browsePlayers', 'Browse players')}</Link>
                <Link to="/admin/matches" className="rounded-lg border border-border bg-background px-3 py-2 text-xs font-semibold hover:bg-accent">{t('admin.auditMatches', 'Audit matches')}</Link>
                <Link to="/admin/deposits" className="rounded-lg bg-primary px-3 py-2 text-xs font-semibold text-primary-foreground hover:bg-primary/90">{t('admin.reviewFinance', 'Review finance')}</Link>
              </div>
            </CardContent>
          </Card>
        </>
      )}
    </div>
  )
}
