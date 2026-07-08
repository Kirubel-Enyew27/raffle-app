import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Ticket } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { cn, formatCurrency } from '@/lib/utils'
import { dashboardApi } from './api'

const statusColor: Record<string, string> = {
  active: 'bg-green-100 text-green-700',
  scheduled: 'bg-blue-100 text-blue-700',
  completed: 'bg-muted text-muted-foreground',
  cancelled: 'bg-red-100 text-red-700',
}

export function ActiveRaffles() {
  const { t } = useTranslation()
  const { data, isLoading, error } = useQuery({
    queryKey: ['raffles', 'active'],
    queryFn: () => dashboardApi.raffles(6),
  })

  const raffles = data?.raffles?.filter(r => r.status === 'active' || r.status === 'scheduled') ?? []

  const statusLabel: Record<string, string> = {
    active: t('raffles.status.active'),
    scheduled: t('raffles.status.upcoming'),
    completed: t('raffles.status.completed'),
    cancelled: t('raffles.status.cancelled'),
  }

  return (
    <Card className="col-span-full">
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">{t('dashboard.activeRaffles')}</CardTitle>
        <Link to="/raffles" className="text-xs text-primary hover:underline">{t('dashboard.viewAll')}</Link>
      </CardHeader>
      <CardContent>
        {isLoading && (
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} className="h-28 rounded-lg" />)}
          </div>
        )}
        {error && <p className="text-sm text-destructive">{t('common.error')}</p>}
        {!isLoading && raffles.length === 0 && (
          <p className="py-6 text-center text-sm text-muted-foreground">{t('raffles.noRaffles')}</p>
        )}
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {raffles.map(r => (
            <Link key={r.id} to={`/raffles/${r.id}`}
              className="group rounded-lg border p-4 transition-colors hover:bg-accent">
              <div className="mb-2 flex items-start justify-between gap-2">
                <p className="font-medium leading-tight group-hover:text-primary line-clamp-1">{r.title}</p>
                <span className={cn('shrink-0 rounded-full px-2 py-0.5 text-xs font-medium', statusColor[r.status])}>
                  {statusLabel[r.status] || r.status}
                </span>
              </div>
              <p className="mb-3 text-xs text-muted-foreground line-clamp-2">{r.description}</p>
              <div className="flex items-center justify-between text-xs">
                <span className="flex items-center gap-1 text-muted-foreground">
                  <Ticket className="h-3 w-3" />{r.sold_tickets}/{r.total_tickets}
                </span>
                <span className="font-semibold">{formatCurrency(r.ticket_price)}</span>
              </div>
            </Link>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}
