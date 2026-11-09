import { useMemo } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { useMutation } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import type { TFunction } from 'i18next'
import { AuthCard } from '@/features/auth/AuthCard'
import { clearPendingRedirect, readPendingRedirect, resolvePostAuthPath } from '@/features/auth/landing'
import { authApi } from '@/features/auth/api'
import {
  authAlertClass,
  authInputClass,
  authLinkClass,
  authPrimaryButtonClass,
} from '@/features/auth/authStyles'
import { useAuth } from '@/contexts/AuthContext'
import { usePageTitle } from '@/hooks/usePageTitle'
import { FormField } from '@/components/forms/FormField'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'

const makeSchema = (t: TFunction) => z.object({
  fullName: z.string().min(2, t('validation.enterFullName')),
  email: z.string().email(t('validation.validEmail')).optional().or(z.literal('')),
  phone: z.string().regex(/^\+?[0-9]{7,15}$/, t('validation.validPhone')).optional().or(z.literal('')),
  password: z.string().min(6, t('validation.atLeastCharacters', { count: 6 })),
  confirm: z.string(),
}).refine(d => d.password === d.confirm, {
  message: t('validation.passwordsDoNotMatch'),
  path: ['confirm'],
}).refine(d => d.email || d.phone, {
  message: t('validation.emailOrPhoneRequired'),
  path: ['email'],
})
type Fields = z.infer<ReturnType<typeof makeSchema>>

export function Component() {
  const { t } = useTranslation()
  // Validation messages must follow the active language, not the first render.
  const schema = useMemo(() => makeSchema(t), [t])
  usePageTitle(t('auth.signUp'))
  const { login } = useAuth()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const redirectParam = searchParams.get('redirect')
  const pendingRedirect = readPendingRedirect(redirectParam)

  const { register, handleSubmit, formState: { errors } } = useForm<Fields>({
    resolver: zodResolver(schema),
  })

  const { mutate, isPending, error } = useMutation({
    mutationFn: ({ fullName, email, phone, password }: Fields) => authApi.register(email || '', password, fullName, phone),
    onSuccess: async (_user, { email, phone, password }) => {
      // Auto-login after registration (use email or phone)
      const identifier = (email || phone)!
      const { token, user, refresh_token } = await authApi.login(identifier, password)
      login(token, user, refresh_token)
      clearPendingRedirect()
      navigate(resolvePostAuthPath({ redirect: pendingRedirect, role: user.role }), { replace: true })
    },
  })

  return (
    <AuthCard>
      <form onSubmit={handleSubmit(d => mutate(d))} className="space-y-5">
        {error && (
          <p role="alert" className={authAlertClass}>
            {error.message}
          </p>
        )}

        <FormField label={t('auth.fullName')} htmlFor="fullName" error={errors.fullName?.message}>
          <Input id="fullName" type="text" autoComplete="name" className={authInputClass} {...register('fullName')} />
        </FormField>

        <FormField label={t('auth.phone')} htmlFor="phone" error={errors.phone?.message}>
          <Input id="phone" type="tel" autoComplete="tel" placeholder={t('auth.phonePlaceholder')} className={authInputClass} {...register('phone')} />
        </FormField>

        <FormField label={t('auth.email')} htmlFor="email" error={errors.email?.message}>
          <Input id="email" type="email" autoComplete="email" placeholder={t('auth.optional')} className={authInputClass} {...register('email')} />
        </FormField>

        <FormField label={t('auth.password')} htmlFor="password" error={errors.password?.message}>
          <Input id="password" type="password" autoComplete="new-password" className={authInputClass} {...register('password')} />
        </FormField>

        <FormField label={t('auth.confirmPassword')} htmlFor="confirm" error={errors.confirm?.message}>
          <Input id="confirm" type="password" autoComplete="new-password" className={authInputClass} {...register('confirm')} />
        </FormField>

        <Button type="submit" className={authPrimaryButtonClass} disabled={isPending}>
          {isPending ? t('auth.creatingAccount') : t('auth.signUp')}
        </Button>

        <p className="text-center text-[14px] text-muted-foreground">
          {t('auth.alreadyHaveAccount')}{' '}
          <Link
            to={redirectParam ? `/login?redirect=${encodeURIComponent(redirectParam)}` : '/login'}
            className={authLinkClass}
          >
            {t('auth.signIn')}
          </Link>
        </p>
      </form>
    </AuthCard>
  )
}
