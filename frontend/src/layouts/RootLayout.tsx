import { useEffect } from 'react'
import { Outlet } from 'react-router-dom'
import { ErrorBoundary } from '@/components/errors/ErrorBoundary'
import { initTelegramWebApp } from '@/lib/telegram'

/** Wraps every route — error boundary + any global chrome (toasts, etc.) */
export function RootLayout() {
  useEffect(() => {
    initTelegramWebApp()
  }, [])

  return (
    <ErrorBoundary>
      <Outlet />
    </ErrorBoundary>
  )
}
