import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { ArrowDownRight, ArrowUpRight, ChevronLeft, ChevronRight, Inbox, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { cn } from '@/lib/utils'
import { STATUS_VARIANT, formatNumber } from './AdminUtils'

const STATUS_I18N_MAP: Record<string, string> = {
  active: 'admin.statusActive',
  banned: 'admin.statusBanned',
  completed: 'admin.statusCompleted',
  cancelled: 'admin.statusCancelled',
  tied: 'admin.statusTied',
  in_progress: 'admin.statusInProgress',
  approved: 'admin.statusApproved',
  rejected: 'admin.statusRejected',
  pending: 'admin.statusPending',
  settled: 'admin.statusSettled',
  refunded: 'admin.statusRefunded',
}

export function StatusBadge({ status }: { status: string }) {
  const { t } = useTranslation()
  const key = STATUS_I18N_MAP[status] ?? `admin.status${status.split('_').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join('')}`
  const label = t(key, status.replaceAll('_', ' '))
  return <Badge variant={STATUS_VARIANT[status] ?? 'outline'} className="capitalize">{label}</Badge>
}

const FINANCIAL_KEY: Record<string, string> = {
  settled: 'admin.statusSettled',
  refunded: 'admin.statusRefunded',
  reserved: 'admin.statusReserved',
  pending: 'admin.statusPending',
  needs_attention: 'admin.statusNeedsAttention',
  not_applicable: 'admin.statusNotApplicable',
}

/** Financial status cross-referenced with wallet transactions. */
export function FinancialStatusBadge({ status }: { status: string }) {
  const { t } = useTranslation()
  const key = FINANCIAL_KEY[status]
  const label = key ? t(key, status.replaceAll('_', ' ')) : status.replaceAll('_', ' ')
  return (
    <Badge variant={STATUS_VARIANT[status] ?? 'outline'} className="capitalize">
      {label}
    </Badge>
  )
}

export function PageHeader({
  eyebrow,
  title,
  description,
  actions,
}: {
  eyebrow?: string
  title: string
  description?: string
  actions?: ReactNode
}) {
  return (
    <header className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
      <div className="min-w-0">
        {eyebrow && (
          <p className="mb-1 text-[10px] font-bold uppercase tracking-[0.22em] text-primary">{eyebrow}</p>
        )}
        <h1 className="text-2xl font-black tracking-tight sm:text-3xl">{title}</h1>
        {description && <p className="mt-1 max-w-2xl text-sm text-muted-foreground">{description}</p>}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </header>
  )
}

export function KpiCard({
  label,
  value,
  icon: Icon,
  detail,
  change,
  accent = 'brand',
  loading,
}: {
  label: string
  value: ReactNode
  icon: React.ElementType
  detail?: string
  change?: number | null
  accent?: 'brand' | 'green' | 'amber' | 'blue' | 'neutral'
  loading?: boolean
}) {
  const accents = {
    brand: 'bg-primary/10 text-primary',
    green: 'bg-success/10 text-success',
    amber: 'bg-warning/10 text-warning',
    blue: 'bg-info/10 text-info',
    neutral: 'bg-muted text-muted-foreground',
  }
  const positive = (change ?? 0) >= 0
  return (
    <Card className="group relative overflow-hidden border-border/70 transition-colors hover:border-border">
      <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-primary/50 to-transparent opacity-0 transition-opacity group-hover:opacity-100" />
      <CardContent className="p-4 sm:p-5">
        <div className="flex items-start justify-between gap-3">
          <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{label}</p>
          <span className={cn('flex h-8 w-8 shrink-0 items-center justify-center rounded-lg', accents[accent])}>
            <Icon className="h-4 w-4" />
          </span>
        </div>
        <div className="mt-3 flex flex-wrap items-end gap-2">
          {loading ? <span className="h-8 w-24 animate-pulse rounded bg-muted" /> : (
            <p className="text-2xl font-black tracking-tight tabular-nums">{value}</p>
          )}
          {change !== undefined && change !== null && !loading && (
            <span className={cn(
              'mb-0.5 inline-flex items-center gap-0.5 rounded-full px-1.5 py-0.5 text-[10px] font-bold',
              positive ? 'bg-success/10 text-success' : 'bg-destructive/10 text-destructive',
            )}>
              {positive ? <ArrowUpRight className="h-3 w-3" /> : <ArrowDownRight className="h-3 w-3" />}
              {Math.abs(change).toFixed(1)}%
            </span>
          )}
        </div>
        {detail && <p className="mt-1 text-xs text-muted-foreground">{detail}</p>}
      </CardContent>
    </Card>
  )
}

export function Panel({
  title,
  description,
  action,
  children,
  className,
}: {
  title: string
  description?: string
  action?: ReactNode
  children: ReactNode
  className?: string
}) {
  return (
    <Card className={cn('overflow-hidden border-border/70', className)}>
      <div className="flex flex-wrap items-start justify-between gap-3 border-b border-border/60 px-4 py-3 sm:px-5">
        <div>
          <h2 className="text-sm font-bold">{title}</h2>
          {description && <p className="mt-0.5 text-xs text-muted-foreground">{description}</p>}
        </div>
        {action}
      </div>
      {children}
    </Card>
  )
}

export function EmptyState({ title, description }: { title?: string; description?: string }) {
  const { t } = useTranslation()
  const displayTitle = title ?? t('admin.nothingToShow', 'Nothing to show yet')
  return (
    <div className="flex flex-col items-center justify-center px-4 py-14 text-center">
      <span className="mb-3 flex h-11 w-11 items-center justify-center rounded-xl bg-muted text-muted-foreground">
        <Inbox className="h-5 w-5" />
      </span>
      <p className="text-sm font-semibold">{displayTitle}</p>
      {description && <p className="mt-1 max-w-sm text-xs text-muted-foreground">{description}</p>}
    </div>
  )
}

export function ErrorState({ message, onRetry }: { message?: string; onRetry?: () => void }) {
  const { t } = useTranslation()
  return (
    <div className="flex flex-col items-center justify-center px-4 py-12 text-center">
      <p className="text-sm font-semibold text-destructive">{t('admin.unableToLoad', 'Unable to load this data')}</p>
      <p className="mt-1 max-w-md text-xs text-muted-foreground">{message || t('admin.serverDidNotRespond', 'The server did not respond. Try again.')}</p>
      {onRetry && (
        <Button variant="outline" size="sm" className="mt-3 gap-1.5" onClick={onRetry}>
          <RefreshCw className="h-3.5 w-3.5" /> {t('admin.retry', 'Retry')}
        </Button>
      )}
    </div>
  )
}

export function Avatar({ name, src, size = 'md' }: { name?: string; src?: string; size?: 'sm' | 'md' | 'lg' }) {
  const dims = { sm: 'h-7 w-7 text-[10px]', md: 'h-9 w-9 text-xs', lg: 'h-14 w-14 text-lg' }
  return (
    <span className={cn('flex shrink-0 items-center justify-center overflow-hidden rounded-full border border-border/60 bg-muted font-bold text-muted-foreground', dims[size])}>
      {src ? <img src={src} alt="" className="h-full w-full object-cover" /> : (name?.charAt(0).toUpperCase() || '?')}
    </span>
  )
}

export function Pagination({
  total,
  limit,
  offset,
  onChange,
  loading,
}: {
  total: number
  limit: number
  offset: number
  onChange: (offset: number) => void
  loading?: boolean
}) {
  const { t } = useTranslation()
  if (total <= limit) return null
  const page = Math.floor(offset / limit) + 1
  const pages = Math.max(1, Math.ceil(total / limit))
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border/60 px-4 py-3 text-xs sm:px-5">
      <span className="text-muted-foreground">
        {t('admin.showingRange', {
          from: offset + 1,
          to: Math.min(offset + limit, total),
          total: formatNumber(total),
          page,
          pages,
          defaultValue: `${offset + 1}–${Math.min(offset + limit, total)} of ${formatNumber(total)} · Page ${page} of ${pages}`
        })}
      </span>
      <div className="flex gap-2">
        <Button variant="outline" size="sm" className="gap-1" disabled={offset <= 0 || loading} onClick={() => onChange(Math.max(0, offset - limit))}>
          <ChevronLeft className="h-3.5 w-3.5" /> {t('admin.previous', 'Previous')}
        </Button>
        <Button variant="outline" size="sm" className="gap-1" disabled={offset + limit >= total || loading} onClick={() => onChange(offset + limit)}>
          {t('admin.next', 'Next')} <ChevronRight className="h-3.5 w-3.5" />
        </Button>
      </div>
    </div>
  )
}

export function DateRangeBar({
  from,
  to,
  onChange,
  children,
}: {
  from: string
  to: string
  onChange: (from: string, to: string) => void
  children?: ReactNode
}) {
  const { t } = useTranslation()
  const preset = (days: number) => {
    const end = new Date()
    const start = new Date()
    start.setDate(start.getDate() - days + 1)
    onChange(start.toISOString().slice(0, 10), end.toISOString().slice(0, 10))
  }
  const presets = [
    { days: 1, label: t('admin.today', 'Today') },
    { days: 7, label: t('admin.d7', '7D') },
    { days: 30, label: t('admin.d30', '30D') },
    { days: 90, label: t('admin.d90', '90D') }
  ]
  return (
    <div className="flex flex-wrap items-center gap-2">
      {children}
      <div className="flex rounded-lg border border-input bg-background p-0.5">
        {presets.map(item => (
          <button
            key={item.days}
            type="button"
            onClick={() => preset(item.days)}
            className="rounded-md px-2.5 py-1.5 text-[11px] font-bold text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
          >
            {item.label}
          </button>
        ))}
      </div>
      <span className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">UTC</span>
      <div className="flex items-center gap-1.5">
        <input
          type="date"
          aria-label={t('admin.fromDate', 'From date')}
          value={from}
          max={to}
          onChange={event => onChange(event.target.value, to)}
          className="h-9 min-w-0 rounded-lg border border-input bg-background px-2 text-xs"
        />
        <span className="text-muted-foreground">–</span>
        <input
          type="date"
          aria-label={t('admin.toDate', 'To date')}
          value={to}
          min={from}
          onChange={event => onChange(from, event.target.value)}
          className="h-9 min-w-0 rounded-lg border border-input bg-background px-2 text-xs"
        />
      </div>
    </div>
  )
}

export function ViewLink({ to, children }: { to: string; children: ReactNode }) {
  return <Link to={to} className="text-xs font-semibold text-primary hover:underline">{children}</Link>
}

