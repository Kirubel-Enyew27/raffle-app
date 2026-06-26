import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ShieldCheck, DollarSign } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Select } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { usePageTitle } from '@/hooks/usePageTitle'
import { formatCurrency } from '@/lib/utils'
import { adminApi, type WinnerDetail } from '@/features/admin/api'
import { WinnerVerificationModal } from '@/features/admin/WinnerVerificationModal'
import { MarkPaidModal } from '@/features/admin/MarkPaidModal'

const PAGE_SIZE = 20

const usd = (n: number) => formatCurrency(n)

type Modal =
  | { type: 'verify'; winner: WinnerDetail }
  | { type: 'pay'; winner: WinnerDetail }
  | null

export function Component() {
  usePageTitle('Admin Winners')
  const [offset, setOffset] = useState(0)
  const [paidFilter, setPaidFilter] = useState<'' | 'true' | 'false'>('')
  const [modal, setModal] = useState<Modal>(null)

  const paid = paidFilter === '' ? undefined : paidFilter === 'true'

  const { data, isLoading } = useQuery({
    queryKey: ['admin-winners', offset, paidFilter],
    queryFn: () => adminApi.winners(PAGE_SIZE, offset, paid),
  })

  const winners = data?.winners ?? []
  const total = data?.total ?? 0
  const pages = Math.ceil(total / PAGE_SIZE)
  const page = Math.floor(offset / PAGE_SIZE) + 1

  return (
    <div className="p-6 space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Winners</h1>
          <p className="text-sm text-muted-foreground">{total} total</p>
        </div>
      </div>

      {/* Filter */}
      <div className="flex gap-2">
        <Select
          value={paidFilter}
          onChange={e => { setPaidFilter(e.target.value as typeof paidFilter); setOffset(0) }}
          className="w-44"
        >
          <option value="">All winners</option>
          <option value="false">Unpaid only</option>
          <option value="true">Paid only</option>
        </Select>
      </div>

      {/* Table */}
      <div className="rounded-xl border bg-card overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/40">
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">Winner</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">Raffle</th>
              <th className="px-4 py-3 text-right font-medium text-muted-foreground">Ticket #</th>
              <th className="px-4 py-3 text-right font-medium text-muted-foreground">Prize</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">Status</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">Payment Ref</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">Won</th>
              <th className="px-4 py-3 text-right font-medium text-muted-foreground">Actions</th>
            </tr>
          </thead>
          <tbody>
            {isLoading
              ? Array.from({ length: 8 }).map((_, i) => (
                  <tr key={i} className="border-b">
                    {Array.from({ length: 8 }).map((_, j) => (
                      <td key={j} className="px-4 py-3"><Skeleton className="h-4 w-full" /></td>
                    ))}
                  </tr>
                ))
              : winners.length === 0
              ? (
                <tr>
                  <td colSpan={8} className="px-4 py-12 text-center text-muted-foreground">
                    No winners found.
                  </td>
                </tr>
              )
              : winners.map(w => (
                  <tr key={w.id} className="border-b last:border-0 hover:bg-muted/20">
                    <td className="px-4 py-3">
                      <p className="font-medium">{w.user_email || '—'}</p>
                      <p className="font-mono text-xs text-muted-foreground">{w.user_id.slice(0, 8)}…</p>
                    </td>
                    <td className="px-4 py-3 max-w-[180px]">
                      <p className="truncate">{w.raffle_title || '—'}</p>
                    </td>
                    <td className="px-4 py-3 text-right font-mono">{w.ticket_number}</td>
                    <td className="px-4 py-3 text-right font-semibold">{usd(w.prize_amount)}</td>
                    <td className="px-4 py-3">
                      <Badge variant={w.prize_paid ? 'success' : 'warning'}>
                        {w.prize_paid ? 'Paid' : 'Unpaid'}
                      </Badge>
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">
                      {w.payment_reference
                        ? <span className="font-mono text-xs">{w.payment_reference}</span>
                        : '—'}
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">
                      {new Date(w.created_at).toLocaleDateString()}
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center justify-end gap-1">
                        <Button
                          variant="ghost" size="icon"
                          title="Verification details"
                          onClick={() => setModal({ type: 'verify', winner: w })}
                        >
                          <ShieldCheck className="h-4 w-4" />
                        </Button>
                        {!w.prize_paid && (
                          <Button
                            variant="ghost" size="icon"
                            title="Mark as paid"
                            className="text-green-600 hover:text-green-700"
                            onClick={() => setModal({ type: 'pay', winner: w })}
                          >
                            <DollarSign className="h-4 w-4" />
                          </Button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {pages > 1 && (
        <div className="flex items-center justify-between text-sm">
          <span className="text-muted-foreground">Page {page} of {pages}</span>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" disabled={offset === 0} onClick={() => setOffset(o => o - PAGE_SIZE)}>
              Previous
            </Button>
            <Button variant="outline" size="sm" disabled={offset + PAGE_SIZE >= total} onClick={() => setOffset(o => o + PAGE_SIZE)}>
              Next
            </Button>
          </div>
        </div>
      )}

      {modal?.type === 'verify' && (
        <WinnerVerificationModal winner={modal.winner} onClose={() => setModal(null)} />
      )}
      {modal?.type === 'pay' && (
        <MarkPaidModal winner={modal.winner} onClose={() => setModal(null)} />
      )}
    </div>
  )
}
