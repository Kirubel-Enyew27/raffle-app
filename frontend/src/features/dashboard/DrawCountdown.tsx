import { useQuery } from '@tanstack/react-query'
import { Clock } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { useCountdown } from '@/hooks/useCountdown'
import { dashboardApi } from './api'

function CountdownUnit({ value, label }: { value: number; label: string }) {
  return (
    <div className="flex flex-col items-center">
      <span className="text-2xl font-bold tabular-nums">{String(value).padStart(2, '0')}</span>
      <span className="text-xs text-muted-foreground">{label}</span>
    </div>
  )
}

function Divider() {
  return <span className="mb-4 text-xl font-bold text-muted-foreground">:</span>
}

function LiveCountdown({ drawDate, title }: { drawDate: string; title: string }) {
  const { t } = useTranslation()
  const { days, hours, minutes, seconds, expired } = useCountdown(drawDate)
  if (expired) return (
    <div className="text-center">
      <p className="text-sm font-medium">{title}</p>
      <p className="mt-1 text-xs text-muted-foreground">{t('raffles.status.completed')}</p>
    </div>
  )
  return (
    <div>
      <p className="mb-3 text-sm font-medium line-clamp-1">{title}</p>
      <div className="flex items-end gap-1">
        <CountdownUnit value={days} label={t('common.loading') === t('common.loading') ? 'days' : 'ቀናት'} />
        <Divider />
        <CountdownUnit value={hours} label="hrs" />
        <Divider />
        <CountdownUnit value={minutes} label="min" />
        <Divider />
        <CountdownUnit value={seconds} label="sec" />
      </div>
    </div>
  )
}

export function DrawCountdown() {
  const { t } = useTranslation()
  const { data, isLoading, error } = useQuery({
    queryKey: ['raffles', 'active'],
    queryFn: () => dashboardApi.raffles(6),
    // shared with ActiveRaffles — served from cache
  })

  const next = data?.raffles
    ?.filter(r => r.status === 'active' && r.draw_date)
    .sort((a, b) => new Date(a.draw_date).getTime() - new Date(b.draw_date).getTime())[0]

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">{t('dashboard.nextDraw')}</CardTitle>
        <Clock className="h-4 w-4 text-muted-foreground" />
      </CardHeader>
      <CardContent>
        {isLoading && <Skeleton className="h-16 w-full" />}
        {error && <p className="text-sm text-destructive">{t('common.error')}</p>}
        {!isLoading && !next && (
          <p className="py-2 text-sm text-muted-foreground">{t('common.noData')}</p>
        )}
        {next && <LiveCountdown drawDate={next.draw_date} title={next.title} />}
      </CardContent>
    </Card>
  )
}
