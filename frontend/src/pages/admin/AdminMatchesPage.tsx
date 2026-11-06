import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Search, Swords, X } from 'lucide-react'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { usePageTitle } from '@/hooks/usePageTitle'
import { adminApi, type AdminMatch } from '@/features/admin/api'
import { Avatar, DateRangeBar, EmptyState, ErrorState, FinancialStatusBadge, PageHeader, Pagination, StatusBadge } from '@/features/admin/AdminUi'
import { formatDate, formatDuration, formatMoney, formatNumber, shortId } from '@/features/admin/AdminUtils'

const PAGE_SIZE = 25
const GAME_LABEL: Record<string, string> = {
  'ethiopian-dama': 'Ethiopian Dama',
  'tic-tac-toe': 'Tic-Tac-Toe',
  'rock-paper-scissors': 'Rock-Paper-Scissors',
  'connect-four': 'Connect Four',
}

function MatchDrawer({ match, onClose }: { match: AdminMatch; onClose: () => void }) {
  return (
    <div className="fixed inset-0 z-50 flex justify-end">
      <button className="absolute inset-0 bg-black/55 backdrop-blur-[2px]" aria-label="Close match details" onClick={onClose} />
      <aside className="relative h-full w-full max-w-lg overflow-y-auto border-l border-border bg-card shadow-2xl">
        <div className="sticky top-0 z-10 flex items-center justify-between gap-3 border-b border-border/70 bg-card/95 px-4 py-4 backdrop-blur">
          <div>
            <p className="text-[10px] font-bold uppercase tracking-[0.2em] text-primary">Match detail</p>
            <h2 className="mt-0.5 text-lg font-black">{match.game_title}</h2>
          </div>
          <button onClick={onClose} className="flex h-9 w-9 items-center justify-center rounded-lg border border-border hover:bg-accent" aria-label="Close"><X className="h-4 w-4" /></button>
        </div>
        <div className="space-y-4 p-4">
          <div className="flex flex-wrap gap-2">
            <StatusBadge status={match.status} />
            <FinancialStatusBadge status={match.financial_status} />
            <span className="rounded-full bg-muted px-2.5 py-0.5 text-xs font-semibold capitalize">{match.competition_mode}</span>
          </div>
          <div className="grid grid-cols-2 gap-2">
            {[
              ['Stake', match.competition_mode === 'earn' ? formatMoney(match.stake_amount) : 'Free'],
              ['Prize pool', formatMoney(match.prize_pool)],
              ['Service fee', formatMoney(match.service_fee)],
              ['Duration', formatDuration(match.duration_seconds)],
            ].map(([label, value]) => (
              <div key={label} className="rounded-xl border border-border/60 bg-muted/20 p-3">
                <p className="text-[9px] uppercase tracking-wider text-muted-foreground">{label}</p>
                <p className="mt-1 text-sm font-bold tabular-nums">{value}</p>
              </div>
            ))}
          </div>
          <div>
            <p className="mb-2 text-[10px] font-bold uppercase tracking-wider text-muted-foreground">Players</p>
            <div className="space-y-2">
              {match.players.map(player => (
                <Link key={player.user_id} to={`/admin/users/${player.user_id}`} className="flex items-center gap-3 rounded-xl border border-border/60 p-3 hover:bg-accent">
                  <Avatar name={player.display_name} src={player.avatar_url} />
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-semibold">{player.display_name}</p>
                    <p className="text-[10px] capitalize text-muted-foreground">{player.status} · score {player.score} · player {player.player_index + 1}</p>
                  </div>
                  {player.user_id === match.winner_user_id && <span className="rounded-full bg-success/10 px-2 py-0.5 text-[10px] font-bold text-success">Winner</span>}
                </Link>
              ))}
            </div>
          </div>
          <div className="space-y-2 rounded-xl border border-border/60 p-3 text-xs">
            <div className="flex justify-between gap-3"><span className="text-muted-foreground">Match ID</span><span className="break-all text-right font-mono text-[10px]">{match.id}</span></div>
            <div className="flex justify-between gap-3"><span className="text-muted-foreground">Type</span><span className="capitalize">{match.match_type.replaceAll('_', ' ')}</span></div>
            <div className="flex justify-between gap-3"><span className="text-muted-foreground">Created</span><span>{formatDate(match.created_at, true)}</span></div>
            <div className="flex justify-between gap-3"><span className="text-muted-foreground">Started</span><span>{formatDate(match.started_at, true)}</span></div>
            <div className="flex justify-between gap-3"><span className="text-muted-foreground">Completed</span><span>{formatDate(match.completed_at, true)}</span></div>
            <div className="flex justify-between gap-3"><span className="text-muted-foreground">Winner</span><span className="font-semibold">{match.winner_name || '—'}</span></div>
          </div>
          <p className="text-[10px] leading-relaxed text-muted-foreground">
            Settlement status is derived from the persisted match result and stake mode. Wallet ledger entries are the source of truth for fee and payout amounts.
          </p>
        </div>
      </aside>
    </div>
  )
}

export function Component() {
  usePageTitle('Matches · KWIK Admin')
  const [search, setSearch] = useState('')
  const [query, setQuery] = useState('')
  const [game, setGame] = useState('')
  const [status, setStatus] = useState('')
  const [mode, setMode] = useState('')
  const [sort, setSort] = useState('newest')
  const [range, setRange] = useState({ from: '', to: '' })
  const [offset, setOffset] = useState(0)
  const [selected, setSelected] = useState<AdminMatch | null>(null)
  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['admin', 'matches', query, game, status, mode, sort, range, offset],
    queryFn: () => adminApi.matches({ search: query || undefined, game: game || undefined, status: status || undefined, mode: mode || undefined, sort, from: range.from || undefined, to: range.to || undefined, limit: PAGE_SIZE, offset }),
  })

  return (
    <div className="mx-auto w-full max-w-[1550px] space-y-5 p-3 sm:p-5 lg:p-7">
      <PageHeader
        eyebrow="Competition monitoring"
        title="Matches"
        description="Follow match outcomes, paid settlement, and the players behind every P2P result."
        actions={<span className="rounded-full border border-border bg-card px-3 py-1.5 text-xs font-semibold">{formatNumber(data?.total ?? 0)} matches</span>}
      />
      <Card className="border-border/70 p-3 sm:p-4">
        <form onSubmit={event => { event.preventDefault(); setOffset(0); setQuery(search.trim()) }} className="relative">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input value={search} onChange={event => setSearch(event.target.value)} placeholder="Search by match ID or player name" className="h-10 pl-9" />
        </form>
        <div className="mt-2 flex flex-wrap gap-2">
          <Select value={game} onChange={event => { setGame(event.target.value); setOffset(0) }} className="h-9 w-[170px] text-xs">
            <option value="">All games</option>
            {Object.entries(GAME_LABEL).map(([slug, label]) => <option key={slug} value={slug}>{label}</option>)}
          </Select>
          <Select value={status} onChange={event => { setStatus(event.target.value); setOffset(0) }} className="h-9 w-[145px] text-xs">
            <option value="">All statuses</option>
            {['waiting', 'ready', 'in_progress', 'completed', 'tied', 'forfeited', 'cancelled'].map(item => <option key={item} value={item}>{item.replaceAll('_', ' ')}</option>)}
          </Select>
          <Select value={mode} onChange={event => { setMode(event.target.value); setOffset(0) }} className="h-9 w-[135px] text-xs">
            <option value="">All modes</option>
            <option value="fun">Free play</option>
            <option value="earn">Compete to Earn</option>
          </Select>
          <Select value={sort} onChange={event => { setSort(event.target.value); setOffset(0) }} className="h-9 w-[150px] text-xs">
            <option value="newest">Newest first</option>
            <option value="oldest">Oldest first</option>
            <option value="stake">Highest stake</option>
            <option value="duration">Longest duration</option>
          </Select>
          <DateRangeBar from={range.from} to={range.to} onChange={(from, to) => { setRange({ from, to }); setOffset(0) }} />
          {(query || game || status || mode || sort !== 'newest' || range.from || range.to) && (
            <Button variant="ghost" size="sm" onClick={() => { setSearch(''); setQuery(''); setGame(''); setStatus(''); setMode(''); setSort('newest'); setRange({ from: '', to: '' }); setOffset(0) }}>Clear</Button>
          )}
        </div>
      </Card>

      <Card className="overflow-hidden border-border/70">
        {isError ? <ErrorState message={(error as Error)?.message} onRetry={() => refetch()} /> : (
          <>
            <div className="hidden overflow-x-auto lg:block">
              <table className="w-full min-w-[1050px] text-left text-sm">
                <thead className="border-b border-border/60 bg-muted/30 text-[10px] uppercase tracking-wider text-muted-foreground">
                  <tr>
                    <th className="px-4 py-3 font-semibold">Match</th>
                    <th className="px-4 py-3 font-semibold">Players</th>
                    <th className="px-4 py-3 font-semibold">Mode</th>
                    <th className="px-4 py-3 text-right font-semibold">Stake / pool</th>
                    <th className="px-4 py-3 font-semibold">Result</th>
                    <th className="px-4 py-3 font-semibold">Settlement</th>
                    <th className="px-4 py-3 font-semibold">Started</th>
                    <th className="px-4 py-3 text-right font-semibold">Duration</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border/50">
                  {isLoading && Array.from({ length: 8 }).map((_, index) => <tr key={index}>{Array.from({ length: 8 }).map((__, cell) => <td key={cell} className="px-4 py-3"><Skeleton className="h-4 w-full" /></td>)}</tr>)}
                  {!isLoading && data?.items.length === 0 && <tr><td colSpan={8}><EmptyState title="No matches found" description="Try widening the date range or clearing a filter." /></td></tr>}
                  {data?.items.map(match => (
                    <tr key={match.id} className="cursor-pointer transition-colors hover:bg-muted/25" onClick={() => setSelected(match)}>
                      <td className="px-4 py-3"><p className="font-semibold">{match.game_title}</p><p className="font-mono text-[10px] text-muted-foreground">{shortId(match.id)}</p></td>
                      <td className="px-4 py-3"><p className="max-w-[190px] truncate text-xs font-medium">{match.players.map(player => player.display_name).join(' vs ')}</p><p className="text-[10px] capitalize text-muted-foreground">{match.match_type.replaceAll('_', ' ')}</p></td>
                      <td className="px-4 py-3"><span className="rounded-full bg-muted px-2 py-0.5 text-[10px] font-bold capitalize">{match.competition_mode}</span></td>
                      <td className="px-4 py-3 text-right text-xs"><p className="font-semibold">{match.competition_mode === 'earn' ? formatMoney(match.stake_amount) : 'Free'}</p><p className="text-[10px] text-muted-foreground">{formatMoney(match.prize_pool)} pool</p></td>
                      <td className="px-4 py-3"><StatusBadge status={match.status} />{match.winner_name && <p className="mt-1 max-w-[130px] truncate text-[10px] text-muted-foreground">Winner: {match.winner_name}</p>}</td>
                      <td className="px-4 py-3"><FinancialStatusBadge status={match.financial_status} /><p className="mt-1 text-[10px] text-muted-foreground">{formatMoney(match.service_fee)} fee</p></td>
                      <td className="px-4 py-3 text-xs text-muted-foreground">{formatDate(match.started_at || match.created_at)}</td>
                      <td className="px-4 py-3 text-right text-xs font-medium">{formatDuration(match.duration_seconds)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="divide-y divide-border/60 lg:hidden">
              {isLoading && Array.from({ length: 4 }).map((_, i) => <div key={i} className="p-4"><Skeleton className="h-24 w-full" /></div>)}
              {!isLoading && data?.items.length === 0 && <EmptyState title="No matches found" />}
              {data?.items.map(match => (
                <button key={match.id} onClick={() => setSelected(match)} className="block w-full p-4 text-left transition-colors hover:bg-muted/30">
                  <div className="flex items-start justify-between gap-2">
                    <div><p className="text-sm font-bold">{match.game_title}</p><p className="font-mono text-[10px] text-muted-foreground">{shortId(match.id)} · {formatDate(match.started_at || match.created_at)}</p></div>
                    <StatusBadge status={match.status} />
                  </div>
                  <p className="mt-2 truncate text-xs">{match.players.map(player => player.display_name).join(' vs ')}</p>
                  <div className="mt-2 flex flex-wrap items-center gap-2 text-[10px] text-muted-foreground">
                    <span className="font-bold capitalize">{match.competition_mode}</span><span>·</span>
                    <span>{match.competition_mode === 'earn' ? formatMoney(match.stake_amount) : 'Free'}</span><span>·</span>
                    <span>{formatMoney(match.prize_pool)} pool</span><span>·</span>
                    <span>{formatDuration(match.duration_seconds)}</span>
                  </div>
                </button>
              ))}
            </div>
            <Pagination total={data?.total ?? 0} limit={PAGE_SIZE} offset={offset} onChange={setOffset} loading={isLoading} />
          </>
        )}
      </Card>
      <p className="flex items-center gap-1.5 text-[10px] text-muted-foreground"><Swords className="h-3 w-3" /> Select a match for the full player and settlement view.</p>
      {selected && <MatchDrawer match={selected} onClose={() => setSelected(null)} />}
    </div>
  )
}
