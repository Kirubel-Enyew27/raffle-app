import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { LanguageSelector } from '@/components/ui/LanguageSelector'

interface AuthCardProps {
  title: string
  subtitle?: string
  children: ReactNode
}

export function AuthCard({ title, subtitle, children }: AuthCardProps) {
  const { t } = useTranslation()
  
  return (
    <div className="w-full max-w-sm px-4">
      <div className="mb-6 text-center">
        <span className="text-2xl font-bold tracking-tight">{t('app.title')}</span>
      </div>
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-xl">{title}</CardTitle>
          {subtitle && <p className="text-sm text-muted-foreground">{subtitle}</p>}
        </CardHeader>
        <CardContent>{children}</CardContent>
      </Card>
      <div className="mt-4 flex justify-center">
        <LanguageSelector />
      </div>
    </div>
  )
}
