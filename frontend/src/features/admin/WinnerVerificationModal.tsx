import { useEffect } from 'react'
import { X, CheckCircle } from 'lucide-react'
import type { WinnerDetail } from '@/features/admin/api'

interface Props {
  winner: WinnerDetail
  onClose: () => void
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="grid grid-cols-[160px_1fr] gap-2 border-b py-2.5 last:border-0">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className="break-all font-mono text-xs">{value}</span>
    </div>
  )
}

export function WinnerVerificationModal({ winner, onClose }: Props) {
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
            <h2 id="modal-title" className="text-lg font-semibold">Draw Verification</h2>
          </div>
          <button onClick={onClose} aria-label="Close"><X className="h-5 w-5 text-muted-foreground" /></button>
        </div>

        <div className="px-6 py-4 space-y-0">
          <Row label="Raffle" value={<span className="font-sans">{winner.raffle_title}</span>} />
          <Row label="Winner" value={<span className="font-sans">{winner.user_email}</span>} />
          <Row label="Winning Ticket #" value={winner.ticket_number} />
          <Row label="Draw Time" value={winner.draw_timestamp ? new Date(winner.draw_timestamp).toLocaleString() : '—'} />
          <Row label="Prize Amount" value={
            <span className="font-sans font-semibold">
              {winner.prize_amount.toLocaleString('en-US', { style: 'currency', currency: 'USD' })}
            </span>
          } />
        </div>

        <div className="border-t bg-muted/30 px-6 py-4">
          <p className="mb-3 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Cryptographic Proof</p>
          <div className="space-y-0">
            <Row label="Commit Hash" value={proof?.commit_hash || '—'} />
            <Row label="Server Seed Hash" value={proof?.server_seed_hash || '—'} />
            <Row label="Revealed Seed" value={proof?.revealed_seed || '—'} />
            <Row label="Combined Hash" value={proof?.combined_hash || '—'} />
            <Row label="Winning Number" value={proof?.winning_number ?? '—'} />
          </div>
        </div>

        <div className="border-t px-6 py-3 flex justify-end">
          {proof?.verification_url && (
            <a
              href={proof.verification_url}
              target="_blank" rel="noopener noreferrer"
              className="text-sm text-primary underline-offset-4 hover:underline"
            >
              Verify on-chain →
            </a>
          )}
        </div>
      </div>
    </div>
  )
}
