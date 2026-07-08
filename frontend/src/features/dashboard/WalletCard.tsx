import { useQuery } from '@tanstack/react-query'
import { Wallet } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { walletApi } from '@/features/wallet/api'
import { formatCurrency } from '@/lib/utils'

export function WalletCard() {
  const { t } = useTranslation()
  const { data, isLoading, error } = useQuery({
    queryKey: ['wallet'],
    queryFn: walletApi.balance,
    staleTime: 0,
  })

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">{t('wallet.balance')}</CardTitle>
        <Wallet className="h-4 w-4 text-muted-foreground" />
      </CardHeader>
      <CardContent>
        {isLoading && <Skeleton className="h-8 w-32" />}
        {error && <p className="text-sm text-destructive">{t('common.error')}</p>}
        {data && (
          <>
            <p className="text-3xl font-bold">
              {formatCurrency(data.balance, data.currency)}
            </p>
            <p className="mt-1 text-xs text-muted-foreground">ETB &bull; Ethiopian Birr</p>
          </>
        )}
      </CardContent>
    </Card>
  )
}
