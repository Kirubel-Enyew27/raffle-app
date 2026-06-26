import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/**
 * Format a number as Ethiopian Birr (ETB).
 * Example: formatCurrency(1500.5) → "1,500.50 Br"
 */
export function formatCurrency(amount: number, currency?: string): string {
  if (currency && currency !== 'ETB') {
    return amount.toLocaleString('en-US', { style: 'currency', currency, maximumFractionDigits: 2 })
  }
  return `${amount.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} Br`
}

