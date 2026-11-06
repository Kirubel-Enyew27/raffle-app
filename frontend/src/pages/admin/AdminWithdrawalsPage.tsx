import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CheckCircle, Search, X, XCircle } from 'lucide-react'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { usePageTitle } from '@/hooks/usePageTitle'
import { adminApi, type AdminWithdrawal } from '@/features/admin/api'
import { Avatar, DateRangeBar, EmptyState, ErrorState, PageHeader, Pagination, StatusBadge } from '@/features/admin/AdminUi'
import { formatDate, formatMoney, formatNumber, shortId } from '@/features/admin/AdminUtils'

const PAGE_SIZE = 25

function WithdrawalDrawer({
  withdrawal,
  onClose,
  onApprove,
  onReject,
  busy,
}: {
  withdrawal: AdminWithdrawal
  onClose: () => void
  onApprove: () => void
  onReject: () => void
  busy: boolean
}) {
  const pending = withdrawal.status === 'pending'
  return (
    <div className="fixed inset-0 z-50 flex justify-end">
      <button className="absolute inset-0 bg-black/55 backdrop-blur-[2px]" aria-label="Close withdrawal details" onClick={onClose} />
      <aside className="relative h-full w-full max-w-md overflow-y-auto border-l border-border bg-card shadow-2xl">
        <div className="sticky top-0 flex items-center justify-between border-b border-border/70 bg-card/95 px-4 py-4 backdrop-blur">
          <div><p className="text-[10px] font-bold uppercase tracking-[0.2em] text-primary">Withdrawal detail</p><h2 className="text-lg font-black">{formatMoney(withdrawal.amount, withdrawal.currency)}</h2></div>
          <button onClick={onClose} className="flex h-9 w-9 items-center justify-center rounded-lg border border-border hover:bg-accent" aria-label="Close"><X className="h-4 w-4" /></button>
        </div>
        <div className="space-y-4 p-4">
          <StatusBadge status={withdrawal.status} />
          <div className="flex items-center gap-2.5 rounded-xl border border-border/60 p-3"><Avatar name={withdrawal.user_name || withdrawal.user_email} /><div className="min-w-0"><p className="truncate text-sm font-semibold">{withdrawal.user_name || 'Unnamed player'}</p><p className="truncate text-[10px] text-muted-foreground">{withdrawal.user_email}</p></div></div>
          <div className="space-y-2 rounded-xl border border-border/60 p-3 text-xs">
            <div className="flex justify-between gap-3"><span className="text-muted-foreground">Transaction</span><span className="break-all text-right font-mono text-[10px]">{withdrawal.id}</span></div>
            <div className="flex justify-between gap-3"><span className="text-muted-foreground">Reference</span><span className="break-all text-right font-mono text-[10px]">{withdrawal.reference || '—'}</span></div>
            <div className="flex justify-between gap-3"><span className="text-muted-foreground">Description</span><span className="max-w-[220px] text-right">{withdrawal.description || '—'}</span></div>
            <div className="flex justify-between gap-3"><span className="text-muted-foreground">Requested</span><span>{formatDate(withdrawal.created_at, true)}</span></div>
            <div className="flex justify-between gap-3"><span className="text-muted-foreground">Updated</span><span>{formatDate(withdrawal.updated_at, true)}</span></div>
          </div>
          {pending && (
            <div className="rounded-xl border border-warning/30 bg-warning/5 p-3">
              <p className="text-xs font-semibold">Financial action</p>
              <p className="mt-0.5 text-[10px] text-muted-foreground">Approval atomically debits the wallet and marks the request completed. Rejection only closes the request: pending withdrawals do not reserve or hold funds.</p>
              <div className="mt-3 flex gap-2">
                <Button size="sm" className="flex-1 gap-1" disabled={busy} onClick={onApprove}><CheckCircle className="h-3.5 w-3.5" /> Approve</Button>
                <Button size="sm" variant="outline" className="flex-1 gap-1 text-destructive hover:text-destructive" disabled={busy} onClick={onReject}><XCircle className="h-3.5 w-3.5" /> Reject</Button>
              </div>
            </div>
          )}
        </div>
      </aside>
    </div>
  )
}

export function Component() {
  usePageTitle('Withdrawals · KWIK Admin')
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('')
  const [range, setRange] = useState({ from: '', to: '' })
  const [offset, setOffset] = useState(0)
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['admin', 'withdrawals', query, status, range, offset],
    queryFn: () => adminApi.withdrawals({ search: query || undefined, status: status || undefined, from: range.from || undefined, to: range.to || undefined, limit: PAGE_SIZE, offset }),
  })
  const selected = data?.items.find(item => item.id === selectedId) ?? null

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: ['admin', 'withdrawals'] })
    queryClient.invalidateQueries({ queryKey: ['admin-pending-withdrawals'] })
    queryClient.invalidateQueries({ queryKey: ['admin', 'overview'] })
  }
  const approve = useMutation({
    mutationFn: (id: string) => adminApi.approveWithdrawal(id),
    onSuccess: () => { refresh(); setNotice('Withdrawal approved.'); setSelectedId(null) },
    onError: () => setNotice('Could not approve this withdrawal.'),
  })
  const reject = useMutation({
    mutationFn: (id: string) => adminApi.rejectWithdrawal(id),
    onSuccess: () => { refresh(); setNotice('Withdrawal rejected.'); setSelectedId(null) },
    onError: () => setNotice('Could not reject this withdrawal.'),
  })
  const busy = approve.isPending || reject.isPending

  return (
    <div className="mx-auto w-full max-w-[1450px] space-y-5 p-3 sm:p-5 lg:p-7">
      <PageHeader
        eyebrow="Money out"
        title="Withdrawals"
        description="Review the full withdrawal history and act on requests that still need an authorized decision."
        actions={<span className="rounded-full border border-border bg-card px-3 py-1.5 text-xs font-semibold">{formatNumber(data?.total ?? 0)} requests</span>}
      />
      {notice && (
        <div className="flex items-center justify-between gap-3 rounded-xl border border-border bg-card px-4 py-3 text-xs font-medium">
          {notice}
          <button onClick={() => setNotice(null)} className="text-muted-foreground hover:text-foreground"><X className="h-4 w-4" /></button>
        </div>
      )}
      <Card className="border-border/70 p-3 sm:p-4">
        <form onSubmit={event => { event.preventDefault(); setOffset(0); setQuery(search.trim()) }} className="relative">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input value={search} onChange={event => setSearch(event.target.value)} placeholder="Search by transaction ID, reference, name, or email" className="h-10 pl-9" />
        </form>
        <div className="mt-2 flex flex-wrap gap-2">
          <Select value={status} onChange={event => { setStatus(event.target.value); setOffset(0) }} className="h-9 w-[155px] text-xs">
            <option value="">All statuses</option>
            {['pending', 'completed', 'failed', 'rejected', 'cancelled'].map(item => <option key={item} value={item}>{item}</option>)}
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
                  <tr><th className="px-4 py-3 font-semibold">Reference</th><th className="px-4 py-3 font-semibold">Player</th><th className="px-4 py-3 text-right font-semibold">Amount</th><th className="px-4 py-3 font-semibold">Status</th><th className="px-4 py-3 font-semibold">Requested</th><th className="px-4 py-3 font-semibold">Updated</th><th className="px-4 py-3 text-right font-semibold">Action</th></tr>
                </thead>
                <tbody className="divide-y divide-border/50">
                  {isLoading && Array.from({ length: 8 }).map((_, index) => <tr key={index}>{Array.from({ length: 7 }).map((__, cell) => <td key={cell} className="px-4 py-3"><Skeleton className="h-4 w-full" /></td>)}</tr>)}
                  {!isLoading && data?.items.length === 0 && <tr><td colSpan={7}><EmptyState title="No withdrawals found" description="Try a different status, date range, or search term." /></td></tr>}
                  {data?.items.map(withdrawal => (
                    <tr key={withdrawal.id} className="cursor-pointer transition-colors hover:bg-muted/25" onClick={() => setSelectedId(withdrawal.id)}>
                      <td className="px-4 py-3"><p className="font-mono text-[10px] text-muted-foreground">{shortId(withdrawal.id)}</p><p className="text-[10px] text-muted-foreground">{withdrawal.reference || '—'}</p></td>
                      <td className="px-4 py-3"><p className="max-w-[190px] truncate text-xs font-semibold">{withdrawal.user_name || 'Unnamed player'}</p><p className="max-w-[190px] truncate text-[10px] text-muted-foreground">{withdrawal.user_email}</p></td>
                      <td className="px-4 py-3 text-right font-bold tabular-nums">{formatMoney(withdrawal.amount, withdrawal.currency)}</td>
                      <td className="px-4 py-3"><StatusBadge status={withdrawal.status} /></td>
                      <td className="px-4 py-3 text-xs text-muted-foreground">{formatDate(withdrawal.created_at)}</td>
                      <td className="px-4 py-3 text-xs text-muted-foreground">{formatDate(withdrawal.updated_at)}</td>
                      <td className="px-4 py-3 text-right">
                        {withdrawal.status === 'pending' ? (
                          <div className="flex justify-end gap-1.5" onClick={event => event.stopPropagation()}>
                            <Button size="sm" className="h-7 gap-1 px-2" disabled={busy} onClick={() => approve.mutate(withdrawal.id)}><CheckCircle className="h-3 w-3" /> Approve</Button>
                            <Button size="sm" variant="outline" className="h-7 gap-1 px-2 text-destructive hover:text-destructive" disabled={busy} onClick={() => reject.mutate(withdrawal.id)}><XCircle className="h-3 w-3" /> Reject</Button>
                          </div>
                        ) : <span className="text-[10px] text-muted-foreground">Processed</span>}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="divide-y divide-border/60 md:hidden">
              {isLoading && Array.from({ length: 4 }).map((_, i) => <div key={i} className="p-4"><Skeleton className="h-20 w-full" /></div>)}
              {!isLoading && data?.items.length === 0 && <EmptyState title="No withdrawals found" />}
              {data?.items.map(withdrawal => (
                <button key={withdrawal.id} onClick={() => setSelectedId(withdrawal.id)} className="block w-full p-4 text-left hover:bg-muted/30">
                  <div className="flex items-start justify-between gap-2"><div><p className="text-sm font-bold">{formatMoney(withdrawal.amount, withdrawal.currency)}</p><p className="text-[10px] text-muted-foreground">{withdrawal.user_name || withdrawal.user_email}</p></div><StatusBadge status={withdrawal.status} /></div>
                  <p className="mt-2 text-[10px] text-muted-foreground">{formatDate(withdrawal.created_at, true)}</p>
                </button>
              ))}
            </div>
            <Pagination total={data?.total ?? 0} limit={PAGE_SIZE} offset={offset} onChange={setOffset} loading={isLoading} />
          </>
        )}
      </Card>
      <p className="text-[10px] text-muted-foreground">Approval checks available funds at the moment an administrator acts. Pending requests are not reserved, so the wallet balance is the final authority.</p>
      {selected && (
        <WithdrawalDrawer
          withdrawal={selected}
          onClose={() => setSelectedId(null)}
          onApprove={() => approve.mutate(selected.id)}
          onReject={() => reject.mutate(selected.id)}
          busy={busy}
        />
      )}
    </div>
  )
}
