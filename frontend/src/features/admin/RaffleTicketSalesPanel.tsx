import { useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { X } from 'lucide-react'
import { Skeleton } from '@/components/ui/skeleton'
import { adminApi } from '@/features/admin/api'
import { formatCurrency } from '@/lib/utils'
import type { Raffle } from '@/types/api'

interface Props { raffle: Raffle; onClose: () => void }

const usd = (n: number) => formatCurrency(n)

export function RaffleTicketSalesPanel({ raffle, onClose }: Props) {
  const { t } = useTranslation()
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
  const soldPct = raffle.total_tickets > 0 ? Math.round((raffle.sold_tickets / raffle.total_tickets) * 100) : 0

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose} role="dialog" aria-modal="true" aria-labelledby="modal-title">
      <div className="flex w-full max-w-2xl flex-col rounded-xl bg-card shadow-xl" onClick={e => e.stopPropagation()}>
        <div className="flex items-center justify-between border-b px-6 py-4">
          <div>
            <h2 id="modal-title" className="text-lg font-semibold">{t('modal.ticketSales')}</h2>
            <p className="text-sm text-muted-foreground">{raffle.title}</p>
          </div>
          <button onClick={onClose} aria-label={t('common.close')}><X className="h-5 w-5 text-muted-foreground" /></button>
        </div>

        <div className="overflow-auto p-6" style={{ maxHeight: '60vh' }}>
          <div className="mb-6 grid grid-cols-3 gap-4">
            <div className="rounded-lg border p-4">
              <p className="text-xs text-muted-foreground">{t('modal.sold')}</p>
              <p className="mt-1 text-2xl font-bold">{raffle.sold_tickets}</p>
              <p className="text-xs text-muted-foreground">{t('modal.soldOf', { total: raffle.total_tickets, pct: soldPct })}</p>
            </div>
            <div className="rounded-lg border p-4">
              <p className="text-xs text-muted-foreground">{t('modal.revenue')}</p>
              <p className="mt-1 text-2xl font-bold">{usd(revenue)}</p>
              <p className="text-xs text-muted-foreground">{t('modal.perTicket', { price: usd(raffle.ticket_price) })}</p>
            </div>
            <div className="rounded-lg border p-4">
              <p className="text-xs text-muted-foreground">{t('modal.prizeAmount')}</p>
              <p className="mt-1 text-2xl font-bold">{usd(raffle.prize_pool)}</p>
              <p className="text-xs text-muted-foreground">{raffle.currency}</p>
            </div>
          </div>

          <div className="mb-6">
            <div className="mb-1 flex justify-between text-xs text-muted-foreground">
              <span>{t('modal.ticketsSold')}</span><span>{soldPct}%</span>
            </div>
            <div className="h-3 w-full overflow-hidden rounded-full bg-muted">
              <div className="h-full rounded-full bg-primary transition-all" style={{ width: `${soldPct}%` }} />
            </div>
          </div>

          {isLoading ? (
            <div className="space-y-2">{Array.from({ length: 5 }).map((_, i) => <Skeleton key={i} className="h-9 w-full" />)}</div>
          ) : !data || data.tickets.length === 0 ? (
            <p className="text-center text-muted-foreground py-4">{t('modal.noTicketsSold')}</p>
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b text-left text-muted-foreground">
                  <th className="pb-2 font-medium">{t('modal.rowNumber')}</th>
                  <th className="pb-2 font-medium">{t('modal.ticketNo')}</th>
                  <th className="pb-2 font-medium">{t('modal.userId')}</th>
                  <th className="pb-2 font-medium text-right">{t('modal.purchased')}</th>
                </tr>
              </thead>
              <tbody>
                {data.tickets.map((ticket, i) => (
                  <tr key={ticket.id} className="border-b last:border-0">
                    <td className="py-1.5 text-muted-foreground">{i + 1}</td>
                    <td className="py-1.5 font-mono">{ticket.ticket_number}</td>
                    <td className="py-1.5 font-mono text-xs">{ticket.user_id}</td>
                    <td className="py-1.5 text-right text-muted-foreground">{new Date(ticket.created_at).toLocaleDateString()}</td>
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
