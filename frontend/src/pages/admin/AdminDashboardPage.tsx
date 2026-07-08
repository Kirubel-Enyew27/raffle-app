import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import {
  AreaChart, Area, BarChart, Bar,
  XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend,
} from 'recharts'
import { TrendingUp, Users, Ticket, DollarSign, Activity, Calendar } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Select } from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { adminApi, defaultRange } from '@/features/admin/api'
import { usePageTitle } from '@/hooks/usePageTitle'
import { cn, formatCurrency } from '@/lib/utils'

const usd = (n: number) => formatCurrency(n)
const num = (n: number) => n.toLocaleString()

interface Range { from: string; to: string }

function DateRangePicker({ value, onChange }: { value: Range; onChange: (r: Range) => void }) {
  const preset = (days: number) => {
    const to = new Date()
    const from = new Date(to); from.setDate(from.getDate() - days + 1)
    onChange({ from: from.toISOString().split('T')[0], to: to.toISOString().split('T')[0] })
  }
  return (
    <div className="flex flex-wrap items-center gap-2">
      {[7, 30, 90].map(d => (
        <Button key={d} variant="outline" size="sm" onClick={() => preset(d)}>{d}d</Button>
      ))}
      <input type="date" value={value.from} max={value.to}
        onChange={e => onChange({ ...value, from: e.target.value })}
        className="h-8 rounded-md border border-input bg-background px-2 text-sm" />
      <span className="text-muted-foreground">–</span>
      <input type="date" value={value.to} min={value.from}
        onChange={e => onChange({ ...value, to: e.target.value })}
        className="h-8 rounded-md border border-input bg-background px-2 text-sm" />
    </div>
  )
}

function StatCard({ label, value, icon: Icon, sub, loading, className }: {
  label: string; value: string | number; icon: React.ElementType; sub?: string; loading?: boolean; className?: string
}) {
  return (
    <Card className={className}>
      <CardHeader className="flex flex-row items-center justify-between pb-1">
        <CardTitle className="text-sm font-medium text-muted-foreground">{label}</CardTitle>
        <Icon className="h-4 w-4 text-muted-foreground" />
      </CardHeader>
      <CardContent>
        {loading ? <Skeleton className="h-8 w-28" /> : <p className="text-2xl font-bold">{value}</p>}
        {sub && <p className="mt-0.5 text-xs text-muted-foreground">{sub}</p>}
      </CardContent>
    </Card>
  )
}

function RevenueChart({ range, period }: { range: Range; period: string }) {
  const { t } = useTranslation()
  const { data, isLoading } = useQuery({
    queryKey: ['admin', 'revenue', range, period],
    queryFn: () => adminApi.revenue(range.from, range.to, period),
  })
  const rows = data?.items ?? []
  return (
    <Card className="col-span-full lg:col-span-2">
      <CardHeader className="pb-2"><CardTitle className="text-base">{t('admin.revenueProfit')}</CardTitle></CardHeader>
      <CardContent>
        {isLoading ? <Skeleton className="h-56 w-full" />
          : rows.length === 0 ? <p className="py-10 text-center text-sm text-muted-foreground">{t('admin.noDataPeriod')}</p>
          : (
            <ResponsiveContainer width="100%" height={220}>
              <AreaChart data={rows} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
                <defs>
                  <linearGradient id="gRevenue" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#6366f1" stopOpacity={0.3} />
                    <stop offset="95%" stopColor="#6366f1" stopOpacity={0} />
                  </linearGradient>
                  <linearGradient id="gProfit" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#22c55e" stopOpacity={0.3} />
                    <stop offset="95%" stopColor="#22c55e" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
                <XAxis dataKey="period" tick={{ fontSize: 11 }} tickLine={false} axisLine={false} />
                <YAxis tick={{ fontSize: 11 }} tickLine={false} axisLine={false} tickFormatter={v => `${(v/1000).toFixed(0)}k Br`} />
                <Tooltip formatter={(v) => usd(Number(v))} />
                <Legend />
                <Area type="monotone" dataKey="ticket_revenue" name={t('admin.ticketRevenue')} stroke="#6366f1" fill="url(#gRevenue)" strokeWidth={2} dot={false} />
                <Area type="monotone" dataKey="profit" name={t('reports.profit')} stroke="#22c55e" fill="url(#gProfit)" strokeWidth={2} dot={false} />
              </AreaChart>
            </ResponsiveContainer>
          )}
      </CardContent>
    </Card>
  )
}

function TicketSalesChart({ range, period }: { range: Range; period: string }) {
  const { t } = useTranslation()
  const { data, isLoading } = useQuery({
    queryKey: ['admin', 'tickets', range, period],
    queryFn: () => adminApi.ticketSales(range.from, range.to, period),
  })
  const rows = data?.items ?? []
  return (
    <Card>
      <CardHeader className="pb-2"><CardTitle className="text-base">{t('admin.ticketSalesChart')}</CardTitle></CardHeader>
      <CardContent>
        {isLoading ? <Skeleton className="h-56 w-full" />
          : rows.length === 0 ? <p className="py-10 text-center text-sm text-muted-foreground">{t('admin.noData')}</p>
          : (
            <ResponsiveContainer width="100%" height={220}>
              <BarChart data={rows} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" vertical={false} />
                <XAxis dataKey="period" tick={{ fontSize: 11 }} tickLine={false} axisLine={false} />
                <YAxis tick={{ fontSize: 11 }} tickLine={false} axisLine={false} />
                <Tooltip />
                <Bar dataKey="tickets_sold" name={t('admin.ticketsSold')} fill="#6366f1" radius={[3, 3, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          )}
      </CardContent>
    </Card>
  )
}

function ActiveUsersChart({ range, period }: { range: Range; period: string }) {
  const { t } = useTranslation()
  const { data, isLoading } = useQuery({
    queryKey: ['admin', 'users', range, period],
    queryFn: () => adminApi.activeUsers(range.from, range.to, period),
  })
  const rows = data?.items ?? []
  return (
    <Card>
      <CardHeader className="pb-2"><CardTitle className="text-base">{t('admin.activeUsersChart')}</CardTitle></CardHeader>
      <CardContent>
        {isLoading ? <Skeleton className="h-56 w-full" />
          : rows.length === 0 ? <p className="py-10 text-center text-sm text-muted-foreground">{t('admin.noData')}</p>
          : (
            <ResponsiveContainer width="100%" height={220}>
              <AreaChart data={rows} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
                <defs>
                  <linearGradient id="gUsers" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#f59e0b" stopOpacity={0.3} />
                    <stop offset="95%" stopColor="#f59e0b" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
                <XAxis dataKey="period" tick={{ fontSize: 11 }} tickLine={false} axisLine={false} />
                <YAxis tick={{ fontSize: 11 }} tickLine={false} axisLine={false} />
                <Tooltip />
                <Area type="monotone" dataKey="active_users" name={t('admin.activeUsersChart')} stroke="#f59e0b" fill="url(#gUsers)" strokeWidth={2} dot={false} />
              </AreaChart>
            </ResponsiveContainer>
          )}
      </CardContent>
    </Card>
  )
}

const STATUS_VARIANT: Record<string, 'success' | 'secondary' | 'outline' | 'warning' | 'destructive'> = {
  active: 'success', scheduled: 'secondary', completed: 'outline', cancelled: 'destructive', draft: 'warning',
}
const PAGE_SIZE = 8

function RaffleTable() {
  const { t } = useTranslation()
  const [page, setPage] = useState(0)
  const [status, setStatus] = useState('')
  const { data, isLoading } = useQuery({
    queryKey: ['admin', 'raffles', page, status],
    queryFn: () => adminApi.raffles(PAGE_SIZE, page * PAGE_SIZE, status || undefined),
  })
  const raffles = data?.raffles ?? []
  const total = data?.total ?? 0
  const totalPages = Math.ceil(total / PAGE_SIZE)

  return (
    <Card className="col-span-full">
      <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-3 pb-3">
        <CardTitle className="text-base">{t('admin.raffles')}</CardTitle>
        <Select value={status} onChange={e => { setStatus(e.target.value); setPage(0) }} className="w-36">
          <option value="">{t('admin.allStatuses')}</option>
          <option value="active">{t('raffles.status.active')}</option>
          <option value="scheduled">{t('raffles.status.upcoming')}</option>
          <option value="completed">{t('raffles.status.completed')}</option>
          <option value="cancelled">{t('raffles.status.cancelled')}</option>
        </Select>
      </CardHeader>
      <CardContent>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-left text-xs text-muted-foreground">
                {[t('admin.raffleTitle'), t('admin.status'), t('admin.ticketsSold'), t('admin.prizePool'), t('admin.drawDate')].map(h => (
                  <th key={h} className="pb-2 pr-4 font-medium">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {isLoading && Array.from({ length: PAGE_SIZE }).map((_, i) => (
                <tr key={i} className="border-b">
                  {Array.from({ length: 5 }).map((_, j) => (
                    <td key={j} className="py-2.5 pr-4"><Skeleton className="h-4 w-full" /></td>
                  ))}
                </tr>
              ))}
              {!isLoading && raffles.length === 0 && (
                <tr><td colSpan={5} className="py-8 text-center text-muted-foreground">{t('admin.noRafflesFound')}</td></tr>
              )}
              {raffles.map(r => (
                <tr key={r.id} className="border-b last:border-0 hover:bg-muted/30 transition-colors">
                  <td className="py-2.5 pr-4">
                    <Link to="/admin/raffles" className="font-medium hover:text-primary line-clamp-1">{r.title}</Link>
                  </td>
                  <td className="py-2.5 pr-4"><Badge variant={STATUS_VARIANT[r.status] ?? 'outline'}>{r.status}</Badge></td>
                  <td className="py-2.5 pr-4 tabular-nums">
                    {num(r.sold_tickets)}/{num(r.total_tickets)}
                    <span className="ml-1 text-xs text-muted-foreground">({Math.round(r.sold_tickets / r.total_tickets * 100)}%)</span>
                  </td>
                  <td className="py-2.5 pr-4 tabular-nums">{usd(r.prize_pool)}</td>
                  <td className="py-2.5 text-muted-foreground">
                    {new Date(r.draw_date).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {totalPages > 1 && (
          <div className="mt-4 flex items-center justify-between text-sm">
            <span className="text-muted-foreground">{page * PAGE_SIZE + 1}–{Math.min((page + 1) * PAGE_SIZE, total)} {t('common.of')} {num(total)}</span>
            <div className="flex gap-2">
              <Button variant="outline" size="sm" disabled={page === 0} onClick={() => setPage(p => p - 1)}>{t('admin.previous')}</Button>
              <Button variant="outline" size="sm" disabled={page >= totalPages - 1} onClick={() => setPage(p => p + 1)}>{t('admin.next')}</Button>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function UpcomingDraws() {
  const { t } = useTranslation()
  const { data, isLoading } = useQuery({
    queryKey: ['admin', 'raffles', 'scheduled-active'],
    queryFn: () => adminApi.raffles(5, 0, 'active'),
  })
  const upcoming = (data?.raffles ?? []).filter(r => r.draw_date)
    .sort((a, b) => new Date(a.draw_date).getTime() - new Date(b.draw_date).getTime())
  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <CardTitle className="text-base">{t('admin.upcomingDraws')}</CardTitle>
          <Calendar className="h-4 w-4 text-muted-foreground" />
        </div>
      </CardHeader>
      <CardContent className="space-y-2.5">
        {isLoading && Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="flex gap-3"><Skeleton className="h-4 flex-1" /><Skeleton className="h-4 w-20" /></div>
        ))}
        {!isLoading && upcoming.length === 0 && (
          <p className="py-4 text-center text-sm text-muted-foreground">{t('admin.noUpcomingDraws')}</p>
        )}
        {upcoming.map(r => {
          const diff = new Date(r.draw_date).getTime() - Date.now()
          const days = Math.max(0, Math.floor(diff / 86_400_000))
          const hours = Math.max(0, Math.floor((diff % 86_400_000) / 3_600_000))
          return (
            <div key={r.id} className="flex items-start justify-between gap-2">
              <div className="min-w-0">
                <p className="truncate text-sm font-medium">{r.title}</p>
                <p className="text-xs text-muted-foreground">
                  {new Date(r.draw_date).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })}
                </p>
              </div>
              <span className={cn('shrink-0 rounded-full px-2 py-0.5 text-xs font-medium tabular-nums',
                days === 0 ? 'bg-red-100 text-red-700' : days <= 3 ? 'bg-yellow-100 text-yellow-700' : 'bg-muted text-muted-foreground')}>
                {days > 0 ? `${days}d` : `${hours}h`}
              </span>
            </div>
          )
        })}
      </CardContent>
    </Card>
  )
}

export function Component() {
  const { t } = useTranslation()
  usePageTitle(t('admin.dashboard'))
  const [range, setRange] = useState(defaultRange)
  const [period, setPeriod] = useState('daily')
  const { data: summary, isLoading: summaryLoading } = useQuery({
    queryKey: ['admin', 'profit', range],
    queryFn: () => adminApi.profit(range.from, range.to),
  })

  return (
    <div className="space-y-6 p-4 sm:p-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold">{t('admin.dashboard')}</h1>
          <p className="text-sm text-muted-foreground">{t('admin.platformOverview')}</p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <Select value={period} onChange={e => setPeriod(e.target.value)} className="w-28">
            <option value="daily">{t('admin.daily')}</option>
            <option value="weekly">{t('admin.weekly')}</option>
            <option value="monthly">{t('admin.monthly')}</option>
          </Select>
          <DateRangePicker value={range} onChange={setRange} />
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-6">
        <StatCard label={t('admin.netProfit')} value={usd(summary?.net_profit ?? 0)}
          icon={TrendingUp} loading={summaryLoading}
          sub={t('admin.revenueLabel', { value: usd(summary?.total_ticket_revenue ?? 0) })} />
        <StatCard label={t('admin.ticketRevenue')} value={usd(summary?.total_ticket_revenue ?? 0)}
          icon={DollarSign} loading={summaryLoading} />
        <StatCard label={t('admin.ticketsSoldKpi')} value={num(summary?.total_tickets_sold ?? 0)}
          icon={Ticket} loading={summaryLoading} />
        <StatCard label={t('admin.activeUsersKpi')} value={num(summary?.total_active_users ?? 0)}
          icon={Users} loading={summaryLoading} />
        <StatCard label={t('admin.depositVolume')} value={usd(summary?.total_deposit_volume ?? 0)}
          icon={Activity} loading={summaryLoading} />
        <StatCard label={t('admin.winnersKpi')} value={num(summary?.total_winners ?? 0)}
          icon={TrendingUp} loading={summaryLoading}
          sub={t('admin.prizesPaid', { value: usd(summary?.total_prize_paid ?? 0) })} />
      </div>

      <div className="grid gap-4 lg:grid-cols-3">
        <RevenueChart range={range} period={period} />
        <UpcomingDraws />
      </div>
      <div className="grid gap-4 lg:grid-cols-2">
        <TicketSalesChart range={range} period={period} />
        <ActiveUsersChart range={range} period={period} />
      </div>
      <div className="grid"><RaffleTable /></div>
    </div>
  )
}
