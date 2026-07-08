import { useTranslation } from 'react-i18next'

// Extracts a display-ready message from a React Query error.
export function useApiError(error: unknown): string | null {
  const { t } = useTranslation()
  if (!error) return null
  if (error instanceof Error) return error.message
  return t('errors.unexpectedError')
}
