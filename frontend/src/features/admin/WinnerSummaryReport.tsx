import { useQuery } from '@tanstack/react-query'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { ReportsControls, exportCSV, type Range } from './ReportsControls'
import { adminApi } from './api'

const usd = (n: number) => n.toLocaleString('en-US', { style: 'currency', currency: 'USD', maximumFractionDigits: 2 })

interface Props { range: Range; onRange: (r: Range) => void }

export function WinnerSummaryReport({ range, onRange }: Props) {
  const { data, isLoading } = useQuery({
    queryKey: ['reports', 'winners', range],
    queryFn: () => adminApi.winnerSummary(range.from, range.to, 100),
  })
  const rows = data?.items ?? []

  function doExport() {
    exportCSV('winners.csv',
      ['Winner ID', 'Email', 'Raffle', 'Prize', 'Paid', 'Payment Ref', 'Date'],
      rows.map(r => [
        r.winner_id, r.user_email, r.raffle_title, r.prize_amount,
        r.prize_paid ? 'Yes' : 'No', r.payment_reference ?? '', r.created_at,
      ]),
    )
  }

  return (
    <div className="space-y-4">
      <ReportsControls range={range} onRange={onRange} onExport={rows.length > 0 ? doExport : undefined} />

      <Card>
        <CardContent className="pt-4 overflow-x-auto">
          {isLoading ? (
            <div className="space-y-2">{Array.from({ length: 6 }).map((_, i) => <Skeleton key={i} className="h-9 w-full" />)}</div>
          ) : rows.length === 0 ? (
            <p className="py-10 text-center text-sm text-muted-foreground">No winners in this period</p>
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b text-left text-xs text-muted-foreground">
                  {['Email', 'Raffle', 'Prize', 'Status', 'Payment Ref', 'Won'].map(h => (
                    <th key={h} className="pb-2 pr-4 font-medium">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {rows.map(r => (
                  <tr key={r.winner_id} className="border-b last:border-0 hover:bg-muted/20">
                    <td className="py-2 pr-4">{r.user_email}</td>
                    <td className="py-2 pr-4 max-w-[180px] truncate">{r.raffle_title}</td>
                    <td className="py-2 pr-4 tabular-nums font-medium">{usd(r.prize_amount)}</td>
                    <td className="py-2 pr-4">
                      <Badge variant={r.prize_paid ? 'success' : 'warning'}>
                        {r.prize_paid ? 'Paid' : 'Unpaid'}
                      </Badge>
                    </td>
                    <td className="py-2 pr-4 font-mono text-xs text-muted-foreground">
                      {r.payment_reference || '—'}
                    </td>
                    <td className="py-2 text-muted-foreground">
                      {new Date(r.created_at).toLocaleDateString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
