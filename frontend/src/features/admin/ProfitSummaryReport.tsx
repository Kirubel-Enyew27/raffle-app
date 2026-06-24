import { useQuery } from '@tanstack/react-query'
import { TrendingUp, DollarSign, Ticket, Users, ArrowDownCircle, ArrowUpCircle, Trophy } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { ReportsControls, exportCSV, type Range } from './ReportsControls'
import { adminApi } from './api'

const usd = (n: number) => n.toLocaleString('en-US', { style: 'currency', currency: 'USD', maximumFractionDigits: 0 })

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
  const { data, isLoading } = useQuery({
    queryKey: ['reports', 'profit', range],
    queryFn: () => adminApi.profit(range.from, range.to),
  })

  function doExport() {
    if (!data) return
    exportCSV('profit-summary.csv',
      ['Metric', 'Value'],
      [
        ['Net Profit', data.net_profit],
        ['Ticket Revenue', data.total_ticket_revenue],
        ['Deposit Volume', data.total_deposit_volume],
        ['Withdraw Volume', data.total_withdraw_volume],
        ['Prize Paid', data.total_prize_paid],
        ['Tickets Sold', data.total_tickets_sold],
        ['Winners', data.total_winners],
        ['Active Users', data.total_active_users],
      ],
    )
  }

  return (
    <div className="space-y-4">
      <ReportsControls range={range} onRange={onRange} onExport={data ? doExport : undefined} />

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <KpiCard label="Net Profit" value={usd(data?.net_profit ?? 0)} icon={TrendingUp} loading={isLoading}
          sub={`Revenue: ${usd(data?.total_ticket_revenue ?? 0)}`} green={(data?.net_profit ?? 0) >= 0} />
        <KpiCard label="Ticket Revenue" value={usd(data?.total_ticket_revenue ?? 0)} icon={DollarSign} loading={isLoading} />
        <KpiCard label="Deposit Volume" value={usd(data?.total_deposit_volume ?? 0)} icon={ArrowDownCircle} loading={isLoading} />
        <KpiCard label="Withdraw Volume" value={usd(data?.total_withdraw_volume ?? 0)} icon={ArrowUpCircle} loading={isLoading} />
        <KpiCard label="Prize Paid" value={usd(data?.total_prize_paid ?? 0)} icon={Trophy} loading={isLoading} />
        <KpiCard label="Tickets Sold" value={(data?.total_tickets_sold ?? 0).toLocaleString()} icon={Ticket} loading={isLoading} />
        <KpiCard label="Winners" value={(data?.total_winners ?? 0).toLocaleString()} icon={Trophy} loading={isLoading} />
        <KpiCard label="Active Users" value={(data?.total_active_users ?? 0).toLocaleString()} icon={Users} loading={isLoading} />
      </div>
    </div>
  )
}
