import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend } from 'recharts'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { ReportsControls, exportCSV, type Range } from './ReportsControls'
import { adminApi } from './api'

interface Props { range: Range; onRange: (r: Range) => void; period: string; onPeriod: (p: string) => void }

export function TicketsReport({ range, onRange, period, onPeriod }: Props) {
  const { t } = useTranslation()
  const { data, isLoading } = useQuery({
    queryKey: ['reports', 'tickets', range, period],
    queryFn: () => adminApi.ticketSales(range.from, range.to, period, 90),
  })
  const rows = data?.items ?? []

  function doExport() {
    exportCSV('ticket-sales.csv',
      [t('reports.period'), t('reports.ticketsSold'), t('reports.rafflesHeld')],
      rows.map(r => [r.period, r.tickets_sold, r.raffles_held]),
    )
  }

  return (
    <div className="space-y-4">
      <ReportsControls range={range} onRange={onRange} period={period} onPeriod={onPeriod}
        onExport={rows.length > 0 ? doExport : undefined} />

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">{t('reports.ticketsSold')}</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? <Skeleton className="h-56 w-full" /> : rows.length === 0 ? (
            <p className="py-10 text-center text-sm text-muted-foreground">{t('reports.noData')}</p>
          ) : (
            <ResponsiveContainer width="100%" height={220}>
              <BarChart data={rows} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" vertical={false} />
                <XAxis dataKey="period" tick={{ fontSize: 11 }} tickLine={false} axisLine={false} />
                <YAxis tick={{ fontSize: 11 }} tickLine={false} axisLine={false} />
                <Tooltip />
                <Legend />
                <Bar dataKey="tickets_sold" name={t('reports.ticketsSold')} fill="#6366f1" radius={[3, 3, 0, 0]} />
                <Bar dataKey="raffles_held" name={t('reports.rafflesHeld')} fill="#a5b4fc" radius={[3, 3, 0, 0]} />
              </BarChart>
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
                  {[t('reports.period'), t('reports.ticketsSold'), t('reports.rafflesHeld')].map(h => (
                    <th key={h} className="pb-2 pr-4 font-medium">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {rows.map(r => (
                  <tr key={r.period} className="border-b last:border-0 hover:bg-muted/20">
                    <td className="py-2 pr-4 font-mono text-xs">{r.period}</td>
                    <td className="py-2 pr-4 tabular-nums font-medium">{r.tickets_sold.toLocaleString()}</td>
                    <td className="py-2 tabular-nums">{r.raffles_held}</td>
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
