import { useEffect } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { ErrorBoundary } from '@/components/errors/ErrorBoundary'
import { KwikSplash } from '@/components/brand/KwikSplash'
import { useAuth } from '@/contexts/AuthContext'
import { useTheme } from '@/contexts/ThemeContext'
import {
  getTelegramWebApp,
  initTelegramWebApp,
  isTelegramWebApp,
  subscribeTelegramViewport,
  syncTelegramChrome,
} from '@/lib/telegram'

/** Wraps every route — error boundary + global Telegram shell integration. */
export function RootLayout() {
  const { isLoading } = useAuth()
  const { resolvedTheme } = useTheme()
  const location = useLocation()
  const navigate = useNavigate()

  useEffect(() => {
    initTelegramWebApp()
  }, [])

  // KWIK keeps its own dark/red identity inside Telegram; only the native
  // chrome (header, background, bottom bar) follows the resolved theme.
  useEffect(() => {
    syncTelegramChrome(resolvedTheme === 'dark')
  }, [resolvedTheme])

  // Follow Telegram viewport, safe-area and theme changes instead of measuring
  // the window once at startup.
  useEffect(() => subscribeTelegramViewport(), [])

  // Native BackButton: visible only when there is in-app history to pop, and
  // wired to the router rather than letting Telegram close the Mini App.
  useEffect(() => {
    const back = getTelegramWebApp()?.BackButton
    if (!back || !isTelegramWebApp()) return

    const goBack = () => navigate(-1)
    const historyIndex = (window.history.state as { idx?: number } | null)?.idx ?? 0
    if (historyIndex > 0) {
      back.onClick(goBack)
      back.show()
    } else {
      back.hide()
    }

    return () => {
      back.offClick(goBack)
      back.hide()
    }
  }, [location.pathname, navigate])

  return (
    <ErrorBoundary>
      {isLoading ? <KwikSplash /> : <Outlet />}
    </ErrorBoundary>
  )
}
