import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { TrendingUp, DollarSign, Ticket, Users, ArrowDownCircle, ArrowUpCircle, Trophy } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { ReportsControls, exportCSV, type Range } from './ReportsControls'
import { adminApi } from './api'
import { formatCurrency } from '@/lib/utils'

const usd = (n: number) => formatCurrency(n)

interface KpiProps { label: string; value: string; icon: React.ElementType; sub?: string; loading?: boolean; green?: boolean }

function KpiCard({ label, value, icon: Icon, sub, loading, green }: KpiProps) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between pb-1">
        <CardTitle className="text-sm font-medium text-muted-foreground">{label}</CardTitle>
        <Icon className="h-4 w-4 text-muted-foreground" />
      </CardHeader>
      <CardContent>
        {loading ? <Skeleton className="h-8 w-28" /> : (
          <p className={`text-2xl font-bold ${green ? 'text-green-600' : ''}`}>{value}</p>
        )}
        {sub && <p className="mt-0.5 text-xs text-muted-foreground">{sub}</p>}
      </CardContent>
    </Card>
  )
}

interface Props { range: Range; onRange: (r: Range) => void }

export function ProfitSummaryReport({ range, onRange }: Props) {
  const { t } = useTranslation()
  const { data, isLoading } = useQuery({
    queryKey: ['reports', 'profit', range],
    queryFn: () => adminApi.profit(range.from, range.to),
  })

  function doExport() {
    if (!data) return
    exportCSV('profit-summary.csv',
      ['Metric', 'Value'],
      [
        [t('admin.netProfit'),        data.net_profit],
        [t('admin.ticketRevenue'),    data.total_ticket_revenue],
        [t('admin.depositVolume'),    data.total_deposit_volume],
        [t('admin.withdrawVolume'),   data.total_withdraw_volume],
        [t('admin.prizePaid'),        data.total_prize_paid],
        [t('admin.ticketsSoldKpi'),   data.total_tickets_sold],
        [t('admin.winnersKpi'),       data.total_winners],
        [t('admin.activeUsersKpi'),   data.total_active_users],
      ],
    )
  }

  return (
    <div className="space-y-4">
      <ReportsControls range={range} onRange={onRange} onExport={data ? doExport : undefined} />
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <KpiCard label={t('admin.netProfit')} value={usd(data?.net_profit ?? 0)} icon={TrendingUp} loading={isLoading}
          sub={t('admin.revenueLabel', { value: usd(data?.total_ticket_revenue ?? 0) })} green={(data?.net_profit ?? 0) >= 0} />
        <KpiCard label={t('admin.ticketRevenue')} value={usd(data?.total_ticket_revenue ?? 0)} icon={DollarSign} loading={isLoading} />
        <KpiCard label={t('admin.depositVolume')} value={usd(data?.total_deposit_volume ?? 0)} icon={ArrowDownCircle} loading={isLoading} />
        <KpiCard label={t('admin.withdrawVolume')} value={usd(data?.total_withdraw_volume ?? 0)} icon={ArrowUpCircle} loading={isLoading} />
        <KpiCard label={t('admin.prizePaid')} value={usd(data?.total_prize_paid ?? 0)} icon={Trophy} loading={isLoading} />
        <KpiCard label={t('admin.ticketsSoldKpi')} value={(data?.total_tickets_sold ?? 0).toLocaleString()} icon={Ticket} loading={isLoading} />
        <KpiCard label={t('admin.winnersKpi')} value={(data?.total_winners ?? 0).toLocaleString()} icon={Trophy} loading={isLoading} />
        <KpiCard label={t('admin.activeUsersKpi')} value={(data?.total_active_users ?? 0).toLocaleString()} icon={Users} loading={isLoading} />
      </div>
    </div>
  )
}
