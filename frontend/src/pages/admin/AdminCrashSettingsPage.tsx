import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Save, Settings2, Info } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { crashApi } from '@/features/crash/api'
import { usePageTitle } from '@/hooks/usePageTitle'

interface SettingRow {
  key: string
  labelKey: string
  descKey: string
  unitKey: string
  /** If set, the stored integer (e.g. basis points) is divided by this to show a human value */
  displayDivisor?: number
  min: number
  max: number
  step: number
}

const SETTINGS: SettingRow[] = [
  {
    key: 'service_fee_bp',
    labelKey: 'admin.crashSettingsRows.service_fee_bp_label',
    descKey: 'admin.crashSettingsRows.service_fee_bp_desc',
    unitKey: 'admin.crashSettingsRows.service_fee_bp_unit',
    displayDivisor: 100,
    min: 0,
    max: 50,
    step: 0.01,
  },
  {
    key: 'min_participants',
    labelKey: 'admin.crashSettingsRows.min_participants_label',
    descKey: 'admin.crashSettingsRows.min_participants_desc',
    unitKey: 'admin.crashSettingsRows.min_participants_unit',
    min: 1,
    max: 100,
    step: 1,
  },
  {
    key: 'max_participants',
    labelKey: 'admin.crashSettingsRows.max_participants_label',
    descKey: 'admin.crashSettingsRows.max_participants_desc',
    unitKey: 'admin.crashSettingsRows.max_participants_unit',
    min: 2,
    max: 1000,
    step: 1,
  },
  {
    key: 'min_bet_cents',
    labelKey: 'admin.crashSettingsRows.min_bet_cents_label',
    descKey: 'admin.crashSettingsRows.min_bet_cents_desc',
    unitKey: 'admin.crashSettingsRows.min_bet_cents_unit',
    min: 100,
    max: 1_000_000,
    step: 100,
  },
  {
    key: 'max_bet_cents',
    labelKey: 'admin.crashSettingsRows.max_bet_cents_label',
    descKey: 'admin.crashSettingsRows.max_bet_cents_desc',
    unitKey: 'admin.crashSettingsRows.max_bet_cents_unit',
    min: 1000,
    max: 100_000_000,
    step: 100,
  },
  {
    key: 'betting_seconds',
    labelKey: 'admin.crashSettingsRows.betting_seconds_label',
    descKey: 'admin.crashSettingsRows.betting_seconds_desc',
    unitKey: 'admin.crashSettingsRows.betting_seconds_unit',
    min: 5,
    max: 120,
    step: 1,
  },
  {
    key: 'waiting_seconds',
    labelKey: 'admin.crashSettingsRows.waiting_seconds_label',
    descKey: 'admin.crashSettingsRows.waiting_seconds_desc',
    unitKey: 'admin.crashSettingsRows.waiting_seconds_unit',
    min: 0,
    max: 60,
    step: 1,
  },
  {
    key: 'crashed_seconds',
    labelKey: 'admin.crashSettingsRows.crashed_seconds_label',
    descKey: 'admin.crashSettingsRows.crashed_seconds_desc',
    unitKey: 'admin.crashSettingsRows.crashed_seconds_unit',
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
  const { t } = useTranslation()
  usePageTitle(`${t('admin.crashGameSettings')} · KWIK Admin`)
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
          <h1 className="text-2xl font-bold">{t('admin.crashGameSettings')}</h1>
          <p className="text-sm text-muted-foreground">
            {t('admin.crashSettingsDesc')}
          </p>
        </div>
      </div>

      {/* Fee model explanation */}
      <div className="flex items-start gap-2 rounded-lg border border-info/30 bg-info/5 px-4 py-3 text-xs text-info">
        <Info className="mt-0.5 h-3.5 w-3.5 shrink-0" />
        <div className="space-y-1">
          <p>
            <strong>{t('admin.p2pRevenueModel')}</strong> {t('admin.p2pRevenueModelDesc')}
          </p>
          <p>
            {t('admin.crashFeeExample', {
              feePct: feePct.toFixed(2),
              fee: (900 * feePct / 100).toFixed(2),
              payout: (1000 - 900 * feePct / 100).toFixed(2),
            })}
          </p>
        </div>
      </div>

      {isLoading && (
        <p className="text-sm text-muted-foreground">{t('admin.loadingSettings')}</p>
      )}

      {isError && (
        <p className="text-sm text-destructive">{t('admin.failedToLoadSettings')}</p>
      )}

      {settings && (
        <div className="space-y-4">
          {SETTINGS.map((row) => (
            <Card key={row.key}>
              <CardHeader className="flex-row items-center justify-between space-y-0 py-3">
                <CardTitle className="flex items-center gap-2 text-sm">
                  {t(row.labelKey)}
                  <Badge variant="outline" className="text-[10px]">
                    {row.key}
                  </Badge>
                </CardTitle>
                {saved === row.key && (
                  <Badge variant="success" className="text-xs">{t('admin.saved')}</Badge>
                )}
              </CardHeader>
              <CardContent className="space-y-2 pb-4">
                <p className="text-xs text-muted-foreground">{t(row.descKey)}</p>
                <div className="flex items-center gap-2">
                  <div className="flex-1">
                    <Label htmlFor={`setting-${row.key}`} className="sr-only">
                      {t(row.labelKey)}
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
                  <span className="min-w-[4rem] text-xs text-muted-foreground">{t(row.unitKey)}</span>
                  <Button
                    size="sm"
                    onClick={() => handleSave(row)}
                    disabled={update.isPending || !(row.key in edits)}
                  >
                    <Save className="h-3.5 w-3.5" />
                    {t('admin.save')}
                  </Button>
                </div>
                <p className="text-[10px] text-muted-foreground">
                  {t('admin.currentSetting', { value: displayValue(row, settings[row.key] ?? '—'), unit: t(row.unitKey) })}
                </p>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}
