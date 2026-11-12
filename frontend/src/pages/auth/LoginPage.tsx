import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import type { TFunction } from 'i18next'
import { ArrowRight, Eye, EyeOff, Loader2, Lock, Mail, Send } from 'lucide-react'
import { AuthShell } from '@/features/auth/AuthShell'
import { GoogleSignInButton } from '@/features/auth/GoogleSignInButton'
import { googleErrorKey } from '@/features/auth/google'
import { clearPendingRedirect, readPendingRedirect, resolvePostAuthPath } from '@/features/auth/landing'
import { authApi } from '@/features/auth/api'
import {
  authAccentLinkClass,
  authAlertClass,
  authInputClass,
  authLinkClass,
  authPrimaryButtonClass,
} from '@/features/auth/authStyles'
import { useAuth } from '@/contexts/AuthContext'
import { usePageTitle } from '@/hooks/usePageTitle'
import { isTelegramWebApp } from '@/lib/telegram'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'

const makeSchema = (t: TFunction) => z.object({
  identifier: z.string().min(1, t('validation.emailOrPhone')),
  password: z.string().min(1, t('validation.enterPassword')),
})
type Fields = z.infer<ReturnType<typeof makeSchema>>

export function Component() {
  const { t } = useTranslation()
  // `t` is rebound by react-i18next when the language changes, so validation
  // messages follow the active locale instead of freezing at first render.
  const schema = useMemo(() => makeSchema(t), [t])
  usePageTitle(t('auth.signIn'))
  const { login, loginWithTelegram, telegramError } = useAuth()
  const navigate = useNavigate()
  const inTelegram = isTelegramWebApp()
  const [telegramPending, setTelegramPending] = useState(false)
  const [searchParams] = useSearchParams()
  const redirectParam = searchParams.get('redirect')
  const pendingRedirect = readPendingRedirect(redirectParam)

  const { register, handleSubmit, formState: { errors } } = useForm<Fields>({
    resolver: zodResolver(schema),
  })

  const [showPassword, setShowPassword] = useState(false)

  // Only offer Google when this server actually has credentials configured:
  // an unconfigured server answers {"google":false} and the button would only
  // lead to a 503 page.
  const { data: providers, isError: providersUnavailable } = useQuery({
    queryKey: ['auth', 'providers'],
    queryFn: () => authApi.providers(),
    staleTime: 5 * 60 * 1000,
    retry: 1,
  })
  const googleEnabled = providers?.google === true

  // Hiding the button is correct when the server says Google is off, but a
  // failed lookup is a different problem (stale backend, proxy, CORS) and used
  // to disappear without a trace. Surface it during development.
  useEffect(() => {
    if (import.meta.env.DEV && providersUnavailable) {
      console.warn('[auth] /auth/providers is unreachable, so Google sign-in stays hidden')
    }
  }, [providersUnavailable])

  // A failed Google round trip comes back as ?error=<reason>.
  const googleError = searchParams.get('error')
  // The API redirects here with ?reason=session_expired after a failed refresh.
  const sessionExpired = searchParams.get('reason') === 'session_expired'

  const { mutate, isPending, error } = useMutation({
    mutationFn: ({ identifier, password }: Fields) => authApi.login(identifier, password),
    onSuccess: ({ token, user, refresh_token }) => {
      login(token, user, refresh_token)
      clearPendingRedirect()
      // Players land on the games lobby, admins on their dashboard; an explicit
      // internal ?redirect= still wins so deep links survive the sign-in.
      navigate(resolvePostAuthPath({ redirect: pendingRedirect, role: user.role }), { replace: true })
    },
  })

  const handleTelegramSignIn = async () => {
    setTelegramPending(true)
    try {
      // On success AuthLayout redirects to the role's landing page.
      await loginWithTelegram()
    } finally {
      setTelegramPending(false)
    }
  }

  return (
    <AuthShell>
      {inTelegram && (
        <div className="mb-6 space-y-4">
          {telegramError && (
            <p role="alert" className={authAlertClass}>
              {telegramError}
            </p>
          )}
          <Button
            type="button"
            onClick={() => void handleTelegramSignIn()}
            disabled={telegramPending}
            aria-busy={telegramPending}
            className={authPrimaryButtonClass}
          >
            {telegramPending ? (
              <Loader2 className="h-[18px] w-[18px] animate-spin" />
            ) : (
              <Send className="h-[18px] w-[18px]" strokeWidth={2} />
            )}
            {telegramPending ? t('auth.telegramSigningIn') : t('auth.telegramContinue')}
          </Button>
          <p className="text-center text-xs text-muted-foreground">
            {t('auth.telegramContinueNote')}
          </p>
          <div className="flex items-center gap-4">
            <span aria-hidden="true" className="h-px flex-1 bg-border" />
            <span className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">
              {t('auth.orSignInWithPassword')}
            </span>
            <span aria-hidden="true" className="h-px flex-1 bg-border" />
          </div>
        </div>
      )}

      <form onSubmit={handleSubmit(d => mutate(d))} noValidate className="space-y-5">
        {sessionExpired && (
          <p role="alert" className={authAlertClass}>
            {t('auth.sessionExpired')}
          </p>
        )}

        {googleError && (
          <p role="alert" className={authAlertClass}>
            {t(googleErrorKey(googleError))}
          </p>
        )}

        {error && (
          <p role="alert" className={authAlertClass}>
            {error.message}
          </p>
        )}

        <div className="space-y-2">
          <label htmlFor="identifier" className="block text-[13px] font-semibold text-foreground">
            {t('auth.emailAddress')}
          </label>
          <div className="relative">
            <Mail
              aria-hidden="true"
              strokeWidth={1.75}
              className="pointer-events-none absolute left-4 top-1/2 h-[18px] w-[18px] -translate-y-1/2 text-muted-foreground"
            />
            <Input
              id="identifier"
              type="text"
              autoComplete="username"
              placeholder={t('auth.loginEmailPlaceholder')}
              aria-invalid={errors.identifier ? true : undefined}
              className={`${authInputClass} pl-12`}
              {...register('identifier')}
            />
          </div>
          {errors.identifier?.message && (
            <p className="text-xs text-destructive">{errors.identifier.message}</p>
          )}
        </div>

        <div className="space-y-2">
          <label htmlFor="password" className="block text-[13px] font-semibold text-foreground">
            {t('auth.password')}
          </label>
          <div className="relative">
            <Lock
              aria-hidden="true"
              strokeWidth={1.75}
              className="pointer-events-none absolute left-4 top-1/2 h-[18px] w-[18px] -translate-y-1/2 text-muted-foreground"
            />
            <Input
              id="password"
              type={showPassword ? 'text' : 'password'}
              autoComplete="current-password"
              aria-invalid={errors.password ? true : undefined}
              className={`${authInputClass} pl-12 pr-12`}
              {...register('password')}
            />
            <button
              type="button"
              onClick={() => setShowPassword(prev => !prev)}
              aria-label={showPassword ? t('auth.hidePassword') : t('auth.showPassword')}
              className="absolute inset-y-0 right-0 flex w-12 items-center justify-center rounded-r-xl text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring/40"
            >
              {showPassword ? (
                <Eye className="h-[18px] w-[18px]" strokeWidth={1.75} />
              ) : (
                <EyeOff className="h-[18px] w-[18px]" strokeWidth={1.75} />
              )}
            </button>
          </div>
          {errors.password?.message && (
            <p className="text-xs text-destructive">{errors.password.message}</p>
          )}
        </div>

        <div className="flex justify-end">
          <Link to="/forgot-password" className={`text-[13px] ${authAccentLinkClass}`}>
            {t('auth.forgotPassword')}
          </Link>
        </div>

        <Button
          type="submit"
          disabled={isPending}
          aria-busy={isPending}
          className={authPrimaryButtonClass}
        >
          {isPending ? (
            <Loader2 className="h-[18px] w-[18px] animate-spin" />
          ) : (
            <ArrowRight className="h-[18px] w-[18px]" strokeWidth={2.5} />
          )}
          {isPending ? t('auth.signingIn') : t('auth.enterArena')}
        </Button>
      </form>

      <div className="mt-8">
        {googleEnabled && (
          <>
            <div className="flex items-center gap-4">
              <span aria-hidden="true" className="h-px flex-1 bg-border" />
              <span className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">
                {t('auth.orContinueWith')}
              </span>
              <span aria-hidden="true" className="h-px flex-1 bg-border" />
            </div>
            <div className="mt-6">
              <GoogleSignInButton redirect={pendingRedirect} />
            </div>
          </>
        )}

        <p className="mt-8 text-center text-[14px] text-muted-foreground">
          {t('auth.newCompetitor')}{' '}
          <Link
            to={redirectParam ? `/register?redirect=${encodeURIComponent(redirectParam)}` : '/register'}
            className={authLinkClass}
          >
            {t('auth.createAccount')}
          </Link>
        </p>
      </div>
    </AuthShell>
  )
}
