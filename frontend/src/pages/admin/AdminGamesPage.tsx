import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Gamepad2, Swords, Trophy, Users } from 'lucide-react'
import { Area, AreaChart, Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { Card, CardContent } from '@/components/ui/card'
import { Select } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { usePageTitle } from '@/hooks/usePageTitle'
import { adminApi, defaultRange, type AdminGame } from '@/features/admin/api'
import { DateRangeBar, EmptyState, ErrorState, KpiCard, PageHeader, Panel } from '@/features/admin/AdminUi'
import { chartTooltipStyle, formatNumber } from '@/features/admin/AdminUtils'
import { CrashArtwork, GameArtwork } from '@/features/p2p/GameArtwork'
import { cn } from '@/lib/utils'

const ACCENTS: Record<string, string> = {
  'ethiopian-dama': 'from-amber-400/20',
  'tic-tac-toe': 'from-primary/20',
  'rock-paper-scissors': 'from-violet-400/20',
  'connect-four': 'from-emerald-400/20',
  crash: 'from-primary/25',
}

function GameArt({ slug }: { slug: string }) {
  return (
    <div className="h-full w-full">
      {slug === 'crash' ? <CrashArtwork /> : <GameArtwork slug={slug} />}
    </div>
  )
}

function GameCard({ game, selected, onSelect, max }: { game: AdminGame; selected: boolean; onSelect: () => void; max: number }) {
  const total = game.free_matches + game.paid_matches
  const paidShare = total ? (game.paid_matches / total) * 100 : 0
  return (
    <button
      type="button"
      onClick={onSelect}
      className={cn(
        'group overflow-hidden rounded-2xl border bg-card text-left transition-all hover:-translate-y-0.5 hover:shadow-lg',
        selected ? 'border-primary ring-1 ring-primary/30' : 'border-border/70 hover:border-border',
      )}
    >
      <div className={cn('relative h-28 overflow-hidden bg-gradient-to-br to-transparent', ACCENTS[game.slug] ?? 'from-muted')}>
        <GameArt slug={game.slug} />
        <span className="absolute left-3 top-3 rounded-full bg-black/55 px-2 py-0.5 text-[9px] font-black uppercase tracking-wider text-white backdrop-blur">
          {game.active_matches > 0 ? `${game.active_matches} live` : 'No live matches'}
        </span>
      </div>
      <CardContent className="p-4">
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0">
            <h3 className="truncate text-sm font-bold">{game.title}</h3>
            <p className="mt-0.5 line-clamp-1 text-[10px] text-muted-foreground">{game.description}</p>
          </div>
          <span className="shrink-0 rounded-lg bg-muted px-2 py-1 text-[10px] font-black tabular-nums">{formatNumber(game.total_matches)}</span>
        </div>
        <div className="mt-3 grid grid-cols-3 gap-2 text-center">
          <div><p className="text-[8px] uppercase tracking-wider text-muted-foreground">Active</p><p className="text-xs font-bold">{game.active_matches}</p></div>
          <div><p className="text-[8px] uppercase tracking-wider text-muted-foreground">Completed</p><p className="text-xs font-bold">{game.completed_matches}</p></div>
          <div><p className="text-[8px] uppercase tracking-wider text-muted-foreground">Players</p><p className="text-xs font-bold">{game.unique_players}</p></div>
        </div>
        <div className="mt-3">
          <div className="mb-1 flex justify-between text-[9px] text-muted-foreground"><span>Free {game.free_matches}</span><span>Paid {game.paid_matches}</span></div>
          <div className="h-1.5 overflow-hidden rounded-full bg-success/15">
            <div className="h-full bg-success" style={{ width: `${100 - paidShare}%` }} />
            <div className="-mt-1.5 ml-auto h-full bg-primary" style={{ width: `${paidShare}%` }} />
          </div>
        </div>
        <div className="mt-3 h-1 overflow-hidden rounded-full bg-muted">
          <div className="h-full rounded-full bg-foreground/30" style={{ width: `${Math.max(2, (game.total_matches / max) * 100)}%` }} />
        </div>
      </CardContent>
    </button>
  )
}

export function Component() {
  usePageTitle('Games Analytics · KWIK Admin')
  const [range, setRange] = useState(defaultRange)
  const [period, setPeriod] = useState('day')
  const [selectedSlug, setSelectedSlug] = useState('')
  const { data: games, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['admin', 'games', range, period],
    queryFn: () => adminApi.games(range.from, range.to, period),
  })
  const selected = games?.find(game => game.slug === selectedSlug) ?? games?.[0]
  const max = Math.max(...(games?.map(game => game.total_matches) ?? [0]), 1)
  const totals = (games ?? []).reduce((acc, game) => ({
    matches: acc.matches + game.total_matches,
    active: acc.active + game.active_matches,
    completed: acc.completed + game.completed_matches,
    players: acc.players + game.unique_players,
  }), { matches: 0, active: 0, completed: 0, players: 0 })

  return (
    <div className="mx-auto w-full max-w-[1500px] space-y-5 p-3 sm:p-5 lg:p-7">
      <PageHeader
        eyebrow="Game intelligence"
        title="Games analytics"
        description="See which KWIK games attract players, which matches convert to completion, and how free and paid play are distributed."
        actions={
          <DateRangeBar from={range.from} to={range.to} onChange={(from, to) => setRange({ from, to })}>
            <Select value={period} onChange={event => setPeriod(event.target.value)} className="h-9 w-24 text-xs">
              <option value="day">Daily</option>
              <option value="week">Weekly</option>
              <option value="month">Monthly</option>
            </Select>
          </DateRangeBar>
        }
      />

      <div className="grid grid-cols-2 gap-2.5 lg:grid-cols-4">
        <KpiCard label="Matches" value={formatNumber(totals.matches)} icon={Swords} detail="Across all games" loading={isLoading} />
        <KpiCard label="Live now" value={formatNumber(totals.active)} icon={Gamepad2} detail="Waiting or in progress" accent="green" loading={isLoading} />
        <KpiCard label="Completed" value={formatNumber(totals.completed)} icon={Trophy} detail="Finished competition" accent="amber" loading={isLoading} />
        <KpiCard label="Player entries" value={formatNumber(totals.players)} icon={Users} detail="Unique player appearances by game" accent="blue" loading={isLoading} />
      </div>

      {isError ? <Card><ErrorState message={(error as Error)?.message} onRetry={() => refetch()} /></Card> : (
        <>
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
            {isLoading && Array.from({ length: 5 }).map((_, i) => <Skeleton key={i} className="h-72 rounded-2xl" />)}
            {games?.map(game => (
              <GameCard key={game.slug} game={game} selected={selected?.slug === game.slug} onSelect={() => setSelectedSlug(game.slug)} max={max} />
            ))}
            {!isLoading && games?.length === 0 && <Card className="sm:col-span-2 xl:col-span-3"><EmptyState title="No game activity yet" /></Card>}
          </div>

          {selected && (
            <div className="grid gap-4 xl:grid-cols-[1.4fr_1fr]">
              <Panel title={`${selected.title} activity`} description="Match volume over the selected period">
                <div className="p-3">
                  {selected.trend.length ? (
                    <ResponsiveContainer width="100%" height={260}>
                      <AreaChart data={selected.trend} margin={{ top: 8, right: 12, left: -20, bottom: 0 }}>
                        <defs>
                          <linearGradient id="gameTrendFill" x1="0" y1="0" x2="0" y2="1">
                            <stop offset="0%" stopColor="#ff5b55" stopOpacity={0.3} />
                            <stop offset="100%" stopColor="#ff5b55" stopOpacity={0} />
                          </linearGradient>
                        </defs>
                        <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" vertical={false} />
                        <XAxis dataKey="period" tick={{ fontSize: 10, fill: 'var(--color-muted-foreground)' }} tickLine={false} axisLine={false} minTickGap={20} />
                        <YAxis tick={{ fontSize: 10, fill: 'var(--color-muted-foreground)' }} tickLine={false} axisLine={false} allowDecimals={false} />
                        <Tooltip contentStyle={chartTooltipStyle()} />
                        <Area type="monotone" dataKey="matches" name="Matches" stroke="#ff5b55" fill="url(#gameTrendFill)" strokeWidth={2} dot={false} />
                      </AreaChart>
                    </ResponsiveContainer>
                  ) : <div className="flex h-[260px] items-center justify-center text-xs text-muted-foreground">No matches for {selected.title} in this period</div>}
                </div>
              </Panel>
              <Panel title="Mode mix" description="Free versus paid competition">
                <div className="p-3">
                  {(selected.free_matches + selected.paid_matches) > 0 ? (
                    <>
                      <ResponsiveContainer width="100%" height={190}>
                        <BarChart data={[{ name: 'Matches', Free: selected.free_matches, Paid: selected.paid_matches }]} margin={{ top: 8, right: 8, left: -20, bottom: 0 }}>
                          <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" vertical={false} />
                          <XAxis dataKey="name" tick={{ fontSize: 10, fill: 'var(--color-muted-foreground)' }} tickLine={false} axisLine={false} />
                          <YAxis tick={{ fontSize: 10, fill: 'var(--color-muted-foreground)' }} tickLine={false} axisLine={false} allowDecimals={false} />
                          <Tooltip contentStyle={chartTooltipStyle()} />
                          <Bar dataKey="Free" stackId="mode" fill="#34d399" radius={[4, 4, 0, 0]} />
                          <Bar dataKey="Paid" stackId="mode" fill="#ff5b55" radius={[4, 4, 0, 0]} />
                        </BarChart>
                      </ResponsiveContainer>
                      <div className="grid grid-cols-2 gap-2">
                        <div className="rounded-xl border border-border/60 bg-success/5 p-3 text-center"><p className="text-[9px] uppercase text-muted-foreground">Free play</p><p className="text-lg font-black text-success">{selected.free_matches}</p></div>
                        <div className="rounded-xl border border-border/60 bg-primary/5 p-3 text-center"><p className="text-[9px] uppercase text-muted-foreground">Compete to earn</p><p className="text-lg font-black text-primary">{selected.paid_matches}</p></div>
                      </div>
                    </>
                  ) : <EmptyState title={`No ${selected.title} mode data`} />}
                </div>
              </Panel>
            </div>
          )}
        </>
      )}
      <p className="text-[10px] text-muted-foreground">Crash is included from persisted game rounds. P2P game counts include matches with at least one player entry; free/paid splits use the match competition mode.</p>
    </div>
  )
}
