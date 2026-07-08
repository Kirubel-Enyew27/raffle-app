import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { X, CheckCircle } from 'lucide-react'
import { formatCurrency } from '@/lib/utils'
import type { WinnerDetail } from '@/features/admin/api'

interface Props { winner: WinnerDetail; onClose: () => void }

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="grid grid-cols-[160px_1fr] gap-2 border-b py-2.5 last:border-0">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className="break-all font-mono text-xs">{value}</span>
    </div>
  )
}

export function WinnerVerificationModal({ winner, onClose }: Props) {
  const { t } = useTranslation()
  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  const proof = winner.draw_proof

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose} role="dialog" aria-modal="true" aria-labelledby="modal-title">
      <div className="w-full max-w-xl rounded-xl bg-card shadow-xl" onClick={e => e.stopPropagation()}>
        <div className="flex items-center justify-between border-b px-6 py-4">
          <div className="flex items-center gap-2">
            <CheckCircle className="h-5 w-5 text-green-500" />
            <h2 id="modal-title" className="text-lg font-semibold">{t('modal.drawVerification')}</h2>
          </div>
          <button onClick={onClose} aria-label={t('common.close')}><X className="h-5 w-5 text-muted-foreground" /></button>
        </div>

        <div className="px-6 py-4 space-y-0">
          <Row label={t('modal.raffle')} value={<span className="font-sans">{winner.raffle_title}</span>} />
          <Row label={t('modal.winner')} value={<span className="font-sans">{winner.user_email}</span>} />
          <Row label={t('modal.winningTicketHash')} value={winner.ticket_number} />
          <Row label={t('modal.drawTime')} value={winner.draw_timestamp ? new Date(winner.draw_timestamp).toLocaleString() : '—'} />
          <Row label={t('modal.prizeAmount')} value={<span className="font-sans font-semibold">{formatCurrency(winner.prize_amount)}</span>} />
        </div>

        <div className="border-t bg-muted/30 px-6 py-4">
          <p className="mb-3 text-xs font-semibold uppercase tracking-wide text-muted-foreground">{t('modal.cryptographicProof')}</p>
          <div className="space-y-0">
            <Row label={t('modal.commitHash')} value={proof?.commit_hash || '—'} />
            <Row label={t('modal.serverSeedHash')} value={proof?.server_seed_hash || '—'} />
            <Row label={t('modal.revealedSeed')} value={proof?.revealed_seed || '—'} />
            <Row label={t('modal.combinedHash')} value={proof?.combined_hash || '—'} />
            <Row label={t('modal.winningNumber')} value={proof?.winning_number ?? '—'} />
          </div>
        </div>

        <div className="border-t px-6 py-3 flex justify-end">
          {proof?.verification_url && (
            <a href={proof.verification_url} target="_blank" rel="noopener noreferrer"
              className="text-sm text-primary underline-offset-4 hover:underline">
              {t('modal.verifyOnChain')}
            </a>
          )}
        </div>
      </div>
    </div>
  )
}
