import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
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

const PAGE_SIZE = 20

export function Component() {
  const { t } = useTranslation()
  usePageTitle(`${t('admin.topPlayers')} · KWIK Admin`)
  const games = [
    { slug: '', label: t('admin.allGames') },
    { slug: 'ethiopian-dama', label: t('p2p.ethiopianDama') },
    { slug: 'tic-tac-toe', label: t('p2p.ticTacToe') },
    { slug: 'rock-paper-scissors', label: t('p2p.rockPaperScissors') },
    { slug: 'connect-four', label: t('p2p.connectFour') },
  ]
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
        eyebrow={t('admin.competitionRankings')}
        title={t('admin.topPlayers')}
        description={t('admin.topPlayersDesc')}
        actions={<span className="rounded-full border border-border bg-card px-3 py-1.5 text-xs font-semibold">{t('admin.rankedCount', { count: formatNumber(data?.pagination.total_players ?? 0) })}</span>}
      />

      <Card className="border-border/70 p-3 sm:p-4">
        <div className="grid gap-2 sm:grid-cols-3 lg:flex">
          <Select value={period} onChange={event => changeFilter(() => setPeriod(event.target.value as LeaderboardPeriod))} className="h-10">
            <option value="all_time">{t('admin.allTime')}</option>
            <option value="daily">{t('admin.today')}</option>
            <option value="weekly">{t('admin.thisWeek')}</option>
            <option value="monthly">{t('admin.thisMonth')}</option>
          </Select>
          <Select value={game} onChange={event => changeFilter(() => setGame(event.target.value))} className="h-10">
            {games.map(item => <option key={item.slug} value={item.slug}>{item.label}</option>)}
          </Select>
          <Select value={sort} onChange={event => changeFilter(() => setSort(event.target.value as LeaderboardSortBy))} className="h-10">
            <option value="wins">{t('admin.mostWins')}</option>
            <option value="win_rate">{t('admin.highestWinRate')}</option>
            <option value="win_streak">{t('admin.longestCurrentStreak')}</option>
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
                      <p className="text-[10px] uppercase tracking-wide text-muted-foreground">{t('admin.winsRate', { wins: player.wins, winRate: player.win_rate })}</p>
                      <div className="mt-2 flex items-center gap-1 text-xs font-bold text-warning">
                        {place === 1 ? <Trophy className="h-4 w-4" /> : <Medal className="h-4 w-4" />}
                        {t('admin.matchesCount', { count: player.matches_played })}
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
                    <th className="px-4 py-3 text-center font-semibold">{t('admin.rank')}</th>
                    <th className="px-4 py-3 font-semibold">{t('admin.player')}</th>
                    <th className="px-4 py-3 text-center font-semibold">{t('admin.wld')}</th>
                    <th className="px-4 py-3 text-center font-semibold">{t('admin.winRate')}</th>
                    <th className="px-4 py-3 text-center font-semibold">{t('admin.currentStreak')}</th>
                    <th className="px-4 py-3 text-center font-semibold">{t('admin.bestStreak')}</th>
                    <th className="px-4 py-3 text-right font-semibold">{t('admin.matches')}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border/50">
                  {isLoading && Array.from({ length: 8 }).map((_, index) => <tr key={index}>{Array.from({ length: 7 }).map((__, cell) => <td key={cell} className="px-4 py-3"><Skeleton className="h-4 w-full" /></td>)}</tr>)}
                  {!isLoading && players.length === 0 && <tr><td colSpan={7}><EmptyState title={t('admin.noRankedPlayers')} description={t('admin.noRankedPlayersDesc')} /></td></tr>}
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
              {!isLoading && players.length === 0 && <EmptyState title={t('admin.noRankedPlayers')} />}
              {players.map(player => (
                <div key={player.player_id} className="flex items-center gap-3 p-4">
                  <span className="w-6 shrink-0 text-center text-xs font-black text-muted-foreground">{player.rank}</span>
                  <Avatar name={player.display_name} src={player.avatar_url ?? undefined} />
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-semibold">{player.display_name}</p>
                    <p className="text-[10px] text-muted-foreground">{t('admin.playerStatsLine', { wins: player.wins, losses: player.losses, winRate: player.win_rate, matches: player.matches_played })}</p>
                  </div>
                  {player.current_win_streak > 0 && <span className="inline-flex shrink-0 items-center gap-0.5 text-xs font-bold text-warning"><Flame className="h-3 w-3" />{player.current_win_streak}</span>}
                </div>
              ))}
            </div>
            <Pagination total={data?.pagination.total_players ?? 0} limit={PAGE_SIZE} offset={(page - 1) * PAGE_SIZE} onChange={offset => setPage(Math.floor(offset / PAGE_SIZE) + 1)} loading={isLoading} />
          </Card>
          <p className="text-[10px] text-muted-foreground">{t('admin.playersFooterNote')}</p>
        </>
      )}
    </div>
  )
}
