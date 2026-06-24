import { useQuery } from '@tanstack/react-query'
import { X } from 'lucide-react'
import { Skeleton } from '@/components/ui/skeleton'
import { adminApi } from '@/features/admin/api'
import type { Raffle } from '@/types/api'

interface Props {
  raffle: Raffle
  onClose: () => void
}

export function RaffleParticipantsModal({ raffle, onClose }: Props) {
  const { data, isLoading } = useQuery({
    queryKey: ['raffle-tickets', raffle.id],
    queryFn: () => adminApi.listRaffleTickets(raffle.id),
  })

  // Group tickets by user_id to get unique participants
  const participants = data
    ? Object.entries(
        data.tickets.reduce<Record<string, { userId: string; tickets: number; numbers: number[] }>>(
          (acc, t) => {
            if (!acc[t.user_id]) acc[t.user_id] = { userId: t.user_id, tickets: 0, numbers: [] }
            acc[t.user_id].tickets++
            acc[t.user_id].numbers.push(t.ticket_number)
            return acc
          },
          {},
        ),
      ).map(([, v]) => v)
    : []

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose}>
      <div className="flex w-full max-w-2xl flex-col rounded-xl bg-card shadow-xl" onClick={e => e.stopPropagation()}>
        <div className="flex items-center justify-between border-b px-6 py-4">
          <div>
            <h2 className="text-lg font-semibold">Participants</h2>
            <p className="text-sm text-muted-foreground">{raffle.title}</p>
          </div>
          <button onClick={onClose}><X className="h-5 w-5 text-muted-foreground" /></button>
        </div>

        <div className="overflow-auto p-6" style={{ maxHeight: '60vh' }}>
          {isLoading ? (
            <div className="space-y-2">
              {Array.from({ length: 5 }).map((_, i) => <Skeleton key={i} className="h-10 w-full" />)}
            </div>
          ) : participants.length === 0 ? (
            <p className="text-center text-muted-foreground py-8">No participants yet.</p>
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b text-left text-muted-foreground">
                  <th className="pb-2 font-medium">User ID</th>
                  <th className="pb-2 font-medium text-right">Tickets</th>
                  <th className="pb-2 font-medium text-right">Numbers</th>
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
          {data && `${participants.length} unique participant(s) · ${data.total} ticket(s)`}
        </div>
      </div>
    </div>
  )
}
