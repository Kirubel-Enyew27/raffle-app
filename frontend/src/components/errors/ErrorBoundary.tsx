import { Component, type ErrorInfo, type ReactNode } from 'react'
import { AlertTriangle, LayoutDashboard, RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import i18n from '@/lib/i18n'

interface Props {
  children: ReactNode
  fallback?: ReactNode
}

interface State {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false, error: null }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('[ErrorBoundary]', error, info)
  }

  private reset = () => this.setState({ hasError: false, error: null })

  private goToDashboard = () => {
    const admin = typeof window !== 'undefined' && window.location.pathname.startsWith('/admin')
    window.location.assign(admin ? '/admin/dashboard' : '/dashboard')
  }

  render() {
    if (this.state.hasError) {
      return (
        this.props.fallback ?? (
          <div className="flex min-h-screen items-center justify-center bg-muted/25 p-4 sm:p-8">
            <Card className="w-full max-w-lg border-border/80 shadow-sm">
              <CardContent className="flex flex-col items-center px-6 py-10 text-center sm:px-10">
                <span className="mb-5 flex h-14 w-14 items-center justify-center rounded-2xl bg-primary/10 text-primary">
                  <AlertTriangle className="h-7 w-7" aria-hidden="true" />
                </span>
                <h2 className="text-balance text-xl font-black tracking-tight sm:text-2xl">
                  {i18n.t('errors.somethingWentWrong', 'Something went wrong')}
                </h2>
                <p className="mt-2 max-w-md text-sm leading-relaxed text-muted-foreground">
                  {i18n.t(
                    'errors.unexpectedErrorDesc',
                    'Please try again. If the problem continues, return to your dashboard.',
                  )}
                </p>
                <div className="mt-7 flex w-full flex-col gap-2 sm:flex-row sm:justify-center">
                  <Button onClick={this.reset} className="gap-2">
                    <RefreshCw className="h-4 w-4" aria-hidden="true" />
                    {i18n.t('errors.tryAgain', 'Try again')}
                  </Button>
                  <Button variant="outline" className="gap-2" onClick={this.goToDashboard}>
                    <LayoutDashboard className="h-4 w-4" aria-hidden="true" />
                    {i18n.t('errors.backToDashboard', 'Back to dashboard')}
                  </Button>
                </div>
              </CardContent>
            </Card>
          </div>
        )
      )
    }
    return this.props.children
  }
}
