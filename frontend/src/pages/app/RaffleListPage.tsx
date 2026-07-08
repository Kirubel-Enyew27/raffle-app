import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Ticket, Search } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { usePageTitle } from '@/hooks/usePageTitle'
import { ticketApi } from '@/features/tickets/api'
import { ErrorMessage } from '@/components/ui/error-message'
import { cn, formatCurrency } from '@/lib/utils'
import type { Raffle } from '@/types/api'

const PAGE_SIZE = 12

const statusVariant: Record<string, 'success' | 'secondary' | 'outline' | 'warning'> = {
  active: 'success', scheduled: 'secondary', completed: 'outline', cancelled: 'warning',
}

function RaffleCard({ raffle, t }: { raffle: Raffle; t: (key: string) => string }) {
  const pct = Math.round((raffle.sold_tickets / raffle.total_tickets) * 100)
  const remaining = raffle.total_tickets - raffle.sold_tickets
  
  const statusLabel: Record<string, string> = {
    active: t('raffles.status.active'),
    scheduled: t('raffles.status.upcoming'),
    completed: t('raffles.status.completed'),
    cancelled: t('raffles.status.cancelled'),
  }
  
  return (
    <Link to={`/raffles/${raffle.id}`}>
      <Card className="h-full transition-shadow hover:shadow-md">
        <CardHeader className="pb-2">
          <div className="flex items-start justify-between gap-2">
            <CardTitle className="line-clamp-2 text-base leading-snug">{raffle.title}</CardTitle>
            <Badge variant={statusVariant[raffle.status] ?? 'outline'} className="shrink-0">
              {statusLabel[raffle.status] || raffle.status}
            </Badge>
          </div>
        </CardHeader>
        <CardContent className="space-y-3">
          <p className="line-clamp-2 text-sm text-muted-foreground">{raffle.description}</p>

          {/* availability bar */}
          <div>
            <div className="mb-1 flex justify-between text-xs text-muted-foreground">
              <span>{raffle.sold_tickets.toLocaleString()} {t('raffles.soldTickets')}</span>
              <span>{remaining.toLocaleString()} {t('raffles.ticketsAvailable')}</span>
            </div>
            <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
              <div
                className={cn('h-full rounded-full transition-all',
                  pct >= 90 ? 'bg-red-500' : pct >= 60 ? 'bg-yellow-500' : 'bg-green-500')}
                style={{ width: `${pct}%` }}
              />
            </div>
          </div>

          <div className="flex items-center justify-between">
            <div>
              <p className="text-lg font-bold">{formatCurrency(raffle.ticket_price)}</p>
              <p className="text-xs text-muted-foreground">{t('raffles.ticketPrice')}</p>
            </div>
            <div className="text-right">
              <p className="font-semibold text-green-600">
                {formatCurrency(raffle.prize_pool)}
              </p>
              <p className="text-xs text-muted-foreground">{t('raffles.prizePool')}</p>
            </div>
          </div>

          <p className="text-xs text-muted-foreground">
            {t('raffles.drawDate')}: {new Date(raffle.draw_date).toLocaleDateString('en-US', {
              month: 'short', day: 'numeric', year: 'numeric',
            })}
          </p>
        </CardContent>
      </Card>
    </Link>
  )
}

function RaffleCardSkeleton() {
  return (
    <Card>
      <CardHeader className="pb-2"><Skeleton className="h-5 w-3/4" /></CardHeader>
      <CardContent className="space-y-3">
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-2 w-full" />
        <div className="flex justify-between">
          <Skeleton className="h-7 w-16" />
          <Skeleton className="h-7 w-20" />
        </div>
      </CardContent>
    </Card>
  )
}

export function Component() {
  const { t } = useTranslation()
  usePageTitle(t('raffles.title'))
  const [page, setPage] = useState(0)
  const [status, setStatus] = useState('active')
  const [search, setSearch] = useState('')

  // The backend ListRaffles endpoint does not filter by status server-side,
  // so we fetch all raffles in one go and filter entirely on the client.
  const { data, isLoading, error } = useQuery({
    queryKey: ['raffles', 'all'],
    queryFn: () => ticketApi.listRaffles({ limit: 500, offset: 0 }),
  })

  const filtered = (data?.raffles ?? [])
    .filter(r => !status || r.status === status)
    .filter(r => !search || r.title.toLowerCase().includes(search.toLowerCase()))
    .sort((a, b) => new Date(b.draw_date).getTime() - new Date(a.draw_date).getTime())

  const totalPages = Math.ceil(filtered.length / PAGE_SIZE)
  const raffles = filtered.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE)

  // Reset to page 0 whenever filters change
  const handleStatusChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    setStatus(e.target.value)
    setPage(0)
  }
  const handleSearchChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setSearch(e.target.value)
    setPage(0)
  }

  return (
    <div className="mx-auto max-w-6xl space-y-6 p-4 sm:p-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold">{t('raffles.title')}</h1>
          <p className="text-sm text-muted-foreground">{t('raffles.allRaffles')}</p>
        </div>
      </div>

      {/* filters */}
      <div className="flex flex-col gap-3 sm:flex-row">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder={t('common.search')}
            className="pl-9"
            value={search}
            onChange={handleSearchChange}
          />
        </div>
        <Select
          value={status}
          onChange={handleStatusChange}
          className="w-full sm:w-40"
        >
          <option value="">{t('raffles.allRaffles')}</option>
          <option value="active">{t('raffles.status.active')}</option>
          <option value="scheduled">{t('raffles.status.upcoming')}</option>
          <option value="completed">{t('raffles.status.completed')}</option>
        </Select>
      </div>

      {error && <ErrorMessage message={t('common.error')} />}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {isLoading
          ? Array.from({ length: 6 }).map((_, i) => <RaffleCardSkeleton key={i} />)
          : raffles.length === 0
            ? (
              <div className="col-span-full py-16 text-center">
                <Ticket className="mx-auto mb-3 h-10 w-10 text-muted-foreground/40" />
                <p className="text-muted-foreground">{t('raffles.noRaffles')}</p>
              </div>
            )
            : raffles.map(r => <RaffleCard key={r.id} raffle={r} t={t} />)
        }
      </div>

      {totalPages > 1 && (
        <div className="flex items-center justify-between text-sm">
          <span className="text-muted-foreground">
            {t('common.page')} {page + 1} {t('common.of')} {totalPages}
          </span>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" disabled={page === 0}
              onClick={() => setPage(p => p - 1)}>{t('common.previous')}</Button>
            <Button variant="outline" size="sm" disabled={page >= totalPages - 1}
              onClick={() => setPage(p => p + 1)}>{t('common.next')}</Button>
          </div>
        </div>
      )}
    </div>
  )
}
