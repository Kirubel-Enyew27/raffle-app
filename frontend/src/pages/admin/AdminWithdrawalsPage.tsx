import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Clock, CheckCircle, XCircle, ArrowUpRight, User } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { usePageTitle } from '@/hooks/usePageTitle'
import { adminApi } from '@/features/admin/api'
import { formatCurrency } from '@/lib/utils'
import type { WalletTransaction } from '@/types/api'

function fmtDate(s: string) {
  return new Date(s).toLocaleString('en-US', {
    month: 'short', day: 'numeric', year: 'numeric', hour: '2-digit', minute: '2-digit',
  })
}

export function Component() {
  usePageTitle('Admin - Withdrawals')
  const qc = useQueryClient()
  const [actionMsg, setActionMsg] = useState<{ id: string; type: 'approve' | 'reject'; success: boolean } | null>(null)

  const { data: withdrawals, isLoading, error } = useQuery({
    queryKey: ['admin-pending-withdrawals'],
    queryFn: adminApi.pendingWithdrawals,
    refetchInterval: 15_000,
  })

  const approveMutation = useMutation({
    mutationFn: (id: string) => adminApi.approveWithdrawal(id),
    onSuccess: (_data, id) => {
      qc.invalidateQueries({ queryKey: ['admin-pending-withdrawals'] })
      setActionMsg({ id, type: 'approve', success: true })
      setTimeout(() => setActionMsg(null), 3000)
    },
    onError: (_err, id) => {
      setActionMsg({ id, type: 'approve', success: false })
      setTimeout(() => setActionMsg(null), 3000)
    },
  })

  const rejectMutation = useMutation({
    mutationFn: (id: string) => adminApi.rejectWithdrawal(id),
    onSuccess: (_data, id) => {
      qc.invalidateQueries({ queryKey: ['admin-pending-withdrawals'] })
      setActionMsg({ id, type: 'reject', success: true })
      setTimeout(() => setActionMsg(null), 3000)
    },
    onError: (_err, id) => {
      setActionMsg({ id, type: 'reject', success: false })
      setTimeout(() => setActionMsg(null), 3000)
    },
  })

  return (
    <div className="space-y-6 p-4 sm:p-6">
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-amber-100 dark:bg-amber-900/30">
          <Clock className="h-5 w-5 text-amber-600 dark:text-amber-400" />
        </div>
        <div>
          <h1 className="text-2xl font-bold">Pending Withdrawals</h1>
          <p className="text-sm text-muted-foreground">
            Review and approve/reject withdrawal requests. Funds are sent manually within 24 hours.
          </p>
        </div>
      </div>

      {error && (
        <div className="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-400">
          Failed to load pending withdrawals. Please try again.
        </div>
      )}

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <Clock className="h-4 w-4 text-amber-500" />
            Requests ({withdrawals?.length ?? 0})
          </CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-3">
              {Array.from({ length: 3 }).map((_, i) => (
                <div key={i} className="flex items-center gap-4 rounded-lg border p-4">
                  <Skeleton className="h-10 w-10 rounded-full" />
                  <div className="flex-1 space-y-1">
                    <Skeleton className="h-4 w-40" />
                    <Skeleton className="h-3 w-24" />
                  </div>
                  <Skeleton className="h-8 w-20" />
                  <Skeleton className="h-8 w-20" />
                </div>
              ))}
            </div>
          ) : !withdrawals || withdrawals.length === 0 ? (
            <div className="flex flex-col items-center py-12">
              <CheckCircle className="mb-3 h-10 w-10 text-muted-foreground/30" />
              <p className="text-lg font-medium text-muted-foreground">No pending withdrawals</p>
              <p className="mt-1 text-sm text-muted-foreground/60">
                All withdrawal requests have been processed.
              </p>
            </div>
          ) : (
            <div className="space-y-3">
              {withdrawals.map((tx: WalletTransaction) => (
                <div
                  key={tx.id}
                  className="flex flex-col gap-3 rounded-lg border p-4 sm:flex-row sm:items-center sm:justify-between"
                >
                  <div className="flex items-start gap-3 sm:items-center">
                    <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-amber-100 dark:bg-amber-900/30">
                      <User className="h-5 w-5 text-amber-600 dark:text-amber-400" />
                    </div>
                    <div>
                      <p className="flex items-center gap-2 text-sm font-medium">
                        {formatCurrency(tx.amount)}
                        <Badge variant="warning" className="text-[10px]">Pending</Badge>
                      </p>
                      <p className="text-xs text-muted-foreground">
                        User: <span className="font-mono">{tx.user_id ? tx.user_id.slice(0, 8)+'…' : '—'}</span>
                        &bull; {fmtDate(tx.created_at)}
                      </p>
                      {tx.reference && (
                        <p className="text-xs text-muted-foreground">Ref: {tx.reference}</p>
                      )}
                      {tx.description && (
                        <p className="mt-0.5 text-xs text-muted-foreground">{tx.description}</p>
                      )}
                    </div>
                  </div>

                  <div className="flex items-center gap-2">
                    {actionMsg?.id === tx.id && actionMsg.success && (
                      <span className="mr-2 text-xs text-green-600">
                        {actionMsg.type === 'approve' ? 'Approved ✓' : 'Rejected ✗'}
                      </span>
                    )}
                    <Button
                      variant="default"
                      size="sm"
                      className="gap-1"
                      disabled={approveMutation.isPending && approveMutation.variables === tx.id}
                      onClick={() => approveMutation.mutate(tx.id)}
                    >
                      <CheckCircle className="h-3.5 w-3.5" />
                      Approve
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      className="gap-1 text-destructive hover:text-destructive"
                      disabled={rejectMutation.isPending && rejectMutation.variables === tx.id}
                      onClick={() => rejectMutation.mutate(tx.id)}
                    >
                      <XCircle className="h-3.5 w-3.5" />
                      Reject
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
