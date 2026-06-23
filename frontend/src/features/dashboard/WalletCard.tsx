import { useQuery } from '@tanstack/react-query'
import { Wallet } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { dashboardApi } from './api'

export function WalletCard() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['wallet'],
    queryFn: dashboardApi.wallet,
  })

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">Wallet Balance</CardTitle>
        <Wallet className="h-4 w-4 text-muted-foreground" />
      </CardHeader>
      <CardContent>
        {isLoading && <Skeleton className="h-8 w-32" />}
        {error && <p className="text-sm text-destructive">Failed to load</p>}
        {data && (
          <>
            <p className="text-3xl font-bold">
              {data.balance.toLocaleString('en-US', { style: 'currency', currency: data.currency || 'USD' })}
            </p>
            <p className="mt-1 text-xs text-muted-foreground">{data.currency}</p>
          </>
        )}
      </CardContent>
    </Card>
  )
}
