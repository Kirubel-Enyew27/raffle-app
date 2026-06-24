import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Link } from 'react-router-dom'
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
  usePageTitle('Forgot password')
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
      <AuthCard title="Check your email" subtitle="If an account exists, you'll receive instructions shortly.">
        <div className="space-y-4 text-center">
          <p className="text-sm text-muted-foreground">
            Didn't receive an email?{' '}
            <button
              onClick={() => setSubmitted(false)}
              className="text-foreground underline underline-offset-4"
            >
              Try again
            </button>
          </p>
          <p className="text-sm text-muted-foreground">
            Know your password?{' '}
            <Link to="/login" className="text-foreground underline underline-offset-4">
              Sign in
            </Link>
          </p>
          <div className="mt-4 rounded-md bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:bg-amber-950 dark:text-amber-300">
            ⚠ Password reset email delivery is not yet configured.
            Contact support if you're locked out.
          </div>
        </div>
      </AuthCard>
    )
  }

  return (
    <AuthCard title="Forgot password" subtitle="Enter your email and we'll send reset instructions">
      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        <FormField label="Email" htmlFor="email" error={errors.email?.message}>
          <Input id="email" type="email" autoComplete="email" {...register('email')} />
        </FormField>

        <Button type="submit" className="w-full">
          Send instructions
        </Button>

        <p className="text-center text-sm text-muted-foreground">
          <Link to="/login" className="text-foreground underline underline-offset-4">
            Back to sign in
          </Link>
        </p>
      </form>
    </AuthCard>
  )
}
