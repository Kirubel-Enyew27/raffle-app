import { useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { X } from 'lucide-react'
import { Skeleton } from '@/components/ui/skeleton'
import { adminApi } from '@/features/admin/api'
import type { Raffle } from '@/types/api'

interface Props {
  raffle: Raffle
  onClose: () => void
}

import { formatCurrency } from '@/lib/utils'
const usd = (n: number) => formatCurrency(n)

export function RaffleTicketSalesPanel({ raffle, onClose }: Props) {
  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  const { data, isLoading } = useQuery({
    queryKey: ['raffle-tickets', raffle.id],
    queryFn: () => adminApi.listRaffleTickets(raffle.id),
  })

  const revenue = data ? data.tickets.length * raffle.ticket_price : 0
  const soldPct = raffle.total_tickets > 0
    ? Math.round((raffle.sold_tickets / raffle.total_tickets) * 100)
    : 0

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose} role="dialog" aria-modal="true" aria-labelledby="modal-title">
      <div className="flex w-full max-w-2xl flex-col rounded-xl bg-card shadow-xl" onClick={e => e.stopPropagation()}>
        <div className="flex items-center justify-between border-b px-6 py-4">
          <div>
            <h2 id="modal-title" className="text-lg font-semibold">Ticket Sales</h2>
            <p className="text-sm text-muted-foreground">{raffle.title}</p>
          </div>
          <button onClick={onClose} aria-label="Close"><X className="h-5 w-5 text-muted-foreground" /></button>
        </div>

        <div className="overflow-auto p-6" style={{ maxHeight: '60vh' }}>
          {/* Summary stats */}
          <div className="mb-6 grid grid-cols-3 gap-4">
            <div className="rounded-lg border p-4">
              <p className="text-xs text-muted-foreground">Sold</p>
              <p className="mt-1 text-2xl font-bold">{raffle.sold_tickets}</p>
              <p className="text-xs text-muted-foreground">of {raffle.total_tickets} ({soldPct}%)</p>
            </div>
            <div className="rounded-lg border p-4">
              <p className="text-xs text-muted-foreground">Revenue</p>
              <p className="mt-1 text-2xl font-bold">{usd(revenue)}</p>
              <p className="text-xs text-muted-foreground">@ {usd(raffle.ticket_price)}/ticket</p>
            </div>
            <div className="rounded-lg border p-4">
              <p className="text-xs text-muted-foreground">Prize Pool</p>
              <p className="mt-1 text-2xl font-bold">{usd(raffle.prize_pool)}</p>
              <p className="text-xs text-muted-foreground">{raffle.currency}</p>
            </div>
          </div>

          {/* Progress bar */}
          <div className="mb-6">
            <div className="mb-1 flex justify-between text-xs text-muted-foreground">
              <span>Tickets sold</span><span>{soldPct}%</span>
            </div>
            <div className="h-3 w-full overflow-hidden rounded-full bg-muted">
              <div className="h-full rounded-full bg-primary transition-all" style={{ width: `${soldPct}%` }} />
            </div>
          </div>

          {/* Ticket list */}
          {isLoading ? (
            <div className="space-y-2">
              {Array.from({ length: 5 }).map((_, i) => <Skeleton key={i} className="h-9 w-full" />)}
            </div>
          ) : !data || data.tickets.length === 0 ? (
            <p className="text-center text-muted-foreground py-4">No tickets sold yet.</p>
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b text-left text-muted-foreground">
                  <th className="pb-2 font-medium">#</th>
                  <th className="pb-2 font-medium">Ticket No.</th>
                  <th className="pb-2 font-medium">User ID</th>
                  <th className="pb-2 font-medium text-right">Purchased</th>
                </tr>
              </thead>
              <tbody>
                {data.tickets.map((t, i) => (
                  <tr key={t.id} className="border-b last:border-0">
                    <td className="py-1.5 text-muted-foreground">{i + 1}</td>
                    <td className="py-1.5 font-mono">{t.ticket_number}</td>
                    <td className="py-1.5 font-mono text-xs">{t.user_id}</td>
                    <td className="py-1.5 text-right text-muted-foreground">
                      {new Date(t.created_at).toLocaleDateString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  )
}
