import { useState, useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Trophy, Ticket, X, ExternalLink, Copy, CheckCircle, Calendar, Hash } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { ErrorMessage } from '@/components/ui/error-message'
import { Skeleton } from '@/components/ui/skeleton'
import { usePageTitle } from '@/hooks/usePageTitle'
import { useAuth } from '@/contexts/AuthContext'
import { winnerApi } from '@/features/winners/api'
import type { WinnerDetail } from '@/types/api'

const PAGE_SIZE = 100

import { formatCurrency } from '@/lib/utils'
const fmt = (n: number) => formatCurrency(n)

// ─── Detail Row ──────────────────────────────────────────
function DetailRow({ label, value, mono }: { label: string; value: React.ReactNode; mono?: boolean }) {
  return (
    <div className="grid grid-cols-[130px_1fr] gap-2 border-b py-2.5 last:border-0">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className={mono ? 'break-all font-mono text-xs' : 'text-sm'}>{value}</span>
    </div>
  )
}

// ─── Modal Shell ──────────────────────────────────────────
function Modal({ title, icon, onClose, children }: { title: string; icon: React.ReactNode; onClose: () => void; children: React.ReactNode }) {
  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-black/50 pt-10 pb-10" onClick={onClose} role="dialog" aria-modal="true" aria-label={title}>
      <div className="w-full max-w-xl rounded-xl bg-card shadow-xl" onClick={e => e.stopPropagation()}>
        <div className="flex items-center justify-between border-b px-6 py-4">
          <div className="flex items-center gap-2">
            {icon}
            <h2 className="text-lg font-semibold">{title}</h2>
          </div>
          <button onClick={onClose} className="rounded-md p-1 transition-colors hover:bg-muted" aria-label="Close">
            <X className="h-5 w-5 text-muted-foreground" />
          </button>
        </div>
        <div className="px-6 py-4">{children}</div>
      </div>
    </div>
  )
}

// ─── Detail View Modal ────────────────────────────────────
function WinnerDetailModal({ winner, onClose }: { winner: WinnerDetail; onClose: () => void }) {
  const [tab, setTab] = useState<'detail' | 'ticket' | 'verification'>('detail')
  const [copied, setCopied] = useState<string | null>(null)

  const copy = (val: string, label: string) => {
    navigator.clipboard.writeText(val)
    setCopied(label)
    setTimeout(() => setCopied(null), 1500)
  }

  const { data: ticket, isLoading: ticketLoading } = useQuery({
    queryKey: ['winning-ticket', winner.id],
    queryFn: () => winnerApi.getWinningTicket(winner.id),
    enabled: tab === 'ticket',
  })

  const { data: verification, isLoading: verLoading } = useQuery({
    queryKey: ['draw-verification', winner.id],
    queryFn: () => winnerApi.getDrawVerification(winner.id),
    enabled: tab === 'verification',
  })

  return (
    <Modal
      title="Winner Details"
      icon={<Trophy className="h-5 w-5 text-yellow-500" />}
      onClose={onClose}
    >
      {/* Tabs */}
      <div className="mb-4 flex gap-1 rounded-lg bg-muted p-1">
        {(['detail', 'ticket', 'verification'] as const).map(t => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`flex-1 rounded-md px-3 py-1.5 text-sm font-medium capitalize transition-colors ${
              tab === t ? 'bg-background shadow-sm' : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            {t === 'detail' ? 'Details' : t === 'ticket' ? 'Winning Ticket' : 'Verification'}
          </button>
        ))}
      </div>

      {/* Tab: Detail */}
      {tab === 'detail' && (
        <div className="space-y-0">
          <DetailRow label="Raffle" value={winner.raffle_title} />
          <DetailRow label="Winner" value={winner.user_email} />
          <DetailRow label="Prize Amount" value={<span className="font-semibold text-green-600">{fmt(winner.prize_amount)}</span>} />
          <DetailRow label="Status" value={<Badge variant={winner.prize_paid ? 'success' : 'warning'}>{winner.prize_paid ? 'Paid' : 'Unpaid'}</Badge>} />
          {winner.payment_reference && <DetailRow label="Payment Ref" value={<span className="font-mono text-xs">{winner.payment_reference}</span>} />}
          {winner.payment_date && <DetailRow label="Paid On" value={new Date(winner.payment_date).toLocaleString()} />}
          <DetailRow label="Won On" value={new Date(winner.created_at).toLocaleString()} />
        </div>
      )}

      {/* Tab: Winning Ticket */}
      {tab === 'ticket' && (
        <div>
          {ticketLoading ? (
            <div className="space-y-3 py-4">
              {Array.from({ length: 5 }).map((_, i) => <Skeleton key={i} className="h-5 w-full" />)}
            </div>
          ) : ticket ? (
            <div className="space-y-0">
              <div className="mb-4 flex items-center gap-3 rounded-lg border bg-muted/30 px-4 py-3">
                <Ticket className="h-8 w-8 text-primary" />
                <div>
                  <p className="text-2xl font-bold tabular-nums">#{ticket.ticket_number}</p>
                  <p className="text-xs text-muted-foreground">Winning Ticket Number</p>
                </div>
              </div>
              <DetailRow label="Ticket ID" value={ticket.ticket_id} mono />
              <DetailRow label="User" value={ticket.user_email} />
              <DetailRow label="Draw Time" value={ticket.draw_timestamp ? new Date(ticket.draw_timestamp).toLocaleString() : '—'} />
            </div>
          ) : (
            <p className="py-6 text-center text-sm text-muted-foreground">Failed to load ticket details.</p>
          )}
        </div>
      )}

      {/* Tab: Verification */}
      {tab === 'verification' && (
        <div>
          {verLoading ? (
            <div className="space-y-3 py-4">
              {Array.from({ length: 6 }).map((_, i) => <Skeleton key={i} className="h-5 w-full" />)}
            </div>
          ) : verification ? (
            <div className="space-y-0">
              <div className="mb-4 flex items-center gap-2 rounded-lg border border-green-200 bg-green-50 px-4 py-3 dark:border-green-800 dark:bg-green-950">
                <CheckCircle className="h-5 w-5 text-green-600" />
                <p className="text-sm font-medium text-green-700 dark:text-green-300">
                  Draw verification data available
                </p>
              </div>

              {(Object.entries({
                'Commit Hash': { value: verification.commit_hash, label: 'commit_hash' },
                'Server Seed Hash': { value: verification.server_seed_hash, label: 'server_seed_hash' },
                'Revealed Seed': { value: verification.revealed_seed, label: 'revealed_seed' },
                'Combined Hash': { value: verification.combined_hash, label: 'combined_hash' },
              }) as [string, { value: string; label: string }][]).map(([display, { value, label }]) => (
                <div key={label} className="group grid grid-cols-[130px_1fr_auto] gap-2 border-b py-2.5 last:border-0">
                  <span className="text-sm text-muted-foreground">{display}</span>
                  <span className="truncate font-mono text-xs">{value}</span>
                  <button
                    onClick={() => copy(value, label)}
                    className="shrink-0 opacity-0 transition-opacity group-hover:opacity-100"
                    title="Copy to clipboard"
                  >
                    {copied === label ? (
                      <CheckCircle className="h-3.5 w-3.5 text-green-500" />
                    ) : (
                      <Copy className="h-3.5 w-3.5 text-muted-foreground" />
                    )}
                  </button>
                </div>
              ))}

              <DetailRow label="Winning Number" value={verification.winning_number} />
              <DetailRow label="Draw ID" value={verification.draw_id} mono />
              <DetailRow label="Draw Time" value={new Date(verification.draw_timestamp).toLocaleString()} />
              <DetailRow label="Ticket ID" value={verification.winning_ticket_id} mono />

              {verification.verification_url && (
                <div className="mt-4 flex justify-end">
                  <a
                    href={verification.verification_url}
                    target="_blank" rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 text-sm text-primary underline-offset-4 hover:underline"
                  >
                    <ExternalLink className="h-3.5 w-3.5" />
                    Verify draw independently
                  </a>
                </div>
              )}
            </div>
          ) : (
            <p className="py-6 text-center text-sm text-muted-foreground">Failed to load verification data.</p>
          )}
        </div>
      )}
    </Modal>
  )
}

// ─── Main Page ────────────────────────────────────────────
export function Component() {
  usePageTitle('My Winnings')
  const { user } = useAuth()
  const [selectedWinner, setSelectedWinner] = useState<WinnerDetail | null>(null)

  const { data, isLoading, error } = useQuery({
    queryKey: ['my-winners'],
    queryFn: () => winnerApi.list(PAGE_SIZE),
  })

  const winners = (data?.winners ?? []).filter(w => w.user_id === user?.id)

  return (
    <div className="mx-auto max-w-6xl space-y-6 p-4 sm:p-6">
      {/* Header */}
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-yellow-100 dark:bg-yellow-900/30">
          <Trophy className="h-5 w-5 text-yellow-600 dark:text-yellow-400" />
        </div>
        <div>
          <h1 className="text-2xl font-bold">My Winnings</h1>
          <p className="text-sm text-muted-foreground">
            {winners.length > 0
              ? `You've won ${winners.length} prize${winners.length !== 1 ? 's' : ''}`
              : 'Track your raffle prizes and view draw verification'}
          </p>
        </div>
      </div>

      {error && <ErrorMessage message="Failed to load winnings. Please try again." />}

      {/* Winners list */}
      <div className="space-y-3">
        {isLoading ? (
          Array.from({ length: 4 }).map((_, i) => (
            <Card key={i}>
              <CardContent className="p-4">
                <div className="flex items-center justify-between">
                  <div className="space-y-2">
                    <Skeleton className="h-5 w-40" />
                    <Skeleton className="h-4 w-24" />
                  </div>
                  <Skeleton className="h-8 w-20" />
                </div>
              </CardContent>
            </Card>
          ))
        ) : winners.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center py-16">
              <Trophy className="mb-3 h-12 w-12 text-muted-foreground/30" />
              <p className="text-lg font-medium text-muted-foreground">No winnings yet</p>
              <p className="mt-1 text-sm text-muted-foreground/60">
                Buy tickets for active raffles for a chance to win!
              </p>
            </CardContent>
          </Card>
        ) : (
          winners.map(w => (
            <Card
              key={w.id}
              className="cursor-pointer transition-all hover:shadow-md hover:ring-1 hover:ring-primary/20"
              onClick={() => setSelectedWinner(w)}
            >
              <CardContent className="p-4">
                <div className="flex items-center justify-between gap-4">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <p className="truncate text-sm font-medium">{w.raffle_title || 'Unknown Raffle'}</p>
                      <Badge variant={w.prize_paid ? 'success' : 'warning'} className="shrink-0">
                        {w.prize_paid ? 'Paid' : 'Unpaid'}
                      </Badge>
                    </div>
                    <div className="mt-1.5 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
                      <span className="inline-flex items-center gap-1">
                        <Hash className="h-3 w-3" />
                        Ticket #{w.ticket_number}
                      </span>
                      <span className="inline-flex items-center gap-1">
                        <Calendar className="h-3 w-3" />
                        {new Date(w.created_at).toLocaleDateString('en-US', {
                          month: 'short', day: 'numeric', year: 'numeric',
                        })}
                      </span>
                      {w.payment_reference && (
                        <span className="inline-flex items-center gap-1 font-mono">
                          Ref: {w.payment_reference}
                        </span>
                      )}
                    </div>
                  </div>
                  <div className="shrink-0 text-right">
                    <p className="text-lg font-bold text-green-600 dark:text-green-400">
                      {fmt(w.prize_amount)}
                    </p>
                    <p className="text-xs text-muted-foreground">prize</p>
                  </div>
                </div>
              </CardContent>
            </Card>
          ))
        )}
      </div>

      {/* Detail Modal */}
      {selectedWinner && (
        <WinnerDetailModal winner={selectedWinner} onClose={() => setSelectedWinner(null)} />
      )}
    </div>
  )
}
