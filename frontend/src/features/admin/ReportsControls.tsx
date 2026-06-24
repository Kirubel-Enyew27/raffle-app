import { Button } from '@/components/ui/button'
import { Select } from '@/components/ui/select'

export interface Range { from: string; to: string }

interface Props {
  range: Range
  onRange: (r: Range) => void
  period?: string
  onPeriod?: (p: string) => void
  onExport?: () => void
}

function preset(days: number): Range {
  const to = new Date()
  const from = new Date(to)
  from.setDate(from.getDate() - days + 1)
  const fmt = (d: Date) => d.toISOString().split('T')[0]
  return { from: fmt(from), to: fmt(to) }
}

export function exportCSV(filename: string, headers: string[], rows: (string | number)[][]) {
  const lines = [headers.join(','), ...rows.map(r => r.map(v => `"${v}"`).join(','))]
  const blob = new Blob([lines.join('\n')], { type: 'text/csv' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = filename
  a.click()
  URL.revokeObjectURL(a.href)
}

export function ReportsControls({ range, onRange, period, onPeriod, onExport }: Props) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      {[7, 30, 90].map(d => (
        <Button key={d} variant="outline" size="sm" onClick={() => onRange(preset(d))}>{d}d</Button>
      ))}
      <input type="date" value={range.from} max={range.to}
        onChange={e => onRange({ ...range, from: e.target.value })}
        className="h-8 rounded-md border border-input bg-background px-2 text-sm" />
      <span className="text-muted-foreground">–</span>
      <input type="date" value={range.to} min={range.from}
        onChange={e => onRange({ ...range, to: e.target.value })}
        className="h-8 rounded-md border border-input bg-background px-2 text-sm" />
      {onPeriod && (
        <Select value={period} onChange={e => onPeriod(e.target.value)} className="w-28">
          <option value="daily">Daily</option>
          <option value="weekly">Weekly</option>
          <option value="monthly">Monthly</option>
        </Select>
      )}
      {onExport && (
        <Button variant="outline" size="sm" onClick={onExport}>Export CSV</Button>
      )}
    </div>
  )
}
