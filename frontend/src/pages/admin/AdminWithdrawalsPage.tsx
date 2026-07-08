import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Clock, CheckCircle, XCircle, User } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { usePageTitle } from '@/hooks/usePageTitle'
import { adminApi } from '@/features/admin/api'
import { formatCurrency } from '@/lib/utils'
import type { WalletTransaction } from '@/types/api'

export function Component() {
  const { t } = useTranslation()
  usePageTitle(t('admin.withdrawals'))
  const qc = useQueryClient()
  const [actionMsg, setActionMsg] = useState<{ id: string; type: 'approve' | 'reject'; success: boolean } | null>(null)

  const { data: withdrawals, isLoading, error } = useQuery({
    queryKey: ['admin-pending-withdrawals'],
    queryFn: adminApi.pendingWithdrawals,
    refetchInterval: 15_000,
  })

  const approveMutation = useMutation({
    mutationFn: (id: string) => adminApi.approveWithdrawal(id),
    onSuccess: (_d, id) => { qc.invalidateQueries({ queryKey: ['admin-pending-withdrawals'] }); setActionMsg({ id, type: 'approve', success: true }); setTimeout(() => setActionMsg(null), 3000) },
    onError: (_e, id) => { setActionMsg({ id, type: 'approve', success: false }); setTimeout(() => setActionMsg(null), 3000) },
  })
  const rejectMutation = useMutation({
    mutationFn: (id: string) => adminApi.rejectWithdrawal(id),
    onSuccess: (_d, id) => { qc.invalidateQueries({ queryKey: ['admin-pending-withdrawals'] }); setActionMsg({ id, type: 'reject', success: true }); setTimeout(() => setActionMsg(null), 3000) },
    onError: (_e, id) => { setActionMsg({ id, type: 'reject', success: false }); setTimeout(() => setActionMsg(null), 3000) },
  })

  const fmtDate = (s: string) => new Date(s).toLocaleString(undefined, { month: 'short', day: 'numeric', year: 'numeric', hour: '2-digit', minute: '2-digit' })

  return (
    <div className="space-y-6 p-4 sm:p-6">
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-amber-100 dark:bg-amber-900/30">
          <Clock className="h-5 w-5 text-amber-600 dark:text-amber-400" />
        </div>
        <div>
          <h1 className="text-2xl font-bold">{t('admin.pendingWithdrawals')}</h1>
          <p className="text-sm text-muted-foreground">{t('admin.pendingWithdrawalsSubtitle')}</p>
        </div>
      </div>

      {error && (
        <div className="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-400">
          {t('admin.withdrawalFailedToLoad')}
        </div>
      )}

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <Clock className="h-4 w-4 text-amber-500" />
            {t('admin.requests', { count: withdrawals?.length ?? 0 })}
          </CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-3">
              {Array.from({ length: 3 }).map((_, i) => (
                <div key={i} className="flex items-center gap-4 rounded-lg border p-4">
                  <Skeleton className="h-10 w-10 rounded-full" />
                  <div className="flex-1 space-y-1"><Skeleton className="h-4 w-40" /><Skeleton className="h-3 w-24" /></div>
                  <Skeleton className="h-8 w-20" /><Skeleton className="h-8 w-20" />
                </div>
              ))}
            </div>
          ) : !withdrawals || withdrawals.length === 0 ? (
            <div className="flex flex-col items-center py-12">
              <CheckCircle className="mb-3 h-10 w-10 text-muted-foreground/30" />
              <p className="text-lg font-medium text-muted-foreground">{t('admin.noPendingWithdrawals')}</p>
              <p className="mt-1 text-sm text-muted-foreground/60">{t('admin.allProcessed')}</p>
            </div>
          ) : (
            <div className="space-y-3">
              {withdrawals.map((tx: WalletTransaction) => (
                <div key={tx.id} className="flex flex-col gap-3 rounded-lg border p-4 sm:flex-row sm:items-center sm:justify-between">
                  <div className="flex items-start gap-3 sm:items-center">
                    <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-amber-100 dark:bg-amber-900/30">
                      <User className="h-5 w-5 text-amber-600 dark:text-amber-400" />
                    </div>
                    <div>
                      <p className="flex items-center gap-2 text-sm font-medium">
                        {formatCurrency(tx.amount)}
                        <Badge variant="warning" className="text-[10px]">{t('wallet.pending')}</Badge>
                      </p>
                      <p className="text-xs text-muted-foreground">
                        {t('admin.userLabel')}: <span className="font-mono">{tx.user_id ? tx.user_id.slice(0, 8) + '…' : '—'}</span>
                        &bull; {fmtDate(tx.created_at)}
                      </p>
                      {tx.reference && <p className="text-xs text-muted-foreground">{t('admin.refLabel')}: {tx.reference}</p>}
                      {tx.description && <p className="mt-0.5 text-xs text-muted-foreground">{tx.description}</p>}
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    {actionMsg?.id === tx.id && actionMsg.success && (
                      <span className="mr-2 text-xs text-green-600">
                        {actionMsg.type === 'approve' ? t('admin.approved') : t('admin.rejected')}
                      </span>
                    )}
                    <Button variant="default" size="sm" className="gap-1"
                      disabled={approveMutation.isPending && approveMutation.variables === tx.id}
                      onClick={() => approveMutation.mutate(tx.id)}>
                      <CheckCircle className="h-3.5 w-3.5" />{t('admin.approve')}
                    </Button>
                    <Button variant="outline" size="sm" className="gap-1 text-destructive hover:text-destructive"
                      disabled={rejectMutation.isPending && rejectMutation.variables === tx.id}
                      onClick={() => rejectMutation.mutate(tx.id)}>
                      <XCircle className="h-3.5 w-3.5" />{t('admin.reject')}
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
