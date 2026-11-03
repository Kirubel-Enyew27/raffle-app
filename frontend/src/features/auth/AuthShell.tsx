import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { LanguageSelector } from '@/components/ui/LanguageSelector'
import { KwikWordmark } from '@/components/brand/KwikWordmark'

interface AuthShellProps {
  children: ReactNode
}

/**
 * The branded frame every authentication screen sits in: near-black page, the
 * KWIK wordmark with the red tagline, and the language switcher pinned below the
 * form. This page is intentionally dark regardless of the app theme.
 *
 * The wordmark is the shared `KwikWordmark` component so every branded surface
 * uses the same asset.
 */
export function AuthShell({ children }: AuthShellProps) {
  const { t } = useTranslation()

  return (
    <div className="dark flex min-h-screen w-full justify-center bg-[#0a0a0b] px-6 py-10 text-white sm:px-8">
      <div className="flex w-full max-w-[440px] flex-col">
        <header>
          <KwikWordmark className="h-11 w-auto" />
          <p className="mt-4 text-[11px] font-bold uppercase leading-none tracking-[0.2em] text-[#f2473f]">
            {t('auth.tagline')}
          </p>
        </header>

        {/* No flex-1: short screens (e.g. forgot password) keep the language
            switcher right under the form instead of pinned to the viewport
            bottom, which left a large empty gap on phones. */}
        <main className="mt-14">{children}</main>

        <div className="mt-8 flex justify-center">
          <LanguageSelector />
        </div>
      </div>
    </div>
  )
}
