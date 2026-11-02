import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Loader2 } from 'lucide-react'
import { AuthCard } from '@/features/auth/AuthCard'
import { authApi } from '@/features/auth/api'
import { parseGoogleCallbackHash } from '@/features/auth/google'
import { resolvePostAuthPath } from '@/features/auth/landing'
import { useAuth } from '@/contexts/AuthContext'
import { usePageTitle } from '@/hooks/usePageTitle'
import { Button } from '@/components/ui/button'

const EMPTY_RESULT = { code: null, redirect: null, error: null } as const

/**
 * Landing point after the backend completes a Google sign-in.
 *
 * The redirect carries a one-time code in the URL fragment. It is read once,
 * stripped from the address bar immediately, and exchanged for the session via
 * a POST request, so the access token never touches a URL.
 */
export function Component() {
  const { t } = useTranslation()
  usePageTitle(t('auth.signIn'))
  const { login } = useAuth()
  const navigate = useNavigate()

  // Read the fragment once during the first render: it is single use, so it
  // must not be re-parsed on later renders (or twice by StrictMode).
  const [callback] = useState(() =>
    typeof window === 'undefined' ? EMPTY_RESULT : parseGoogleCallbackHash(window.location.hash),
  )
  const [error, setError] = useState<string | null>(
    callback.code ? null : t('auth.googleMissingCode'),
  )
  const started = useRef(false)

  useEffect(() => {
    if (started.current) return
    started.current = true

    window.history.replaceState(null, '', window.location.pathname)
    if (!callback.code) return

    authApi
      .exchangeGoogleCode(callback.code)
      .then(({ token, user, refresh_token, redirect: serverRedirect }) => {
        login(token, user, refresh_token)
        // Google sign-in follows the same rule as the password form: an
        // explicit internal redirect if one was requested, otherwise the role's
        // landing page.
        navigate(resolvePostAuthPath({ redirect: callback.redirect || serverRedirect, role: user.role }), {
          replace: true,
        })
      })
      .catch((err: Error) => {
        setError(err.message || t('auth.googleSignInFailed'))
      })
    // The code is redeemed exactly once for this mount.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return (
    <div className="flex min-h-screen items-center justify-center bg-muted/40">
      <AuthCard title={t('auth.signIn')}>
        {error ? (
          <div className="space-y-4">
            <p role="alert" className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {error}
            </p>
            <Button className="w-full" onClick={() => navigate('/login', { replace: true })}>
              {t('auth.backToSignIn')}
            </Button>
          </div>
        ) : (
          <div className="flex flex-col items-center gap-3 py-4 text-sm text-muted-foreground">
            <Loader2 className="h-5 w-5 animate-spin text-primary" aria-hidden="true" />
            <p>{t('auth.googleCompleting')}</p>
            <Link to="/login" className="text-xs underline underline-offset-4">
              {t('auth.backToSignIn')}
            </Link>
          </div>
        )}
      </AuthCard>
    </div>
  )
}
