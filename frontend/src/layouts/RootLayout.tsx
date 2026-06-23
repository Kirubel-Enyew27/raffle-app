import { Outlet } from 'react-router-dom'
import { ErrorBoundary } from '@/components/errors/ErrorBoundary'

/** Wraps every route — error boundary + any global chrome (toasts, etc.) */
export function RootLayout() {
  return (
    <ErrorBoundary>
      <Outlet />
    </ErrorBoundary>
  )
}
