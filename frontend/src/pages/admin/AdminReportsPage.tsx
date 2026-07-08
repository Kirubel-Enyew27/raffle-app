import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { defaultRange } from '@/features/admin/api'
import { RevenueReport } from '@/features/admin/RevenueReport'
import { TicketsReport } from '@/features/admin/TicketsReport'
import { WinnerSummaryReport } from '@/features/admin/WinnerSummaryReport'
import { ProfitSummaryReport } from '@/features/admin/ProfitSummaryReport'
import { usePageTitle } from '@/hooks/usePageTitle'
import { cn } from '@/lib/utils'

type Tab = 'profit' | 'revenue' | 'tickets' | 'winners'

export function Component() {
  const { t } = useTranslation()
  usePageTitle(t('admin.reportsTitle'))
  const [tab, setTab] = useState<Tab>('profit')
  const [range, setRange] = useState(defaultRange)
  const [period, setPeriod] = useState('daily')

  const tabs: { id: Tab; label: string }[] = [
    { id: 'profit',  label: t('admin.tabProfit')  },
    { id: 'revenue', label: t('admin.tabRevenue') },
    { id: 'tickets', label: t('admin.tabTickets') },
    { id: 'winners', label: t('admin.tabWinners') },
  ]

  return (
    <div className="space-y-6 p-4 sm:p-6">
      <div>
        <h1 className="text-2xl font-bold">{t('admin.reportsTitle')}</h1>
        <p className="text-sm text-muted-foreground">{t('admin.reportsSubtitle')}</p>
      </div>
      <div className="flex gap-1 border-b">
        {tabs.map(tb => (
          <button key={tb.id} onClick={() => setTab(tb.id)}
            className={cn('px-4 py-2 text-sm font-medium transition-colors border-b-2 -mb-px',
              tab === tb.id ? 'border-primary text-foreground' : 'border-transparent text-muted-foreground hover:text-foreground')}>
            {tb.label}
          </button>
        ))}
      </div>
      {tab === 'profit'  && <ProfitSummaryReport range={range} onRange={setRange} />}
      {tab === 'revenue' && <RevenueReport range={range} onRange={setRange} period={period} onPeriod={setPeriod} />}
      {tab === 'tickets' && <TicketsReport  range={range} onRange={setRange} period={period} onPeriod={setPeriod} />}
      {tab === 'winners' && <WinnerSummaryReport range={range} onRange={setRange} />}
    </div>
  )
}
