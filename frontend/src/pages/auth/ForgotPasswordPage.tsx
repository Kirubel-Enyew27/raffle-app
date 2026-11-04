import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { AuthCard } from '@/features/auth/AuthCard'
import { authInputClass, authLinkClass, authPrimaryButtonClass } from '@/features/auth/authStyles'
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
  const onSubmit = () => setSubmitted(true)

  if (submitted) {
    return (
      <AuthCard title={t('auth.checkEmail')} subtitle={t('auth.checkEmailMessage')}>
        <div className="space-y-4 text-center">
          <p className="text-[14px] text-muted-foreground">
            {t('auth.didntReceiveEmail')}{' '}
            <button onClick={() => setSubmitted(false)} className={authLinkClass}>
              {t('auth.tryAgain')}
            </button>
          </p>
          <p className="text-[14px] text-muted-foreground">
            {t('auth.knowPassword')}{' '}
            <Link to="/login" className={authLinkClass}>
              {t('auth.signIn')}
            </Link>
          </p>
          <div className="mt-4 rounded-lg border border-warning/30 bg-warning/10 px-4 py-3 text-xs text-warning">
            {t('auth.resetNotConfigured')}
          </div>
        </div>
      </AuthCard>
    )
  }

  return (
    <AuthCard title={t('auth.forgotPasswordTitle')} subtitle={t('auth.forgotPasswordSubtitle')}>
      <form onSubmit={handleSubmit(onSubmit)} className="space-y-5">
        <FormField label={t('auth.email')} htmlFor="email" error={errors.email?.message}>
          <Input id="email" type="email" autoComplete="email" className={authInputClass} {...register('email')} />
        </FormField>

        <Button type="submit" className={authPrimaryButtonClass}>
          {t('auth.sendInstructions')}
        </Button>

        <p className="text-center text-[14px] text-muted-foreground">
          <Link to="/login" className={authLinkClass}>
            {t('auth.backToSignIn')}
          </Link>
        </p>
      </form>
    </AuthCard>
  )
}
