import { useState, useRef } from 'react'
import { useForm } from 'react-hook-form'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { usePageTitle } from '@/hooks/usePageTitle'
import { useAuth } from '@/contexts/AuthContext'
import { api } from '@/lib/api'
import { authStorage } from '@/lib/authStorage'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { FormField } from '@/components/forms/FormField'
import { Loader2, CheckCircle2, AlertCircle, KeyRound, User as UserIcon, Camera, X } from 'lucide-react'
import type { User } from '@/types/api'
import { cn } from '@/lib/utils'

interface ProfileForm { full_name: string; phone: string }
interface PasswordForm { old_password: string; new_password: string; confirm_password: string }

export function Component() {
  const { t } = useTranslation()
  usePageTitle(t('profile.settings'))
  const { user, login } = useAuth()
  const queryClient = useQueryClient()
  const [successMsg, setSuccessMsg] = useState('')
  const [avatarPreview, setAvatarPreview] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const { register, handleSubmit, formState: { errors, isDirty } } = useForm<ProfileForm>({
    defaultValues: { full_name: user?.full_name || '', phone: user?.phone || '' },
  })

  const updateMutation = useMutation({
    mutationFn: async (data: ProfileForm) => {
      const res = await api.put('/auth/profile', data)
      return res.data.data as User
    },
    onSuccess: (updatedUser) => {
      login(authStorage.getToken()!, updatedUser)
      queryClient.invalidateQueries({ queryKey: ['user'] })
      setSuccessMsg(t('profile.updateSuccess'))
      setTimeout(() => setSuccessMsg(''), 3000)
    },
  })

  const avatarMutation = useMutation({
    mutationFn: async (file: File) => {
      const formData = new FormData()
      formData.append('avatar', file)
      const res = await api.post('/auth/avatar', formData, { headers: { 'Content-Type': 'multipart/form-data' } })
      return res.data.data as User
    },
    onSuccess: (updatedUser) => {
      login(authStorage.getToken()!, updatedUser)
      setAvatarPreview(null)
      setSuccessMsg(t('profile.avatarUpdated'))
      setTimeout(() => setSuccessMsg(''), 3000)
    },
  })

  const handleAvatarChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
      alert(t('profile.invalidFileType'))
      return
    }
    if (file.size > 2 * 1024 * 1024) {
      alert(t('profile.fileTooLarge'))
      return
    }
    const reader = new FileReader()
    reader.onload = () => setAvatarPreview(reader.result as string)
    reader.readAsDataURL(file)
    avatarMutation.mutate(file)
  }

  const currentAvatar = avatarPreview || user?.avatar_url || null

  const { register: registerPw, handleSubmit: handleSubmitPw, reset: resetPw, watch: watchPw,
    formState: { errors: pwErrors, isDirty: pwDirty } } = useForm<PasswordForm>()
  const newPassword = watchPw('new_password')

  const passwordMutation = useMutation({
    mutationFn: async (data: { old_password: string; new_password: string }) => {
      const res = await api.post('/auth/change-password', data)
      return res.data
    },
    onSuccess: () => {
      setSuccessMsg(t('profile.passwordChanged'))
      resetPw()
      setTimeout(() => setSuccessMsg(''), 3000)
    },
  })

  return (
    <div className="mx-auto max-w-2xl space-y-6 px-3 py-4 sm:p-6">
      <div>
        <h1 className="text-xl font-bold sm:text-2xl">{t('profile.settings')}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t('profile.settingsSubtitle')}</p>
      </div>

      {/* Personal Information */}
      <Card>
        <CardHeader className="flex-row items-center gap-3 space-y-0 pb-3">
          <CardTitle className="flex items-center gap-2 text-base sm:text-lg">
            <UserIcon className="h-4 w-4 shrink-0" />
            <span className="truncate">{t('profile.personalInfo')}</span>
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          {/* Avatar */}
          <div className="flex flex-col items-center gap-4 sm:flex-row sm:items-start">
            <div className="relative shrink-0">
              <div className="flex h-20 w-20 items-center justify-center overflow-hidden rounded-full border-2 border-border bg-muted sm:h-24 sm:w-24">
                {currentAvatar
                  ? <img src={currentAvatar} alt="Avatar" className="h-full w-full object-cover" />
                  : <UserIcon className="h-8 w-8 text-muted-foreground sm:h-10 sm:w-10" />}
              </div>
              <button type="button" onClick={() => fileInputRef.current?.click()}
                disabled={avatarMutation.isPending}
                className="absolute -bottom-1 -right-1 flex h-7 w-7 items-center justify-center rounded-full border-2 border-background bg-primary text-primary-foreground shadow hover:bg-primary/90 disabled:opacity-50 sm:h-8 sm:w-8"
                aria-label={t('profile.uploadAvatar')}>
                {avatarMutation.isPending ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Camera className="h-3.5 w-3.5" />}
              </button>
              <input ref={fileInputRef} type="file" accept="image/jpeg,image/png,image/webp" className="hidden" onChange={handleAvatarChange} />
              {avatarPreview && (
                <button type="button" onClick={() => { setAvatarPreview(null); if (fileInputRef.current) fileInputRef.current.value = '' }}
                  className="absolute -bottom-1 left-1/2 -translate-x-16 flex h-5 w-5 items-center justify-center rounded-full bg-destructive text-destructive-foreground shadow hover:bg-destructive/90"
                  aria-label={t('profile.cancelUpload')}>
                  <X className="h-3 w-3" />
                </button>
              )}
            </div>
            <div className="flex flex-col items-center gap-1 sm:items-start sm:pt-2">
              <p className="text-sm font-medium">{user?.full_name || user?.email}</p>
              <p className="text-xs text-muted-foreground">{t('profile.avatarFormat')}</p>
              <Button type="button" variant="outline" size="sm" onClick={() => fileInputRef.current?.click()}
                disabled={avatarMutation.isPending} className="mt-1">
                {avatarMutation.isPending
                  ? <><Loader2 className="mr-1 h-3 w-3 animate-spin" />{t('profile.uploading')}</>
                  : t('profile.changePhoto')}
              </Button>
            </div>
          </div>

          {/* Profile form */}
          <form onSubmit={handleSubmit(d => { setSuccessMsg(''); updateMutation.mutate(d) })} className="space-y-4">
            <FormField label={t('profile.email')} htmlFor="email">
              <Input id="email" value={user?.email || ''} disabled className="cursor-not-allowed opacity-60" />
              <p className="mt-1 text-xs text-muted-foreground">{t('profile.emailCannotChange')}</p>
            </FormField>

            <FormField label={t('profile.fullName')} htmlFor="full_name" error={errors.full_name?.message}>
              <Input id="full_name" placeholder={t('profile.fullNamePlaceholder')} autoComplete="name"
                {...register('full_name', { minLength: { value: 2, message: t('profile.fullNameError') } })} />
            </FormField>

            <FormField label={t('profile.phone')} htmlFor="phone" error={errors.phone?.message}>
              <Input id="phone" placeholder={t('profile.phonePlaceholder')} autoComplete="tel"
                {...register('phone', { pattern: { value: /^\+?[0-9\s\-()]{7,20}$/, message: t('profile.phoneError') } })} />
            </FormField>

            {successMsg && (
              <div className="flex items-center gap-2 rounded-md border border-green-200 bg-green-50 px-3 py-2 text-sm text-green-700 dark:border-green-800 dark:bg-green-950 dark:text-green-400">
                <CheckCircle2 className="h-4 w-4 shrink-0" />{successMsg}
              </div>
            )}
            {updateMutation.isError && (
              <div className="flex items-center gap-2 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-400">
                <AlertCircle className="h-4 w-4 shrink-0" />
                {updateMutation.error?.message || t('profile.updateError')}
              </div>
            )}

            <div className="flex flex-col gap-3 pt-2 sm:flex-row sm:items-center">
              <Button type="submit" disabled={updateMutation.isPending || !isDirty}
                className={cn('w-full sm:w-auto', updateMutation.isPending && 'cursor-wait')}>
                {updateMutation.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
                {updateMutation.isPending ? t('profile.saving') : t('profile.saveChanges')}
              </Button>
              {updateMutation.isSuccess && (
                <span className="text-center text-sm text-muted-foreground sm:text-left">{t('profile.allChangesSaved')}</span>
              )}
            </div>
          </form>
        </CardContent>
      </Card>

      {/* Change Password */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-base sm:text-lg">
            <KeyRound className="h-4 w-4 shrink-0" />
            {t('profile.changePassword')}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmitPw(d => { setSuccessMsg(''); passwordMutation.mutate({ old_password: d.old_password, new_password: d.new_password }) })} className="space-y-4">
            <FormField label={t('profile.currentPassword')} htmlFor="old_password" error={pwErrors.old_password?.message}>
              <Input id="old_password" type="password" placeholder={t('profile.currentPasswordPlaceholder')} autoComplete="current-password"
                {...registerPw('old_password', { required: t('profile.currentPasswordError') })} />
            </FormField>

            <FormField label={t('profile.newPassword')} htmlFor="new_password" error={pwErrors.new_password?.message}>
              <Input id="new_password" type="password" placeholder={t('profile.newPasswordPlaceholder')} autoComplete="new-password"
                {...registerPw('new_password', {
                  required: t('profile.newPasswordError'),
                  minLength: { value: 6, message: t('profile.newPasswordMinError') },
                })} />
            </FormField>

            <FormField label={t('profile.confirmPassword')} htmlFor="confirm_password" error={pwErrors.confirm_password?.message}>
              <Input id="confirm_password" type="password" placeholder={t('profile.confirmPasswordPlaceholder')} autoComplete="new-password"
                {...registerPw('confirm_password', {
                  required: t('profile.confirmPasswordError'),
                  validate: v => v === newPassword || t('profile.passwordMismatch'),
                })} />
            </FormField>

            {passwordMutation.isError && (
              <div className="flex items-center gap-2 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-400">
                <AlertCircle className="h-4 w-4 shrink-0" />
                {passwordMutation.error?.message || t('profile.passwordError')}
              </div>
            )}

            <div className="flex flex-col gap-3 pt-2 sm:flex-row sm:items-center">
              <Button type="submit" disabled={passwordMutation.isPending || !pwDirty}
                className={cn('w-full sm:w-auto', passwordMutation.isPending && 'cursor-wait')}>
                {passwordMutation.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
                {passwordMutation.isPending ? t('profile.changing') : t('profile.changePassword')}
              </Button>
              {passwordMutation.isSuccess && (
                <span className="text-center text-sm text-muted-foreground sm:text-left">{t('profile.passwordUpdated')}</span>
              )}
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
