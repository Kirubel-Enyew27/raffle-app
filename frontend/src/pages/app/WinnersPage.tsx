import { useState, useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Trophy, Ticket, X, ExternalLink, Copy, CheckCircle, Calendar, Hash } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { ErrorMessage } from '@/components/ui/error-message'
import { Skeleton } from '@/components/ui/skeleton'
import { usePageTitle } from '@/hooks/usePageTitle'
import { useAuth } from '@/contexts/AuthContext'
import { winnerApi } from '@/features/winners/api'
import { formatCurrency } from '@/lib/utils'
import type { WinnerDetail } from '@/types/api'

const PAGE_SIZE = 100

function DetailRow({ label, value, mono }: { label: string; value: React.ReactNode; mono?: boolean }) {
  return (
    <div className="grid grid-cols-[130px_1fr] gap-2 border-b py-2.5 last:border-0">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className={mono ? 'break-all font-mono text-xs' : 'text-sm'}>{value}</span>
    </div>
  )
}

function Modal({ title, icon, onClose, children }: {
  title: string; icon: React.ReactNode; onClose: () => void; children: React.ReactNode
}) {
  const { t } = useTranslation()
  useEffect(() => {
    const h = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  }, [onClose])
  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-black/50 pt-10 pb-10"
      onClick={onClose} role="dialog" aria-modal="true" aria-label={title}>
      <div className="w-full max-w-xl rounded-xl bg-card shadow-xl" onClick={e => e.stopPropagation()}>
        <div className="flex items-center justify-between border-b px-6 py-4">
          <div className="flex items-center gap-2">{icon}<h2 className="text-lg font-semibold">{title}</h2></div>
          <button onClick={onClose} className="rounded-md p-1 hover:bg-muted" aria-label={t('common.close')}>
            <X className="h-5 w-5 text-muted-foreground" />
          </button>
        </div>
        <div className="px-6 py-4">{children}</div>
      </div>
    </div>
  )
}

function WinnerDetailModal({ winner, onClose }: { winner: WinnerDetail; onClose: () => void }) {
  const { t } = useTranslation()
  const [tab, setTab] = useState<'detail' | 'ticket' | 'verification'>('detail')
  const [copied, setCopied] = useState<string | null>(null)

  const copy = (val: string, key: string) => {
    navigator.clipboard.writeText(val)
    setCopied(key)
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

  const tabs = [
    { id: 'detail' as const,       label: t('winners.details')       },
    { id: 'ticket' as const,       label: t('winners.winningTicket') },
    { id: 'verification' as const, label: t('winners.verification')  },
  ]

  return (
    <Modal title={t('winners.winnerDetails')} icon={<Trophy className="h-5 w-5 text-yellow-500" />} onClose={onClose}>
      {/* Tabs */}
      <div className="mb-4 flex gap-1 rounded-lg bg-muted p-1">
        {tabs.map(tb => (
          <button key={tb.id} onClick={() => setTab(tb.id)}
            className={`flex-1 rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
              tab === tb.id ? 'bg-background shadow-sm' : 'text-muted-foreground hover:text-foreground'
            }`}>
            {tb.label}
          </button>
        ))}
      </div>

      {/* Detail */}
      {tab === 'detail' && (
        <div>
          <DetailRow label={t('winners.raffle_label')} value={winner.raffle_title} />
          <DetailRow label={t('winners.winner_label')} value={winner.user_email} />
          <DetailRow label={t('winners.prizeAmount')} value={<span className="font-semibold text-green-600">{formatCurrency(winner.prize_amount)}</span>} />
          <DetailRow label={t('winners.status_label')} value={
            <Badge variant={winner.prize_paid ? 'success' : 'warning'}>
              {winner.prize_paid ? t('winners.paid') : t('winners.unpaid')}
            </Badge>
          } />
          {winner.payment_reference && <DetailRow label={t('winners.paymentRef')} value={<span className="font-mono text-xs">{winner.payment_reference}</span>} />}
          {winner.payment_date && <DetailRow label={t('winners.paidOn')} value={new Date(winner.payment_date).toLocaleString()} />}
          <DetailRow label={t('winners.wonOn')} value={new Date(winner.created_at).toLocaleString()} />
        </div>
      )}

      {/* Winning Ticket */}
      {tab === 'ticket' && (
        <div>
          {ticketLoading ? (
            <div className="space-y-3 py-4">{Array.from({ length: 5 }).map((_, i) => <Skeleton key={i} className="h-5 w-full" />)}</div>
          ) : ticket ? (
            <div>
              <div className="mb-4 flex items-center gap-3 rounded-lg border bg-muted/30 px-4 py-3">
                <Ticket className="h-8 w-8 text-primary" />
                <div>
                  <p className="text-2xl font-bold tabular-nums">#{ticket.ticket_number}</p>
                  <p className="text-xs text-muted-foreground">{t('winners.winningTicketNumber')}</p>
                </div>
              </div>
              <DetailRow label={t('winners.ticketId')} value={ticket.ticket_id} mono />
              <DetailRow label={t('winners.user')} value={ticket.user_email} />
              <DetailRow label={t('winners.drawTime')} value={ticket.draw_timestamp ? new Date(ticket.draw_timestamp).toLocaleString() : '—'} />
            </div>
          ) : (
            <p className="py-6 text-center text-sm text-muted-foreground">{t('winners.failedToLoadTicket')}</p>
          )}
        </div>
      )}

      {/* Verification */}
      {tab === 'verification' && (
        <div>
          {verLoading ? (
            <div className="space-y-3 py-4">{Array.from({ length: 6 }).map((_, i) => <Skeleton key={i} className="h-5 w-full" />)}</div>
          ) : verification ? (
            <div>
              <div className="mb-4 flex items-center gap-2 rounded-lg border border-green-200 bg-green-50 px-4 py-3 dark:border-green-800 dark:bg-green-950">
                <CheckCircle className="h-5 w-5 text-green-600" />
                <p className="text-sm font-medium text-green-700 dark:text-green-300">{t('winners.drawVerificationAvailable')}</p>
              </div>

              {([
                { key: 'commit_hash',      labelKey: 'winners.commitHash'      },
                { key: 'server_seed_hash', labelKey: 'winners.serverSeedHash' },
                { key: 'revealed_seed',    labelKey: 'winners.revealedSeed'    },
                { key: 'combined_hash',    labelKey: 'winners.combinedHash'    },
              ] as const).map(({ key, labelKey }) => {
                const value = verification[key as keyof typeof verification] as string
                return (
                  <div key={key} className="group grid grid-cols-[130px_1fr_auto] gap-2 border-b py-2.5 last:border-0">
                    <span className="text-sm text-muted-foreground">{t(labelKey)}</span>
                    <span className="truncate font-mono text-xs">{value}</span>
                    <button onClick={() => copy(value, key)} title={t('winners.copyToClipboard')}
                      className="shrink-0 opacity-0 transition-opacity group-hover:opacity-100">
                      {copied === key
                        ? <CheckCircle className="h-3.5 w-3.5 text-green-500" />
                        : <Copy className="h-3.5 w-3.5 text-muted-foreground" />}
                    </button>
                  </div>
                )
              })}

              <DetailRow label={t('winners.winningNumber')} value={verification.winning_number} />
              <DetailRow label={t('winners.drawId')} value={verification.draw_id} mono />
              <DetailRow label={t('winners.drawTime')} value={new Date(verification.draw_timestamp).toLocaleString()} />
              <DetailRow label={t('winners.ticketId')} value={verification.winning_ticket_id} mono />

              {verification.verification_url && (
                <div className="mt-4 flex justify-end">
                  <a href={verification.verification_url} target="_blank" rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 text-sm text-primary underline-offset-4 hover:underline">
                    <ExternalLink className="h-3.5 w-3.5" />
                    {t('winners.verifyIndependently')}
                  </a>
                </div>
              )}
            </div>
          ) : (
            <p className="py-6 text-center text-sm text-muted-foreground">{t('winners.failedToLoadVerification')}</p>
          )}
        </div>
      )}
    </Modal>
  )
}

export function Component() {
  const { t } = useTranslation()
  usePageTitle(t('winners.myWinnings'))
  const { user } = useAuth()
  const [selectedWinner, setSelectedWinner] = useState<WinnerDetail | null>(null)

  const { data, isLoading, error } = useQuery({
    queryKey: ['my-winners'],
    queryFn: () => winnerApi.list(PAGE_SIZE),
  })

  const winners = (data?.winners ?? []).filter(w => w.user_id === user?.id)

  const subtitle = winners.length > 0
    ? t('winners.wonCount_other', { count: winners.length })
    : t('winners.trackPrizes')

  return (
    <div className="mx-auto max-w-6xl space-y-6 p-4 sm:p-6">
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-yellow-100 dark:bg-yellow-900/30">
          <Trophy className="h-5 w-5 text-yellow-600 dark:text-yellow-400" />
        </div>
        <div>
          <h1 className="text-2xl font-bold">{t('winners.myWinnings')}</h1>
          <p className="text-sm text-muted-foreground">{subtitle}</p>
        </div>
      </div>

      {error && <ErrorMessage message={t('winners.failedToLoad')} />}

      <div className="space-y-3">
        {isLoading ? (
          Array.from({ length: 4 }).map((_, i) => (
            <Card key={i}><CardContent className="p-4">
              <div className="flex items-center justify-between">
                <div className="space-y-2"><Skeleton className="h-5 w-40" /><Skeleton className="h-4 w-24" /></div>
                <Skeleton className="h-8 w-20" />
              </div>
            </CardContent></Card>
          ))
        ) : winners.length === 0 ? (
          <Card><CardContent className="flex flex-col items-center py-16">
            <Trophy className="mb-3 h-12 w-12 text-muted-foreground/30" />
            <p className="text-lg font-medium text-muted-foreground">{t('winners.noWinningsYet')}</p>
            <p className="mt-1 text-sm text-muted-foreground/60">{t('winners.noWinningsDesc')}</p>
          </CardContent></Card>
        ) : (
          winners.map(w => (
            <Card key={w.id} className="cursor-pointer transition-all hover:shadow-md hover:ring-1 hover:ring-primary/20"
              onClick={() => setSelectedWinner(w)}>
              <CardContent className="p-4">
                <div className="flex items-center justify-between gap-4">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <p className="truncate text-sm font-medium">{w.raffle_title}</p>
                      <Badge variant={w.prize_paid ? 'success' : 'warning'} className="shrink-0">
                        {w.prize_paid ? t('winners.paid') : t('winners.unpaid')}
                      </Badge>
                    </div>
                    <div className="mt-1.5 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
                      <span className="inline-flex items-center gap-1">
                        <Hash className="h-3 w-3" />{t('winners.ticketHash')}{w.ticket_number}
                      </span>
                      <span className="inline-flex items-center gap-1">
                        <Calendar className="h-3 w-3" />
                        {new Date(w.created_at).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })}
                      </span>
                      {w.payment_reference && (
                        <span className="inline-flex items-center gap-1 font-mono">
                          {t('winners.refPrefix')}{w.payment_reference}
                        </span>
                      )}
                    </div>
                  </div>
                  <div className="shrink-0 text-right">
                    <p className="text-lg font-bold text-green-600 dark:text-green-400">{formatCurrency(w.prize_amount)}</p>
                    <p className="text-xs text-muted-foreground">{t('winners.prizeLabel')}</p>
                  </div>
                </div>
              </CardContent>
            </Card>
          ))
        )}
      </div>

      {selectedWinner && <WinnerDetailModal winner={selectedWinner} onClose={() => setSelectedWinner(null)} />}
    </div>
  )
}
