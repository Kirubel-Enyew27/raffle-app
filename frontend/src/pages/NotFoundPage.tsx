import { useTranslation } from 'react-i18next'
import { usePageTitle } from '@/hooks/usePageTitle'

export function Component() {
  const { t } = useTranslation()
  usePageTitle(t('notFound.title'))
  return (
    <div className="flex min-h-screen items-center justify-center">
      <div className="text-center space-y-2">
        <h1 className="text-4xl font-bold">{t('notFound.heading')}</h1>
        <p className="text-muted-foreground">{t('notFound.message')}</p>
      </div>
    </div>
  )
}
