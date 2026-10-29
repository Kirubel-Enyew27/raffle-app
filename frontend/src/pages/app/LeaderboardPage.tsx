import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import {
  Trophy,
  Flame,
  Crown,
  Medal,
  Gamepad2,
  Calendar,
  ArrowUpDown,
  ChevronLeft,
  ChevronRight,
  RefreshCw,
  AlertCircle,
  Swords,
  User,
} from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { useAuth } from '@/contexts/AuthContext'
import { p2pApi } from '@/features/p2p/api'
import type { LeaderboardPeriod, LeaderboardSortBy, LeaderboardPlayer } from '@/types/api'

export function Component() {
  const { t } = useTranslation()
  const { user } = useAuth()

  const [period, setPeriod] = useState<LeaderboardPeriod>('all_time')
  const [selectedGame, setSelectedGame] = useState<string>('')
  const [sortBy, setSortBy] = useState<LeaderboardSortBy>('wins')
  const [page, setPage] = useState<number>(1)
  const limit = 20

  // Fetch available games for filter
  const { data: games = [] } = useQuery({
    queryKey: ['p2p-games'],
    queryFn: p2pApi.listGames,
  })

  // Fetch leaderboard data
  const {
    data: leaderboardData,
    isLoading,
    isError,
    error,
    refetch,
    isFetching,
  } = useQuery({
    queryKey: ['p2p-leaderboard', period, selectedGame, sortBy, page],
    queryFn: () =>
      p2pApi.getLeaderboard({
        period,
        game: selectedGame || undefined,
        sort_by: sortBy,
        page,
        limit,
      }),
  })

  const players = leaderboardData?.players || []
  const currentPlayer = leaderboardData?.current_player
  const pagination = leaderboardData?.pagination

  const topThree = page === 1 ? players.slice(0, 3) : []
  const remainingPlayers = page === 1 ? players.slice(3) : players

  const handlePeriodChange = (newPeriod: LeaderboardPeriod) => {
    setPeriod(newPeriod)
    setPage(1)
  }

  const handleGameChange = (newGame: string) => {
    setSelectedGame(newGame)
    setPage(1)
  }

  const handleSortChange = (newSort: LeaderboardSortBy) => {
    setSortBy(newSort)
    setPage(1)
  }

  return (
    <div className="container mx-auto max-w-6xl space-y-6 px-4 py-8">
      {/* Header Banner */}
      <div className="relative overflow-hidden rounded-2xl bg-gradient-to-r from-amber-500/10 via-primary/10 to-purple-500/10 p-6 md:p-8 border border-border">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div>
            <div className="inline-flex items-center gap-2 rounded-full bg-amber-500/10 px-3 py-1 text-xs font-semibold text-amber-600 dark:text-amber-400">
              <Trophy className="h-3.5 w-3.5" />
              <span>{t('leaderboard.topPodium', 'Hall of Champions')}</span>
            </div>
            <h1 className="mt-2 text-2xl md:text-3xl font-extrabold tracking-tight text-foreground">
              {t('leaderboard.title', 'Player Leaderboard')}
            </h1>
            <p className="mt-1 text-sm text-muted-foreground">
              {t('leaderboard.subtitle', 'Track top players, win streaks, and live win rates across all P2P matches')}
            </p>
          </div>

          <Button
            variant="outline"
            size="sm"
            onClick={() => refetch()}
            disabled={isFetching}
            className="self-start md:self-auto gap-2"
          >
            <RefreshCw className={`h-4 w-4 ${isFetching ? 'animate-spin' : ''}`} />
            <span>{t('leaderboard.retry', 'Refresh')}</span>
          </Button>
        </div>
      </div>

      {/* Control Filters Bar */}
      <Card className="border border-border bg-card">
        <CardContent className="p-4 space-y-4">
          {/* Row 1: Periods & Sort */}
          <div className="flex flex-wrap items-center justify-between gap-4">
            {/* Period Tabs */}
            <div className="flex flex-wrap items-center gap-1.5 rounded-lg bg-muted/60 p-1 text-xs font-medium">
              <Calendar className="ml-2 mr-1 h-3.5 w-3.5 text-muted-foreground hidden sm:inline" />
              {(
                [
                  { id: 'all_time', label: t('leaderboard.allTime', 'All Time') },
                  { id: 'daily', label: t('leaderboard.daily', 'Daily') },
                  { id: 'weekly', label: t('leaderboard.weekly', 'Weekly') },
                  { id: 'monthly', label: t('leaderboard.monthly', 'Monthly') },
                ] as const
              ).map((p) => (
                <button
                  key={p.id}
                  type="button"
                  onClick={() => handlePeriodChange(p.id)}
                  className={`rounded-md px-3 py-1.5 transition-all ${
                    period === p.id
                      ? 'bg-background text-foreground shadow-sm font-semibold'
                      : 'text-muted-foreground hover:text-foreground'
                  }`}
                >
                  {p.label}
                </button>
              ))}
            </div>

            {/* Sort Dropdown / Selector */}
            <div className="flex items-center gap-2">
              <span className="text-xs font-medium text-muted-foreground flex items-center gap-1">
                <ArrowUpDown className="h-3.5 w-3.5" />
                {t('leaderboard.sortBy', 'Sort By')}:
              </span>
              <div className="flex items-center gap-1 bg-muted/60 p-1 rounded-lg text-xs">
                {(
                  [
                    { id: 'wins', label: t('leaderboard.wins', 'Wins') },
                    { id: 'win_streak', label: t('leaderboard.winStreak', 'Streak') },
                    { id: 'win_rate', label: t('leaderboard.winRate', 'Win Rate') },
                  ] as const
                ).map((s) => (
                  <button
                    key={s.id}
                    type="button"
                    onClick={() => handleSortChange(s.id)}
                    className={`rounded-md px-2.5 py-1 transition-all ${
                      sortBy === s.id
                        ? 'bg-background text-foreground shadow-sm font-semibold'
                        : 'text-muted-foreground hover:text-foreground'
                    }`}
                  >
                    {s.label}
                  </button>
                ))}
              </div>
            </div>
          </div>

          {/* Row 2: Game Selector */}
          <div className="flex flex-wrap items-center gap-2 pt-2 border-t border-border/50">
            <span className="text-xs font-medium text-muted-foreground flex items-center gap-1 mr-1">
              <Gamepad2 className="h-3.5 w-3.5" />
              {t('nav.games', 'Game')}:
            </span>
            <Button
              size="sm"
              variant={selectedGame === '' ? 'default' : 'outline'}
              className="h-7 text-xs px-2.5"
              onClick={() => handleGameChange('')}
            >
              {t('leaderboard.allGames', 'All Games')}
            </Button>
            {games.map((g) => (
              <Button
                key={g.slug}
                size="sm"
                variant={selectedGame === g.slug ? 'default' : 'outline'}
                className="h-7 text-xs px-2.5"
                onClick={() => handleGameChange(g.slug)}
              >
                {g.title}
              </Button>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* User's Own Ranking Banner (if logged in and ranked) */}
      {user && (
        <Card className="border border-primary/30 bg-primary/5 shadow-sm">
          <CardContent className="p-4 flex flex-col sm:flex-row items-center justify-between gap-4">
            <div className="flex items-center gap-3">
              <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-primary/20 text-primary font-bold">
                {user.avatar_url ? (
                  <img src={user.avatar_url} alt="" className="h-full w-full rounded-full object-cover" />
                ) : (
                  <User className="h-5 w-5" />
                )}
              </div>
              <div>
                <div className="flex items-center gap-2">
                  <span className="text-sm font-bold text-foreground">
                    {user.full_name || t('leaderboard.yourRank', 'Your Ranking')}
                  </span>
                  {currentPlayer ? (
                    <Badge variant="default" className="text-xs bg-primary text-primary-foreground">
                      #{currentPlayer.rank}
                    </Badge>
                  ) : (
                    <Badge variant="outline" className="text-xs text-muted-foreground">
                      {t('leaderboard.notRanked', 'Unranked')}
                    </Badge>
                  )}
                </div>
                <p className="text-xs text-muted-foreground">
                  {currentPlayer
                    ? `${currentPlayer.matches_played} ${t('leaderboard.matches', 'matches')} • ${currentPlayer.wins} ${t('leaderboard.wins', 'wins')} • ${currentPlayer.win_rate}% ${t('leaderboard.rate', 'win rate')}`
                    : t('leaderboard.playToRank', 'Play a finalized match to establish your ranking in this period!')}
                </p>
              </div>
            </div>

            {currentPlayer && (
              <div className="flex items-center gap-4 text-xs font-medium">
                <div className="text-center px-3 py-1 rounded-lg bg-background/80 border border-border">
                  <p className="text-[10px] text-muted-foreground uppercase">{t('leaderboard.currentStreak', 'Current')}</p>
                  <p className="font-bold text-amber-500 flex items-center justify-center gap-1">
                    <Flame className="h-3 w-3 fill-amber-500" />
                    {currentPlayer.current_win_streak}
                  </p>
                </div>
                <div className="text-center px-3 py-1 rounded-lg bg-background/80 border border-border">
                  <p className="text-[10px] text-muted-foreground uppercase">{t('leaderboard.longestStreak', 'Best')}</p>
                  <p className="font-bold text-primary flex items-center justify-center gap-1">
                    {currentPlayer.longest_win_streak}
                  </p>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* Loading State */}
      {isLoading && (
        <div className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <Skeleton className="h-48 rounded-xl" />
            <Skeleton className="h-56 rounded-xl" />
            <Skeleton className="h-48 rounded-xl" />
          </div>
          <Skeleton className="h-96 rounded-xl" />
        </div>
      )}

      {/* Error State */}
      {isError && (
        <Card className="border-destructive/40 bg-destructive/10 text-destructive">
          <CardContent className="p-8 text-center space-y-3">
            <AlertCircle className="h-10 w-10 mx-auto text-destructive" />
            <h3 className="font-bold text-base">{t('leaderboard.errorTitle', 'Unable to load leaderboard')}</h3>
            <p className="text-sm opacity-90 max-w-md mx-auto">
              {(error as any)?.message || 'A network error occurred while connecting to the leaderboard service.'}
            </p>
            <Button variant="outline" size="sm" onClick={() => refetch()} className="mt-2">
              <RefreshCw className="mr-2 h-4 w-4" />
              {t('leaderboard.retry', 'Try Again')}
            </Button>
          </CardContent>
        </Card>
      )}

      {/* Empty State */}
      {!isLoading && !isError && players.length === 0 && (
        <Card className="border-border">
          <CardContent className="p-12 text-center space-y-3">
            <Swords className="h-12 w-12 mx-auto text-muted-foreground/60" />
            <h3 className="text-base font-semibold text-foreground">
              {t('leaderboard.emptyTitle', 'No matches recorded yet')}
            </h3>
            <p className="text-sm text-muted-foreground max-w-md mx-auto">
              {t('leaderboard.emptyDesc', 'No finalized matches found for this period or game. Play a match now to top the leaderboard!')}
            </p>
          </CardContent>
        </Card>
      )}

      {/* Top 3 Podium Presentation (Only shown on Page 1 when players exist) */}
      {!isLoading && !isError && topThree.length > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 pt-2 pb-2">
          {/* 2nd Place */}
          {topThree[1] ? (
            <PodiumCard
              player={topThree[1]}
              rank={2}
              badgeLabel={t('leaderboard.runnerUp', '2nd Place')}
              crownColor="text-slate-400"
              bgColor="bg-slate-500/10 border-slate-400/40"
              orderClass="order-2 md:order-1 md:mt-6"
            />
          ) : (
            <div className="hidden md:block" />
          )}

          {/* 1st Place (Champion) */}
          {topThree[0] && (
            <PodiumCard
              player={topThree[0]}
              rank={1}
              badgeLabel={t('leaderboard.champion', 'Champion')}
              crownColor="text-amber-500"
              bgColor="bg-amber-500/10 border-amber-500/50 shadow-md shadow-amber-500/10"
              orderClass="order-1 md:order-2"
              isFirst
            />
          )}

          {/* 3rd Place */}
          {topThree[2] ? (
            <PodiumCard
              player={topThree[2]}
              rank={3}
              badgeLabel={t('leaderboard.thirdPlace', '3rd Place')}
              crownColor="text-amber-700 dark:text-amber-600"
              bgColor="bg-amber-700/10 border-amber-700/30"
              orderClass="order-3 md:mt-8"
            />
          ) : (
            <div className="hidden md:block" />
          )}
        </div>
      )}

      {/* Leaderboard Table */}
      {!isLoading && !isError && players.length > 0 && (
        <Card className="border border-border bg-card overflow-hidden shadow-sm">
          <CardHeader className="px-6 py-4 border-b border-border bg-muted/20">
            <div className="flex items-center justify-between">
              <CardTitle className="text-sm font-semibold tracking-wide uppercase text-muted-foreground flex items-center gap-2">
                <Trophy className="h-4 w-4 text-primary" />
                <span>{t('leaderboard.title', 'Player Leaderboard')}</span>
              </CardTitle>
              {pagination && (
                <span className="text-xs text-muted-foreground">
                  {pagination.total_players} {t('leaderboard.players', 'players')}
                </span>
              )}
            </div>
          </CardHeader>

          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="bg-muted/40 text-xs text-muted-foreground uppercase tracking-wider border-b border-border">
                <tr>
                  <th className="py-3 px-4 w-16 text-center">{t('leaderboard.rank', 'Rank')}</th>
                  <th className="py-3 px-4">{t('leaderboard.player', 'Player')}</th>
                  <th className="py-3 px-4 text-center">{t('leaderboard.record', 'W / L / D')}</th>
                  <th className="py-3 px-4 text-center">{t('leaderboard.rate', 'Win Rate')}</th>
                  <th className="py-3 px-4 text-center">{t('leaderboard.currentStreak', 'Streak')}</th>
                  <th className="py-3 px-4 text-center hidden md:table-cell">{t('leaderboard.longestStreak', 'Best')}</th>
                  <th className="py-3 px-4 text-right">{t('leaderboard.matches', 'Matches')}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border/60">
                {(page === 1 ? remainingPlayers : players).map((p) => {
                  const isCurrent = user?.id === p.player_id
                  return (
                    <tr
                      key={p.player_id}
                      className={`transition-colors hover:bg-muted/30 ${
                        isCurrent ? 'bg-primary/10 font-medium' : ''
                      }`}
                    >
                      {/* Rank */}
                      <td className="py-3.5 px-4 text-center font-bold">
                        <span
                          className={`inline-flex items-center justify-center h-6 w-6 rounded-full text-xs ${
                            p.rank === 1
                              ? 'bg-amber-500 text-amber-950 font-extrabold'
                              : p.rank === 2
                              ? 'bg-slate-300 text-slate-900 font-extrabold'
                              : p.rank === 3
                              ? 'bg-amber-700 text-amber-100 font-extrabold'
                              : 'text-muted-foreground'
                          }`}
                        >
                          {p.rank}
                        </span>
                      </td>

                      {/* Player Profile */}
                      <td className="py-3.5 px-4">
                        <div className="flex items-center gap-3">
                          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-muted text-xs font-bold text-foreground">
                            {p.avatar_url ? (
                              <img src={p.avatar_url} alt="" className="h-full w-full rounded-full object-cover" />
                            ) : (
                              p.display_name.charAt(0).toUpperCase()
                            )}
                          </div>
                          <div>
                            <span className="font-semibold text-foreground flex items-center gap-1.5">
                              {p.display_name}
                              {isCurrent && (
                                <Badge variant="outline" className="text-[10px] py-0 px-1 border-primary text-primary">
                                  You
                                </Badge>
                              )}
                            </span>
                          </div>
                        </div>
                      </td>

                      {/* Record (W / L / D) */}
                      <td className="py-3.5 px-4 text-center text-xs font-semibold">
                        <span className="text-emerald-500">{p.wins}W</span>
                        <span className="text-muted-foreground mx-1">•</span>
                        <span className="text-rose-500">{p.losses}L</span>
                        {p.draws > 0 && (
                          <>
                            <span className="text-muted-foreground mx-1">•</span>
                            <span className="text-amber-500">{p.draws}D</span>
                          </>
                        )}
                      </td>

                      {/* Win Rate */}
                      <td className="py-3.5 px-4 text-center">
                        <div className="flex flex-col items-center gap-1">
                          <span className="font-bold text-xs">{p.win_rate}%</span>
                          <div className="w-16 h-1.5 rounded-full bg-muted overflow-hidden">
                            <div
                              className="h-full bg-emerald-500 rounded-full"
                              style={{ width: `${Math.min(100, p.win_rate)}%` }}
                            />
                          </div>
                        </div>
                      </td>

                      {/* Current Streak */}
                      <td className="py-3.5 px-4 text-center font-bold">
                        {p.current_win_streak > 0 ? (
                          <Badge
                            variant="outline"
                            className="text-xs bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/30 gap-1"
                          >
                            <Flame className="h-3 w-3 fill-amber-500" />
                            {p.current_win_streak}
                          </Badge>
                        ) : (
                          <span className="text-xs text-muted-foreground">—</span>
                        )}
                      </td>

                      {/* Longest Streak */}
                      <td className="py-3.5 px-4 text-center hidden md:table-cell text-xs font-medium text-muted-foreground">
                        {p.longest_win_streak > 0 ? (
                          <span className="flex items-center justify-center gap-1">
                            {p.longest_win_streak}
                          </span>
                        ) : (
                          '—'
                        )}
                      </td>

                      {/* Total Completed Matches */}
                      <td className="py-3.5 px-4 text-right font-medium text-xs text-muted-foreground">
                        {p.matches_played}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>

          {/* Pagination Controls */}
          {pagination && pagination.total_pages > 1 && (
            <div className="flex items-center justify-between px-6 py-3 border-t border-border bg-muted/10 text-xs">
              <span className="text-muted-foreground">
                {t('leaderboard.page', 'Page')} {pagination.page} {t('leaderboard.of', 'of')}{' '}
                {pagination.total_pages}
              </span>
              <div className="flex items-center gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setPage((prev) => Math.max(1, prev - 1))}
                  disabled={pagination.page <= 1}
                  className="h-8 gap-1"
                >
                  <ChevronLeft className="h-3.5 w-3.5" />
                  <span>{t('leaderboard.previous', 'Previous')}</span>
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setPage((prev) => Math.min(pagination.total_pages, prev + 1))}
                  disabled={pagination.page >= pagination.total_pages}
                  className="h-8 gap-1"
                >
                  <span>{t('leaderboard.next', 'Next')}</span>
                  <ChevronRight className="h-3.5 w-3.5" />
                </Button>
              </div>
            </div>
          )}
        </Card>
      )}
    </div>
  )
}

function PodiumCard({
  player,
  rank,
  badgeLabel,
  crownColor,
  bgColor,
  orderClass,
  isFirst = false,
}: {
  player: LeaderboardPlayer
  rank: number
  badgeLabel: string
  crownColor: string
  bgColor: string
  orderClass: string
  isFirst?: boolean
}) {
  return (
    <Card className={`border relative overflow-hidden transition-transform hover:-translate-y-1 ${bgColor} ${orderClass}`}>
      <CardContent className="p-5 flex flex-col items-center text-center space-y-3">
        {/* Crown or Medal */}
        <div className="relative">
          <div
            className={`flex items-center justify-center rounded-full border-2 border-background shadow-md ${
              isFirst ? 'h-16 w-16' : 'h-13 w-13'
            } bg-muted text-base font-extrabold`}
          >
            {player.avatar_url ? (
              <img src={player.avatar_url} alt="" className="h-full w-full rounded-full object-cover" />
            ) : (
              player.display_name.charAt(0).toUpperCase()
            )}
          </div>
          <div className="absolute -top-3 -right-2">
            {isFirst ? (
              <Crown className={`h-6 w-6 ${crownColor} fill-current drop-shadow-sm`} />
            ) : (
              <Medal className={`h-5 w-5 ${crownColor}`} />
            )}
          </div>
          <span
            className={`absolute -bottom-1 -left-1 flex h-5 w-5 items-center justify-center rounded-full text-[10px] font-extrabold shadow ${
              isFirst
                ? 'bg-amber-500 text-amber-950'
                : rank === 2
                ? 'bg-slate-300 text-slate-900'
                : 'bg-amber-700 text-amber-100'
            }`}
          >
            {rank}
          </span>
        </div>

        {/* Name & Badge */}
        <div>
          <Badge variant="outline" className={`text-[10px] uppercase font-bold py-0 px-2 mb-1 border-current/40 ${crownColor}`}>
            {badgeLabel}
          </Badge>
          <h4 className="font-bold text-sm text-foreground truncate max-w-[180px]">{player.display_name}</h4>
        </div>

        {/* Stats Grid */}
        <div className="grid grid-cols-3 gap-2 w-full pt-2 border-t border-border/40 text-xs">
          <div>
            <p className="text-[10px] text-muted-foreground uppercase font-semibold">Wins</p>
            <p className="font-extrabold text-emerald-500">{player.wins}</p>
          </div>
          <div>
            <p className="text-[10px] text-muted-foreground uppercase font-semibold">Rate</p>
            <p className="font-extrabold text-foreground">{player.win_rate}%</p>
          </div>
          <div>
            <p className="text-[10px] text-muted-foreground uppercase font-semibold">Streak</p>
            <p className="font-extrabold text-amber-500 flex items-center justify-center gap-0.5">
              <Flame className="h-3 w-3 fill-amber-500" />
              {player.current_win_streak}
            </p>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
