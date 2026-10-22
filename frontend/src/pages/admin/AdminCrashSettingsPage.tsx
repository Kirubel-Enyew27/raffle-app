import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Save, Settings2, Info } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { crashApi } from '@/features/crash/api'

interface SettingRow {
  key: string
  label: string
  description: string
  unit: string
  /** If set, the stored integer (e.g. basis points) is divided by this to show a human value */
  displayDivisor?: number
  min: number
  max: number
  step: number
}

const SETTINGS: SettingRow[] = [
  {
    key: 'service_fee_bp',
    label: 'Service Fee',
    description: 'Platform fee deducted only from a winner\'s positive profit.',
    unit: '%',
    displayDivisor: 100,
    min: 0,
    max: 50,
    step: 0.01,
  },
  {
    key: 'min_participants',
    label: 'Minimum Participants',
    description: 'Minimum number of players required before a round can start.',
    unit: 'players',
    min: 1,
    max: 100,
    step: 1,
  },
  {
    key: 'max_participants',
    label: 'Maximum Participants',
    description: 'Maximum number of players allowed per round.',
    unit: 'players',
    min: 2,
    max: 1000,
    step: 1,
  },
  {
    key: 'min_bet_cents',
    label: 'Minimum Stake',
    description: 'Minimum stake in minor units (100 = 1.00 ETB).',
    unit: 'ETB (×100)',
    min: 100,
    max: 1_000_000,
    step: 100,
  },
  {
    key: 'max_bet_cents',
    label: 'Maximum Stake',
    description: 'Maximum stake in minor units (100 = 1.00 ETB).',
    unit: 'ETB (×100)',
    min: 1000,
    max: 100_000_000,
    step: 100,
  },
  {
    key: 'betting_seconds',
    label: 'Betting Window',
    description: 'Duration of the betting window per round.',
    unit: 'seconds',
    min: 5,
    max: 120,
    step: 1,
  },
  {
    key: 'waiting_seconds',
    label: 'Waiting Window',
    description: 'Pre-betting countdown displayed to players.',
    unit: 'seconds',
    min: 0,
    max: 60,
    step: 1,
  },
  {
    key: 'crashed_seconds',
    label: 'Crash Display Window',
    description: 'How long the crash result is shown before settlement.',
    unit: 'seconds',
    min: 0,
    max: 30,
    step: 1,
  },
]

function displayValue(row: SettingRow, raw: string): string {
  const n = parseFloat(raw)
  if (isNaN(n)) return raw
  if (row.displayDivisor) return (n / row.displayDivisor).toFixed(2)
  return raw
}

function storageValue(row: SettingRow, display: string): string {
  const n = parseFloat(display)
  if (isNaN(n)) return display
  if (row.displayDivisor) return String(Math.round(n * row.displayDivisor))
  return display
}

export function Component() {
  const queryClient = useQueryClient()

  const { data: settings, isLoading, isError } = useQuery({
    queryKey: ['crash-admin-settings'],
    queryFn: crashApi.getSettings,
  })

  const [edits, setEdits] = useState<Record<string, string>>({})
  const [saved, setSaved] = useState<string | null>(null)

  const update = useMutation({
    mutationFn: ({ key, value }: { key: string; value: string }) =>
      crashApi.updateSetting(key, value),
    onSuccess: (_, { key }) => {
      setSaved(key)
      setTimeout(() => setSaved(null), 2000)
      void queryClient.invalidateQueries({ queryKey: ['crash-admin-settings'] })
    },
  })

  const getDisplayed = (row: SettingRow) => {
    const raw = edits[row.key] ?? settings?.[row.key] ?? ''
    return displayValue(row, raw)
  }

  const handleChange = (row: SettingRow, displayVal: string) => {
    setEdits((prev) => ({ ...prev, [row.key]: storageValue(row, displayVal) }))
  }

  const handleSave = (row: SettingRow) => {
    const value = edits[row.key] ?? settings?.[row.key]
    if (value == null) return
    update.mutate({ key: row.key, value })
  }

  const feePct = settings?.service_fee_bp ? parseFloat(settings.service_fee_bp) / 100 : 5

  return (
    <div className="mx-auto max-w-3xl space-y-6 p-6">
      <div className="flex items-center gap-3">
        <Settings2 className="h-6 w-6" />
        <div>
          <h1 className="text-2xl font-bold">Crash Game Settings</h1>
          <p className="text-sm text-muted-foreground">
            Configure the P2P crash game. Changes apply to new rounds; in-flight rounds
            are never affected.
          </p>
        </div>
      </div>

      {/* Fee model explanation */}
      <div className="flex items-start gap-2 rounded-lg border border-blue-500/30 bg-blue-500/5 px-4 py-3 text-xs text-blue-700 dark:text-blue-300">
        <Info className="mt-0.5 h-3.5 w-3.5 shrink-0" />
        <div className="space-y-1">
          <p>
            <strong>P2P Revenue Model:</strong> The platform charges a service fee only on a winner's
            positive profit (gross return − original stake). Losers and break-even players are never charged.
          </p>
          <p>
            Example: stake 100, gross return 1000, profit 900 → fee = {feePct.toFixed(2)}% × 900 = {(900 * feePct / 100).toFixed(2)} ETB. Net payout = {(1000 - 900 * feePct / 100).toFixed(2)} ETB.
          </p>
        </div>
      </div>

      {isLoading && (
        <p className="text-sm text-muted-foreground">Loading settings…</p>
      )}

      {isError && (
        <p className="text-sm text-destructive">Failed to load settings. Ensure you have admin access.</p>
      )}

      {settings && (
        <div className="space-y-4">
          {SETTINGS.map((row) => (
            <Card key={row.key}>
              <CardHeader className="flex-row items-center justify-between space-y-0 py-3">
                <CardTitle className="flex items-center gap-2 text-sm">
                  {row.label}
                  <Badge variant="outline" className="text-[10px]">
                    {row.key}
                  </Badge>
                </CardTitle>
                {saved === row.key && (
                  <Badge variant="success" className="text-xs">Saved</Badge>
                )}
              </CardHeader>
              <CardContent className="space-y-2 pb-4">
                <p className="text-xs text-muted-foreground">{row.description}</p>
                <div className="flex items-center gap-2">
                  <div className="flex-1">
                    <Label htmlFor={`setting-${row.key}`} className="sr-only">
                      {row.label}
                    </Label>
                    <Input
                      id={`setting-${row.key}`}
                      type="number"
                      min={row.min}
                      max={row.max}
                      step={row.step}
                      value={getDisplayed(row)}
                      onChange={(e) => handleChange(row, e.target.value)}
                    />
                  </div>
                  <span className="min-w-[4rem] text-xs text-muted-foreground">{row.unit}</span>
                  <Button
                    size="sm"
                    onClick={() => handleSave(row)}
                    disabled={update.isPending || !(row.key in edits)}
                  >
                    <Save className="h-3.5 w-3.5" />
                    Save
                  </Button>
                </div>
                <p className="text-[10px] text-muted-foreground">
                  Current: {displayValue(row, settings[row.key] ?? '—')} {row.unit}
                </p>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}
