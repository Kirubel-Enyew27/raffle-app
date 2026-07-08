import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { AuthCard } from '@/features/auth/AuthCard'
import { usePageTitle } from '@/hooks/usePageTitle'
import { FormField } from '@/components/forms/FormField'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'

const schema = z.object({
  email: z.string().email('Enter a valid email'),
})
type Fields = z.infer<typeof schema>

export function Component() {
  const { t } = useTranslation()
  usePageTitle(t('auth.forgotPasswordTitle'))
  const [submitted, setSubmitted] = useState(false)

  const { register, handleSubmit, formState: { errors } } = useForm<Fields>({
    resolver: zodResolver(schema),
  })

  // The backend has no unauthenticated reset endpoint; we simulate the
  // standard UX (show confirmation) and direct the user to reset-password
  // where they can provide their current password.
  const onSubmit = (_: Fields) => setSubmitted(true)

  if (submitted) {
    return (
      <AuthCard title={t('auth.checkEmail')} subtitle={t('auth.checkEmailMessage')}>
        <div className="space-y-4 text-center">
          <p className="text-sm text-muted-foreground">
            {t('auth.didntReceiveEmail')}{' '}
            <button
              onClick={() => setSubmitted(false)}
              className="text-foreground underline underline-offset-4"
            >
              {t('auth.tryAgain')}
            </button>
          </p>
          <p className="text-sm text-muted-foreground">
            {t('auth.knowPassword')}{' '}
            <Link to="/login" className="text-foreground underline underline-offset-4">
              {t('auth.signIn')}
            </Link>
          </p>
          <div className="mt-4 rounded-md bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:bg-amber-950 dark:text-amber-300">
            {t('auth.resetNotConfigured')}
          </div>
        </div>
      </AuthCard>
    )
  }

  return (
    <AuthCard title={t('auth.forgotPasswordTitle')} subtitle={t('auth.forgotPasswordSubtitle')}>
      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        <FormField label={t('auth.email')} htmlFor="email" error={errors.email?.message}>
          <Input id="email" type="email" autoComplete="email" {...register('email')} />
        </FormField>

        <Button type="submit" className="w-full">
          {t('auth.sendInstructions')}
        </Button>

        <p className="text-center text-sm text-muted-foreground">
          <Link to="/login" className="text-foreground underline underline-offset-4">
            {t('auth.backToSignIn')}
          </Link>
        </p>
      </form>
    </AuthCard>
  )
}
