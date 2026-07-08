import { useEffect, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { formatCurrency } from '@/lib/utils'
import { adminApi, type WinnerDetail } from '@/features/admin/api'

interface Props { winner: WinnerDetail; onClose: () => void }

export function MarkPaidModal({ winner, onClose }: Props) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const [ref, setRef] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  const mutation = useMutation({
    mutationFn: () => adminApi.markPaid(winner.id, ref),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['admin-winners'] }); onClose() },
    onError: (e: Error) => setError(e.message),
  })

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose} role="dialog" aria-modal="true" aria-labelledby="modal-title">
      <div className="w-full max-w-md rounded-xl bg-card p-6 shadow-xl" onClick={e => e.stopPropagation()}>
        <div className="mb-4 flex items-center justify-between">
          <h2 id="modal-title" className="text-lg font-semibold">{t('modal.markPrizeAsPaid')}</h2>
          <button onClick={onClose} aria-label={t('common.close')}><X className="h-5 w-5 text-muted-foreground" /></button>
        </div>
        <div className="mb-4 rounded-lg border bg-muted/30 p-3 text-sm space-y-1">
          <p><span className="text-muted-foreground">{t('modal.winnerLabel')}</span>{winner.user_email}</p>
          <p><span className="text-muted-foreground">{t('modal.raffleLabel')}</span>{winner.raffle_title}</p>
          <p><span className="text-muted-foreground">{t('modal.prizeLabel')}</span><span className="font-semibold">{formatCurrency(winner.prize_amount)}</span></p>
        </div>
        {error && <p className="mb-3 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</p>}
        <div className="mb-5">
          <Label htmlFor="ref">{t('modal.paymentReference')}</Label>
          <Input id="ref" className="mt-1" placeholder={t('modal.paymentReferencePlaceholder')} value={ref} onChange={e => setRef(e.target.value)} />
        </div>
        <div className="flex justify-end gap-2">
          <Button variant="outline" onClick={onClose}>{t('modal.cancel')}</Button>
          <Button disabled={!ref.trim() || mutation.isPending} onClick={() => mutation.mutate()}>
            {mutation.isPending ? t('modal.processing') : t('modal.confirmPayment')}
          </Button>
        </div>
      </div>
    </div>
  )
}
