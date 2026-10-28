import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { useMutation } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { AuthCard } from '@/features/auth/AuthCard'
import { authApi } from '@/features/auth/api'
import { useAuth } from '@/contexts/AuthContext'
import { usePageTitle } from '@/hooks/usePageTitle'
import { FormField } from '@/components/forms/FormField'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Smartphone } from 'lucide-react'

const schema = z.object({
  fullName: z.string().min(2, 'Enter your full name'),
  email: z.string().email('Enter a valid email').optional().or(z.literal('')),
  phone: z.string().regex(/^\+?[0-9]{7,15}$/, 'Enter a valid phone number').optional().or(z.literal('')),
  password: z.string().min(6, 'At least 6 characters'),
  confirm: z.string(),
}).refine(d => d.password === d.confirm, {
  message: 'Passwords do not match',
  path: ['confirm'],
}).refine(d => d.email || d.phone, {
  message: 'Enter your email or phone number',
  path: ['email'],
})
type Fields = z.infer<typeof schema>

export function Component() {
  const { t } = useTranslation()
  usePageTitle(t('auth.signUp'))
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

  const { mutate, isPending, error } = useMutation({
    mutationFn: ({ fullName, email, phone, password }: Fields) => authApi.register(email || '', password, fullName, phone),
    onSuccess: async (_user, { email, phone, password }) => {
      // Auto-login after registration (use email or phone)
      const identifier = (email || phone)!
      const { token, user } = await authApi.login(identifier, password)
      login(token, user)
      if (typeof sessionStorage !== 'undefined') {
        sessionStorage.removeItem('pending_redirect')
      }
      if (pendingRedirect && pendingRedirect.startsWith('/') && !pendingRedirect.startsWith('//')) {
        navigate(pendingRedirect)
      } else {
        navigate('/dashboard')
      }
    },
  })

  return (
    <AuthCard title={t('auth.signUp')} subtitle={t('auth.startEntering')}>
      <form onSubmit={handleSubmit(d => mutate(d))} className="space-y-4">
        {error && (
          <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error.message}
          </p>
        )}

        <FormField label={t('auth.fullName')} htmlFor="fullName" error={errors.fullName?.message}>
          <Input id="fullName" type="text" autoComplete="name" {...register('fullName')} />
          <p className="mt-2 flex items-center gap-1.5 rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-xs text-blue-700 dark:border-blue-800 dark:bg-blue-950 dark:text-blue-400">
            <Smartphone className="h-3.5 w-3.5 shrink-0" />
            {t('auth.telebirrNotice')}
          </p>
        </FormField>

        <FormField label={t('auth.phone')} htmlFor="phone" error={errors.phone?.message}>
          <Input id="phone" type="tel" autoComplete="tel" placeholder={t('auth.phonePlaceholder')} {...register('phone')} />
        </FormField>

        <FormField label={t('auth.email')} htmlFor="email" error={errors.email?.message}>
          <Input id="email" type="email" autoComplete="email" placeholder={t('auth.optional')} {...register('email')} />
        </FormField>

        <FormField label={t('auth.password')} htmlFor="password" error={errors.password?.message}>
          <Input id="password" type="password" autoComplete="new-password" {...register('password')} />
        </FormField>

        <FormField label={t('auth.confirmPassword')} htmlFor="confirm" error={errors.confirm?.message}>
          <Input id="confirm" type="password" autoComplete="new-password" {...register('confirm')} />
        </FormField>

        <Button type="submit" className="w-full" disabled={isPending}>
          {isPending ? t('auth.creatingAccount') : t('auth.signUp')}
        </Button>

        <p className="text-center text-sm text-muted-foreground">
          {t('auth.alreadyHaveAccount')}{' '}
          <Link
            to={redirectParam ? `/login?redirect=${encodeURIComponent(redirectParam)}` : '/login'}
            className="text-foreground underline underline-offset-4"
          >
            {t('auth.signIn')}
          </Link>
        </p>
      </form>
    </AuthCard>
  )
}
