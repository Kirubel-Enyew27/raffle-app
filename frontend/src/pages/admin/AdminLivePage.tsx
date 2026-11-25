import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Clock3, RefreshCw, Swords, Users, WifiOff } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { usePageTitle } from '@/hooks/usePageTitle'
import { adminApi } from '@/features/admin/api'
import { Avatar, EmptyState, ErrorState, KpiCard, PageHeader, StatusBadge } from '@/features/admin/AdminUi'
import { formatDuration, formatMoney, formatNumber, relativeTime } from '@/features/admin/AdminUtils'


export function Component() {
  const { t } = useTranslation()
  usePageTitle(`${t('admin.liveActivity')} · KWIK Admin`)
  const { data, isLoading, isError, error, refetch, isFetching, dataUpdatedAt } = useQuery({
    queryKey: ['admin', 'live'],
    queryFn: adminApi.live,
    refetchInterval: 15_000,
    refetchIntervalInBackground: false,
  })

  const getGameTitle = (slug: string, fallback: string) => {
    switch (slug) {
      case 'ethiopian-dama': return t('p2p.ethiopianDama')
      case 'tic-tac-toe': return t('p2p.ticTacToe')
      case 'rock-paper-scissors': return t('p2p.rockPaperScissors')
      case 'connect-four': return t('p2p.connectFour')
      default: return fallback
    }
  }

  return (
    <div className="mx-auto w-full max-w-[1500px] space-y-5 p-3 sm:p-5 lg:p-7">
      <PageHeader
        eyebrow={t('admin.realtimeOperations')}
        title={t('admin.liveActivity')}
        description={t('admin.liveActivityDesc')}
        actions={
          <Button variant="outline" size="sm" className="gap-1.5" onClick={() => refetch()} disabled={isFetching}>
            <RefreshCw className={isFetching ? 'h-3.5 w-3.5 animate-spin' : 'h-3.5 w-3.5'} /> {t('admin.refresh')}
          </Button>
        }
      />

      <div className="grid grid-cols-2 gap-2.5 lg:grid-cols-4">
        <KpiCard label={t('admin.livePlayers')} value={formatNumber(data?.live_players ?? 0)} icon={Users} detail={t('admin.distinctPlayersInActiveSessions')} accent="green" loading={isLoading} />
        <KpiCard label={t('admin.activeP2PMatches')} value={formatNumber(data?.active_matches ?? 0)} icon={Swords} detail={t('admin.waitingReadyInProgress')} accent="brand" loading={isLoading} />
        <KpiCard label={t('admin.snapshotTime')} value={data ? new Date(data.generated_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }) : '—'} icon={Clock3} detail={dataUpdatedAt ? t('admin.updatedAgo', { time: relativeTime(new Date(dataUpdatedAt).toISOString()) }) : t('admin.waitingForData')} accent="neutral" loading={isLoading} />
      </div>

      {isError ? <Card><ErrorState message={(error as Error)?.message} onRetry={() => refetch()} /></Card> : (
        <>
          <div className="flex items-center justify-between gap-3">
            <h2 className="text-sm font-bold">{t('admin.activeP2PMatches')}</h2>
            <span className="inline-flex items-center gap-1.5 text-[10px] font-semibold text-success">
              <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-success" /> {t('admin.liveFeed')}
            </span>
          </div>

          {isLoading ? <Skeleton className="h-48 w-full" /> : data?.matches.length === 0 ? (
            <Card><EmptyState title={t('admin.noRunningMatches')} description={t('admin.noRunningMatchesDesc')} /></Card>
          ) : (
            <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
              {data?.matches.map(match => (
                <Card key={match.id} className="overflow-hidden border-border/70">
                  <div className="flex items-center justify-between gap-2 border-b border-border/60 bg-muted/20 px-4 py-3">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-bold">{getGameTitle(match.game_slug, match.game_title)}</p>
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
                          <p className="text-[10px] capitalize text-muted-foreground">{t('admin.playerStatusScore', { status: player.status, score: player.score })}</p>
                        </div>
                      </div>
                    ))}
                    <div className="grid grid-cols-3 gap-2 border-t border-border/60 pt-3 text-center">
                      <div><p className="text-[9px] uppercase text-muted-foreground">{t('admin.mode')}</p><p className="text-xs font-bold capitalize">{match.competition_mode === 'earn' ? t('admin.competeToEarn') : t('admin.free')}</p></div>
                      <div><p className="text-[9px] uppercase text-muted-foreground">{t('admin.stake')}</p><p className="text-xs font-bold">{match.competition_mode === 'earn' ? formatMoney(match.stake_amount) : t('admin.free')}</p></div>
                      <div><p className="text-[9px] uppercase text-muted-foreground">{t('admin.duration')}</p><p className="text-xs font-bold">{formatDuration(match.duration_seconds)}</p></div>
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          )}

          <div className="flex items-start gap-2 rounded-xl border border-border/60 bg-muted/20 p-3 text-[10px] text-muted-foreground">
            <WifiOff className="mt-0.5 h-3.5 w-3.5 shrink-0" />
            <p>{t('admin.liveFooterNote')}</p>
          </div>
        </>
      )}
    </div>
  )
}
