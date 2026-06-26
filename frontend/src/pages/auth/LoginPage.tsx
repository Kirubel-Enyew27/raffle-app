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

const schema = z.object({
  identifier: z.string().min(1, 'Enter your email or phone'),
  password: z.string().min(1, 'Password is required'),
})
type Fields = z.infer<typeof schema>

export function Component() {
  usePageTitle('Sign in')
  const { login } = useAuth()
  const navigate = useNavigate()

  const { register, handleSubmit, formState: { errors } } = useForm<Fields>({
    resolver: zodResolver(schema),
  })

  const { mutate, isPending, error } = useMutation({
    mutationFn: ({ identifier, password }: Fields) => authApi.login(identifier, password),
    onSuccess: ({ token, user }) => {
      login(token, user)
      navigate(user.role === 'admin' ? '/admin/dashboard' : '/dashboard')
    },
  })

  return (
    <AuthCard title="Sign in" subtitle="Welcome back">
      <form onSubmit={handleSubmit(d => mutate(d))} className="space-y-4">
        {error && (
          <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error.message}
          </p>
        )}

        <FormField label="Email or phone" htmlFor="identifier" error={errors.identifier?.message}>
          <Input id="identifier" type="text" autoComplete="username" placeholder="email@example.com or +251912345678" {...register('identifier')} />
        </FormField>

        <FormField label="Password" htmlFor="password" error={errors.password?.message}>
          <Input id="password" type="password" autoComplete="current-password" {...register('password')} />
        </FormField>

        <div className="flex justify-end">
          <Link to="/forgot-password" className="text-xs text-muted-foreground hover:text-foreground">
            Forgot password?
          </Link>
        </div>

        <Button type="submit" className="w-full" disabled={isPending}>
          {isPending ? 'Signing in…' : 'Sign in'}
        </Button>

        <p className="text-center text-sm text-muted-foreground">
          No account?{' '}
          <Link to="/register" className="text-foreground underline underline-offset-4">
            Register
          </Link>
        </p>
      </form>
    </AuthCard>
  )
}
