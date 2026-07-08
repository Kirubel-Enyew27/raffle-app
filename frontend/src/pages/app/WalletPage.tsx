import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { ArrowDownLeft, ArrowUpRight, Wallet, Smartphone, Copy, Check, Clock } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { usePageTitle } from '@/hooks/usePageTitle'
import { walletApi } from '@/features/wallet/api'
import { cn, formatCurrency } from '@/lib/utils'

const TELEBIRR_PHONE = '+251948260013'
const PAGE_SIZE = 10

function BalanceCard() {
  const { t } = useTranslation()
  const { data, isLoading } = useQuery({ queryKey: ['wallet'], queryFn: walletApi.balance, staleTime: 0 })
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">{t('wallet.availableBalance')}</CardTitle>
        <Wallet className="h-4 w-4 text-muted-foreground" />
      </CardHeader>
      <CardContent>
        {isLoading ? <Skeleton className="h-9 w-40" /> : (
          <>
            <p className="text-3xl font-bold">{formatCurrency(data?.balance ?? 0, data?.currency)}</p>
            <p className="mt-1 text-xs text-muted-foreground">{t('wallet.ethiopianBirr')}</p>
          </>
        )}
      </CardContent>
    </Card>
  )
}

function TelebirrDepositCard() {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)
  const qc = useQueryClient()

  const handleCopy = () => {
    navigator.clipboard.writeText(TELEBIRR_PHONE)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
    qc.invalidateQueries({ queryKey: ['wallet'] })
    qc.invalidateQueries({ queryKey: ['transactions'] })
  }

  return (
    <Card className="border-primary/20 bg-primary/5">
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2 text-base">
            <Smartphone className="h-4 w-4 text-green-600" />
            {t('wallet.depositViaTelebirr')}
          </CardTitle>
          <ArrowDownLeft className="h-4 w-4 text-green-600" />
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="rounded-lg border bg-card p-4 text-center">
          <p className="mb-1 text-xs text-muted-foreground">{t('wallet.sendMoneyTo')}</p>
          <p className="text-xl font-bold tracking-wide">{TELEBIRR_PHONE}</p>
          <Button variant="outline" size="sm" onClick={handleCopy} className="mt-2 gap-1.5">
            {copied
              ? <><Check className="h-3.5 w-3.5" /> {t('wallet.copied')}</>
              : <><Copy className="h-3.5 w-3.5" /> {t('wallet.copyNumber')}</>}
          </Button>
        </div>
        <div className="space-y-2 text-sm text-muted-foreground">
          <h4 className="font-medium text-foreground">{t('wallet.howItWorks')}</h4>
          <ol className="list-inside list-decimal space-y-1.5">
            <li>{t('wallet.depositStep1')}</li>
            <li>{t('wallet.depositStep2', { phone: TELEBIRR_PHONE })}</li>
            <li>{t('wallet.depositStep3')}</li>
            <li>{t('wallet.depositStep4')}</li>
            <li>{t('wallet.depositStep5')}</li>
          </ol>
          <p className="mt-3 flex items-center gap-1.5 rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-xs text-blue-700 dark:border-blue-800 dark:bg-blue-950 dark:text-blue-400">
            <Smartphone className="h-3.5 w-3.5 shrink-0" />
            {t('wallet.telebirrNameNotice')}
          </p>
        </div>
      </CardContent>
    </Card>
  )
}

function WithdrawCard() {
  const { t } = useTranslation()
  const [amount, setAmount] = useState('')
  const [phone, setPhone] = useState('')
  const [showForm, setShowForm] = useState(false)
  const qc = useQueryClient()
  const { data: wallet } = useQuery({ queryKey: ['wallet'], queryFn: walletApi.balance, staleTime: 0 })

  const withdrawMutation = useMutation({
    mutationFn: ({ amount, phone }: { amount: number; phone: string }) =>
      walletApi.withdraw(amount, phone, `Withdrawal to ${phone}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['wallet'] })
      qc.invalidateQueries({ queryKey: ['transactions'] })
      setAmount(''); setPhone(''); setShowForm(false)
    },
  })

  const parsedAmount = parseFloat(amount)
  const isValid = parsedAmount > 0 && parsedAmount <= (wallet?.balance ?? 0) && phone.trim().length >= 7

  const reset = () => { setShowForm(false); setAmount(''); setPhone('') }

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2 text-base">
            <ArrowUpRight className="h-4 w-4 text-red-500" />
            {t('wallet.withdraw')}
          </CardTitle>
          {!showForm && (
            <Button variant="outline" size="sm" onClick={() => setShowForm(true)}>
              {t('wallet.requestWithdrawal')}
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent>
        {!showForm ? (
          <p className="text-sm text-muted-foreground">{t('wallet.withdrawDescription')}</p>
        ) : (
          <div className="space-y-4">
            {withdrawMutation.isSuccess && (
              <div className="flex items-center gap-2 rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-sm text-blue-700 dark:border-blue-800 dark:bg-blue-950 dark:text-blue-400">
                <Clock className="h-4 w-4 shrink-0" />
                {t('wallet.withdrawSubmitted')}
              </div>
            )}
            {withdrawMutation.isError && (
              <div className="flex items-center gap-2 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-400">
                {withdrawMutation.error?.message || t('wallet.withdrawError')}
              </div>
            )}
            {!withdrawMutation.isSuccess ? (
              <>
                <div>
                  <label className="text-sm font-medium" htmlFor="withdraw-amount">{t('wallet.amount')}</label>
                  <div className="relative mt-1">
                    <Input id="withdraw-amount" type="number" step="0.01"
                      placeholder={t('wallet.amountPlaceholder')} className="pr-10"
                      value={amount} onChange={e => setAmount(e.target.value)} />
                    <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">
                      {t('wallet.currencySuffix')}
                    </span>
                  </div>
                  {wallet && (
                    <p className="mt-1 text-xs text-muted-foreground">
                      {t('wallet.balanceHelper', { balance: formatCurrency(wallet.balance), max: formatCurrency(wallet.balance) })}
                    </p>
                  )}
                </div>
                <div>
                  <label className="text-sm font-medium" htmlFor="withdraw-phone">{t('wallet.telebirrPhoneNumber')}</label>
                  <Input id="withdraw-phone" type="tel" placeholder={t('auth.phonePlaceholder')}
                    className="mt-1" value={phone} onChange={e => setPhone(e.target.value)} />
                  <p className="mt-1 text-xs text-muted-foreground">{t('wallet.telebirrPhoneHelper')}</p>
                </div>
                <div className="rounded-lg border bg-amber-50 p-3 text-xs text-amber-700 dark:bg-amber-950 dark:text-amber-400">
                  <p className="flex items-center gap-1.5 font-medium">
                    <Clock className="h-3.5 w-3.5" />
                    {t('wallet.withdrawalReviewNotice')}
                  </p>
                </div>
                <div className="flex gap-2">
                  <Button variant="outline" onClick={reset} className="flex-1">{t('wallet.cancel')}</Button>
                  <Button variant="destructive" disabled={!isValid || withdrawMutation.isPending}
                    className="flex-1" onClick={() => withdrawMutation.mutate({ amount: parsedAmount, phone })}>
                    {withdrawMutation.isPending ? t('wallet.submitting') : t('wallet.requestWithdrawal')}
                  </Button>
                </div>
              </>
            ) : (
              <Button variant="outline" onClick={reset}>{t('wallet.requestAnotherWithdrawal')}</Button>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function TransactionHistory() {
  const { t } = useTranslation()
  const [page, setPage] = useState(0)
  const [typeFilter, setTypeFilter] = useState('all')

  const { data, isLoading, error } = useQuery({
    queryKey: ['transactions', page, typeFilter],
    queryFn: () => walletApi.transactions({
      limit: PAGE_SIZE, offset: page * PAGE_SIZE,
      ...(typeFilter !== 'all' ? { type: typeFilter } : {}),
    }),
  })

  const txs = data?.transactions ?? []
  const total = data?.total ?? 0
  const totalPages = Math.ceil(total / PAGE_SIZE)

  const fmtDate = (s: string) => new Date(s).toLocaleString(undefined, {
    month: 'short', day: 'numeric', year: 'numeric', hour: '2-digit', minute: '2-digit',
  })

  // Translate transaction status/type labels
  const txLabel = (tx: typeof txs[0]) => {
    if (tx.status === 'completed') return tx.type === 'deposit' ? t('wallet.credit') : t('wallet.debit')
    if (tx.status === 'pending') return t('wallet.pending')
    if (tx.status === 'rejected') return t('wallet.failed')
    return tx.status
  }

  return (
    <Card className="col-span-full">
      <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-3 pb-3">
        <CardTitle className="text-base">{t('wallet.transactionHistory')}</CardTitle>
        <Select value={typeFilter} onChange={e => { setTypeFilter(e.target.value); setPage(0) }} className="w-36">
          <option value="all">{t('wallet.allTypes')}</option>
          <option value="deposit">{t('wallet.deposits')}</option>
          <option value="withdraw">{t('wallet.withdrawals')}</option>
        </Select>
      </CardHeader>
      <CardContent>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-left text-xs text-muted-foreground">
                <th className="pb-2 pr-4 font-medium">{t('wallet.type')}</th>
                <th className="pb-2 pr-4 font-medium">{t('wallet.amount')}</th>
                <th className="pb-2 pr-4 font-medium">{t('wallet.balanceAfter')}</th>
                <th className="pb-2 pr-4 font-medium">{t('wallet.reference')}</th>
                <th className="pb-2 font-medium">{t('wallet.date')}</th>
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
                <tr><td colSpan={5} className="py-6 text-center text-destructive">{t('wallet.failedToLoad')}</td></tr>
              )}
              {!isLoading && txs.length === 0 && (
                <tr><td colSpan={5} className="py-8 text-center text-sm text-muted-foreground">{t('wallet.noTransactions')}</td></tr>
              )}
              {txs.map(tx => (
                <tr key={tx.id} className="border-b last:border-0 hover:bg-muted/30 transition-colors">
                  <td className="py-3 pr-4">
                    <span className="flex items-center gap-1.5">
                      {tx.type === 'deposit'
                        ? <ArrowDownLeft className="h-3.5 w-3.5 text-green-600" />
                        : <ArrowUpRight className="h-3.5 w-3.5 text-red-500" />}
                      <Badge variant={
                        tx.status === 'pending' ? 'warning'
                          : tx.status === 'rejected' ? 'destructive'
                          : tx.type === 'deposit' ? 'success' : 'destructive'
                      }>
                        {txLabel(tx)}
                      </Badge>
                    </span>
                  </td>
                  <td className={cn('py-3 pr-4 font-semibold tabular-nums',
                    tx.type === 'deposit' ? 'text-green-600' : 'text-red-500')}>
                    {tx.type === 'deposit' ? '+' : '-'}{formatCurrency(tx.amount)}
                  </td>
                  <td className="py-3 pr-4 tabular-nums text-muted-foreground">{formatCurrency(tx.balance_after)}</td>
                  <td className="max-w-[10rem] py-3 pr-4">
                    <span className="block truncate text-muted-foreground">{tx.reference || '—'}</span>
                  </td>
                  <td className="py-3 whitespace-nowrap text-muted-foreground">{fmtDate(tx.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {totalPages > 1 && (
          <div className="mt-4 flex items-center justify-between text-sm">
            <span className="text-muted-foreground">
              {page * PAGE_SIZE + 1}–{Math.min((page + 1) * PAGE_SIZE, total)} {t('common.of')} {total}
            </span>
            <div className="flex gap-2">
              <Button variant="outline" size="sm" disabled={page === 0} onClick={() => setPage(p => p - 1)}>
                {t('wallet.previous')}
              </Button>
              <Button variant="outline" size="sm" disabled={page >= totalPages - 1} onClick={() => setPage(p => p + 1)}>
                {t('wallet.next')}
              </Button>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

export function Component() {
  const { t } = useTranslation()
  usePageTitle(t('wallet.title'))
  return (
    <div className="mx-auto max-w-6xl space-y-6 p-4 sm:p-6">
      <h1 className="text-2xl font-bold">{t('wallet.title')}</h1>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <BalanceCard />
        <TelebirrDepositCard />
        <WithdrawCard />
      </div>
      <TransactionHistory />
    </div>
  )
}
