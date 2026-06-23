import type { ReactNode } from 'react'
import { Label } from '@/components/ui/label'
import { cn } from '@/lib/utils'

interface FormFieldProps {
  label: string
  htmlFor: string
  error?: string
  children: ReactNode
  className?: string
}

/**
 * Wraps a form control with a label and inline error message.
 * Works with react-hook-form: pass `error={errors.fieldName?.message}`.
 */
export function FormField({ label, htmlFor, error, children, className }: FormFieldProps) {
  return (
    <div className={cn('space-y-1', className)}>
      <Label htmlFor={htmlFor}>{label}</Label>
      {children}
      {error && <p className="text-xs text-destructive">{error}</p>}
    </div>
  )
}
