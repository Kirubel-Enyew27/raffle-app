import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ExternalLink, Search, WalletCards, X } from 'lucide-react'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { usePageTitle } from '@/hooks/usePageTitle'
import { adminApi, type AdminDeposit } from '@/features/admin/api'
import { Avatar, DateRangeBar, EmptyState, ErrorState, PageHeader, Pagination, StatusBadge } from '@/features/admin/AdminUi'
import { formatDate, formatMoney, formatNumber, shortId } from '@/features/admin/AdminUtils'

const PAGE_SIZE = 25

function DepositDrawer({ deposit, onClose }: { deposit: AdminDeposit; onClose: () => void }) {
  return (
    <div className="fixed inset-0 z-50 flex justify-end">
      <button className="absolute inset-0 bg-black/55 backdrop-blur-[2px]" aria-label="Close deposit details" onClick={onClose} />
      <aside className="relative h-full w-full max-w-md overflow-y-auto border-l border-border bg-card shadow-2xl">
        <div className="sticky top-0 flex items-center justify-between border-b border-border/70 bg-card/95 px-4 py-4 backdrop-blur">
          <div><p className="text-[10px] font-bold uppercase tracking-[0.2em] text-primary">Deposit detail</p><h2 className="text-lg font-black">{formatMoney(deposit.amount, deposit.currency)}</h2></div>
          <button onClick={onClose} className="flex h-9 w-9 items-center justify-center rounded-lg border border-border hover:bg-accent" aria-label="Close"><X className="h-4 w-4" /></button>
        </div>
        <div className="space-y-4 p-4">
          <StatusBadge status={deposit.status} />
          <div className="rounded-xl border border-border/60 p-3">
            <div className="flex items-center gap-2.5">
              <Avatar name={deposit.user_name || deposit.user_email} />
              <div className="min-w-0"><p className="truncate text-sm font-semibold">{deposit.user_name || 'Unnamed player'}</p><p className="truncate text-[10px] text-muted-foreground">{deposit.user_email}</p></div>
            </div>
          </div>
          <div className="space-y-2 rounded-xl border border-border/60 p-3 text-xs">
            <div className="flex justify-between gap-3"><span className="text-muted-foreground">Transaction</span><span className="break-all text-right font-mono text-[10px]">{deposit.id}</span></div>
            <div className="flex justify-between gap-3"><span className="text-muted-foreground">Payment method</span><span className="font-semibold capitalize">{deposit.payment_method.replaceAll('_', ' ')}</span></div>
            <div className="flex justify-between gap-3"><span className="text-muted-foreground">Reference</span><span className="break-all text-right font-mono text-[10px]">{deposit.external_reference || '—'}</span></div>
            <div className="flex justify-between gap-3"><span className="text-muted-foreground">Created</span><span>{formatDate(deposit.created_at, true)}</span></div>
            <div className="flex justify-between gap-3"><span className="text-muted-foreground">Credited</span><span>{formatDate(deposit.credited_at, true)}</span></div>
          </div>
          {deposit.receipt_url && (
            <a href={deposit.receipt_url} target="_blank" rel="noreferrer" className="flex items-center justify-center gap-1.5 rounded-lg border border-border px-3 py-2 text-xs font-semibold hover:bg-accent">
              Open payment receipt <ExternalLink className="h-3.5 w-3.5" />
            </a>
          )}
          <p className="text-[10px] leading-relaxed text-muted-foreground">The persisted deposit record is the source of truth. No status is inferred or changed from this screen.</p>
        </div>
      </aside>
    </div>
  )
}

export function Component() {
  usePageTitle('Deposits · KWIK Admin')
  const [search, setSearch] = useState('')
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('')
  const [range, setRange] = useState({ from: '', to: '' })
  const [offset, setOffset] = useState(0)
  const [selected, setSelected] = useState<AdminDeposit | null>(null)
  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['admin', 'deposits', query, status, range, offset],
    queryFn: () => adminApi.deposits({ search: query || undefined, status: status || undefined, from: range.from || undefined, to: range.to || undefined, limit: PAGE_SIZE, offset }),
  })

  return (
    <div className="mx-auto w-full max-w-[1450px] space-y-5 p-3 sm:p-5 lg:p-7">
      <PageHeader
        eyebrow="Money in"
        title="Deposits"
        description="Monitor incoming wallet funding, payment references, and processing status from persisted deposit records."
        actions={<span className="rounded-full border border-border bg-card px-3 py-1.5 text-xs font-semibold">{formatNumber(data?.total ?? 0)} deposits</span>}
      />
      <Card className="border-border/70 p-3 sm:p-4">
        <form onSubmit={event => { event.preventDefault(); setOffset(0); setQuery(search.trim()) }} className="relative">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input value={search} onChange={event => setSearch(event.target.value)} placeholder="Search by transaction ID, reference, name, or email" className="h-10 pl-9" />
        </form>
        <div className="mt-2 flex flex-wrap gap-2">
          <Select value={status} onChange={event => { setStatus(event.target.value); setOffset(0) }} className="h-9 w-[155px] text-xs">
            <option value="">All statuses</option>
            {['pending', 'credited', 'completed', 'failed', 'rejected', 'verifying', 'manual_review', 'expired'].map(item => <option key={item} value={item}>{item.replaceAll('_', ' ')}</option>)}
          </Select>
          <DateRangeBar from={range.from} to={range.to} onChange={(from, to) => { setRange({ from, to }); setOffset(0) }} />
          {(query || status || range.from || range.to) && <Button variant="ghost" size="sm" onClick={() => { setSearch(''); setQuery(''); setStatus(''); setRange({ from: '', to: '' }); setOffset(0) }}>Clear</Button>}
        </div>
      </Card>

      <Card className="overflow-hidden border-border/70">
        {isError ? <ErrorState message={(error as Error)?.message} onRetry={() => refetch()} /> : (
          <>
            <div className="hidden overflow-x-auto md:block">
              <table className="w-full min-w-[900px] text-left text-sm">
                <thead className="border-b border-border/60 bg-muted/30 text-[10px] uppercase tracking-wider text-muted-foreground">
                  <tr><th className="px-4 py-3 font-semibold">Reference</th><th className="px-4 py-3 font-semibold">Player</th><th className="px-4 py-3 text-right font-semibold">Amount</th><th className="px-4 py-3 font-semibold">Method</th><th className="px-4 py-3 font-semibold">Status</th><th className="px-4 py-3 font-semibold">Created</th><th className="px-4 py-3 font-semibold">Credited</th></tr>
                </thead>
                <tbody className="divide-y divide-border/50">
                  {isLoading && Array.from({ length: 8 }).map((_, index) => <tr key={index}>{Array.from({ length: 7 }).map((__, cell) => <td key={cell} className="px-4 py-3"><Skeleton className="h-4 w-full" /></td>)}</tr>)}
                  {!isLoading && data?.items.length === 0 && <tr><td colSpan={7}><EmptyState title="No deposits found" description="Try a different status, date range, or search term." /></td></tr>}
                  {data?.items.map(deposit => (
                    <tr key={deposit.id} className="cursor-pointer transition-colors hover:bg-muted/25" onClick={() => setSelected(deposit)}>
                      <td className="px-4 py-3"><p className="font-mono text-[10px] text-muted-foreground">{shortId(deposit.id)}</p><p className="text-[10px] text-muted-foreground">{deposit.external_reference || 'No external ref'}</p></td>
                      <td className="px-4 py-3"><p className="max-w-[190px] truncate text-xs font-semibold">{deposit.user_name || 'Unnamed player'}</p><p className="max-w-[190px] truncate text-[10px] text-muted-foreground">{deposit.user_email}</p></td>
                      <td className="px-4 py-3 text-right font-bold tabular-nums">{formatMoney(deposit.amount, deposit.currency)}</td>
                      <td className="px-4 py-3 text-xs capitalize text-muted-foreground">{deposit.payment_method.replaceAll('_', ' ')}</td>
                      <td className="px-4 py-3"><StatusBadge status={deposit.status} /></td>
                      <td className="px-4 py-3 text-xs text-muted-foreground">{formatDate(deposit.created_at)}</td>
                      <td className="px-4 py-3 text-xs text-muted-foreground">{formatDate(deposit.credited_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="divide-y divide-border/60 md:hidden">
              {isLoading && Array.from({ length: 4 }).map((_, i) => <div key={i} className="p-4"><Skeleton className="h-20 w-full" /></div>)}
              {!isLoading && data?.items.length === 0 && <EmptyState title="No deposits found" />}
              {data?.items.map(deposit => (
                <button key={deposit.id} onClick={() => setSelected(deposit)} className="block w-full p-4 text-left hover:bg-muted/30">
                  <div className="flex items-start justify-between gap-2"><div><p className="text-sm font-bold">{formatMoney(deposit.amount, deposit.currency)}</p><p className="text-[10px] text-muted-foreground">{deposit.user_name || deposit.user_email}</p></div><StatusBadge status={deposit.status} /></div>
                  <p className="mt-2 text-[10px] text-muted-foreground capitalize">{deposit.payment_method.replaceAll('_', ' ')} · {formatDate(deposit.created_at)}</p>
                </button>
              ))}
            </div>
            <Pagination total={data?.total ?? 0} limit={PAGE_SIZE} offset={offset} onChange={setOffset} loading={isLoading} />
          </>
        )}
      </Card>
      <p className="flex items-center gap-1.5 text-[10px] text-muted-foreground"><WalletCards className="h-3 w-3" /> Select a row to inspect its payment reference and credited timestamp.</p>
      {selected && <DepositDrawer deposit={selected} onClose={() => setSelected(null)} />}
    </div>
  )
}
