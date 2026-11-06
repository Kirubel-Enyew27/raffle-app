import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Search, Users, UserCheck, Ban, Gamepad2 } from 'lucide-react'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { usePageTitle } from '@/hooks/usePageTitle'
import { adminApi } from '@/features/admin/api'
import { Avatar, EmptyState, ErrorState, PageHeader, Pagination, StatusBadge } from '@/features/admin/AdminUi'
import { formatDate, formatMoney, formatNumber, relativeTime } from '@/features/admin/AdminUtils'
import { cn } from '@/lib/utils'

const PAGE_SIZE = 25

export function Component() {
  usePageTitle('Users · KWIK Admin')
  const [search, setSearch] = useState('')
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('')
  const [sort, setSort] = useState('newest')
  const [offset, setOffset] = useState(0)

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['admin', 'users', query, status, sort, offset],
    queryFn: () => adminApi.users({ search: query || undefined, status: status || undefined, sort, limit: PAGE_SIZE, offset }),
  })

  const submitSearch = (event: React.FormEvent) => {
    event.preventDefault()
    setOffset(0)
    setQuery(search.trim())
  }

  return (
    <div className="mx-auto w-full max-w-[1500px] space-y-5 p-3 sm:p-5 lg:p-7">
      <PageHeader
        eyebrow="Player operations"
        title="Users"
        description="Search registered players and inspect their wallet, match record, and recent activity."
        actions={<span className="rounded-full border border-border bg-card px-3 py-1.5 text-xs font-semibold">{formatNumber(data?.total ?? 0)} players</span>}
      />

      <Card className="border-border/70 p-3 sm:p-4">
        <div className="flex flex-col gap-2 lg:flex-row lg:items-center">
          <form onSubmit={submitSearch} className="relative min-w-0 flex-1">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={search}
              onChange={event => setSearch(event.target.value)}
              placeholder="Search by name, email, phone, or user ID"
              className="h-10 pl-9"
            />
          </form>
          <div className="flex flex-wrap gap-2">
            <Select value={status} onChange={event => { setStatus(event.target.value); setOffset(0) }} className="h-10 w-[150px]">
              <option value="">All accounts</option>
              <option value="active">Active</option>
              <option value="playing">Playing now</option>
              <option value="banned">Banned</option>
              <option value="admin">Administrators</option>
            </Select>
            <Select value={sort} onChange={event => { setSort(event.target.value); setOffset(0) }} className="h-10 w-[165px]">
              <option value="newest">Newest first</option>
              <option value="oldest">Oldest first</option>
              <option value="matches">Most matches</option>
              <option value="wins">Most wins</option>
              <option value="balance">Highest balance</option>
            </Select>
          </div>
        </div>
        <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-[10px] text-muted-foreground">
          <span className="inline-flex items-center gap-1"><Users className="h-3 w-3" /> Registered</span>
          <span className="inline-flex items-center gap-1"><UserCheck className="h-3 w-3 text-success" /> Active account</span>
          <span className="inline-flex items-center gap-1"><Gamepad2 className="h-3 w-3 text-primary" /> Playing now</span>
          <span className="inline-flex items-center gap-1"><Ban className="h-3 w-3 text-destructive" /> Banned</span>
        </div>
      </Card>

      <Card className="overflow-hidden border-border/70">
        {isError ? <ErrorState message={(error as Error)?.message} onRetry={() => refetch()} /> : (
          <>
            <div className="hidden overflow-x-auto md:block">
              <table className="w-full min-w-[900px] text-left text-sm">
                <thead className="border-b border-border/60 bg-muted/30 text-[10px] uppercase tracking-wider text-muted-foreground">
                  <tr>
                    <th className="px-4 py-3 font-semibold">Player</th>
                    <th className="px-4 py-3 font-semibold">Status</th>
                    <th className="px-4 py-3 text-right font-semibold">Wallet</th>
                    <th className="px-4 py-3 text-right font-semibold">Record</th>
                    <th className="px-4 py-3 text-right font-semibold">Win rate</th>
                    <th className="px-4 py-3 font-semibold">Registered</th>
                    <th className="px-4 py-3 font-semibold">Last active</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border/50">
                  {isLoading && Array.from({ length: 8 }).map((_, index) => (
                    <tr key={index}>{Array.from({ length: 7 }).map((__, cell) => <td key={cell} className="px-4 py-3"><Skeleton className="h-4 w-full" /></td>)}</tr>
                  ))}
                  {!isLoading && data?.items.length === 0 && (
                    <tr><td colSpan={7}><EmptyState title="No users match this search" description="Try a different name, account status, or sort order." /></td></tr>
                  )}
                  {data?.items.map(user => (
                    <tr key={user.id} className="transition-colors hover:bg-muted/25">
                      <td className="px-4 py-3">
                        <Link to={`/admin/users/${user.id}`} className="flex items-center gap-3">
                          <Avatar name={user.full_name || user.email} src={user.avatar_url} />
                          <span className="min-w-0">
                            <span className="block truncate font-semibold hover:text-primary">{user.full_name || 'Unnamed player'}</span>
                            <span className="block truncate text-xs text-muted-foreground">{user.email}</span>
                          </span>
                        </Link>
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex flex-wrap gap-1.5">
                          {user.is_banned ? <StatusBadge status="banned" /> : <StatusBadge status="active" />}
                          {user.role === 'admin' && <span className="rounded-full bg-info/10 px-2 py-0.5 text-[10px] font-bold text-info">Admin</span>}
                          {user.is_playing && <span className="rounded-full bg-primary/10 px-2 py-0.5 text-[10px] font-bold text-primary">Playing</span>}
                        </div>
                      </td>
                      <td className="px-4 py-3 text-right">
                        <p className="font-semibold tabular-nums">{formatMoney(user.wallet_balance)}</p>
                        <p className="text-[10px] text-muted-foreground">{formatMoney(user.reserved_balance)} reserved</p>
                      </td>
                      <td className="px-4 py-3 text-right">
                        <p className="text-xs font-semibold"><span className="text-success">{user.wins}W</span> <span className="text-destructive">{user.losses}L</span></p>
                        <p className="text-[10px] text-muted-foreground">{user.matches_played} played</p>
                      </td>
                      <td className="px-4 py-3 text-right">
                        <div className="ml-auto w-16">
                          <p className="text-xs font-bold">{user.win_rate.toFixed(1)}%</p>
                          <div className="mt-1 h-1 overflow-hidden rounded-full bg-muted"><div className="h-full bg-success" style={{ width: `${Math.min(100, user.win_rate)}%` }} /></div>
                        </div>
                      </td>
                      <td className="px-4 py-3 text-xs text-muted-foreground">{formatDate(user.created_at)}</td>
                      <td className="px-4 py-3 text-xs text-muted-foreground">{relativeTime(user.last_active_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            <div className="divide-y divide-border/60 md:hidden">
              {isLoading && Array.from({ length: 4 }).map((_, index) => <div key={index} className="p-4"><Skeleton className="h-20 w-full" /></div>)}
              {!isLoading && data?.items.length === 0 && <EmptyState title="No users match this search" />}
              {data?.items.map(user => (
                <Link key={user.id} to={`/admin/users/${user.id}`} className="block p-4 transition-colors hover:bg-muted/30">
                  <div className="flex items-start gap-3">
                    <Avatar name={user.full_name || user.email} src={user.avatar_url} />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-start justify-between gap-2">
                        <div className="min-w-0">
                          <p className="truncate text-sm font-semibold">{user.full_name || 'Unnamed player'}</p>
                          <p className="truncate text-xs text-muted-foreground">{user.email}</p>
                        </div>
                        {user.is_banned ? <StatusBadge status="banned" /> : user.is_playing ? <StatusBadge status="in_progress" /> : <StatusBadge status="active" />}
                      </div>
                      <div className="mt-2 grid grid-cols-3 gap-2 rounded-lg bg-muted/40 p-2 text-center">
                        <div><p className="text-[9px] uppercase text-muted-foreground">Balance</p><p className="truncate text-xs font-bold">{formatMoney(user.wallet_balance)}</p></div>
                        <div><p className="text-[9px] uppercase text-muted-foreground">Matches</p><p className="text-xs font-bold">{user.matches_played}</p></div>
                        <div><p className="text-[9px] uppercase text-muted-foreground">Win rate</p><p className="text-xs font-bold">{user.win_rate.toFixed(0)}%</p></div>
                      </div>
                      <p className="mt-2 text-[10px] text-muted-foreground">Joined {formatDate(user.created_at)} · Active {relativeTime(user.last_active_at)}</p>
                    </div>
                  </div>
                </Link>
              ))}
            </div>

            <Pagination total={data?.total ?? 0} limit={PAGE_SIZE} offset={offset} onChange={setOffset} loading={isLoading} />
          </>
        )}
      </Card>
      <p className={cn('text-[10px] text-muted-foreground', !data && 'opacity-0')}>
        “Playing now” is derived from active P2P match participation, not a browser-presence signal.
      </p>
    </div>
  )
}
