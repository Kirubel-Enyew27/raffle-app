import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { ArrowDownLeft, ArrowUpRight, Wallet } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { usePageTitle } from '@/hooks/usePageTitle'
import { FormField } from '@/components/forms/FormField'
import { ErrorMessage } from '@/components/ui/error-message'
import { walletApi } from '@/features/wallet/api'
import { cn } from '@/lib/utils'

// ─── constants ────────────────────────────────────────────────────────────────
const PAGE_SIZE = 10

// ─── schemas ──────────────────────────────────────────────────────────────────
const txSchema = z.object({
  amount: z.string().min(1, 'Amount is required'),
  reference: z.string().min(1, 'Reference is required'),
  description: z.string().optional(),
})
type TxFields = z.infer<typeof txSchema>

// ─── BalanceCard ──────────────────────────────────────────────────────────────
function BalanceCard() {
  const { data, isLoading } = useQuery({ queryKey: ['wallet'], queryFn: walletApi.balance })
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">Available Balance</CardTitle>
        <Wallet className="h-4 w-4 text-muted-foreground" />
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className="h-9 w-40" />
        ) : (
          <>
            <p className="text-3xl font-bold">
              {(data?.balance ?? 0).toLocaleString('en-US', {
                style: 'currency',
                currency: data?.currency ?? 'USD',
              })}
            </p>
            <p className="mt-1 text-xs text-muted-foreground">{data?.currency ?? 'USD'}</p>
          </>
        )}
      </CardContent>
    </Card>
  )
}

// ─── TxForm ───────────────────────────────────────────────────────────────────
interface TxFormProps {
  type: 'deposit' | 'withdraw'
  onSuccess: () => void
}

function TxForm({ type, onSuccess }: TxFormProps) {
  const qc = useQueryClient()
  const { register, handleSubmit, reset, formState: { errors } } = useForm<TxFields>({
    resolver: zodResolver(txSchema),
  })

  const { mutate, isPending, error } = useMutation({
    mutationFn: ({ amount, reference, description }: TxFields) => {
      const parsed = parseFloat(amount)
      if (isNaN(parsed) || parsed <= 0) throw new Error('Amount must be greater than 0')
      return type === 'deposit'
        ? walletApi.deposit(parsed, reference, description ?? '')
        : walletApi.withdraw(parsed, reference, description ?? '')
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['wallet'] })
      qc.invalidateQueries({ queryKey: ['transactions'] })
      reset()
      onSuccess()
    },
  })

  return (
    <form onSubmit={handleSubmit(d => mutate(d))} className="space-y-3">
      {error && <ErrorMessage message={error.message} />}

      <FormField label="Amount" htmlFor="amount" error={errors.amount?.message as string | undefined}>
        <div className="relative">
          <span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">$</span>
          <Input id="amount" type="number" step="0.01" placeholder="0.00" className="pl-7"
            {...register('amount')} />
        </div>
      </FormField>

      <FormField label="Reference" htmlFor="reference" error={errors.reference?.message as string | undefined}>
        <Input id="reference" placeholder="Bank ref / transaction ID" {...register('reference')} />
      </FormField>

      <FormField label="Description (optional)" htmlFor="description">
        <Input id="description" placeholder="Optional note" {...register('description')} />
      </FormField>

      <Button type="submit" className="w-full" disabled={isPending}
        variant={type === 'withdraw' ? 'destructive' : 'default'}>
        {isPending
          ? (type === 'deposit' ? 'Processing deposit…' : 'Processing withdrawal…')
          : (type === 'deposit' ? 'Deposit funds' : 'Withdraw funds')}
      </Button>
    </form>
  )
}

// ─── TransactionHistory ───────────────────────────────────────────────────────
function TransactionHistory() {
  const [page, setPage] = useState(0)
  const [typeFilter, setTypeFilter] = useState<string>('all')

  const { data, isLoading, error } = useQuery({
    queryKey: ['transactions', page, typeFilter],
    queryFn: () => walletApi.transactions({
      limit: PAGE_SIZE,
      offset: page * PAGE_SIZE,
      ...(typeFilter !== 'all' ? { type: typeFilter } : {}),
    }),
  })

  const txs = data?.transactions ?? []
  const total = data?.total ?? 0
  const totalPages = Math.ceil(total / PAGE_SIZE)

  const fmt = (n: number) => n.toLocaleString('en-US', { style: 'currency', currency: 'USD' })
  const fmtDate = (s: string) => new Date(s).toLocaleString('en-US', {
    month: 'short', day: 'numeric', year: 'numeric', hour: '2-digit', minute: '2-digit',
  })

  return (
    <Card className="col-span-full">
      <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-3 pb-3">
        <CardTitle className="text-base">Transaction History</CardTitle>
        <div className="flex items-center gap-2">
          <Select
            value={typeFilter}
            onChange={e => { setTypeFilter(e.target.value); setPage(0) }}
            className="w-36"
          >
            <option value="all">All types</option>
            <option value="deposit">Deposits</option>
            <option value="withdrawal">Withdrawals</option>
          </Select>
        </div>
      </CardHeader>

      <CardContent>
        {/* table — scrollable on small screens */}
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-left text-xs text-muted-foreground">
                <th className="pb-2 pr-4 font-medium">Type</th>
                <th className="pb-2 pr-4 font-medium">Amount</th>
                <th className="pb-2 pr-4 font-medium">Balance after</th>
                <th className="pb-2 pr-4 font-medium">Reference</th>
                <th className="pb-2 font-medium">Date</th>
              </tr>
            </thead>
            <tbody>
              {isLoading && Array.from({ length: PAGE_SIZE }).map((_, i) => (
                <tr key={i} className="border-b last:border-0">
                  {Array.from({ length: 5 }).map((_, j) => (
                    <td key={j} className="py-3 pr-4"><Skeleton className="h-4 w-full" /></td>
                  ))}
                </tr>
              ))}

              {error && (
                <tr><td colSpan={5} className="py-6 text-center text-destructive">
                  Failed to load transactions
                </td></tr>
              )}

              {!isLoading && txs.length === 0 && (
                <tr><td colSpan={5} className="py-8 text-center text-sm text-muted-foreground">
                  No transactions found
                </td></tr>
              )}

              {txs.map(tx => (
                <tr key={tx.id} className="border-b last:border-0 hover:bg-muted/30 transition-colors">
                  <td className="py-3 pr-4">
                    <span className="flex items-center gap-1.5">
                      {tx.type === 'deposit'
                        ? <ArrowDownLeft className="h-3.5 w-3.5 text-green-600" />
                        : <ArrowUpRight className="h-3.5 w-3.5 text-red-500" />}
                      <Badge variant={tx.type === 'deposit' ? 'success' : 'destructive'}>
                        {tx.type}
                      </Badge>
                    </span>
                  </td>
                  <td className={cn('py-3 pr-4 font-semibold tabular-nums',
                    tx.type === 'deposit' ? 'text-green-600' : 'text-red-500')}>
                    {tx.type === 'deposit' ? '+' : '-'}{fmt(tx.amount)}
                  </td>
                  <td className="py-3 pr-4 tabular-nums text-muted-foreground">{fmt(tx.balance_after)}</td>
                  <td className="max-w-[10rem] py-3 pr-4">
                    <span className="block truncate text-muted-foreground">{tx.reference || '—'}</span>
                  </td>
                  <td className="py-3 whitespace-nowrap text-muted-foreground">{fmtDate(tx.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="mt-4 flex items-center justify-between text-sm">
            <span className="text-muted-foreground">
              {page * PAGE_SIZE + 1}–{Math.min((page + 1) * PAGE_SIZE, total)} of {total}
            </span>
            <div className="flex gap-2">
              <Button variant="outline" size="sm" disabled={page === 0}
                onClick={() => setPage(p => p - 1)}>
                Previous
              </Button>
              <Button variant="outline" size="sm" disabled={page >= totalPages - 1}
                onClick={() => setPage(p => p + 1)}>
                Next
              </Button>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────
export function Component() {
  usePageTitle('Wallet')
  const [activeForm, setActiveForm] = useState<'deposit' | 'withdraw' | null>(null)

  return (
    <div className="mx-auto max-w-6xl space-y-6 p-4 sm:p-6">
      <h1 className="text-2xl font-bold">Wallet</h1>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <BalanceCard />

        {/* Deposit card */}
        <Card className={cn(activeForm === 'deposit' && 'ring-2 ring-primary')}>
          <CardHeader className="pb-3">
            <div className="flex items-center justify-between">
              <CardTitle className="text-base">Deposit</CardTitle>
              <ArrowDownLeft className="h-4 w-4 text-green-600" />
            </div>
          </CardHeader>
          <CardContent>
            {activeForm !== 'deposit' ? (
              <Button className="w-full" onClick={() => setActiveForm('deposit')}>
                Add funds
              </Button>
            ) : (
              <TxForm type="deposit" onSuccess={() => setActiveForm(null)} />
            )}
          </CardContent>
        </Card>

        {/* Withdraw card */}
        <Card className={cn(activeForm === 'withdraw' && 'ring-2 ring-destructive')}>
          <CardHeader className="pb-3">
            <div className="flex items-center justify-between">
              <CardTitle className="text-base">Withdraw</CardTitle>
              <ArrowUpRight className="h-4 w-4 text-red-500" />
            </div>
          </CardHeader>
          <CardContent>
            {activeForm !== 'withdraw' ? (
              <Button variant="outline" className="w-full" onClick={() => setActiveForm('withdraw')}>
                Withdraw funds
              </Button>
            ) : (
              <TxForm type="withdraw" onSuccess={() => setActiveForm(null)} />
            )}
          </CardContent>
        </Card>
      </div>

      <TransactionHistory />
    </div>
  )
}
