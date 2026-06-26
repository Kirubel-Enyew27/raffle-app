import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Link, useNavigate } from 'react-router-dom'
import { useMutation } from '@tanstack/react-query'
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
  usePageTitle('Create account')
  const { login } = useAuth()
  const navigate = useNavigate()

  const { register, handleSubmit, formState: { errors } } = useForm<Fields>({
    resolver: zodResolver(schema),
  })

  const { mutate, isPending, error } = useMutation({
    mutationFn: ({ fullName, email, phone, password }: Fields) => authApi.register(email, password, fullName, phone),
    onSuccess: async (_user, { email, phone, password }) => {
      // Auto-login after registration (use email or phone)
      const identifier = (email || phone)!
      const { token, user } = await authApi.login(identifier, password)
      login(token, user)
      navigate('/dashboard')
    },
  })

  return (
    <AuthCard title="Create account" subtitle="Start entering raffles today">
      <form onSubmit={handleSubmit(d => mutate(d))} className="space-y-4">
        {error && (
          <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error.message}
          </p>
        )}

        <FormField label="Full name" htmlFor="fullName" error={errors.fullName?.message}>
          <Input id="fullName" type="text" autoComplete="name" {...register('fullName')} />
          <p className="mt-2 flex items-center gap-1.5 rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-xs text-blue-700 dark:border-blue-800 dark:bg-blue-950 dark:text-blue-400">
            <Smartphone className="h-3.5 w-3.5 shrink-0" />
            Make sure the full name matches the name registered on Telebirr for automatic processing.
          </p>
        </FormField>

        <FormField label="Phone" htmlFor="phone" error={errors.phone?.message}>
          <Input id="phone" type="tel" autoComplete="tel" placeholder="e.g. +251912345678" {...register('phone')} />
        </FormField>

        <FormField label="Email" htmlFor="email" error={errors.email?.message}>
          <Input id="email" type="email" autoComplete="email" placeholder="optional" {...register('email')} />
        </FormField>

        <FormField label="Password" htmlFor="password" error={errors.password?.message}>
          <Input id="password" type="password" autoComplete="new-password" {...register('password')} />
        </FormField>

        <FormField label="Confirm password" htmlFor="confirm" error={errors.confirm?.message}>
          <Input id="confirm" type="password" autoComplete="new-password" {...register('confirm')} />
        </FormField>

        <Button type="submit" className="w-full" disabled={isPending}>
          {isPending ? 'Creating account…' : 'Create account'}
        </Button>

        <p className="text-center text-sm text-muted-foreground">
          Already have an account?{' '}
          <Link to="/login" className="text-foreground underline underline-offset-4">
            Sign in
          </Link>
        </p>
      </form>
    </AuthCard>
  )
}
