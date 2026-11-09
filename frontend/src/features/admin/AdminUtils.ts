import i18n from '@/lib/i18n'
import { formatCurrency } from '@/lib/utils'

export const formatNumber = (value: number) => value.toLocaleString('en-US')
export const formatMoney = (value: number, currency?: string) => formatCurrency(value ?? 0, currency)

export function formatDate(value?: string, withTime = false) {
  if (!value) return '—'
  return new Date(value).toLocaleString(undefined, withTime
    ? { month: 'short', day: 'numeric', year: 'numeric', hour: '2-digit', minute: '2-digit' }
    : { month: 'short', day: 'numeric', year: 'numeric' })
}

export function shortId(value?: string) {
  if (!value) return '—'
  return `${value.slice(0, 8)}…`
}

export function formatDuration(seconds: number) {
  if (!seconds || seconds < 0) return '—'
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ${seconds % 60}s`
  return `${Math.floor(minutes / 60)}h ${minutes % 60}m`
}

export function relativeTime(value?: string) {
  if (!value) return '—'
  const diff = Date.now() - new Date(value).getTime()
  const minutes = Math.floor(diff / 60_000)
  if (minutes < 1) return i18n.t('time.justNow')
  if (minutes < 60) return i18n.t('time.minutesAgo', { count: minutes })
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return i18n.t('time.hoursAgo', { count: hours })
  const days = Math.floor(hours / 24)
  if (days < 30) return i18n.t('time.daysAgo', { count: days })
  return formatDate(value)
}

export const STATUS_VARIANT: Record<string, 'default' | 'secondary' | 'destructive' | 'outline' | 'success' | 'warning'> = {
  active: 'success',
  completed: 'success',
  credited: 'success',
  settled: 'success',
  approved: 'success',
  scheduled: 'warning',
  draft: 'secondary',
  closed: 'destructive',
  in_progress: 'default',
  running: 'default',
  ready: 'warning',
  waiting: 'warning',
  pending: 'warning',
  reserved: 'warning',
  verifying: 'warning',
  manual_review: 'warning',
  tied: 'outline',
  draw: 'outline',
  fun: 'secondary',
  refunded: 'secondary',
  not_applicable: 'secondary',
  cancelled: 'destructive',
  failed: 'destructive',
  rejected: 'destructive',
  banned: 'destructive',
  forfeited: 'destructive',
  needs_attention: 'destructive',
}

export function chartTooltipStyle() {
  return {
    backgroundColor: 'var(--color-popover)',
    border: '1px solid var(--color-border)',
    borderRadius: 12,
    color: 'var(--color-popover-foreground)',
    fontSize: 12,
    boxShadow: '0 12px 28px rgb(0 0 0 / 0.14)',
  }
}
