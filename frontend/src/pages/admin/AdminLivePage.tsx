import { useQuery } from '@tanstack/react-query'
import { Clock3, Gamepad2, RefreshCw, Swords, Users, WifiOff } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { usePageTitle } from '@/hooks/usePageTitle'
import { adminApi } from '@/features/admin/api'
import { Avatar, EmptyState, ErrorState, KpiCard, PageHeader, StatusBadge } from '@/features/admin/AdminUi'
import { formatDate, formatDuration, formatMoney, formatNumber, relativeTime } from '@/features/admin/AdminUtils'

const GAME_LABEL: Record<string, string> = {
  'ethiopian-dama': 'Ethiopian Dama',
  'tic-tac-toe': 'Tic-Tac-Toe',
  'rock-paper-scissors': 'Rock-Paper-Scissors',
  'connect-four': 'Connect Four',
}

export function Component() {
  usePageTitle('Live Activity · KWIK Admin')
  const { data, isLoading, isError, error, refetch, isFetching, dataUpdatedAt } = useQuery({
    queryKey: ['admin', 'live'],
    queryFn: adminApi.live,
    refetchInterval: 15_000,
    refetchIntervalInBackground: false,
  })

  return (
    <div className="mx-auto w-full max-w-[1500px] space-y-5 p-3 sm:p-5 lg:p-7">
      <PageHeader
        eyebrow="Realtime operations"
        title="Live activity"
        description="Players inside active P2P matches and Crash rounds. Refreshes every 15 seconds while this page is open."
        actions={
          <Button variant="outline" size="sm" className="gap-1.5" onClick={() => refetch()} disabled={isFetching}>
            <RefreshCw className={isFetching ? 'h-3.5 w-3.5 animate-spin' : 'h-3.5 w-3.5'} /> Refresh
          </Button>
        }
      />

      <div className="grid grid-cols-2 gap-2.5 lg:grid-cols-4">
        <KpiCard label="Live players" value={formatNumber(data?.live_players ?? 0)} icon={Users} detail="Distinct players in active sessions" accent="green" loading={isLoading} />
        <KpiCard label="Active P2P matches" value={formatNumber(data?.active_matches ?? 0)} icon={Swords} detail="Waiting, ready, or in progress" accent="brand" loading={isLoading} />
        <KpiCard label="Crash bets" value={formatNumber(data?.active_crash_bets ?? 0)} icon={Gamepad2} detail="Active bets in open rounds" accent="amber" loading={isLoading} />
        <KpiCard label="Snapshot time" value={data ? new Date(data.generated_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }) : '—'} icon={Clock3} detail={dataUpdatedAt ? `Updated ${relativeTime(new Date(dataUpdatedAt).toISOString())}` : 'Waiting for data'} accent="neutral" loading={isLoading} />
      </div>

      {isError ? <Card><ErrorState message={(error as Error)?.message} onRetry={() => refetch()} /></Card> : (
        <>
          <div className="flex items-center justify-between gap-3">
            <h2 className="text-sm font-bold">Active P2P matches</h2>
            <span className="inline-flex items-center gap-1.5 text-[10px] font-semibold text-success">
              <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-success" /> Live feed
            </span>
          </div>

          {isLoading ? <Skeleton className="h-48 w-full" /> : data?.matches.length === 0 ? (
            <Card><EmptyState title="No P2P matches are running" description="When players enter a match it will appear here with both line-ups and stake details." /></Card>
          ) : (
            <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
              {data?.matches.map(match => (
                <Card key={match.id} className="overflow-hidden border-border/70">
                  <div className="flex items-center justify-between gap-2 border-b border-border/60 bg-muted/20 px-4 py-3">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-bold">{GAME_LABEL[match.game_slug] ?? match.game_title}</p>
                      <p className="text-[10px] text-muted-foreground">{match.match_type.replaceAll('_', ' ')} · {relativeTime(match.created_at)}</p>
                    </div>
                    <StatusBadge status={match.status} />
                  </div>
                  <CardContent className="space-y-3 p-4">
                    {match.players.map(player => (
                      <div key={player.user_id} className="flex items-center gap-2.5">
                        <Avatar name={player.display_name} src={player.avatar_url} size="sm" />
                        <div className="min-w-0 flex-1">
                          <p className="truncate text-xs font-semibold">{player.display_name}</p>
                          <p className="text-[10px] capitalize text-muted-foreground">{player.status} · score {player.score}</p>
                        </div>
                      </div>
                    ))}
                    <div className="grid grid-cols-3 gap-2 border-t border-border/60 pt-3 text-center">
                      <div><p className="text-[9px] uppercase text-muted-foreground">Mode</p><p className="text-xs font-bold capitalize">{match.competition_mode}</p></div>
                      <div><p className="text-[9px] uppercase text-muted-foreground">Stake</p><p className="text-xs font-bold">{match.competition_mode === 'earn' ? formatMoney(match.stake_amount) : 'Free'}</p></div>
                      <div><p className="text-[9px] uppercase text-muted-foreground">Duration</p><p className="text-xs font-bold">{formatDuration(match.duration_seconds)}</p></div>
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          )}

          <div className="flex items-center justify-between gap-3 pt-1">
            <h2 className="text-sm font-bold">Active Crash rounds</h2>
            <span className="text-[10px] text-muted-foreground">{data?.crash_rounds.length ?? 0} shown · newest first</span>
          </div>
          {isLoading ? <Skeleton className="h-40 w-full" /> : data?.crash_rounds.length === 0 ? (
            <Card><EmptyState title="No Crash rounds are open" description="Waiting, betting, and running rounds will show their players and stakes here." /></Card>
          ) : (
            <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
              {data?.crash_rounds.map(round => (
                <Card key={round.id} className="border-border/70">
                  <div className="flex items-center justify-between gap-2 border-b border-border/60 px-4 py-3">
                    <div>
                      <p className="text-sm font-bold">Round #{round.round_number}</p>
                      <p className="text-[10px] text-muted-foreground">{formatDate(round.created_at, true)}</p>
                    </div>
                    <StatusBadge status={round.state.toLowerCase()} />
                  </div>
                  <CardContent className="p-4">
                    <div className="mb-3 grid grid-cols-3 gap-2 text-center">
                      <div><p className="text-[9px] uppercase text-muted-foreground">Players</p><p className="text-sm font-bold">{round.players}</p></div>
                      <div><p className="text-[9px] uppercase text-muted-foreground">Stakes</p><p className="text-sm font-bold">{formatMoney(round.total_stake)}</p></div>
                      <div><p className="text-[9px] uppercase text-muted-foreground">Multiplier</p><p className="text-sm font-bold">{round.crash_multiplier ? `${round.crash_multiplier.toFixed(2)}×` : '—'}</p></div>
                    </div>
                    <div className="space-y-1.5">
                      {round.bets.length === 0 && <p className="py-2 text-center text-[10px] text-muted-foreground">No active bets yet</p>}
                      {round.bets.slice(0, 4).map(bet => (
                        <div key={`${round.id}-${bet.user_id}`} className="flex items-center gap-2">
                          <Avatar name={bet.display_name} src={bet.avatar_url} size="sm" />
                          <span className="min-w-0 flex-1 truncate text-xs font-medium">{bet.display_name}</span>
                          <span className="text-[10px] font-bold tabular-nums text-muted-foreground">{formatMoney(bet.stake_amount)}</span>
                        </div>
                      ))}
                      {round.bets.length > 4 && <p className="text-[10px] text-muted-foreground">+{round.bets.length - 4} more players</p>}
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          )}

          <div className="flex items-start gap-2 rounded-xl border border-border/60 bg-muted/20 p-3 text-[10px] text-muted-foreground">
            <WifiOff className="mt-0.5 h-3.5 w-3.5 shrink-0" />
            <p>Live counts come from active match participation and active Crash bets. KWIK does not expose reliable browser-presence data, so recently active users are never labelled as online here.</p>
          </div>
        </>
      )}
    </div>
  )
}
