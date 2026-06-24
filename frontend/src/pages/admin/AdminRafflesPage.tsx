import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, Pencil, XCircle, Users, BarChart2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Select } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { usePageTitle } from '@/hooks/usePageTitle'
import { adminApi } from '@/features/admin/api'
import { RaffleFormModal } from '@/features/admin/RaffleFormModal'
import { RaffleParticipantsModal } from '@/features/admin/RaffleParticipantsModal'
import { RaffleTicketSalesPanel } from '@/features/admin/RaffleTicketSalesPanel'
import type { Raffle } from '@/types/api'

const PAGE_SIZE = 10

const statusVariant: Record<string, 'default' | 'success' | 'warning' | 'destructive' | 'secondary'> = {
  active: 'success',
  scheduled: 'warning',
  draft: 'secondary',
  completed: 'default',
  closed: 'destructive',
  cancelled: 'destructive',
}

const usd = (n: number) =>
  n.toLocaleString('en-US', { style: 'currency', currency: 'USD', maximumFractionDigits: 2 })

type Modal =
  | { type: 'create' }
  | { type: 'edit'; raffle: Raffle }
  | { type: 'participants'; raffle: Raffle }
  | { type: 'sales'; raffle: Raffle }
  | null

export function Component() {
  usePageTitle('Admin Raffles')
  const qc = useQueryClient()
  const [offset, setOffset] = useState(0)
  const [statusFilter, setStatusFilter] = useState('')
  const [modal, setModal] = useState<Modal>(null)

  const { data, isLoading } = useQuery({
    queryKey: ['admin-raffles', offset, statusFilter],
    queryFn: () => adminApi.raffles(PAGE_SIZE, offset, statusFilter || undefined),
  })

  const closeMutation = useMutation({
    mutationFn: (id: string) => adminApi.closeRaffle(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin-raffles'] }),
  })

  const total = data?.total ?? 0
  const raffles = data?.raffles ?? []
  const pages = Math.ceil(total / PAGE_SIZE)
  const page = Math.floor(offset / PAGE_SIZE) + 1

  return (
    <div className="p-6 space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Raffles</h1>
          <p className="text-sm text-muted-foreground">{total} total</p>
        </div>
        <Button onClick={() => setModal({ type: 'create' })}>
          <Plus className="h-4 w-4" /> New Raffle
        </Button>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-2">
        <Select value={statusFilter} onChange={e => { setStatusFilter(e.target.value); setOffset(0) }} className="w-40">
          <option value="">All statuses</option>
          <option value="draft">Draft</option>
          <option value="active">Active</option>
          <option value="scheduled">Scheduled</option>
          <option value="completed">Completed</option>
          <option value="closed">Closed</option>
          <option value="cancelled">Cancelled</option>
        </Select>
      </div>

      {/* Table */}
      <div className="rounded-xl border bg-card overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/40">
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">Title</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">Status</th>
              <th className="px-4 py-3 text-right font-medium text-muted-foreground">Price</th>
              <th className="px-4 py-3 text-right font-medium text-muted-foreground">Tickets</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">Draw Date</th>
              <th className="px-4 py-3 text-right font-medium text-muted-foreground">Actions</th>
            </tr>
          </thead>
          <tbody>
            {isLoading
              ? Array.from({ length: PAGE_SIZE }).map((_, i) => (
                  <tr key={i} className="border-b">
                    {Array.from({ length: 6 }).map((_, j) => (
                      <td key={j} className="px-4 py-3"><Skeleton className="h-5 w-full" /></td>
                    ))}
                  </tr>
                ))
              : raffles.length === 0
              ? (
                <tr>
                  <td colSpan={6} className="px-4 py-12 text-center text-muted-foreground">
                    No raffles found.
                  </td>
                </tr>
              )
              : raffles.map(r => (
                  <tr key={r.id} className="border-b last:border-0 hover:bg-muted/20">
                    <td className="px-4 py-3">
                      <p className="font-medium">{r.title}</p>
                      <p className="text-xs text-muted-foreground line-clamp-1">{r.description}</p>
                    </td>
                    <td className="px-4 py-3">
                      <Badge variant={statusVariant[r.status] ?? 'secondary'}>
                        {r.status}
                      </Badge>
                    </td>
                    <td className="px-4 py-3 text-right">{usd(r.ticket_price)}</td>
                    <td className="px-4 py-3 text-right">
                      <span>{r.sold_tickets}</span>
                      <span className="text-muted-foreground">/{r.total_tickets}</span>
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">
                      {new Date(r.draw_date).toLocaleDateString()}
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center justify-end gap-1">
                        <Button
                          variant="ghost" size="icon"
                          title="Edit"
                          onClick={() => setModal({ type: 'edit', raffle: r })}
                        >
                          <Pencil className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost" size="icon"
                          title="Participants"
                          onClick={() => setModal({ type: 'participants', raffle: r })}
                        >
                          <Users className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost" size="icon"
                          title="Ticket sales"
                          onClick={() => setModal({ type: 'sales', raffle: r })}
                        >
                          <BarChart2 className="h-4 w-4" />
                        </Button>
                        {r.status !== 'closed' && r.status !== 'cancelled' && (
                          <Button
                            variant="ghost" size="icon"
                            title="Close raffle"
                            className="text-destructive hover:text-destructive"
                            disabled={closeMutation.isPending}
                            onClick={() => {
                              if (confirm(`Close "${r.title}"? This cannot be undone.`)) {
                                closeMutation.mutate(r.id)
                              }
                            }}
                          >
                            <XCircle className="h-4 w-4" />
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

      {/* Modals */}
      {modal?.type === 'create' && (
        <RaffleFormModal onClose={() => setModal(null)} />
      )}
      {modal?.type === 'edit' && (
        <RaffleFormModal raffle={modal.raffle} onClose={() => setModal(null)} />
      )}
      {modal?.type === 'participants' && (
        <RaffleParticipantsModal raffle={modal.raffle} onClose={() => setModal(null)} />
      )}
      {modal?.type === 'sales' && (
        <RaffleTicketSalesPanel raffle={modal.raffle} onClose={() => setModal(null)} />
      )}
    </div>
  )
}
