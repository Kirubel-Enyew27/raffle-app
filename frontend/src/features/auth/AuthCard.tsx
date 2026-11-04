import type { ReactNode } from 'react'
import { AuthShell } from '@/features/auth/AuthShell'

interface AuthCardProps {
  /** Optional: some screens start straight with their form. */
  title?: string
  subtitle?: string
  children: ReactNode
}

/**
 * Content block for the secondary auth screens (register, forgot password,
 * reset password). It uses the same KWIK frame as the login page so the whole
 * sign-in journey looks like one product in either theme.
 */
export function AuthCard({ title, subtitle, children }: AuthCardProps) {
  return (
    <AuthShell>
      {title && (
        <h1 className="text-balance text-[30px] font-extrabold leading-[1.18] tracking-[-0.02em] text-foreground">
          {title}
        </h1>
      )}
      {subtitle && (
        <p className={`${title ? 'mt-4' : ''} text-[15px] leading-relaxed text-muted-foreground`}>{subtitle}</p>
      )}
      <div className={title || subtitle ? 'mt-8' : undefined}>{children}</div>
    </AuthShell>
  )
}
