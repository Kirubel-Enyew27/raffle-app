import { useQuery } from '@tanstack/react-query'
import {
  AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend,
} from 'recharts'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { ReportsControls, exportCSV, type Range } from './ReportsControls'
import { adminApi } from './api'

import { formatCurrency } from '@/lib/utils'
const usd = (n: number) => formatCurrency(n)

interface Props { range: Range; onRange: (r: Range) => void; period: string; onPeriod: (p: string) => void }

export function RevenueReport({ range, onRange, period, onPeriod }: Props) {
  const { data, isLoading } = useQuery({
    queryKey: ['reports', 'revenue', range, period],
    queryFn: () => adminApi.revenue(range.from, range.to, period, 90),
  })
  const rows = data?.items ?? []

  function doExport() {
    exportCSV('revenue.csv',
      ['Period', 'Ticket Revenue', 'Deposit Volume', 'Withdraw Volume', 'Prize Paid', 'Profit'],
      rows.map(r => [r.period, r.ticket_revenue, r.deposit_volume, r.withdraw_volume, r.prize_paid, r.profit]),
    )
  }

  return (
    <div className="space-y-4">
      <ReportsControls range={range} onRange={onRange} period={period} onPeriod={onPeriod}
        onExport={rows.length > 0 ? doExport : undefined} />

      <Card>
        <CardHeader className="pb-2"><CardTitle className="text-base">Revenue & Profit</CardTitle></CardHeader>
        <CardContent>
          {isLoading ? <Skeleton className="h-56 w-full" /> : rows.length === 0 ? (
            <p className="py-10 text-center text-sm text-muted-foreground">No data</p>
          ) : (
            <ResponsiveContainer width="100%" height={220}>
              <AreaChart data={rows} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
                <defs>
                  <linearGradient id="gRev" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#6366f1" stopOpacity={0.3} /><stop offset="95%" stopColor="#6366f1" stopOpacity={0} />
                  </linearGradient>
                  <linearGradient id="gPro" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#22c55e" stopOpacity={0.3} /><stop offset="95%" stopColor="#22c55e" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
                <XAxis dataKey="period" tick={{ fontSize: 11 }} tickLine={false} axisLine={false} />
                <YAxis tick={{ fontSize: 11 }} tickLine={false} axisLine={false} tickFormatter={v => `${(v/1000).toFixed(0)}k Br`} />
                <Tooltip formatter={(v) => usd(Number(v))} />
                <Legend />
                <Area type="monotone" dataKey="ticket_revenue" name="Revenue" stroke="#6366f1" fill="url(#gRev)" strokeWidth={2} dot={false} />
                <Area type="monotone" dataKey="deposit_volume" name="Deposits" stroke="#3b82f6" fill="none" strokeWidth={1.5} strokeDasharray="4 2" dot={false} />
                <Area type="monotone" dataKey="withdraw_volume" name="Withdrawals" stroke="#f59e0b" fill="none" strokeWidth={1.5} strokeDasharray="4 2" dot={false} />
                <Area type="monotone" dataKey="profit" name="Profit" stroke="#22c55e" fill="url(#gPro)" strokeWidth={2} dot={false} />
              </AreaChart>
            </ResponsiveContainer>
          )}
        </CardContent>
      </Card>

      {rows.length > 0 && (
        <Card>
          <CardContent className="pt-4 overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b text-left text-xs text-muted-foreground">
                  {['Period','Ticket Rev','Deposits','Withdrawals','Prize Paid','Profit'].map(h => (
                    <th key={h} className="pb-2 pr-4 font-medium">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {rows.map(r => (
                  <tr key={r.period} className="border-b last:border-0 hover:bg-muted/20">
                    <td className="py-2 pr-4 font-mono text-xs">{r.period}</td>
                    <td className="py-2 pr-4 tabular-nums">{usd(r.ticket_revenue)}</td>
                    <td className="py-2 pr-4 tabular-nums">{usd(r.deposit_volume)}</td>
                    <td className="py-2 pr-4 tabular-nums">{usd(r.withdraw_volume)}</td>
                    <td className="py-2 pr-4 tabular-nums">{usd(r.prize_paid)}</td>
                    <td className={`py-2 tabular-nums font-medium ${r.profit >= 0 ? 'text-green-600' : 'text-destructive'}`}>{usd(r.profit)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
