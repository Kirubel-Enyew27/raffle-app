import { useTranslation } from 'react-i18next'
import { KwikWordmark } from '@/components/brand/KwikWordmark'

interface KwikSplashProps {
  message?: string
}

/** Branded first-paint screen shown while the local session is hydrated. */
export function KwikSplash({ message }: KwikSplashProps) {
  const { t } = useTranslation()
  const label = message ?? t('common.loadingArena')
  return (
    <div
      role="status"
      aria-live="polite"
      className="flex min-h-screen items-center justify-center bg-background px-6 text-foreground"
    >
      <div className="flex flex-col items-center text-center">
        <div className="overflow-hidden rounded-2xl border border-border/60 shadow-sm">
          <KwikWordmark className="h-20 w-60 sm:h-24 sm:w-72" />
        </div>
        <div aria-hidden="true" className="mt-7 h-1 w-36 overflow-hidden rounded-full bg-muted">
          <span className="block h-full w-1/2 animate-pulse rounded-full bg-primary motion-reduce:animate-none" />
        </div>
        <p className="mt-4 text-[11px] font-bold uppercase tracking-[0.2em] text-muted-foreground">
          {label}
        </p>
      </div>
    </div>
  )
}
