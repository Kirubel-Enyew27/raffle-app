import { useEffect, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { adminApi, type WinnerDetail } from '@/features/admin/api'

interface Props {
  winner: WinnerDetail
  onClose: () => void
}

export function MarkPaidModal({ winner, onClose }: Props) {
  const qc = useQueryClient()

  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])
  const [ref, setRef] = useState('')
  const [error, setError] = useState('')

  const mutation = useMutation({
    mutationFn: () => adminApi.markPaid(winner.id, ref),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin-winners'] })
      onClose()
    },
    onError: (e: Error) => setError(e.message),
  })

  const usd = winner.prize_amount.toLocaleString('en-US', { style: 'currency', currency: 'USD' })

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose} role="dialog" aria-modal="true" aria-labelledby="modal-title">
      <div className="w-full max-w-md rounded-xl bg-card p-6 shadow-xl" onClick={e => e.stopPropagation()}>
        <div className="mb-4 flex items-center justify-between">
          <h2 id="modal-title" className="text-lg font-semibold">Mark Prize as Paid</h2>
          <button onClick={onClose} aria-label="Close"><X className="h-5 w-5 text-muted-foreground" /></button>
        </div>

        <div className="mb-4 rounded-lg border bg-muted/30 p-3 text-sm space-y-1">
          <p><span className="text-muted-foreground">Winner: </span>{winner.user_email}</p>
          <p><span className="text-muted-foreground">Raffle: </span>{winner.raffle_title}</p>
          <p><span className="text-muted-foreground">Prize: </span><span className="font-semibold">{usd}</span></p>
        </div>

        {error && <p className="mb-3 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</p>}

        <div className="mb-5">
          <Label htmlFor="ref">Payment Reference</Label>
          <Input
            id="ref"
            className="mt-1"
            placeholder="e.g. TXN-12345, wire ref, etc."
            value={ref}
            onChange={e => setRef(e.target.value)}
          />
        </div>

        <div className="flex justify-end gap-2">
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          <Button
            disabled={!ref.trim() || mutation.isPending}
            onClick={() => mutation.mutate()}
          >
            {mutation.isPending ? 'Processing…' : 'Confirm Payment'}
          </Button>
        </div>
      </div>
    </div>
  )
}
