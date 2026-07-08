import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Link, useNavigate } from 'react-router-dom'
import { useMutation } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { AuthCard } from '@/features/auth/AuthCard'
import { authApi } from '@/features/auth/api'
import { usePageTitle } from '@/hooks/usePageTitle'
import { FormField } from '@/components/forms/FormField'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'

const schema = z.object({
  old_password: z.string().min(1, 'Current password is required'),
  new_password: z.string().min(6, 'At least 6 characters'),
  confirm: z.string(),
}).refine(d => d.new_password === d.confirm, {
  message: 'Passwords do not match',
  path: ['confirm'],
})
type Fields = z.infer<typeof schema>

export function Component() {
  const { t } = useTranslation()
  usePageTitle(t('auth.resetPasswordTitle'))
  const navigate = useNavigate()

  const { register, handleSubmit, formState: { errors } } = useForm<Fields>({
    resolver: zodResolver(schema),
  })

  const { mutate, isPending, error, isSuccess } = useMutation({
    mutationFn: ({ old_password, new_password }: Fields) =>
      authApi.changePassword(old_password, new_password),
  })

  if (isSuccess) {
    return (
      <AuthCard title={t('auth.passwordUpdated')} subtitle={t('auth.passwordUpdatedMessage')}>
        <Button className="w-full" onClick={() => navigate('/login')}>
          {t('auth.backToSignIn')}
        </Button>
      </AuthCard>
    )
  }

  return (
    <AuthCard title={t('auth.resetPasswordTitle')} subtitle={t('auth.resetPasswordSubtitle')}>
      <form onSubmit={handleSubmit(d => mutate(d))} className="space-y-4">
        {error && (
          <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error.message}
          </p>
        )}

        <FormField label={t('auth.currentPassword')} htmlFor="old_password" error={errors.old_password?.message}>
          <Input id="old_password" type="password" autoComplete="current-password" {...register('old_password')} />
        </FormField>

        <FormField label={t('auth.newPassword')} htmlFor="new_password" error={errors.new_password?.message}>
          <Input id="new_password" type="password" autoComplete="new-password" {...register('new_password')} />
        </FormField>

        <FormField label={t('auth.confirmNewPassword')} htmlFor="confirm" error={errors.confirm?.message}>
          <Input id="confirm" type="password" autoComplete="new-password" {...register('confirm')} />
        </FormField>

        <Button type="submit" className="w-full" disabled={isPending}>
          {isPending ? t('auth.updating') : t('auth.updatePassword')}
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
