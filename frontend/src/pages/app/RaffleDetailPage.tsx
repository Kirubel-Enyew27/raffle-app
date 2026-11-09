import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { ChevronLeft, Minus, Plus, Ticket, Trophy, Calendar, Users } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { ticketApi } from '@/features/tickets/api'
import { walletApi } from '@/features/wallet/api'
import { usePageTitle } from '@/hooks/usePageTitle'
import { ErrorMessage } from '@/components/ui/error-message'
import { useCountdown } from '@/hooks/useCountdown'
import { cn, formatCurrency } from '@/lib/utils'

const statusVariant: Record<string, 'success' | 'secondary' | 'outline' | 'warning'> = {
  active: 'success', scheduled: 'secondary', completed: 'outline', cancelled: 'warning',
}

function CountdownBadge({ drawDate }: { drawDate: string }) {
  const { t } = useTranslation()
  const { days, hours, minutes, expired } = useCountdown(drawDate)
  if (expired) return <span className="text-sm text-muted-foreground">{t('raffles.drawCompleted', 'Draw completed')}</span>
  return (
    <span className="text-sm font-medium tabular-nums">
      {days > 0 ? `${days}d ` : ''}{String(hours).padStart(2, '0')}h {String(minutes).padStart(2, '0')}m
    </span>
  )
}

// Success state shown inline after purchase
function PurchaseSuccess({ tickets, totalSpent, onDismiss }: {
  tickets: { ticket_number: number }[]
  totalSpent: number
  onDismiss: () => void
}) {
  const { t } = useTranslation()
  return (
    <div className="rounded-lg border border-success/30 bg-success/10 p-4">
      <div className="mb-3 flex items-center gap-2 text-success">
        <Ticket className="h-5 w-5" />
        <p className="font-semibold">{t('raffles.ticketsPurchased', 'Tickets purchased!')}</p>
      </div>
      <div className="mb-3 flex flex-wrap gap-2">
        {tickets.map(tItem => (
          <span key={tItem.ticket_number}
            className="rounded-md border border-success/30 bg-surface px-2 py-1 text-sm font-mono font-medium">
            #{tItem.ticket_number}
          </span>
        ))}
      </div>
      <p className="mb-3 text-sm text-success">
        {t('raffles.totalCharged', 'Total charged:')} <strong>{formatCurrency(totalSpent)}</strong>
      </p>
      <Button variant="outline" size="sm" onClick={onDismiss}>{t('raffles.buyMore', 'Buy more')}</Button>
    </div>
  )
}

export function Component() {
  const { t } = useTranslation()
  usePageTitle(t('raffles.raffleDetail', 'Raffle detail'))
  const { id } = useParams<{ id: string }>()
  const qc = useQueryClient()
  const [quantity, setQuantity] = useState(1)
  const [purchaseResult, setPurchaseResult] = useState<{ tickets: { ticket_number: number }[]; total_spent: number } | null>(null)

  const { data: raffle, isLoading, error } = useQuery({
    queryKey: ['raffle', id],
    queryFn: () => ticketApi.getRaffle(id!),
    refetchInterval: 30_000, // poll every 30s for availability
  })

  const { data: wallet } = useQuery({
    queryKey: ['wallet'],
    queryFn: walletApi.balance,
    staleTime: 0,
  })

  const { mutate: purchase, isPending, error: purchaseError } = useMutation({
    mutationFn: () => ticketApi.purchase(id!, quantity),
    onSuccess: (result) => {
      setPurchaseResult({ tickets: result.tickets as { ticket_number: number }[], total_spent: result.total_spent })
      setQuantity(1)
      qc.invalidateQueries({ queryKey: ['wallet'] })
      qc.invalidateQueries({ queryKey: ['raffle', id] })
      qc.invalidateQueries({ queryKey: ['raffles'] })
    },
  })

  const remaining = raffle ? raffle.total_tickets - raffle.sold_tickets : 0
  const cost = raffle ? quantity * raffle.ticket_price : 0
  const canAfford = wallet ? wallet.balance >= cost : false
  const maxQty = Math.min(10, remaining)
  const isActive = raffle?.status === 'active'
  const pct = raffle ? Math.round((raffle.sold_tickets / raffle.total_tickets) * 100) : 0

  if (isLoading) {
    return (
      <div className="mx-auto max-w-3xl space-y-4 p-4 sm:p-6">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-6 w-24" />
        <Skeleton className="h-40 w-full" />
        <Skeleton className="h-60 w-full" />
      </div>
    )
  }

  if (error || !raffle) {
    return (
      <div className="mx-auto max-w-3xl p-4 sm:p-6">
        <p className="text-destructive">{t('raffles.raffleNotFound', 'Raffle not found.')}</p>
        <Link to="/raffles" className="mt-2 inline-block text-sm text-primary hover:underline">
          ← {t('raffles.backToRaffles', 'Back to raffles')}
        </Link>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-3xl space-y-6 p-4 sm:p-6">
      <Link to="/raffles" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ChevronLeft className="h-4 w-4" /> {t('raffles.backToRaffles', 'Back to raffles')}
      </Link>

      {/* header */}
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold">{raffle.title}</h1>
          <p className="mt-1 text-muted-foreground">{raffle.description}</p>
        </div>
        <Badge variant={statusVariant[raffle.status] ?? 'outline'} className="text-sm">
          {t(`raffles.status.${raffle.status}`, raffle.status)}
        </Badge>
      </div>

      {/* stats */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        {[
          { icon: Trophy, label: t('raffles.prizePoolLabel', 'Prize Pool'), value: formatCurrency(raffle.prize_pool) },
          { icon: Ticket, label: t('raffles.ticketPriceLabel', 'Ticket Price'), value: formatCurrency(raffle.ticket_price) },
          { icon: Users, label: t('raffles.ticketsLeft', 'Tickets Left'), value: remaining.toLocaleString() },
          { icon: Calendar, label: t('raffles.drawIn', 'Draw In'), value: <CountdownBadge drawDate={raffle.draw_date} /> },
        ].map(({ icon: Icon, label, value }) => (
          <Card key={label}>
            <CardContent className="flex flex-col gap-1 p-4">
              <Icon className="h-4 w-4 text-muted-foreground" />
              <p className="text-xs text-muted-foreground">{label}</p>
              <p className="font-semibold">{value}</p>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* availability bar */}
      <Card>
        <CardContent className="p-4">
          <div className="mb-1.5 flex justify-between text-sm">
            <span className="text-muted-foreground">{raffle.sold_tickets.toLocaleString()} {t('raffles.soldTickets', 'sold')}</span>
            <span className="font-medium">{pct}{t('raffles.percentFull', '% full')}</span>
          </div>
          <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
            <div
              className={cn('h-full rounded-full transition-all',
                pct >= 90 ? 'bg-destructive' : pct >= 60 ? 'bg-warning' : 'bg-success')}
              style={{ width: `${pct}%` }}
            />
          </div>
          <p className="mt-1.5 text-xs text-muted-foreground">
            {t('raffles.ticketsRemaining', { remaining: remaining.toLocaleString(), total: raffle.total_tickets.toLocaleString() })}
          </p>
        </CardContent>
      </Card>

      {/* purchase panel */}
      {isActive && remaining > 0 ? (
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">{t('raffles.purchaseTickets', 'Purchase Tickets')}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {purchaseResult ? (
              <PurchaseSuccess
                tickets={purchaseResult.tickets}
                totalSpent={purchaseResult.total_spent}
                onDismiss={() => setPurchaseResult(null)}
              />
            ) : (
              <>
                {purchaseError && <ErrorMessage message={purchaseError.message} />}

                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-sm font-medium">{t('raffles.quantity', 'Quantity')}</p>
                    <p className="text-xs text-muted-foreground">{t('raffles.maxPerPurchase', 'Max 10 per purchase')}</p>
                  </div>
                  <div className="flex items-center gap-3">
                    <Button variant="outline" size="icon" className="h-8 w-8"
                      disabled={quantity <= 1}
                      onClick={() => setQuantity(q => Math.max(1, q - 1))}>
                      <Minus className="h-3 w-3" />
                    </Button>
                    <span className="w-6 text-center text-lg font-bold tabular-nums">{quantity}</span>
                    <Button variant="outline" size="icon" className="h-8 w-8"
                      disabled={quantity >= maxQty}
                      onClick={() => setQuantity(q => Math.min(maxQty, q + 1))}>
                      <Plus className="h-3 w-3" />
                    </Button>
                  </div>
                </div>

                {/* cost summary */}
                <div className="rounded-lg bg-muted/50 p-3 space-y-1.5 text-sm">
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">{quantity} × {formatCurrency(raffle.ticket_price)}</span>
                    <span className="font-medium">{formatCurrency(cost)}</span>
                  </div>
                  {wallet && (
                    <div className="flex justify-between border-t pt-1.5">
                      <span className="text-muted-foreground">{t('raffles.yourBalance', 'Your balance')}</span>
                      <span className={cn('font-medium', !canAfford && 'text-destructive')}>
                        {formatCurrency(wallet.balance)}
                      </span>
                    </div>
                  )}
                </div>

                {wallet && !canAfford && (
                  <p className="text-sm text-destructive">
                    {t('raffles.insufficientBalance', 'Insufficient balance.')}{' '}
                    <Link to="/wallet" className="underline">{t('raffles.topUpWallet', 'Top up your wallet')}</Link>
                  </p>
                )}

                <Button
                  className="w-full"
                  disabled={isPending || !canAfford || quantity < 1}
                  onClick={() => purchase()}
                >
                  {isPending ? t('raffles.processing', 'Processing…') : t('raffles.buyButton', { n: quantity, plural: quantity > 1 ? 's' : '', cost: formatCurrency(cost) })}
                </Button>
              </>
            )}
          </CardContent>
        </Card>
      ) : (
        <Card>
          <CardContent className="p-6 text-center text-muted-foreground">
            {raffle.status === 'completed'
              ? t('raffles.raffleEnded', 'This raffle has ended.')
              : raffle.status === 'cancelled'
                ? t('raffles.raffleCancelled', 'This raffle was cancelled.')
                : remaining === 0
                  ? t('raffles.raffleSoldOut', 'This raffle is sold out.')
                  : t('raffles.ticketSalesNotStarted', 'Ticket sales have not started yet.')}
          </CardContent>
        </Card>
      )}
    </div>
  )
}
