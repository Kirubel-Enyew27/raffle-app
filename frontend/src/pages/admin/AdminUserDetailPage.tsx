import { Link, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, Ban, CalendarDays, Clock, Gamepad2, Mail, Phone, ShieldCheck, Wallet } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { usePageTitle } from '@/hooks/usePageTitle'
import { adminApi } from '@/features/admin/api'
import { Avatar, EmptyState, ErrorState, StatusBadge } from '@/features/admin/AdminUi'
import { formatDate, formatMoney, formatNumber, relativeTime, shortId } from '@/features/admin/AdminUtils'
import { cn } from '@/lib/utils'

const GAME_LABEL: Record<string, string> = {
  'ethiopian-dama': 'Ethiopian Dama',
  'tic-tac-toe': 'Tic-Tac-Toe',
  'rock-paper-scissors': 'Rock-Paper-Scissors',
  'connect-four': 'Connect Four',
  crash: 'Crash',
}

export function Component() {
  const { id = '' } = useParams()
  usePageTitle('Player detail · KWIK Admin')
  const { data: player, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['admin', 'user', id],
    queryFn: () => adminApi.user(id),
    enabled: Boolean(id),
  })

  if (isLoading) {
    return <div className="space-y-4 p-4 sm:p-6"><Skeleton className="h-16 w-full" /><Skeleton className="h-64 w-full" /><Skeleton className="h-64 w-full" /></div>
  }
  if (isError || !player) {
    return (
      <div className="p-4 sm:p-6">
        <Link to="/admin/users" className="mb-4 inline-flex items-center gap-1.5 text-xs font-semibold text-primary"><ArrowLeft className="h-3.5 w-3.5" /> Back to users</Link>
        <Card><ErrorState message={(error as Error)?.message} onRetry={() => refetch()} /></Card>
      </div>
    )
  }

  const available = Math.max(0, player.wallet_balance - player.reserved_balance)
  const stats = [
    { label: 'Matches', value: formatNumber(player.matches_played), icon: Gamepad2 },
    { label: 'Wins', value: formatNumber(player.wins), icon: ShieldCheck, tone: 'text-success' },
    { label: 'Losses', value: formatNumber(player.losses), icon: Ban, tone: 'text-destructive' },
    { label: 'Win rate', value: `${player.win_rate.toFixed(1)}%`, icon: Clock },
  ]

  return (
    <div className="mx-auto w-full max-w-[1400px] space-y-5 p-3 sm:p-5 lg:p-7">
      <Link to="/admin/users" className="inline-flex items-center gap-1.5 text-xs font-semibold text-muted-foreground hover:text-foreground">
        <ArrowLeft className="h-3.5 w-3.5" /> Back to users
      </Link>

      <div className="relative overflow-hidden rounded-2xl border border-border/70 bg-card p-4 sm:p-6">
        <div className="pointer-events-none absolute -right-14 -top-20 h-56 w-56 rounded-full bg-primary/10 blur-3xl" />
        <div className="relative flex flex-col gap-4 sm:flex-row sm:items-center">
          <Avatar name={player.full_name || player.email} src={player.avatar_url} size="lg" />
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="truncate text-2xl font-black">{player.full_name || 'Unnamed player'}</h1>
              {player.is_banned ? <StatusBadge status="banned" /> : <StatusBadge status="active" />}
              {player.role === 'admin' && <span className="rounded-full bg-info/10 px-2 py-0.5 text-[10px] font-bold text-info">Administrator</span>}
              {player.is_playing && <span className="rounded-full bg-primary/10 px-2 py-0.5 text-[10px] font-bold text-primary">In a match</span>}
            </div>
            <p className="mt-1 break-all text-sm text-muted-foreground">{player.email}</p>
            <p className="mt-1 font-mono text-[10px] text-muted-foreground">ID {player.id}</p>
          </div>
          <div className="rounded-xl border border-border/70 bg-muted/30 px-4 py-3">
            <p className="text-[9px] font-bold uppercase tracking-wider text-muted-foreground">Available balance</p>
            <p className="mt-1 text-xl font-black tabular-nums">{formatMoney(available)}</p>
            <p className="text-[10px] text-muted-foreground">{formatMoney(player.reserved_balance)} reserved of {formatMoney(player.wallet_balance)}</p>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-2.5 md:grid-cols-4">
        {stats.map(({ label, value, icon: Icon, tone }) => (
          <Card key={label} className="border-border/70">
            <CardContent className="flex items-center justify-between p-4">
              <div>
                <p className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">{label}</p>
                <p className={cn('mt-1 text-xl font-black tabular-nums', tone)}>{value}</p>
              </div>
              <Icon className="h-5 w-5 text-muted-foreground/50" />
            </CardContent>
          </Card>
        ))}
      </div>

      <div className="grid gap-4 lg:grid-cols-[0.85fr_1.4fr]">
        <Card className="border-border/70">
          <CardHeader className="pb-3"><CardTitle className="text-sm">Account details</CardTitle></CardHeader>
          <CardContent className="space-y-3 pt-0 text-sm">
            <div className="flex items-start gap-3"><Mail className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" /><div><p className="text-[10px] uppercase text-muted-foreground">Email</p><p className="break-all text-xs font-medium">{player.email}</p></div></div>
            <div className="flex items-start gap-3"><Phone className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" /><div><p className="text-[10px] uppercase text-muted-foreground">Phone</p><p className="text-xs font-medium">{player.phone || 'Not provided'}</p></div></div>
            <div className="flex items-start gap-3"><CalendarDays className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" /><div><p className="text-[10px] uppercase text-muted-foreground">Registered</p><p className="text-xs font-medium">{formatDate(player.created_at, true)}</p></div></div>
            <div className="flex items-start gap-3"><Clock className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" /><div><p className="text-[10px] uppercase text-muted-foreground">Last active</p><p className="text-xs font-medium">{relativeTime(player.last_active_at)}</p></div></div>
            <div className="flex items-start gap-3"><Wallet className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" /><div><p className="text-[10px] uppercase text-muted-foreground">Wallet</p><p className="text-xs font-medium">{formatMoney(player.wallet_balance)} · {formatMoney(player.reserved_balance)} reserved</p></div></div>
          </CardContent>
        </Card>

        <Card className="overflow-hidden border-border/70">
          <CardHeader className="border-b border-border/60 pb-3"><CardTitle className="text-sm">Recent matches</CardTitle></CardHeader>
          {player.recent_matches.length === 0 ? <EmptyState title="No match history" /> : (
            <div className="overflow-x-auto">
              <table className="w-full min-w-[600px] text-left text-xs">
                <thead className="bg-muted/30 text-[10px] uppercase text-muted-foreground">
                  <tr>
                    <th className="px-4 py-2.5">Game</th>
                    <th className="px-4 py-2.5">Result</th>
                    <th className="px-4 py-2.5">Mode</th>
                    <th className="px-4 py-2.5 text-right">Stake</th>
                    <th className="px-4 py-2.5 text-right">Prize pool</th>
                    <th className="px-4 py-2.5">Played</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border/50">
                  {player.recent_matches.map(match => (
                    <tr key={match.id}>
                      <td className="px-4 py-3 font-semibold">{GAME_LABEL[match.game_slug] ?? match.game_slug}</td>
                      <td className="px-4 py-3"><StatusBadge status={match.result} /></td>
                      <td className="px-4 py-3 capitalize text-muted-foreground">{match.competition_mode}</td>
                      <td className="px-4 py-3 text-right tabular-nums">{match.competition_mode === 'earn' ? formatMoney(match.stake_amount) : 'Free'}</td>
                      <td className="px-4 py-3 text-right tabular-nums">{formatMoney(match.prize_pool)}</td>
                      <td className="px-4 py-3 text-muted-foreground">{formatDate(match.completed_at || match.created_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Card>
      </div>

      <Card className="overflow-hidden border-border/70">
        <CardHeader className="border-b border-border/60 pb-3"><CardTitle className="text-sm">Recent wallet transactions</CardTitle></CardHeader>
        {player.recent_transactions.length === 0 ? <EmptyState title="No wallet activity" /> : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[620px] text-left text-xs">
              <thead className="bg-muted/30 text-[10px] uppercase text-muted-foreground">
                <tr><th className="px-4 py-2.5">Type</th><th className="px-4 py-2.5">Status</th><th className="px-4 py-2.5">Reference</th><th className="px-4 py-2.5 text-right">Amount</th><th className="px-4 py-2.5">Date</th></tr>
              </thead>
              <tbody className="divide-y divide-border/50">
                {player.recent_transactions.map(tx => (
                  <tr key={tx.id}>
                    <td className="px-4 py-3 font-semibold capitalize">{tx.type.replaceAll('_', ' ')}</td>
                    <td className="px-4 py-3"><StatusBadge status={tx.status} /></td>
                    <td className="px-4 py-3 font-mono text-[10px] text-muted-foreground">{tx.reference || shortId(tx.id)}</td>
                    <td className={cn('px-4 py-3 text-right font-bold tabular-nums', tx.amount >= 0 ? 'text-success' : 'text-destructive')}>{formatMoney(tx.amount)}</td>
                    <td className="px-4 py-3 text-muted-foreground">{formatDate(tx.created_at, true)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </div>
  )
}
