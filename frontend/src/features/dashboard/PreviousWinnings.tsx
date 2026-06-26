import { useQuery } from '@tanstack/react-query'
import { Trophy } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { cn, formatCurrency } from '@/lib/utils'
import { dashboardApi } from './api'

export function PreviousWinnings() {
  // Fetch completed raffles then load winners for each
  const rafflesQuery = useQuery({
    queryKey: ['raffles', 'all'],
    queryFn: () => dashboardApi.raffles(20),
  })

  const completedIds = rafflesQuery.data?.raffles
    ?.filter(r => r.status === 'completed')
    .map(r => r.id) ?? []

  // Load winners for first 3 completed raffles
  const winnerQueries = useQuery({
    queryKey: ['winners', 'dashboard', completedIds.slice(0, 3)],
    queryFn: async () => {
      const results = await Promise.all(completedIds.slice(0, 3).map(dashboardApi.winnersByRaffle))
      return results.flat()
    },
    enabled: completedIds.length > 0,
  })

  const isLoading = rafflesQuery.isLoading || winnerQueries.isLoading
  const winners = winnerQueries.data ?? []

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">Previous Winnings</CardTitle>
        <Trophy className="h-4 w-4 text-muted-foreground" />
      </CardHeader>
      <CardContent className="space-y-3">
        {isLoading && Array.from({ length: 2 }).map((_, i) => (
          <div key={i} className="space-y-1">
            <Skeleton className="h-4 w-32" />
            <Skeleton className="h-3 w-20" />
          </div>
        ))}

        {!isLoading && winners.length === 0 && (
          <div className="py-4 text-center">
            <Trophy className="mx-auto mb-2 h-8 w-8 text-muted-foreground/40" />
            <p className="text-sm text-muted-foreground">No winnings yet</p>
          </div>
        )}

        {winners.map(w => (
          <div key={w.id} className="flex items-start justify-between gap-2">
            <div className="min-w-0">
              <p className="text-sm font-semibold text-green-600">
                {formatCurrency(w.prize_amount)}
              </p>
              <p className="text-xs text-muted-foreground">
                {new Date(w.created_at).toLocaleDateString()}
              </p>
            </div>
            <span className={cn(
              'shrink-0 rounded-full px-2 py-0.5 text-xs font-medium',
              w.prize_paid ? 'bg-green-100 text-green-700' : 'bg-yellow-100 text-yellow-700',
            )}>
              {w.prize_paid ? 'Paid' : 'Pending'}
            </span>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}
