import { useTranslation } from 'react-i18next'
import { usePageTitle } from '@/hooks/usePageTitle'
import { ScrollText } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'

export function Component() {
  const { t } = useTranslation()
  usePageTitle(t('admin.auditLogTitle'))
  return (
    <div className="p-6 space-y-4">
      <div>
        <h1 className="text-2xl font-bold">{t('admin.auditLogTitle')}</h1>
        <p className="text-sm text-muted-foreground">{t('admin.auditLogSubtitle')}</p>
      </div>
      <Card>
        <CardContent className="flex flex-col items-center py-16">
          <ScrollText className="mb-3 h-12 w-12 text-muted-foreground/30" />
          <p className="text-lg font-medium text-muted-foreground">{t('admin.comingSoon')}</p>
          <p className="mt-1 text-sm text-muted-foreground/60">{t('admin.auditComingSoon')}</p>
        </CardContent>
      </Card>
    </div>
  )
}
