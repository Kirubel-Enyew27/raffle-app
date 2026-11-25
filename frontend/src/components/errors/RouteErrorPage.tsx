import { AlertTriangle, LayoutDashboard, RefreshCw } from 'lucide-react'
import { isRouteErrorResponse, useLocation, useNavigate, useRouteError } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { isChunkLoadError } from '@/lib/chunkRecovery'

/**
 * Rendered by each lazy route's `errorElement`. Because the element belongs to
 * the child route, React Router keeps the surrounding app/admin/auth layout in
 * place and only replaces the page that failed to load.
 */
export function RouteErrorPage() {
  const error = useRouteError()
  const navigate = useNavigate()
  const { pathname } = useLocation()
  const { t } = useTranslation()

  const chunkFailure = isChunkLoadError(error)
  const isAdmin = pathname.startsWith('/admin')
  const dashboardPath = isAdmin ? '/admin/dashboard' : '/dashboard'
  const status = isRouteErrorResponse(error)
    ? [error.status, error.statusText].filter(Boolean).join(' ')
    : ''

  const refresh = () => {
    if (typeof window !== 'undefined') window.location.reload()
  }

  return (
    <div
      role="alert"
      className="flex min-h-[60vh] w-full items-center justify-center bg-muted/25 p-4 sm:p-8"
    >
      <Card className="w-full max-w-lg border-border/80 shadow-sm">
        <CardContent className="flex flex-col items-center px-6 py-10 text-center sm:px-10">
          <span className="mb-5 flex h-14 w-14 items-center justify-center rounded-2xl bg-primary/10 text-primary">
            <AlertTriangle className="h-7 w-7" aria-hidden="true" />
          </span>
          <h1 className="text-balance text-xl font-black tracking-tight sm:text-2xl">
            {chunkFailure
              ? t('errors.pageLoadTitle', "This page couldn't be loaded")
              : t('errors.somethingWentWrong', 'Something went wrong')}
          </h1>
          <p className="mt-2 max-w-md text-sm leading-relaxed text-muted-foreground">
            {chunkFailure
              ? t(
                  'errors.pageLoadDescription',
                  "A part of KWIK didn't download. This can happen after an update or a brief network interruption.",
                )
              : t(
                  'errors.unexpectedErrorDesc',
                  'Please try again. If the problem continues, return to your dashboard.',
                )}
          </p>
          {status && (
            <p className="mt-3 rounded-md bg-muted px-2.5 py-1 text-xs font-semibold text-muted-foreground">
              {status}
            </p>
          )}
          <div className="mt-7 flex w-full flex-col gap-2 sm:flex-row sm:justify-center">
            <Button onClick={refresh} className="gap-2">
              <RefreshCw className="h-4 w-4" aria-hidden="true" />
              {chunkFailure
                ? t('errors.refreshApplication', 'Refresh application')
                : t('errors.tryAgain', 'Try again')}
            </Button>
            <Button
              variant="outline"
              className="gap-2"
              onClick={() => navigate(dashboardPath, { replace: true })}
            >
              <LayoutDashboard className="h-4 w-4" aria-hidden="true" />
              {t('errors.backToDashboard', 'Back to dashboard')}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
