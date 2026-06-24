import { useState } from 'react'
import { defaultRange } from '@/features/admin/api'
import { RevenueReport } from '@/features/admin/RevenueReport'
import { TicketsReport } from '@/features/admin/TicketsReport'
import { WinnerSummaryReport } from '@/features/admin/WinnerSummaryReport'
import { ProfitSummaryReport } from '@/features/admin/ProfitSummaryReport'
import { usePageTitle } from '@/hooks/usePageTitle'
import { cn } from '@/lib/utils'

const TABS = [
  { id: 'profit',   label: 'Profit Summary' },
  { id: 'revenue',  label: 'Revenue' },
  { id: 'tickets',  label: 'Tickets Sold' },
  { id: 'winners',  label: 'Winner Summary' },
] as const

type Tab = typeof TABS[number]['id']

export function Component() {
  usePageTitle('Admin Reports')
  const [tab, setTab] = useState<Tab>('profit')
  const [range, setRange] = useState(defaultRange)
  const [period, setPeriod] = useState('daily')

  return (
    <div className="space-y-6 p-4 sm:p-6">
      <div>
        <h1 className="text-2xl font-bold">Reports</h1>
        <p className="text-sm text-muted-foreground">Platform analytics &amp; exports</p>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b">
        {TABS.map(t => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            className={cn(
              'px-4 py-2 text-sm font-medium transition-colors border-b-2 -mb-px',
              tab === t.id
                ? 'border-primary text-foreground'
                : 'border-transparent text-muted-foreground hover:text-foreground',
            )}
          >
            {t.label}
          </button>
        ))}
      </div>

      {/* Sections */}
      {tab === 'profit'  && <ProfitSummaryReport range={range} onRange={setRange} />}
      {tab === 'revenue' && <RevenueReport range={range} onRange={setRange} period={period} onPeriod={setPeriod} />}
      {tab === 'tickets' && <TicketsReport  range={range} onRange={setRange} period={period} onPeriod={setPeriod} />}
      {tab === 'winners' && <WinnerSummaryReport range={range} onRange={setRange} />}
    </div>
  )
}
