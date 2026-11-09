import { useState, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import {
  ArrowDownLeft,
  ArrowUpRight,
  Wallet,
  Smartphone,
  Building2,
  Copy,
  Check,
  Clock,
  Upload,
  Camera,
  CheckCircle2,
  AlertCircle,
  Loader2,
  ExternalLink,
  ImageIcon,
  X,
  FileCheck,
} from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { usePageTitle } from '@/hooks/usePageTitle'
import { walletApi } from '@/features/wallet/api'
import type { ReceiptDepositResult } from '@/types/api'
import { cn, formatCurrency } from '@/lib/utils'

const TELEBIRR_PHONE = '+251948260013'
const TELEBIRR_ACCOUNT_NAME = 'Kirubel Enyew'
const CBE_ACCOUNT_NUMBER = '1000249277383'
const CBE_ACCOUNT_NAME = 'Kirubel Enyew'
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
            <p className="text-3xl font-bold">
              {formatCurrency(data?.available_balance ?? data?.balance ?? 0, data?.currency)}
            </p>
            <p className="mt-1 text-xs text-muted-foreground">{t('wallet.ethiopianBirr')}</p>
            {(data?.reserved_balance ?? 0) > 0 && (
              <p className="mt-2 text-[11px] font-medium text-warning">
                {t('wallet.reservedBalanceNote', {
                  amount: formatCurrency(data?.reserved_balance ?? 0, data?.currency),
                })}
              </p>
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}

/** Message from a failed API call — the server message when present, else a fallback. */
function apiErrorMessage(err: unknown, fallback: string): string {
  if (typeof err === 'object' && err !== null) {
    const response = (err as { response?: { data?: { message?: string } } }).response
    if (response?.data?.message) return response.data.message
    const message = (err as { message?: string }).message
    if (message) return message
  }
  return fallback
}

function DepositCard() {
  const { t } = useTranslation()
  const [method, setMethod] = useState<'telebirr' | 'cbe'>('telebirr')
  const [copied, setCopied] = useState(false)
  const [transactionId, setTransactionId] = useState('')
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [previewUrl, setPreviewUrl] = useState<string | null>(null)
  const [isExtracting, setIsExtracting] = useState(false)
  const [extractSuccess, setExtractSuccess] = useState<string | null>(null)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [depositResult, setDepositResult] = useState<ReceiptDepositResult | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const qc = useQueryClient()

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const handleFileChange = async (file: File) => {
    setSelectedFile(file)
    const url = URL.createObjectURL(file)
    setPreviewUrl(url)
    setErrorMessage(null)
    setExtractSuccess(null)
    setIsExtracting(true)

    try {
      const data = await walletApi.extractReceipt(file, method)
      if (data?.transaction_id) {
        setTransactionId(data.transaction_id)
        setExtractSuccess(data.transaction_id)
      }
    } catch (err) {
      setErrorMessage(apiErrorMessage(err, t('wallet.depositFailed')))
    } finally {
      setIsExtracting(false)
    }
  }

  const handleRemoveFile = () => {
    if (previewUrl) URL.revokeObjectURL(previewUrl)
    setSelectedFile(null)
    setPreviewUrl(null)
    setExtractSuccess(null)
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  const depositMutation = useMutation({
    // The uploaded receipt is the source of truth: the server reads the amount
    // off it, verifies the transaction and credits the wallet straight away.
    mutationFn: () => {
      setErrorMessage(null)
      return walletApi.verifyAndDeposit({
        method,
        transaction_id: transactionId.trim() || undefined,
        file: selectedFile || undefined,
      })
    },
    onSuccess: (res) => {
      setDepositResult(res)
      qc.invalidateQueries({ queryKey: ['wallet'] })
      qc.invalidateQueries({ queryKey: ['transactions'] })
    },
    onError: (err: unknown) => {
      setErrorMessage(apiErrorMessage(err, t('wallet.depositFailed')))
    },
  })

  const resetForm = () => {
    handleRemoveFile()
    setTransactionId('')
    setDepositResult(null)
    setErrorMessage(null)
    setExtractSuccess(null)
  }

  return (
    <Card className="bg-card">
      <CardHeader className="pb-3">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <CardTitle className="flex items-center gap-2 whitespace-nowrap text-base">
            <ArrowDownLeft className="h-5 w-5 text-success" />
            {t('wallet.depositTitle')}
          </CardTitle>
          <div className="flex shrink-0 gap-1 self-start rounded-lg bg-muted p-1 sm:self-auto">
            <button
              type="button"
              onClick={() => { setMethod('telebirr'); setErrorMessage(null); setExtractSuccess(null) }}
              className={cn(
                'flex items-center gap-1.5 rounded-md px-3 py-1 text-xs font-medium transition-all',
                method === 'telebirr'
                  ? 'bg-background text-foreground shadow-sm'
                  : 'text-muted-foreground hover:text-foreground'
              )}
            >
              <Smartphone className="h-3.5 w-3.5 text-success" />
              {t('wallet.telebirr')}
            </button>
            <button
              type="button"
              onClick={() => { setMethod('cbe'); setErrorMessage(null); setExtractSuccess(null) }}
              className={cn(
                'flex items-center gap-1.5 rounded-md px-3 py-1 text-xs font-medium transition-all',
                method === 'cbe'
                  ? 'bg-background text-foreground shadow-sm'
                  : 'text-muted-foreground hover:text-foreground'
              )}
            >
              <Building2 className="h-3.5 w-3.5 text-purple-600" />
              {t('wallet.cbe')}
            </button>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        {depositResult ? (
          <div className="space-y-4 rounded-xl border border-success/30 bg-success/10 p-5 text-center">
            <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-success/10 text-success">
              <CheckCircle2 className="h-7 w-7" />
            </div>
            <div>
              <h3 className="text-lg font-bold text-success">
                {t('wallet.depositSuccess', { amount: formatCurrency(depositResult.receipt.amount) })}
              </h3>
              <p className="mt-1 text-xs text-muted-foreground">
                {depositResult.receipt.receiver_name && `${t('wallet.accountHolderName')}: ${depositResult.receipt.receiver_name}`}
              </p>
            </div>
            <div className="flex flex-wrap justify-center gap-2 pt-2 text-xs">
              <Badge variant="outline" className="font-mono">
                {t('wallet.reference')}: {depositResult.receipt.transaction_ref}
              </Badge>
              {depositResult.receipt.transfer_date && (
                <Badge variant="outline">
                  {depositResult.receipt.transfer_date}
                </Badge>
              )}
              {depositResult.receipt.receipt_url && (
                <a
                  href={depositResult.receipt.receipt_url}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center gap-1 rounded-md border border-input bg-background px-2.5 py-0.5 font-medium text-primary hover:underline"
                >
                  <ExternalLink className="h-3 w-3" />
                  {t('wallet.viewReceipt')}
                </a>
              )}
            </div>
            <Button onClick={resetForm} variant="outline" className="mt-4">
              {t('wallet.requestAnotherDeposit', { defaultValue: 'Make another deposit' })}
            </Button>
          </div>
        ) : (
          <>
            {/* Payment Destination Details */}
            <div className="rounded-lg border bg-muted/40 p-3.5 sm:p-4">
              {method === 'telebirr' ? (
                <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                  <div>
                    <p className="text-xs text-muted-foreground">{t('wallet.sendMoneyToTelebirr')}</p>
                    <div className="flex items-baseline gap-2">
                      <p className="text-lg font-bold tracking-wide font-mono">{TELEBIRR_PHONE}</p>
                      <span className="text-xs font-medium text-muted-foreground">({TELEBIRR_ACCOUNT_NAME})</span>
                    </div>
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => handleCopy(TELEBIRR_PHONE)}
                    className="gap-1.5 self-start sm:self-auto"
                  >
                    {copied ? (
                      <><Check className="h-3.5 w-3.5 text-success" /> {t('wallet.copied')}</>
                    ) : (
                      <><Copy className="h-3.5 w-3.5" /> {t('wallet.copyNumber')}</>
                    )}
                  </Button>
                </div>
              ) : (
                <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                  <div>
                    <p className="text-xs text-muted-foreground">{t('wallet.sendMoneyToCbe')}</p>
                    <div className="flex items-baseline gap-2">
                      <p className="text-lg font-bold tracking-wide font-mono">{CBE_ACCOUNT_NUMBER}</p>
                      <span className="text-xs font-medium text-muted-foreground">({CBE_ACCOUNT_NAME})</span>
                    </div>
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => handleCopy(CBE_ACCOUNT_NUMBER)}
                    className="gap-1.5 self-start sm:self-auto"
                  >
                    {copied ? (
                      <><Check className="h-3.5 w-3.5 text-success" /> {t('wallet.copied')}</>
                    ) : (
                      <><Copy className="h-3.5 w-3.5" /> {t('wallet.copyAccount')}</>
                    )}
                  </Button>
                </div>
              )}
            </div>

            {/* Error Message */}
            {errorMessage && (
              <div className="flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-xs text-destructive">
                <AlertCircle className="h-4 w-4 shrink-0 mt-0.5" />
                <span>{errorMessage}</span>
              </div>
            )}

            {/* Verification & Upload section */}
            <div className="space-y-4">
              {/* Screenshot instructions */}
              <div className="rounded-lg border border-info/30 bg-info/10 p-3.5 text-xs text-info">
                <p className="flex items-center gap-1.5 font-semibold">
                  <Camera className="h-3.5 w-3.5 shrink-0" />
                  {t('wallet.uploadHowTo')}
                </p>
                <ol className="mt-1.5 list-decimal space-y-1 pl-4 leading-relaxed marker:font-semibold">
                  <li>{t('wallet.uploadStep1', { method: method === 'cbe' ? t('wallet.cbe') : t('wallet.telebirr') })}</li>
                  <li>{t('wallet.uploadStep2', { method: method === 'cbe' ? t('wallet.cbe') : t('wallet.telebirr') })}</li>
                  <li>{t('wallet.uploadStep3')}</li>
                </ol>
              </div>

              <div className="space-y-2">
                <label className="text-xs font-semibold text-foreground flex items-center justify-between">
                  <span>{t('wallet.uploadScreenshot')}</span>
                  {/* <span className="text-[11px] font-normal text-muted-foreground">{t('wallet.autoVerified')}</span> */}
                </label>

                {/* Dropzone / Preview */}
                {!selectedFile ? (
                  <div
                    onClick={() => fileInputRef.current?.click()}
                    onDragOver={(e) => { e.preventDefault(); e.stopPropagation() }}
                    onDrop={(e) => {
                      e.preventDefault(); e.stopPropagation()
                      if (e.dataTransfer.files?.[0]) handleFileChange(e.dataTransfer.files[0])
                    }}
                    className="flex flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed border-muted-foreground/25 bg-muted/20 p-5 text-center cursor-pointer hover:border-primary/50 hover:bg-muted/40 transition-colors"
                  >
                    <Upload className="h-6 w-6 text-muted-foreground" />
                    <div>
                      <p className="text-xs font-medium text-foreground">{t('wallet.uploadScreenshotPrompt')}</p>
                      <p className="text-[11px] text-muted-foreground mt-0.5">
                        {method === 'cbe' ? t('wallet.cbeScreenshotTip') : t('wallet.telebirrScreenshotTip')}
                      </p>
                    </div>
                  </div>
                ) : (
                  <div className="flex items-center gap-3 rounded-lg border bg-muted/20 p-2.5">
                    {previewUrl ? (
                      <img src={previewUrl} alt={t('wallet.receiptPreview')} className="h-14 w-14 rounded-md object-cover border" />
                    ) : (
                      <ImageIcon className="h-10 w-10 text-muted-foreground" />
                    )}
                    <div className="flex-1 min-w-0">
                      <p className="truncate text-xs font-medium text-foreground">{selectedFile.name}</p>
                      {isExtracting ? (
                        <div className="flex items-center gap-1.5 text-xs text-primary mt-0.5">
                          <Loader2 className="h-3 w-3 animate-spin" />
                          <span>{t('wallet.extracting')}</span>
                        </div>
                      ) : extractSuccess ? (
                        <div className="flex items-center gap-1 mt-0.5 text-xs text-success">
                          <span>{t('wallet.extractedSuccess')}: <strong className="font-mono">{extractSuccess}</strong></span>
                        </div>
                      ) : (
                        <p className="text-[11px] text-muted-foreground">{(selectedFile.size / 1024).toFixed(0)} KB</p>
                      )}
                    </div>
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      onClick={handleRemoveFile}
                      className="h-8 w-8 p-0 text-muted-foreground hover:text-destructive"
                    >
                      <X className="h-4 w-4" />
                    </Button>
                  </div>
                )}

                <input
                  ref={fileInputRef}
                  type="file"
                  accept="image/*"
                  className="hidden"
                  onChange={(e) => {
                    if (e.target.files?.[0]) handleFileChange(e.target.files[0])
                  }}
                />
              </div>

              {/* Transaction Number Input */}
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-foreground" htmlFor="tx-number-input">
                  {method === 'cbe' ? t('wallet.cbeReceiptTokenLabel') : t('wallet.transactionNumber')}
                </label>
                <div className="relative">
                  <Input
                    id="tx-number-input"
                    value={transactionId}
                    onChange={(e) => setTransactionId(e.target.value)}
                    placeholder={method === 'telebirr' ? t('wallet.telebirrTokenPlaceholder') : t('wallet.cbeTokenPlaceholder')}
                    className="font-mono text-sm"
                  />
                  {extractSuccess && transactionId === extractSuccess && (
                    <span className="absolute right-3 top-1/2 -translate-y-1/2 text-success">
                      <Check className="h-4 w-4" />
                    </span>
                  )}
                </div>
                {method === 'cbe' ? (
                  <p className="text-[11px] text-muted-foreground">{t('wallet.cbeTip')}</p>
                ) : (
                  <p className="text-[11px] text-muted-foreground">{t('wallet.telebirrTip')}</p>
                )}
              </div>

              {/* Submit button */}
              <Button
                className="w-full gap-2 font-medium"
                disabled={(!transactionId.trim() && !selectedFile) || depositMutation.isPending || isExtracting}
                onClick={() => depositMutation.mutate()}
              >
                {depositMutation.isPending ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" />
                    {t('wallet.verifying')}
                  </>
                ) : (
                  <>
                    <FileCheck className="h-4 w-4" />
                    {t('wallet.verifyAndDeposit')}
                  </>
                )}
              </Button>

              {/* Notice */}
              <p className="text-[11px] text-muted-foreground leading-relaxed">
                {t('wallet.verificationNotice')}
              </p>
            </div>
          </>
        )}
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
            <ArrowUpRight className="h-4 w-4 text-destructive" />
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
              <div className="flex items-center gap-2 rounded-md border border-info/30 bg-info/10 px-3 py-2 text-sm text-info">
                <Clock className="h-4 w-4 shrink-0" />
                {t('wallet.withdrawSubmitted')}
              </div>
            )}
            {withdrawMutation.isError && (
              <div className="flex items-center gap-2 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
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
                <div className="rounded-lg border border-warning/30 bg-warning/10 p-3 text-xs text-warning">
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

  const isCredit = (type: string) =>
    type === 'deposit' || type === 'p2p_stake_refund' || type === 'p2p_payout'
  const isDebit = (type: string) =>
    type === 'withdraw' || type === 'p2p_match_stake' || type === 'p2p_platform_fee'

  // P2P ledger rows keep their own categories instead of being folded into
  // ordinary deposits/withdrawals.
  const txLabel = (tx: typeof txs[0]) => {
    switch (tx.type) {
      case 'p2p_match_stake':
        return tx.status === 'reserved' ? t('wallet.stakeReserved') : t('wallet.stakeSettled')
      case 'p2p_stake_refund':
        return t('wallet.stakeRefund')
      case 'p2p_payout':
        return t('wallet.matchPrize')
      case 'p2p_platform_fee':
        return t('wallet.platformFee')
    }
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
          <option value="p2p_match_stake">{t('wallet.matchStakes', 'Match stakes')}</option>
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
                      {isCredit(tx.type)
                        ? <ArrowDownLeft className="h-3.5 w-3.5 text-success" />
                        : <ArrowUpRight className="h-3.5 w-3.5 text-destructive" />}
                      <Badge variant={
                        tx.status === 'pending' ? 'warning'
                          : tx.status === 'rejected' ? 'destructive'
                          : isCredit(tx.type) ? 'success'
                            : tx.type === 'p2p_platform_fee' ? 'outline' : 'destructive'
                      }>
                        {txLabel(tx)}
                      </Badge>
                    </span>
                  </td>
                  <td className={cn('py-3 pr-4 font-semibold tabular-nums',
                    isCredit(tx.type) ? 'text-success'
                      : isDebit(tx.type) ? 'text-destructive' : 'text-muted-foreground')}>
                    {isCredit(tx.type) ? '+' : isDebit(tx.type) ? '-' : ''}{formatCurrency(tx.amount)}
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
      <div className="grid gap-6 lg:grid-cols-12">
        <div className="space-y-6 lg:col-span-4">
          <BalanceCard />
          <WithdrawCard />
        </div>
        <div className="lg:col-span-8">
          <DepositCard />
        </div>
      </div>
      <TransactionHistory />
    </div>
  )
}
