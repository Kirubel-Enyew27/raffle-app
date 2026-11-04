import { useEffect } from 'react'
import { Outlet } from 'react-router-dom'
import { ErrorBoundary } from '@/components/errors/ErrorBoundary'
import { KwikSplash } from '@/components/brand/KwikSplash'
import { useAuth } from '@/contexts/AuthContext'
import { initTelegramWebApp } from '@/lib/telegram'

/** Wraps every route — error boundary + any global chrome (toasts, etc.) */
export function RootLayout() {
  const { isLoading } = useAuth()

  useEffect(() => {
    initTelegramWebApp()
  }, [])

  return (
    <ErrorBoundary>
      {isLoading ? <KwikSplash /> : <Outlet />}
    </ErrorBoundary>
  )
}
