import { useTranslation } from 'react-i18next'
import { AlertTriangle } from 'lucide-react'
import { cn } from '@/lib/utils'

interface ErrorMessageProps {
  message?: string
  className?: string
}

/**
 * Consistent error message banner used across the app.
 * Replaces ad-hoc `bg-destructive/10 px-3 py-2 text-sm text-destructive` patterns.
 */
export function ErrorMessage({ message, className }: ErrorMessageProps) {
  const { t } = useTranslation()
  const text = message ?? t('errors.somethingWentWrong')
  if (!text) return null
  return (
    <div
      className={cn(
        'flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive',
        className,
      )}
      role="alert"
    >
      <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
      <span>{text}</span>
    </div>
  )
}
