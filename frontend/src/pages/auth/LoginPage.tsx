import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { useMutation } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Eye, EyeOff } from 'lucide-react'
import { AuthCard } from '@/features/auth/AuthCard'
import { authApi } from '@/features/auth/api'
import { useAuth } from '@/contexts/AuthContext'
import { usePageTitle } from '@/hooks/usePageTitle'
import { FormField } from '@/components/forms/FormField'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'

const schema = z.object({
  identifier: z.string().min(1, 'Enter your email or phone'),
  password: z.string().min(1, 'Password is required'),
})
type Fields = z.infer<typeof schema>

export function Component() {
  const { t } = useTranslation()
  usePageTitle(t('auth.signIn'))
  const { login } = useAuth()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const redirectParam = searchParams.get('redirect')
  const pendingRedirect =
    redirectParam ||
    (typeof sessionStorage !== 'undefined' ? sessionStorage.getItem('pending_redirect') : null)

  const { register, handleSubmit, formState: { errors } } = useForm<Fields>({
    resolver: zodResolver(schema),
  })

  const [showPassword, setShowPassword] = useState(false)

  const { mutate, isPending, error } = useMutation({
    mutationFn: ({ identifier, password }: Fields) => authApi.login(identifier, password),
    onSuccess: ({ token, user, refresh_token }) => {
      login(token, user, refresh_token)
      if (typeof sessionStorage !== 'undefined') {
        sessionStorage.removeItem('pending_redirect')
      }
      if (pendingRedirect && pendingRedirect.startsWith('/') && !pendingRedirect.startsWith('//')) {
        navigate(pendingRedirect)
      } else {
        navigate(user.role === 'admin' ? '/admin/dashboard' : '/dashboard')
      }
    },
  })

  return (
    <AuthCard title={t('auth.signIn')} subtitle={t('auth.welcomeBack')}>
      <form onSubmit={handleSubmit(d => mutate(d))} className="space-y-4">
        {error && (
          <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error.message}
          </p>
        )}

        <FormField label={t('auth.emailOrPhone')} htmlFor="identifier" error={errors.identifier?.message}>
          <Input id="identifier" type="text" autoComplete="username" placeholder={t('auth.emailPlaceholder')} {...register('identifier')} />
        </FormField>

        <FormField label={t('auth.password')} htmlFor="password" error={errors.password?.message}>
          <div className="relative">
            <Input
              id="password"
              type={showPassword ? 'text' : 'password'}
              autoComplete="current-password"
              className="pr-10"
              {...register('password')}
            />
            <button
              type="button"
              onClick={() => setShowPassword(prev => !prev)}
              aria-label={showPassword ? 'Hide password' : 'Show password'}
              className="absolute inset-y-0 right-0 flex items-center justify-center w-10 text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring rounded-r-md"
            >
              {showPassword ? (
                <EyeOff className="h-4 w-4" />
              ) : (
                <Eye className="h-4 w-4" />
              )}
            </button>
          </div>
        </FormField>

        <div className="flex justify-end">
          <Link to="/forgot-password" className="text-xs text-muted-foreground hover:text-foreground">
            {t('auth.forgotPassword')}
          </Link>
        </div>

        <Button type="submit" className="w-full" disabled={isPending}>
          {isPending ? t('auth.signingIn') : t('auth.signIn')}
        </Button>

        <p className="text-center text-sm text-muted-foreground">
          {t('auth.noAccount')}{' '}
          <Link
            to={redirectParam ? `/register?redirect=${encodeURIComponent(redirectParam)}` : '/register'}
            className="text-foreground underline underline-offset-4"
          >
            {t('auth.register')}
          </Link>
        </p>
      </form>
    </AuthCard>
  )
}
