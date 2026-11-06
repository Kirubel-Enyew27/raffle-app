import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Flame, Medal, Trophy } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Select } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { usePageTitle } from '@/hooks/usePageTitle'
import { p2pApi } from '@/features/p2p/api'
import { Avatar, EmptyState, ErrorState, PageHeader, Pagination } from '@/features/admin/AdminUi'
import { formatNumber } from '@/features/admin/AdminUtils'
import { cn } from '@/lib/utils'
import type { LeaderboardPeriod, LeaderboardSortBy } from '@/types/api'

const GAMES = [
  { slug: '', label: 'All games' },
  { slug: 'ethiopian-dama', label: 'Ethiopian Dama' },
  { slug: 'tic-tac-toe', label: 'Tic-Tac-Toe' },
  { slug: 'rock-paper-scissors', label: 'Rock-Paper-Scissors' },
  { slug: 'connect-four', label: 'Connect Four' },
]
const PAGE_SIZE = 20

export function Component() {
  usePageTitle('Top Players · KWIK Admin')
  const [period, setPeriod] = useState<LeaderboardPeriod>('all_time')
  const [game, setGame] = useState('')
  const [sort, setSort] = useState<LeaderboardSortBy>('wins')
  const [page, setPage] = useState(1)
  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['admin', 'players', period, game, sort, page],
    queryFn: () => p2pApi.getLeaderboard({ period, game: game || undefined, sort_by: sort, page, limit: PAGE_SIZE }),
  })
  const players = data?.players ?? []
  const podium = page === 1 ? players.slice(0, 3) : []

  const changeFilter = (action: () => void) => { action(); setPage(1) }

  return (
    <div className="mx-auto w-full max-w-[1400px] space-y-5 p-3 sm:p-5 lg:p-7">
      <PageHeader
        eyebrow="Competition rankings"
        title="Top players"
        description="Rankings from finalized P2P matches, using the same leaderboard logic players see. Crash earnings are not part of this leaderboard."
        actions={<span className="rounded-full border border-border bg-card px-3 py-1.5 text-xs font-semibold">{formatNumber(data?.pagination.total_players ?? 0)} ranked</span>}
      />

      <Card className="border-border/70 p-3 sm:p-4">
        <div className="grid gap-2 sm:grid-cols-3 lg:flex">
          <Select value={period} onChange={event => changeFilter(() => setPeriod(event.target.value as LeaderboardPeriod))} className="h-10">
            <option value="all_time">All time</option>
            <option value="daily">Today</option>
            <option value="weekly">This week</option>
            <option value="monthly">This month</option>
          </Select>
          <Select value={game} onChange={event => changeFilter(() => setGame(event.target.value))} className="h-10">
            {GAMES.map(item => <option key={item.slug} value={item.slug}>{item.label}</option>)}
          </Select>
          <Select value={sort} onChange={event => changeFilter(() => setSort(event.target.value as LeaderboardSortBy))} className="h-10">
            <option value="wins">Most wins</option>
            <option value="win_rate">Highest win rate</option>
            <option value="win_streak">Longest current streak</option>
          </Select>
        </div>
      </Card>

      {isError ? <Card><ErrorState message={(error as Error)?.message} onRetry={() => refetch()} /></Card> : (
        <>
          {isLoading ? <div className="grid gap-3 sm:grid-cols-3">{Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} className="h-36" />)}</div> : podium.length === 3 && (
            <div className="grid gap-3 sm:grid-cols-3">
              {[podium[1], podium[0], podium[2]].map((player, index) => {
                const place = index === 1 ? 1 : index === 0 ? 2 : 3
                const styles = place === 1
                  ? 'border-amber-400/40 bg-gradient-to-br from-amber-400/15 to-card'
                  : place === 2 ? 'border-slate-400/30 bg-gradient-to-br from-slate-400/10 to-card' : 'border-amber-700/30 bg-gradient-to-br from-amber-700/10 to-card'
                return (
                  <Card key={player.player_id} className={cn('overflow-hidden border', styles, place === 1 && 'sm:-mt-2 sm:mb-2')}>
                    <CardContent className="flex flex-col items-center p-4 text-center">
                      <div className="relative">
                        <Avatar name={player.display_name} src={player.avatar_url ?? undefined} size="lg" />
                        <span className={cn('absolute -bottom-1 -right-2 flex h-6 w-6 items-center justify-center rounded-full border-2 border-card text-[10px] font-black',
                          place === 1 ? 'bg-amber-400 text-amber-950' : place === 2 ? 'bg-slate-300 text-slate-900' : 'bg-amber-700 text-amber-100')}>
                          {place}
                        </span>
                      </div>
                      <p className="mt-3 max-w-full truncate text-sm font-bold">{player.display_name}</p>
                      <p className="text-[10px] uppercase tracking-wide text-muted-foreground">{player.wins} wins · {player.win_rate}% rate</p>
                      <div className="mt-2 flex items-center gap-1 text-xs font-bold text-warning">
                        {place === 1 ? <Trophy className="h-4 w-4" /> : <Medal className="h-4 w-4" />}
                        {player.matches_played} matches
                      </div>
                    </CardContent>
                  </Card>
                )
              })}
            </div>
          )}

          <Card className="overflow-hidden border-border/70">
            <div className="hidden overflow-x-auto md:block">
              <table className="w-full min-w-[780px] text-left text-sm">
                <thead className="border-b border-border/60 bg-muted/30 text-[10px] uppercase tracking-wider text-muted-foreground">
                  <tr>
                    <th className="px-4 py-3 text-center font-semibold">Rank</th>
                    <th className="px-4 py-3 font-semibold">Player</th>
                    <th className="px-4 py-3 text-center font-semibold">W / L / D</th>
                    <th className="px-4 py-3 text-center font-semibold">Win rate</th>
                    <th className="px-4 py-3 text-center font-semibold">Current streak</th>
                    <th className="px-4 py-3 text-center font-semibold">Best streak</th>
                    <th className="px-4 py-3 text-right font-semibold">Matches</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border/50">
                  {isLoading && Array.from({ length: 8 }).map((_, index) => <tr key={index}>{Array.from({ length: 7 }).map((__, cell) => <td key={cell} className="px-4 py-3"><Skeleton className="h-4 w-full" /></td>)}</tr>)}
                  {!isLoading && players.length === 0 && <tr><td colSpan={7}><EmptyState title="No ranked players in this view" description="Finalized matches in the selected period will establish rankings." /></td></tr>}
                  {players.map(player => (
                    <tr key={player.player_id} className="transition-colors hover:bg-muted/25">
                      <td className="px-4 py-3 text-center"><span className={cn('inline-flex h-7 w-7 items-center justify-center rounded-full text-xs font-black', player.rank <= 3 ? 'bg-warning/15 text-warning' : 'text-muted-foreground')}>{player.rank}</span></td>
                      <td className="px-4 py-3"><div className="flex items-center gap-2.5"><Avatar name={player.display_name} src={player.avatar_url ?? undefined} size="sm" /><span className="font-semibold">{player.display_name}</span></div></td>
                      <td className="px-4 py-3 text-center text-xs"><span className="font-semibold text-success">{player.wins}W</span> <span className="text-muted-foreground">/</span> <span className="font-semibold text-destructive">{player.losses}L</span> <span className="text-muted-foreground">/</span> <span className="font-semibold text-warning">{player.draws}D</span></td>
                      <td className="px-4 py-3"><div className="mx-auto w-20"><p className="text-center text-xs font-bold">{player.win_rate}%</p><div className="mt-1 h-1 overflow-hidden rounded-full bg-muted"><div className="h-full bg-success" style={{ width: `${Math.min(100, player.win_rate)}%` }} /></div></div></td>
                      <td className="px-4 py-3 text-center">{player.current_win_streak > 0 ? <span className="inline-flex items-center gap-1 rounded-full bg-warning/10 px-2 py-0.5 text-xs font-bold text-warning"><Flame className="h-3 w-3" />{player.current_win_streak}</span> : <span className="text-muted-foreground">—</span>}</td>
                      <td className="px-4 py-3 text-center text-xs text-muted-foreground">{player.longest_win_streak}</td>
                      <td className="px-4 py-3 text-right text-xs font-semibold tabular-nums">{player.matches_played}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="divide-y divide-border/60 md:hidden">
              {isLoading && Array.from({ length: 4 }).map((_, i) => <div key={i} className="p-4"><Skeleton className="h-16 w-full" /></div>)}
              {!isLoading && players.length === 0 && <EmptyState title="No ranked players in this view" />}
              {players.map(player => (
                <div key={player.player_id} className="flex items-center gap-3 p-4">
                  <span className="w-6 shrink-0 text-center text-xs font-black text-muted-foreground">{player.rank}</span>
                  <Avatar name={player.display_name} src={player.avatar_url ?? undefined} />
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-semibold">{player.display_name}</p>
                    <p className="text-[10px] text-muted-foreground">{player.wins}W · {player.losses}L · {player.win_rate}% · {player.matches_played} matches</p>
                  </div>
                  {player.current_win_streak > 0 && <span className="inline-flex shrink-0 items-center gap-0.5 text-xs font-bold text-warning"><Flame className="h-3 w-3" />{player.current_win_streak}</span>}
                </div>
              ))}
            </div>
            <Pagination total={data?.pagination.total_players ?? 0} limit={PAGE_SIZE} offset={(page - 1) * PAGE_SIZE} onChange={offset => setPage(Math.floor(offset / PAGE_SIZE) + 1)} loading={isLoading} />
          </Card>
          <p className="text-[10px] text-muted-foreground">Earnings and payouts are not exposed by the leaderboard endpoint, so no monetary ranking is shown. Competition-mode filtering is not available in that response.</p>
        </>
      )}
    </div>
  )
}
