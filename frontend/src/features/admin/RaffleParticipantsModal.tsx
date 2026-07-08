import { useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { X } from 'lucide-react'
import { Skeleton } from '@/components/ui/skeleton'
import { adminApi } from '@/features/admin/api'
import type { Raffle } from '@/types/api'

interface Props { raffle: Raffle; onClose: () => void }

export function RaffleParticipantsModal({ raffle, onClose }: Props) {
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

  const participants = data
    ? Object.entries(
        data.tickets.reduce<Record<string, { userId: string; tickets: number; numbers: number[] }>>(
          (acc, ticket) => {
            if (!acc[ticket.user_id]) acc[ticket.user_id] = { userId: ticket.user_id, tickets: 0, numbers: [] }
            acc[ticket.user_id].tickets++
            acc[ticket.user_id].numbers.push(ticket.ticket_number)
            return acc
          }, {},
        ),
      ).map(([, v]) => v)
    : []

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose} role="dialog" aria-modal="true" aria-labelledby="modal-title">
      <div className="flex w-full max-w-2xl flex-col rounded-xl bg-card shadow-xl" onClick={e => e.stopPropagation()}>
        <div className="flex items-center justify-between border-b px-6 py-4">
          <div>
            <h2 id="modal-title" className="text-lg font-semibold">{t('modal.participants')}</h2>
            <p className="text-sm text-muted-foreground">{raffle.title}</p>
          </div>
          <button onClick={onClose} aria-label={t('common.close')}><X className="h-5 w-5 text-muted-foreground" /></button>
        </div>

        <div className="overflow-auto p-6" style={{ maxHeight: '60vh' }}>
          {isLoading ? (
            <div className="space-y-2">{Array.from({ length: 5 }).map((_, i) => <Skeleton key={i} className="h-10 w-full" />)}</div>
          ) : participants.length === 0 ? (
            <p className="text-center text-muted-foreground py-8">{t('modal.noParticipants')}</p>
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b text-left text-muted-foreground">
                  <th className="pb-2 font-medium">{t('modal.participantUserId')}</th>
                  <th className="pb-2 font-medium text-right">{t('modal.participantTickets')}</th>
                  <th className="pb-2 font-medium text-right">{t('modal.participantNumbers')}</th>
                </tr>
              </thead>
              <tbody>
                {participants.map(p => (
                  <tr key={p.userId} className="border-b last:border-0">
                    <td className="py-2 font-mono text-xs">{p.userId}</td>
                    <td className="py-2 text-right">{p.tickets}</td>
                    <td className="py-2 text-right text-muted-foreground">
                      {p.numbers.slice(0, 6).join(', ')}{p.numbers.length > 6 ? '…' : ''}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        <div className="border-t px-6 py-3 text-sm text-muted-foreground">
          {data && t('modal.participantCount', { participants: participants.length, tickets: data.total })}
        </div>
      </div>
    </div>
  )
}
