import { useEffect, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select } from '@/components/ui/select'
import { adminApi, type CreateRaffleInput } from '@/features/admin/api'
import type { Raffle } from '@/types/api'

interface Props {
  raffle?: Raffle
  onClose: () => void
}

const EMPTY: CreateRaffleInput = {
  title: '', description: '', ticket_price: 0,
  max_tickets: 0, draw_date: '', status: 'draft',
  prize_pool: 0, currency: 'USD',
}

function toLocal(iso: string) {
  if (!iso) return ''
  return iso.slice(0, 16) // datetime-local needs "YYYY-MM-DDTHH:mm"
}

export function RaffleFormModal({ raffle, onClose }: Props) {
  const qc = useQueryClient()

  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])
  const [form, setForm] = useState<CreateRaffleInput>(EMPTY)
  const [error, setError] = useState('')

  useEffect(() => {
    if (raffle) {
      setForm({
        title: raffle.title,
        description: raffle.description,
        ticket_price: raffle.ticket_price,
        max_tickets: raffle.total_tickets,
        draw_date: toLocal(raffle.draw_date),
        status: raffle.status,
        prize_pool: raffle.prize_pool,
        currency: raffle.currency,
      })
    }
  }, [raffle])

  const numberKeys = new Set<keyof CreateRaffleInput>(['ticket_price', 'max_tickets', 'prize_pool'])
  const set = (k: keyof CreateRaffleInput) =>
    (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>) =>
      setForm(f => ({ ...f, [k]: numberKeys.has(k) ? Number(e.target.value) : e.target.value }))

  const mutation = useMutation({
    mutationFn: () => {
      const payload = { ...form, draw_date: new Date(form.draw_date).toISOString() }
      return raffle
        ? adminApi.updateRaffle(raffle.id, payload)
        : adminApi.createRaffle(payload)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin-raffles'] })
      onClose()
    },
    onError: (e: Error) => setError(e.message),
  })

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose} role="dialog" aria-modal="true" aria-labelledby="modal-title">
      <div className="w-full max-w-lg rounded-xl bg-card p-6 shadow-xl" onClick={e => e.stopPropagation()}>
        <div className="mb-4 flex items-center justify-between">
          <h2 id="modal-title" className="text-lg font-semibold">{raffle ? 'Edit Raffle' : 'Create Raffle'}</h2>
          <button onClick={onClose} aria-label="Close"><X className="h-5 w-5 text-muted-foreground" /></button>
        </div>

        {error && <p className="mb-3 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</p>}

        <div className="space-y-3">
          <div>
            <Label>Title</Label>
            <Input value={form.title} onChange={set('title')} placeholder="Raffle title" className="mt-1" />
          </div>
          <div>
            <Label>Description</Label>
            <textarea
              value={form.description} onChange={set('description')}
              placeholder="Description"
              className="mt-1 flex min-h-[72px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label>Ticket Price</Label>
              <Input type="number" min="0.01" step="0.01" value={form.ticket_price} onChange={set('ticket_price')} className="mt-1" />
            </div>
            <div>
              <Label>Total Tickets</Label>
              <Input type="number" min="1" step="1" value={form.max_tickets} onChange={set('max_tickets')} className="mt-1" />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label>Prize Pool</Label>
              <Input type="number" min="0" step="0.01" value={form.prize_pool} onChange={set('prize_pool')} className="mt-1" />
            </div>
            <div>
              <Label>Currency</Label>
              <Select value={form.currency} onChange={set('currency')} className="mt-1">
                <option value="USD">USD</option>
                <option value="EUR">EUR</option>
                <option value="GBP">GBP</option>
              </Select>
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label>Draw Date</Label>
              <Input type="datetime-local" value={form.draw_date} onChange={set('draw_date')} className="mt-1" />
            </div>
            <div>
              <Label>Status</Label>
              <Select value={form.status} onChange={set('status')} className="mt-1">
                <option value="draft">Draft</option>
                <option value="active">Active</option>
                <option value="scheduled">Scheduled</option>
                <option value="closed">Closed</option>
              </Select>
            </div>
          </div>
        </div>

        <div className="mt-5 flex justify-end gap-2">
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          <Button onClick={() => mutation.mutate()} disabled={mutation.isPending}>
            {mutation.isPending ? 'Saving…' : raffle ? 'Update' : 'Create'}
          </Button>
        </div>
      </div>
    </div>
  )
}
