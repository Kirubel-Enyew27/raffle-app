import { useQuery } from '@tanstack/react-query'
import { ArrowDownLeft, ArrowUpRight } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { cn, formatCurrency } from '@/lib/utils'
import { dashboardApi } from './api'

export function RecentTransactions() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['transactions', 5],
    queryFn: () => dashboardApi.transactions(5),
  })

  return (
    <Card className="col-span-full lg:col-span-2">
      <CardHeader className="pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">Recent Transactions</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {isLoading && Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="flex items-center gap-3">
            <Skeleton className="h-8 w-8 rounded-full" />
            <div className="flex-1 space-y-1">
              <Skeleton className="h-3 w-24" />
              <Skeleton className="h-3 w-16" />
            </div>
            <Skeleton className="h-4 w-16" />
          </div>
        ))}

        {error && <p className="text-sm text-destructive">Failed to load transactions</p>}

        {data?.transactions?.length === 0 && (
          <p className="py-4 text-center text-sm text-muted-foreground">No transactions yet</p>
        )}

        {data?.transactions?.map(tx => (
          <div key={tx.id} className="flex items-center gap-3">
            <div className={cn(
              'flex h-8 w-8 items-center justify-center rounded-full',
              tx.type === 'deposit' ? 'bg-green-100 text-green-600' : 'bg-red-100 text-red-600',
            )}>
              {tx.type === 'deposit'
                ? <ArrowDownLeft className="h-4 w-4" />
                : <ArrowUpRight className="h-4 w-4" />}
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium capitalize">{tx.type}</p>
              <p className="truncate text-xs text-muted-foreground">
                {new Date(tx.created_at).toLocaleDateString()}
              </p>
            </div>
            <span className={cn(
              'text-sm font-semibold',
              tx.type === 'deposit' ? 'text-green-600' : 'text-red-600',
            )}>
              {tx.type === 'deposit' ? '+' : '-'}{formatCurrency(tx.amount)}
            </span>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}
